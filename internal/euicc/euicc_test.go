package euicc

import (
	"bytes"
	"encoding/hex"
	"reflect"
	"testing"

	"esimmanager/internal/tlv"
)

func h(s string) []byte {
	b, err := hex.DecodeString(s)
	if err != nil {
		panic(err)
	}
	return b
}

// fake answers APDUs from a script and records what it was sent.
type fake struct {
	sent  [][]byte
	reply [][]byte
}

func (f *fake) Transmit(apdu []byte) ([]byte, error) {
	f.sent = append(f.sent, append([]byte(nil), apdu...))
	r := f.reply[0]
	f.reply = f.reply[1:]
	return r, nil
}

func ok(b []byte) []byte { return append(append([]byte(nil), b...), 0x90, 0x00) }

// Dummy identifiers. The BCD forms are written out by hand so the decoder is
// checked independently of encodeBCD.
const (
	dummyEID     = "89000000000000000000000000000001"
	dummyICCID20 = "89000000000000000019"
	dummyBCD20   = "98000000000000000091"
	dummyICCID19 = "8900000000000000027" // odd length: padded with F
	dummyBCD19   = "980000000000000020f7"
	dummyPKID    = "0102030405060708090a0b0c0d0e0f1011121314"
)

func TestEID(t *testing.T) {
	f := &fake{reply: [][]byte{ok(tlv.Encode(0xBF3E, tlv.Encode(0x5A, h(dummyEID))))}}
	eid, err := New(f).EID()
	if err != nil || eid != dummyEID {
		t.Fatalf("%q %v", eid, err)
	}
	if !bytes.Equal(f.sent[0], h("80e2910006bf3e035c015a")) {
		t.Fatalf("sent %x", f.sent[0])
	}
}

func TestAddressesAndInfo(t *testing.T) {
	addresses := tlv.Encode(0xBF3C, tlv.Encode(0x81, []byte("smds.example.com")))
	info2 := tlv.Encode(0xBF22,
		tlv.Encode(0x81, h("020301")),
		tlv.Encode(0x82, h("020202")),
		tlv.Encode(0x83, h("040200")),
		tlv.Encode(0x84, tlv.Encode(0x81, h("00")), tlv.Encode(0x82, h("0001fef6")), tlv.Encode(0x83, h("377a"))),
		tlv.Encode(0x88, h("0490")),
		tlv.Encode(0xAA, tlv.Encode(0x04, h(dummyPKID))),
		tlv.Encode(0x0C, []byte("XX-YY-ZZ-0000")))
	f := &fake{reply: [][]byte{ok(addresses), ok(info2)}}
	e := New(f)
	a, err := e.Addresses()
	if err != nil || a.RootSMDS != "smds.example.com" || a.DefaultSMDP != "" {
		t.Fatalf("%+v %v", a, err)
	}
	in, err := e.Info()
	if err != nil {
		t.Fatal(err)
	}
	want := Info{ProfileVersion: "2.3.1", SVN: "2.2.2", FirmwareVersion: "4.2.0", FreeNVM: 130806, FreeVM: 14202,
		SASAccreditation: "XX-YY-ZZ-0000", CIPKIDs: []string{dummyPKID},
		RSPCapabilities: []string{"additionalProfile", "testProfileSupport"}}
	if !reflect.DeepEqual(in, want) {
		t.Fatalf("got  %+v\nwant %+v", in, want)
	}
}

func profileInfo(bcd, aid string, state byte, fields ...[]byte) []byte {
	parts := [][]byte{tlv.Encode(0x5A, h(bcd)), tlv.Encode(0x4F, h(aid)), tlv.Encode(0x9F70, []byte{state})}
	return tlv.Encode(0xE3, append(parts, fields...)...)
}

func TestProfiles(t *testing.T) {
	list := tlv.Encode(0xBF2D, tlv.Encode(0xA0,
		profileInfo(dummyBCD20, "a0000005591010ffffffff8900001200", 0,
			tlv.Encode(0x91, []byte("Example Mobile")), tlv.Encode(0x92, []byte("Example 1")),
			tlv.Encode(0x95, []byte{2})),
		profileInfo(dummyBCD19, "a0000005591010ffffffff8900001300", 1,
			tlv.Encode(0x90, []byte("Travel")), tlv.Encode(0x91, []byte("Dummy Telecom")),
			tlv.Encode(0x95, []byte{0}))))
	ps, err := New(&fake{reply: [][]byte{ok(list)}}).Profiles()
	if err != nil {
		t.Fatal(err)
	}
	want := []Profile{
		{ICCID: dummyICCID20, ISDPAID: "a0000005591010ffffffff8900001200", Provider: "Example Mobile",
			Name: "Example 1", Class: "operational"},
		{ICCID: dummyICCID19, ISDPAID: "a0000005591010ffffffff8900001300", Enabled: true, Nickname: "Travel",
			Provider: "Dummy Telecom", Class: "test"},
	}
	if !reflect.DeepEqual(ps, want) {
		t.Fatalf("got  %+v\nwant %+v", ps, want)
	}
}

func TestEnableRequest(t *testing.T) {
	f := &fake{reply: [][]byte{ok(h("bf3103800100"))}}
	r, err := New(f).Enable(dummyICCID19, true)
	if err != nil || r != ResultOK {
		t.Fatalf("%d %v", r, err)
	}
	// BF31 { A0 { 5A iccid(BCD, F-padded) 81 FF } }
	want := h("80e2910014bf3111a00f5a0a" + dummyBCD19 + "8101ff")
	if !bytes.Equal(f.sent[0], want) {
		t.Fatalf("sent %x\nwant %x", f.sent[0], want)
	}
}

func TestChainingAndGetResponse(t *testing.T) {
	// A 300-byte request goes out as 120 + 120 + 60; the last block's answer
	// comes in two parts through 61xx / GET RESPONSE.
	req := bytes.Repeat([]byte{0xAB}, 300)
	f := &fake{reply: [][]byte{h("9000"), h("9000"), h("01026102"), h("03049000")}}
	resp, err := New(f).command(req)
	if err != nil || !bytes.Equal(resp, h("01020304")) {
		t.Fatalf("%x %v", resp, err)
	}
	heads := []string{"80e2110078", "80e2110178", "80e291023c", "80c0000002"}
	for i, want := range heads {
		if hex.EncodeToString(f.sent[i][:5]) != want {
			t.Errorf("apdu %d header %x, want %s", i, f.sent[i][:5], want)
		}
	}
}

func TestStatusError(t *testing.T) {
	_, err := New(&fake{reply: [][]byte{h("6a88")}}).EID()
	if err == nil || err.Error() != "eUICC returned status 6A88" {
		t.Fatalf("%v", err)
	}
}

func TestBCD(t *testing.T) {
	for iccid, bcd := range map[string]string{dummyICCID20: dummyBCD20, dummyICCID19: dummyBCD19} {
		b, err := encodeBCD(iccid, 10)
		if err != nil || hex.EncodeToString(b) != bcd || decodeBCD(b) != iccid {
			t.Fatalf("%s: %x %v", iccid, b, err)
		}
	}
	if _, err := encodeBCD("12a4", 10); err == nil {
		t.Fatal("accepted a non-digit")
	}
}
