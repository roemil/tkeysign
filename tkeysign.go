// Copyright (C) 2022-2024 - Tillitis AB
// SPDX-License-Identifier: GPL-2.0-only

// Package tkeysign provides a connection to the RSA signer app
// running on the TKey. You're expected to pass an existing connection
// to it, so use it like this:
//
//	tk := tkeyclient.New()
//	err := tk.Connect(port)
//	signer := tkeysign.New(tk)
//
// Then use it like this to get the public key of the TKey:
//
//	pubkey, err := signer.GetPubkey()
//
// And like this to sign a message:
//
//	signature, err := signer.Sign(message)
package tkeysign

import (
	"encoding/pem"
	"fmt"
	"os"

	"github.com/tillitis/tkeyclient"
)

var (
	cmdGetPubkey       = appCmd{0x01, "cmdGetPubkey", tkeyclient.CmdLen1}
	rspGetPubkey       = appCmd{0x02, "rspGetPubkey", tkeyclient.CmdLen128}
	cmdSetSize         = appCmd{0x03, "cmdSetSize", tkeyclient.CmdLen32}
	rspSetSize         = appCmd{0x04, "rspSetSize", tkeyclient.CmdLen4}
	cmdSignData        = appCmd{0x05, "cmdSignData", tkeyclient.CmdLen128}
	rspSignData        = appCmd{0x06, "rspSignData", tkeyclient.CmdLen4}
	cmdGetSig          = appCmd{0x07, "cmdGetSig", tkeyclient.CmdLen1}
	rspGetSig          = appCmd{0x08, "rspGetSig", tkeyclient.CmdLen128}
	cmdGetNameVersion  = appCmd{0x09, "cmdGetNameVersion", tkeyclient.CmdLen1}
	rspGetNameVersion  = appCmd{0x0a, "rspGetNameVersion", tkeyclient.CmdLen32}
	cmdGetFirmwareHash = appCmd{0x0b, "cmdGetFirmwareHash", tkeyclient.CmdLen32}
	rspGetFirmwareHash = appCmd{0x0c, "rspGetFirmwareHash", tkeyclient.CmdLen128}
	cmdLoadKey         = appCmd{0x0d, "cmdLoadKey", tkeyclient.CmdLen128}
	cmdEncryptKey      = appCmd{0x0e, "cmdEncryptKey", tkeyclient.CmdLen1}
	rspEncryptKey      = appCmd{0x0f, "rspEncryptKey", tkeyclient.CmdLen128}
	cmdLoadEncKey      = appCmd{0x10, "cmdLoadEncKey", tkeyclient.CmdLen128}
	cmdIsKeyLoaded     = appCmd{0x11, "cmdIsKeyLoaded", tkeyclient.CmdLen1}
	rspIsKeyLoaded     = appCmd{0x12, "rspIsKeyLoaded", tkeyclient.CmdLen4}

	cmdDecryptKey = appCmd{0x13, "cmdDecryptKey", tkeyclient.CmdLen1}
	rspDecryptKey = appCmd{0x14, "rspDecryptKey", tkeyclient.CmdLen4}
	cmdParseKey   = appCmd{0x15, "cmdParseKey", tkeyclient.CmdLen1}
	rspParseKey   = appCmd{0x16, "rspParseKey", tkeyclient.CmdLen4}
)

const MaxSignSize = 4096

type appCmd struct {
	code   byte
	name   string
	cmdLen tkeyclient.CmdLen
}

func (c appCmd) Code() byte {
	return c.code
}

func (c appCmd) CmdLen() tkeyclient.CmdLen {
	return c.cmdLen
}

func (c appCmd) Endpoint() tkeyclient.Endpoint {
	return tkeyclient.DestApp
}

func (c appCmd) String() string {
	return c.name
}

type Signer struct {
	tk *tkeyclient.TillitisKey // A connection to a TKey
}

// New allocates a struct for communicating with the RSA signer
// app running on the TKey. You're expected to pass an existing
// connection to it, so use it like this:
//
//	tk := tkeyclient.New()
//	err := tk.Connect(port)
//	signer := tk1sign.New(tk)
func New(tk *tkeyclient.TillitisKey) Signer {
	var signer Signer

	signer.tk = tk

	return signer
}

// Close closes the connection to the TKey
func (s Signer) Close() error {
	if err := s.tk.Close(); err != nil {
		return fmt.Errorf("tk.Close: %w", err)
	}
	return nil
}

