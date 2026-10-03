// ES9+ TLS trust: the GSMA certificate issuers only.
//
// Copyright (C) 2026 Zhongyang Xia
// SPDX-License-Identifier: LGPL-2.1-only
package es9p

import (
	"crypto/tls"
	"crypto/x509"
	_ "embed"
	"net/http"
	"sync"
	"time"
)

// The production GSMA certificate issuers (see roots/README.md). SGP.22
// requires SM-DP+ and SM-DS TLS certificates to chain to one of them, so
// these are the only roots trusted: not the phone's store, and no web roots.
//
//go:embed roots/osmocom-ci-bundle.pem
var rspRoots []byte

func trustedRoots() *x509.CertPool {
	pool := x509.NewCertPool()
	pool.AppendCertsFromPEM(rspRoots)
	return pool
}

var client = sync.OnceValue(func() *http.Client {
	return newClient(trustedRoots())
})

func newClient(roots *x509.CertPool) *http.Client {
	return &http.Client{
		Timeout: 60 * time.Second,
		// ES9+ never redirects, and following one could leave HTTPS.
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{RootCAs: roots},
		},
	}
}
