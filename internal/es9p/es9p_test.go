package es9p

import (
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// server starts a TLS test server and makes the package trust its
// self-signed certificate for the rest of the test.
func server(t *testing.T, h http.HandlerFunc) string {
	s := httptest.NewTLSServer(h)
	t.Cleanup(s.Close)
	pool := x509.NewCertPool()
	pool.AddCert(s.Certificate())
	saved := client
	client = func() *http.Client { return newClient(pool) }
	t.Cleanup(func() { client = saved })
	return strings.TrimPrefix(s.URL, "https://")
}

func TestUntrustedServer(t *testing.T) {
	// A server whose certificate chains to none of the trusted roots, as an
	// interceptor's would, is refused.
	s := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("request reached an untrusted server")
	}))
	defer s.Close()
	_, err := InitiateAuthentication(strings.TrimPrefix(s.URL, "https://"), nil, nil)
	if err == nil || !strings.Contains(err.Error(), "certificate") {
		t.Fatalf("got %v, want a certificate error", err)
	}
}

func TestRedirectRefused(t *testing.T) {
	// A redirect, here to plain HTTP, is not followed.
	plain := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("request followed a redirect to plain HTTP")
	}))
	defer plain.Close()
	addr := server(t, func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, plain.URL+r.URL.Path, http.StatusTemporaryRedirect)
	})
	_, err := InitiateAuthentication(addr, nil, nil)
	if err == nil || !strings.Contains(err.Error(), "HTTP 307") {
		t.Fatalf("got %v, want HTTP 307", err)
	}
}

func TestOnlyCIsTrusted(t *testing.T) {
	// The pool is the bundle's six CIs and nothing else: no system or web roots.
	if n := len(trustedRoots().Subjects()); n != 6 {
		t.Fatalf("%d trusted roots, want the 6 in the bundle", n)
	}
}

func TestRSPRootsLoaded(t *testing.T) {
	n := 0
	for block, rest := pem.Decode(rspRoots); block != nil; block, rest = pem.Decode(rest) {
		if _, err := x509.ParseCertificate(block.Bytes); err != nil {
			t.Errorf("root %d: %v", n, err)
		}
		n++
	}
	if n != 6 {
		t.Fatalf("%d RSP roots, want 6", n)
	}
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