// GetAppNameVersion gets the name and version of the running app in
// the same style as the stick itself.
func (s Signer) GetAppNameVersion() (*tkeyclient.NameVersion, error) {
	id := 2
	tx, err := tkeyclient.NewFrameBuf(cmdGetNameVersion, id)
	if err != nil {
		return nil, fmt.Errorf("NewFrameBuf: %w", err)
	}

	tkeyclient.Dump("GetAppNameVersion tx", tx)
	if err = s.tk.Write(tx); err != nil {
		return nil, fmt.Errorf("Write: %w", err)
	}

	err = s.tk.SetReadTimeout(2)
	if err != nil {
		return nil, fmt.Errorf("SetReadTimeout: %w", err)
	}

	rx, _, err := s.tk.ReadFrame(rspGetNameVersion, id)
	if err != nil {
		return nil, fmt.Errorf("ReadFrame: %w", err)
	}

	err = s.tk.SetReadTimeout(0)
	if err != nil {
		return nil, fmt.Errorf("SetReadTimeout: %w", err)
	}

	nameVer := &tkeyclient.NameVersion{}
	nameVer.Unpack(rx[2:])

	return nameVer, nil
}

// GetPubkey fetches the public key of the signer.
func (s Signer) GetPubkey() ([]byte, error) {
	id := 2
	tx, err := tkeyclient.NewFrameBuf(cmdGetPubkey, id)
	if err != nil {
		return nil, fmt.Errorf("NewFrameBuf: %w", err)
	}

	tkeyclient.Dump("GetPubkey tx", tx)
	if err = s.tk.Write(tx); err != nil {
		return nil, fmt.Errorf("Write: %w", err)
	}

	rx, _, err := s.tk.ReadFrame(rspGetPubkey, id)
	tkeyclient.Dump("GetPubKey rx", rx)
	if err != nil {
		return nil, fmt.Errorf("ReadFrame: %w", err)
	}

	pubkeyRaw := make([]byte, 256)
	// Skip frame header & app header and get the first 127 bytes
	copy(pubkeyRaw[0:127], rx[2:])

	rx, _, err = s.tk.ReadFrame(rspGetPubkey, id)
	tkeyclient.Dump("GetPubKey rx", rx)
	if err != nil {
		return nil, fmt.Errorf("ReadFrame: %w", err)
	}

	// Skip frame header & app header and get the next 127 bytes
	copy(pubkeyRaw[127:254], rx[2:])

	rx, _, err = s.tk.ReadFrame(rspGetPubkey, id)
	tkeyclient.Dump("GetPubKey rx", rx)
	if err != nil {
		return nil, fmt.Errorf("ReadFrame: %w", err)
	}
	// Skip frame header & app header and get the last 2 bytes
	copy(pubkeyRaw[254:256], rx[2:2+2])

	return pubkeyRaw, nil
}

// Sign signs the message in data and returns an RSA signature.
func (s Signer) Sign(data []byte) ([]byte, error) {
	err := s.setSize(len(data))
	if err != nil {
		return nil, fmt.Errorf("setSize: %w", err)
	}

	var offset int
	for nsent := 0; offset < len(data); offset += nsent {
		nsent, err = s.signLoad(data[offset:])
		if err != nil {
			return nil, fmt.Errorf("signLoad: %w", err)
		}
	}
	if offset > len(data) {
		return nil, fmt.Errorf("transmitted more than expected")
	}

	signature, err := s.getSig()
	if err != nil {
		return nil, fmt.Errorf("getSig: %w", err)
	}

	return signature, nil
}

func (s Signer) GetIsKeyLoaded() (bool, error) {
	id := 2
	tx, err := tkeyclient.NewFrameBuf(cmdIsKeyLoaded, id)
	if err != nil {
		return false, fmt.Errorf("NewFrameBuf: %w", err)
	}

	tkeyclient.Dump("cmdIsKeyLoaded tx", tx)
	if err = s.tk.Write(tx); err != nil {
		return false, fmt.Errorf("Write: %w", err)
	}
	rx, _, err := s.tk.ReadFrame(rspIsKeyLoaded, id)
	tkeyclient.Dump("SetAppSize rx", rx)
	if err != nil {
		return false, fmt.Errorf("ReadFrame: %w", err)
	}

	return rx[2] == 1, nil
}

