package es9p

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func server(t *testing.T, h http.HandlerFunc) string {
	s := httptest.NewTLSServer(h)
	t.Cleanup(s.Close)
	return strings.TrimPrefix(s.URL, "https://")
}

func TestInitiateAuthentication(t *testing.T) {
	addr := server(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/gsma/rsp2/es9plus/initiateAuthentication" ||
			r.Header.Get("X-Admin-Protocol") != "gsma/rsp/v2.2.2" {
			t.Errorf("request %s %v", r.URL.Path, r.Header)
		}
		var req map[string]string
		json.NewDecoder(r.Body).Decode(&req)
		if req["euiccChallenge"] != "AQID" || req["smdpAddress"] == "" {
			t.Errorf("body %v", req)
		}
		w.Write([]byte(`{"header":{"functionExecutionStatus":{"status":"Executed-Success"}},
			"transactionId":"T1","serverSigned1":"MAA=","serverSignature1":"XzcA",
			"euiccCiPKIdToBeUsed":"BAA=","serverCertificate":"MA\nA="}`))
	})
	a, err := InitiateAuthentication(addr, []byte{1, 2, 3}, []byte{0xBF, 0x20, 0})
	if err != nil {
		t.Fatal(err)
	}
	if a.TransactionID != "T1" || string(a.ServerCertificate) != "\x30\x00" {
		t.Fatalf("%+v", a)
	}
}

func TestServerError(t *testing.T) {
	addr := server(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"header":{"functionExecutionStatus":{"status":"Failed",
			"statusCodeData":{"subjectCode":"8.2.6","reasonCode":"3.8"}}}}`))
	})
	_, err := InitiateAuthentication(addr, nil, nil)
	if err == nil || err.Error() != "MatchingID (AC_Token or EventID) is refused (8.2.6/3.8)" || !IsServerError(err) {
		t.Fatalf("%v", err)
	}
}

func TestResponseCap(t *testing.T) {
	addr := server(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write(make([]byte, maxResponse+10))
	})
	if _, err := GetBoundProfilePackage(addr, "T", nil); err == nil || !strings.Contains(err.Error(), "too large") {
		t.Fatalf("%v", err)
	}
}

func TestBadAddress(t *testing.T) {
	if _, err := InitiateAuthentication("evil.example/x", nil, nil); err == nil {
		t.Fatal("accepted a path in the address")
	}
}
