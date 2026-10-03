// Package euicc talks to an eUICC's ISD-R over SGP.22 ES10a/b/c, a Go
// translation of the parts of lpac's libeuicc that eSIM Manager uses.
//
// Copyright (C) 2023-2025 ESTKME TECHNOLOGY LIMITED, Hong Kong
// Copyright (C) 2026 AlphaToad
// SPDX-License-Identifier: LGPL-2.1-only
package euicc

import (
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"esimmanager/internal/tlv"
)

// ISDR is the AID of the ISD-R, the eUICC's profile manager.
var ISDR = []byte{0xA0, 0x00, 0x00, 0x05, 0x59, 0x10, 0x10, 0xFF, 0xFF, 0xFF, 0xFF, 0x89, 0x00, 0x00, 0x01, 0x00}

// Transport sends one command APDU on a logical channel to the ISD-R and
// returns the response with SW1 SW2. It sets the channel bits in CLA.
type Transport interface {
	Transmit(apdu []byte) ([]byte, error)
}

// EUICC issues ES10 commands. It is not safe for concurrent use.
type EUICC struct {
	t Transport
}

func New(t Transport) *EUICC { return &EUICC{t: t} }

// SWError is a status word other than 90xx/61xx.
type SWError struct{ SW1, SW2 byte }

func (e SWError) Error() string { return fmt.Sprintf("eUICC returned status %02X%02X", e.SW1, e.SW2) }

// storeDataMax is the data size of one STORE DATA block, as in libeuicc.
const storeDataMax = 120

// command sends an ES10 request with STORE DATA (80 E2), split into blocks,
// and collects the response, fetching 61xx continuations with GET RESPONSE.
func (e *EUICC) command(req []byte) ([]byte, error) {
	var out []byte
	for block := 0; len(req) > 0; block++ {
		n, p1 := len(req), byte(0x91) // last block
		if n > storeDataMax {
			n, p1 = storeDataMax, 0x11
		}
		apdu := append([]byte{0x80, 0xE2, p1, byte(block), byte(n)}, req[:n]...)
		req = req[n:]

		r, err := e.t.Transmit(apdu)
		for {
			if err != nil {
				return nil, err
			}
			if len(r) < 2 {
				return nil, errors.New("euicc: short response")
			}
			sw1, sw2 := r[len(r)-2], r[len(r)-1]
			out = append(out, r[:len(r)-2]...)
			if sw1 == 0x61 {
				r, err = e.t.Transmit([]byte{0x80, 0xC0, 0x00, 0x00, sw2})
				continue
			}
			if sw1&0xF0 != 0x90 {
				return nil, SWError{sw1, sw2}
			}
			break
		}
	}
	return out, nil
}

// call sends a request and returns the value of the response element with the
// same tag.
func (e *EUICC) call(req []byte, tag uint16) ([]byte, error) {
	resp, err := e.command(req)
	if err != nil {
		return nil, err
	}
	t, ok := tlv.Find(resp, tag)
	if !ok {
		return nil, fmt.Errorf("euicc: no %X in the response", tag)
	}
	return t.Value, nil
}

// resultCode sends a request whose response is {tag {80 result}}.
func (e *EUICC) resultCode(req []byte, tag uint16) (int, error) {
	v, err := e.call(req, tag)
	if err != nil {
		return 0, err
	}
	r, ok := tlv.Find(v, 0x80)
	if !ok {
		return 0, errors.New("euicc: no result code")
	}
	return int(tlv.Int(r.Value)), nil
}

// decodeBCD turns a nibble-swapped BCD ICCID into digits, dropping F padding.
func decodeBCD(b []byte) string {
	var s strings.Builder
	for _, c := range b {
		for _, d := range []byte{c & 0x0F, c >> 4} {
			if d == 0xF {
				return s.String()
			}
			s.WriteByte("0123456789ABCDE"[d])
		}
	}
	return s.String()
}

// encodeBCD is the inverse of decodeBCD, padded with F to size bytes.
func encodeBCD(s string, size int) ([]byte, error) {
	if len(s) > size*2 {
		return nil, fmt.Errorf("euicc: %q is too long", s)
	}
	for len(s) < size*2 {
		s += "F"
	}
	out := make([]byte, size)
	for i := 0; i < size; i++ {
		lo, err := nibble(s[2*i])
		if err != nil {
			return nil, err
		}
		hi, err := nibble(s[2*i+1])
		if err != nil {
			return nil, err
		}
		out[i] = hi<<4 | lo
	}
	return out, nil
}

func nibble(c byte) (byte, error) {
	switch {
	case c >= '0' && c <= '9':
		return c - '0', nil
	case c == 'F' || c == 'f':
		return 0xF, nil
	}
	return 0, fmt.Errorf("euicc: %q is not a digit", c)
}

// profileID encodes an ICCID (digits) or an ISD-P AID (32 hex digits).
func profileID(id string) ([]byte, error) {
	if len(id) == 32 {
		b, err := hex.DecodeString(id)
		if err != nil {
			return nil, err
		}
		return tlv.Encode(0x4F, b), nil
	}
	b, err := encodeBCD(id, 10)
	if err != nil {
		return nil, err
	}
	return tlv.Encode(0x5A, b), nil
}
