package tlv

import (
	"bytes"
	"encoding/hex"
	"reflect"
	"testing"
)

func h(s string) []byte {
	b, err := hex.DecodeString(s)
	if err != nil {
		panic(err)
	}
	return b
}

func TestParseEID(t *testing.T) {
	// A GET EID response, with a dummy EID.
	b := h("bf3e125a1089000000000000000000000000000001")
	eid, ok := Path(b, 0xBF3E, 0x5A)
	if !ok || hex.EncodeToString(eid.Value) != "89000000000000000000000000000001" {
		t.Fatalf("got %x %v", eid.Value, ok)
	}
}

func TestLongLength(t *testing.T) {
	v := bytes.Repeat([]byte{1}, 300)
	enc := Encode(0xBF2D, v)
	if !bytes.Equal(enc[:5], h("bf2d82012c")) {
		t.Fatalf("header %x", enc[:5])
	}
	got, rest, err := Parse(enc)
	if err != nil || len(rest) != 0 || got.Tag != 0xBF2D || !bytes.Equal(got.Value, v) {
		t.Fatalf("roundtrip: %v %v", err, got.Tag)
	}
}

func TestTruncated(t *testing.T) {
	for _, s := range []string{"", "5a", "5a05aa", "bf", "5a8201", "5a84ffffffff00"} {
		if _, _, err := Parse(h(s)); err == nil {
			t.Errorf("%q: no error", s)
		}
	}
}

func TestEncodeInt(t *testing.T) {
	for n, want := range map[uint64]string{0: "00", 5: "05", 0x7F: "7f", 0x80: "0080", 0x1234: "1234"} {
		if got := hex.EncodeToString(EncodeInt(n)); got != want {
			t.Errorf("%d: %s, want %s", n, got, want)
		}
	}
}

func TestBits(t *testing.T) {
	// rspCapability: additionalProfile, testProfileSupport.
	got := Bits(h("0490"), []string{"additionalProfile", "crlSupport", "rpmSupport", "testProfileSupport"})
	if !reflect.DeepEqual(got, []string{"additionalProfile", "testProfileSupport"}) {
		t.Fatalf("%v", got)
	}
}