func (s Signer) LoadKey(rsaKeyPath string) error {
	f, err := os.Open(rsaKeyPath)
	if err != nil {
		return fmt.Errorf("failed to open id_rsa. %s", err.Error())
	}
	key := make([]byte, 1676)
	n1, err := f.Read(key)
	if n1 < 1675 {
		return fmt.Errorf("did not read enough. Read: %d", n1)
	}
	if err != nil {
		return fmt.Errorf("failed to read: %w", err)
	}

	f.Close()

	block, _ := pem.Decode(key)
	if block != nil {
		encrypted_key, err := s.encryptKey(key)
		if err != nil {
			return fmt.Errorf("failed to encrypt: %w", err)
		}
		f, err := os.OpenFile(rsaKeyPath, os.O_RDWR, 0755)
		if err != nil {
			return fmt.Errorf("failed to open id_rsa for write. %s", err.Error())
		}
		n, err := f.Write([]byte(encrypted_key))
		if n != len(key) {
			return fmt.Errorf("did not write enough data: %d", n)
		}
		if err != nil {
			return fmt.Errorf("failed to write: %w", err)
		}
		err = s.parseKey()
		if err != nil {
			return fmt.Errorf("parseKey: %w", err)
		}
	} else {
		err = s.loadEncKey(key)
		if err != nil {
			return fmt.Errorf("failed load keye: %w", err)
		}
		err := s.decryptKey()
		if err != nil {
			return fmt.Errorf("decryptKey: %w", err)
		}
		err = s.parseKey()
		if err != nil {
			return fmt.Errorf("parseKey: %w", err)
		}
	}

	return nil
}

func (s Signer) transferKey(data []byte) error {
	err := s.setSize(len(data))
	if err != nil {
		return fmt.Errorf("setSize: %w", err)
	}
	var offset int
	for nsent := 0; offset < len(data); offset += nsent {
		nsent, err = s.transferPiece(data[offset:])
		if err != nil {
			return fmt.Errorf("keyLoad: %w", err)
		}
	}
	if offset > len(data) {
		return fmt.Errorf("transmitted more than expected")
	}

	return nil
}

func (s Signer) loadEncKey(data []byte) error {
	err := s.setSize(len(data))
	if err != nil {
		return fmt.Errorf("setSize: %w", err)
	}
	var offset int
	for nsent := 0; offset < len(data); offset += nsent {
		nsent, err = s.keyEncLoad(data[offset:])
		if err != nil {
			return fmt.Errorf("keyLoad: %w", err)
		}
	}
	if offset > len(data) {
		return fmt.Errorf("transmitted more than expected")
	}

	return nil
}

func (s Signer) encryptKey(data []byte) ([]byte, error) {
	err := s.transferKey(data)
	if err != nil {
		return nil, fmt.Errorf("transferKey: %w", err)
	}

	encData, err := s.keyEncrypt()
	if err != nil {
		return nil, fmt.Errorf("zv: %w", err)
	}
	return encData, nil
}

func (s Signer) parseKey() error {
	id := 2
	tx, err := tkeyclient.NewFrameBuf(cmdParseKey, id)
	if err != nil {
		return fmt.Errorf("NewFrameBuf: %w", err)
	}
	tkeyclient.Dump("cmdParseKey tx", tx)
	if err = s.tk.Write(tx); err != nil {
		return fmt.Errorf("Write: %w", err)
	}
	rx, _, err := s.tk.ReadFrame(rspParseKey, id)
	tkeyclient.Dump("rspParseKey rx", rx)
	if err != nil {
		return fmt.Errorf("ReadFrame: %w", err)
	}
	if rx[2] != tkeyclient.StatusOK {
		return fmt.Errorf("parseKey NOK")
	}
	return nil
}

func (s Signer) decryptKey() error {
	id := 2
	tx, err := tkeyclient.NewFrameBuf(cmdDecryptKey, id)
	if err != nil {
		return fmt.Errorf("NewFrameBuf: %w", err)
	}
	tkeyclient.Dump("cmdDecryptKey tx", tx)
	if err = s.tk.Write(tx); err != nil {
		return fmt.Errorf("Write: %w", err)
	}
	rx, _, err := s.tk.ReadFrame(rspDecryptKey, id)
	tkeyclient.Dump("rspDecryptKey rx", rx)
	if err != nil {
		return fmt.Errorf("ReadFrame: %w", err)
	}
	if rx[2] != tkeyclient.StatusOK {
		return fmt.Errorf("decryptKey NOK")
	}
	return nil
}

