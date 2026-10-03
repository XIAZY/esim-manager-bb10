// Package tlv encodes and decodes the BER-TLV used by SGP.22 (ES10 and ES8+):
// one- or two-byte tags and definite lengths. Based on lpac's libeuicc
// derutil.
//
// Copyright (C) 2023-2025 ESTKME TECHNOLOGY LIMITED, Hong Kong
// Copyright (C) 2026 AlphaToad
// SPDX-License-Identifier: LGPL-2.1-only
package tlv

import (
	"errors"
	"fmt"
)

// TLV is one decoded element. Value and Raw alias the input.
type TLV struct {
	Tag   uint16
	Value []byte
	Raw   []byte // tag, length and value
}

var errTruncated = errors.New("tlv: truncated")

// Parse decodes the first element of b and returns it with the bytes after it.
func Parse(b []byte) (TLV, []byte, error) {
	if len(b) < 2 {
		return TLV{}, nil, errTruncated
	}
	i := 0
	tag := uint16(b[i])
	i++
	if tag&0x1F == 0x1F {
		tag = tag<<8 | uint16(b[i])
		i++
	}
	if i >= len(b) {
		return TLV{}, nil, errTruncated
	}
	n := int(b[i])
	i++
	if n&0x80 != 0 {
		size := n & 0x7F
		if size == 0 || size > 3 || i+size > len(b) {
			return TLV{}, nil, fmt.Errorf("tlv: bad length for tag %X", tag)
		}
		n = 0
		for _, c := range b[i : i+size] {
			n = n<<8 | int(c)
		}
		i += size
	}
	if n > len(b)-i {
		return TLV{}, nil, errTruncated
	}
	end := i + n
	return TLV{Tag: tag, Value: b[i:end], Raw: b[:end]}, b[end:], nil
}

// Children decodes every element in b.
func Children(b []byte) ([]TLV, error) {
	var out []TLV
	for len(b) > 0 {
		t, rest, err := Parse(b)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
		b = rest
	}
	return out, nil
}

// Find returns the first element of b with the given tag.
func Find(b []byte, tag uint16) (TLV, bool) {
	for len(b) > 0 {
		t, rest, err := Parse(b)
		if err != nil {
			return TLV{}, false
		}
		if t.Tag == tag {
			return t, true
		}
		b = rest
	}
	return TLV{}, false
}

// Path follows nested tags: Path(b, 0xBF2D, 0xA0) finds A0 inside BF2D.
func Path(b []byte, tags ...uint16) (TLV, bool) {
	var t TLV
	for _, tag := range tags {
		var ok bool
		if t, ok = Find(b, tag); !ok {
			return TLV{}, false
		}
		b = t.Value
	}
	return t, true
}

// Header encodes a tag and a length.
func Header(tag uint16, n int) []byte {
	var h []byte
	if tag > 0xFF {
		h = append(h, byte(tag>>8))
	}
	h = append(h, byte(tag))
	switch {
	case n < 0x80:
		h = append(h, byte(n))
	case n <= 0xFF:
		h = append(h, 0x81, byte(n))
	case n <= 0xFFFF:
		h = append(h, 0x82, byte(n>>8), byte(n))
	default:
		h = append(h, 0x83, byte(n>>16), byte(n>>8), byte(n))
	}
	return h
}

// Encode encodes one element whose value is the concatenation of parts.
func Encode(tag uint16, parts ...[]byte) []byte {
	n := 0
	for _, p := range parts {
		n += len(p)
	}
	out := Header(tag, n)
	for _, p := range parts {
		out = append(out, p...)
	}
	return out
}

// Int decodes an unsigned big-endian integer.
func Int(v []byte) uint64 {
	var n uint64
	for _, c := range v {
		n = n<<8 | uint64(c)
	}
	return n
}

// EncodeInt encodes a non-negative integer as a minimal DER INTEGER value.
func EncodeInt(n uint64) []byte {
	var b []byte
	for {
		b = append([]byte{byte(n)}, b...)
		n >>= 8
		if n == 0 {
			break
		}
	}
	if b[0]&0x80 != 0 {
		b = append([]byte{0}, b...)
	}
	return b
}

// Bits decodes a BIT STRING value (first byte: unused bits) into the names
// of the set bits, most significant first.
func Bits(v []byte, names []string) []string {
	if len(v) < 1 {
		return nil
	}
	unused := int(v[0])
	data := v[1:]
	var out []string
	for i, c := range data {
		for bit := 0; bit < 8; bit++ {
			n := i*8 + bit
			if n >= len(names) || (i == len(data)-1 && bit >= 8-unused) {
				break
			}
			if c&(0x80>>bit) != 0 {
				out = append(out, names[n])
			}
		}
	}
	return out
}