// SetSize sets the size of the data to sign.
func (s Signer) setSize(size int) error {
	id := 2
	tx, err := tkeyclient.NewFrameBuf(cmdSetSize, id)
	if err != nil {
		return fmt.Errorf("NewFrameBuf: %w", err)
	}

	// Set size
	tx[2] = byte(size)
	tx[3] = byte(size >> 8)
	tx[4] = byte(size >> 16)
	tx[5] = byte(size >> 24)
	tkeyclient.Dump("SetAppSize tx", tx)
	if err = s.tk.Write(tx); err != nil {
		return fmt.Errorf("Write: %w", err)
	}

	rx, _, err := s.tk.ReadFrame(rspSetSize, id)
	tkeyclient.Dump("SetAppSize rx", rx)
	if err != nil {
		return fmt.Errorf("ReadFrame: %w", err)
	}

	if rx[2] != tkeyclient.StatusOK {
		return fmt.Errorf("SetSignSize NOK")
	}

	return nil
}

// signload loads a chunk of a message to sign and waits for a
// response from the signer.
func (s Signer) signLoad(content []byte) (int, error) {
	id := 2
	tx, err := tkeyclient.NewFrameBuf(cmdSignData, id)
	if err != nil {
		return 0, fmt.Errorf("NewFrameBuf: %w", err)
	}

	payload := make([]byte, cmdSignData.CmdLen().Bytelen()-1)
	copied := copy(payload, content)

	// Add padding if not filling the payload buffer.
	if copied < len(payload) {
		padding := make([]byte, len(payload)-copied)
		copy(payload[copied:], padding)
	}

	copy(tx[2:], payload)

	tkeyclient.Dump("LoadSignData tx", tx)
	if err = s.tk.Write(tx); err != nil {
		return 0, fmt.Errorf("Write: %w", err)
	}

	// Wait for reply
	rx, _, err := s.tk.ReadFrame(rspSignData, id)
	if err != nil {
		return 0, fmt.Errorf("ReadFrame: %w", err)
	}

	if rx[2] != tkeyclient.StatusOK {
		return 0, fmt.Errorf("SignData NOK")
	}

	return copied, nil
}

func (s Signer) transferPiece(content []byte) (int, error) {
	id := 2
	tx, err := tkeyclient.NewFrameBuf(cmdLoadKey, id)
	if err != nil {
		return 0, fmt.Errorf("NewFrameBuf: %w", err)
	}

	payload := make([]byte, cmdLoadKey.CmdLen().Bytelen()-1)
	copied := copy(payload, content)

	// Add padding if not filling the payload buffer.
	if copied < len(payload) {
		padding := make([]byte, len(payload)-copied)
		copy(payload[copied:], padding)
	}

	copy(tx[2:], payload)

	tkeyclient.Dump("cmdLoadKey tx", tx)
	if err = s.tk.Write(tx); err != nil {
		return 0, fmt.Errorf("Write: %w", err)
	}

	// Wait for reply
	rx, _, err := s.tk.ReadFrame(rspSignData, id)
	if err != nil {
		return 0, fmt.Errorf("ReadFrame: %w", err)
	}

	if rx[2] != tkeyclient.StatusOK {
		return 0, fmt.Errorf("SignData NOK")
	}

	return copied, nil
}

func (s Signer) keyEncLoad(content []byte) (int, error) {
	id := 2
	tx, err := tkeyclient.NewFrameBuf(cmdLoadEncKey, id)
	if err != nil {
		return 0, fmt.Errorf("NewFrameBuf: %w", err)
	}

	payload := make([]byte, cmdLoadEncKey.CmdLen().Bytelen()-1)
	copied := copy(payload, content)

	// Add padding if not filling the payload buffer.
	if copied < len(payload) {
		padding := make([]byte, len(payload)-copied)
		copy(payload[copied:], padding)
	}

	copy(tx[2:], payload)

	tkeyclient.Dump("cmdLoadEncKey tx", tx)
	if err = s.tk.Write(tx); err != nil {
		return 0, fmt.Errorf("Write: %w", err)
	}

	// Wait for reply
	rx, _, err := s.tk.ReadFrame(rspSignData, id)
	if err != nil {
		return 0, fmt.Errorf("ReadFrame: %w", err)
	}

	if rx[2] != tkeyclient.StatusOK {
		return 0, fmt.Errorf("SignData NOK")
	}

	return copied, nil
}

func (s Signer) keyEncrypt() ([]byte, error) {
	id := 2

	tx, err := tkeyclient.NewFrameBuf(cmdEncryptKey, id)
	if err != nil {
		return nil, fmt.Errorf("NewFrameBuf: %w", err)
	}

	tkeyclient.Dump("keyEnc tx", tx)
	if err = s.tk.Write(tx); err != nil {
		return nil, fmt.Errorf("Write: %w", err)
	}

	// Wait for reply
	rsp := make([]byte, 1676)
	for i := 0; i < 13; i++ {
		rx, _, err := s.tk.ReadFrame(rspEncryptKey, id)
		if err != nil {
			return nil, fmt.Errorf("ReadFrame i=%d: %w", i, err)
		}
		copied := copy(rsp[i*127:i*127+127], rx[2:])
		if copied != 127 {
			return nil, fmt.Errorf("ReadFrame copied i=%d: ", i)
		}
	}
	// Read the tail
	rx, _, err := s.tk.ReadFrame(rspEncryptKey, id)
	if err != nil {
		return nil, fmt.Errorf("ReadFrame tail: %w", err)
	}
	copied := copy(rsp[1651:], rx[2:2+25])
	if copied != 25 {
		return nil, fmt.Errorf("ReadFrame copied=%d: ", copied)
	}

	return rsp, nil
}

// getSig gets the RSA signature from the signer app, if
// available.
func (s Signer) getSig() ([]byte, error) {
	id := 2
	tx, err := tkeyclient.NewFrameBuf(cmdGetSig, id)
	if err != nil {
		return nil, fmt.Errorf("NewFrameBuf: %w", err)
	}

	tkeyclient.Dump("getSig tx", tx)
	if err = s.tk.Write(tx); err != nil {
		return nil, fmt.Errorf("Write: %w", err)
	}

	rx, _, err := s.tk.ReadFrame(rspGetSig, id)
	if err != nil {
		return nil, fmt.Errorf("ReadFrame: %w", err)
	}

	// if rx[2] != tkeyclient.StatusOK {
	// 	return nil, fmt.Errorf("getSig NOK")
	// }

	signatureRaw := make([]byte, 256)
	// Skip frame header & app header and get the first 127 bytes
	copy(signatureRaw[0:127], rx[2:])

	rx, _, err = s.tk.ReadFrame(rspGetSig, id)
	tkeyclient.Dump("getSig rx", rx)
	if err != nil {
		return nil, fmt.Errorf("ReadFrame: %w", err)
	}
	// Skip frame header & app header and get the next 127 bytes
	copy(signatureRaw[127:254], rx[2:])

	rx, _, err = s.tk.ReadFrame(rspGetSig, id)
	tkeyclient.Dump("getSig rx", rx)
	if err != nil {
		return nil, fmt.Errorf("ReadFrame: %w", err)
	}
	// Skip frame header & app header and get the last 2 bytes
	copy(signatureRaw[254:256], rx[2:2+2])

	return signatureRaw, nil
}

// GetFWDigest asks the signer app to hash len bytes of the firmware.
//
// It returns the resulting SHA512 digest or an error.
func (s Signer) GetFWDigest(len int) ([]byte, error) {
	id := 2
	tx, err := tkeyclient.NewFrameBuf(cmdGetFirmwareHash, id)
	if err != nil {
		return nil, fmt.Errorf("NewFrameBuf: %w", err)
	}

	// Set firmware length as 32 bit LE
	tx[2] = byte(len)
	tx[3] = byte(len >> 8)
	tx[4] = byte(len >> 16)
	tx[5] = byte(len >> 24)

	tkeyclient.Dump("GetFirmwareHash tx", tx)
	if err = s.tk.Write(tx); err != nil {
		return nil, fmt.Errorf("Write: %w", err)
	}

	// Wait for reply
	rx, _, err := s.tk.ReadFrame(rspGetFirmwareHash, id)
	if err != nil {
		return nil, fmt.Errorf("ReadFrame: %w", err)
	}

	tkeyclient.Dump("GetFirmwareHash rx", rx)

	if rx[2] != tkeyclient.StatusOK {
		return nil, fmt.Errorf("GetFirmwareHash NOK")
	}

	// Skip frame header, app header, and status; returning
	// firmware SHA512 digest
	return rx[3 : 3+64], nil
}
