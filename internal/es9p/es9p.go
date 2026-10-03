// Package es9p is the LPA side of SGP.22 ES9+ (LPA to SM-DP+): JSON over
// HTTPS, a Go translation of lpac's libeuicc es9p.
//
// Copyright (C) 2023-2025 ESTKME TECHNOLOGY LIMITED, Hong Kong
// Copyright (C) 2026 AlphaToad
// SPDX-License-Identifier: LGPL-2.1-only
//
// Servers' TLS certificates are verified against the GSMA certificate issuers
// only, never web roots, and redirects are not followed; see roots.go. The
// eUICC also authenticates the server itself (AuthenticateServer,
// PrepareDownload) and checks the profile package's signature.
package es9p

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// maxResponse caps what a server (or anyone on the path) can make us buffer.
// Bound profile packages are tens of kilobytes.
const maxResponse = 4 << 20

// Error is a failure reported by the server, or of the request itself.
type Error struct {
	SubjectCode string
	ReasonCode  string
	Message     string
}

func (e *Error) Error() string {
	if e.SubjectCode == "" {
		return e.Message
	}
	return fmt.Sprintf("%s (%s/%s)", e.Message, e.SubjectCode, e.ReasonCode)
}

// call POSTs a request to https://<server><path> and returns the response
// object, after checking the status the server reports.
func call(server, path string, req map[string]string) (map[string]json.RawMessage, error) {
	if strings.ContainsAny(server, "/ \t\r\n") || server == "" {
		return nil, &Error{Message: fmt.Sprintf("invalid server address %q", server)}
	}
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	hr, err := http.NewRequest("POST", "https://"+server+path, bytes.NewReader(body))
	if err != nil {
		return nil, &Error{Message: err.Error()}
	}
	hr.Header.Set("User-Agent", "gsma-rsp-lpad")
	hr.Header.Set("X-Admin-Protocol", "gsma/rsp/v2.2.2")
	hr.Header.Set("Content-Type", "application/json")

	resp, err := client().Do(hr)
	if err != nil {
		return nil, &Error{Message: "cannot reach the server: " + err.Error()}
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponse+1))
	if err != nil {
		return nil, &Error{Message: "reading the response: " + err.Error()}
	}
	if len(data) > maxResponse {
		return nil, &Error{Message: "the server's response is too large"}
	}
	if resp.StatusCode/100 != 2 {
		return nil, &Error{Message: fmt.Sprintf("the server answered HTTP %d", resp.StatusCode)}
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return nil, nil // HandleNotification and CancelSession may answer 204
	}

	var out map[string]json.RawMessage
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, &Error{Message: "the server's response is not JSON"}
	}
	var header struct {
		FunctionExecutionStatus struct {
			Status         string `json:"status"`
			StatusCodeData *struct {
				SubjectCode string `json:"subjectCode"`
				ReasonCode  string `json:"reasonCode"`
				Message     string `json:"message"`
			} `json:"statusCodeData"`
		} `json:"functionExecutionStatus"`
	}
	if h, ok := out["header"]; ok {
		_ = json.Unmarshal(h, &header)
	}
	if d := header.FunctionExecutionStatus.StatusCodeData; d != nil &&
		header.FunctionExecutionStatus.Status != "Executed-Success" {
		msg := d.Message
		if msg == "" {
			msg = errorMessage(d.SubjectCode, d.ReasonCode)
		}
		return nil, &Error{SubjectCode: d.SubjectCode, ReasonCode: d.ReasonCode, Message: msg}
	}
	return out, nil
}

// fields extracts required base64 or string fields from a response.
func fields(out map[string]json.RawMessage, names ...string) ([]string, error) {
	vals := make([]string, len(names))
	for i, n := range names {
		raw, ok := out[n]
		if !ok || json.Unmarshal(raw, &vals[i]) != nil || vals[i] == "" {
			return nil, &Error{Message: fmt.Sprintf("the server's response has no %s", n)}
		}
	}
	return vals, nil
}

func decode(s string) ([]byte, error) {
	b, err := base64.StdEncoding.DecodeString(strings.Join(strings.Fields(s), ""))
	if err != nil {
		return nil, &Error{Message: "the server sent invalid base64"}
	}
	return b, nil
}

func encode(b []byte) string { return base64.StdEncoding.EncodeToString(b) }

// Authentication is the InitiateAuthentication response.
type Authentication struct {
	TransactionID     string
	ServerSigned1     []byte
	ServerSignature1  []byte
	EuiccCiPKIDToUse  []byte
	ServerCertificate []byte
}

func InitiateAuthentication(server string, challenge, info1 []byte) (*Authentication, error) {
	out, err := call(server, "/gsma/rsp2/es9plus/initiateAuthentication", map[string]string{
		"smdpAddress": server, "euiccChallenge": encode(challenge), "euiccInfo1": encode(info1)})
	if err != nil {
		return nil, err
	}
	f, err := fields(out, "transactionId", "serverSigned1", "serverSignature1", "euiccCiPKIdToBeUsed",
		"serverCertificate")
	if err != nil {
		return nil, err
	}
	a := &Authentication{TransactionID: f[0]}
	for i, dst := range []*[]byte{&a.ServerSigned1, &a.ServerSignature1, &a.EuiccCiPKIDToUse, &a.ServerCertificate} {
		if *dst, err = decode(f[i+1]); err != nil {
			return nil, err
		}
	}
	return a, nil
}

// ClientAuthentication is the AuthenticateClient response.
type ClientAuthentication struct {
	ProfileMetadata []byte
	SMDPSigned2     []byte
	SMDPSignature2  []byte
	SMDPCertificate []byte
}

func AuthenticateClient(server, transactionID string, authServerResponse []byte) (*ClientAuthentication, error) {
	out, err := call(server, "/gsma/rsp2/es9plus/authenticateClient", map[string]string{
		"transactionId": transactionID, "authenticateServerResponse": encode(authServerResponse)})
	if err != nil {
		return nil, err
	}
	f, err := fields(out, "profileMetadata", "smdpSigned2", "smdpSignature2", "smdpCertificate")
	if err != nil {
		return nil, err
	}
	c := &ClientAuthentication{}
	for i, dst := range []*[]byte{&c.ProfileMetadata, &c.SMDPSigned2, &c.SMDPSignature2, &c.SMDPCertificate} {
		if *dst, err = decode(f[i]); err != nil {
			return nil, err
		}
	}
	return c, nil
}

func GetBoundProfilePackage(server, transactionID string, prepareDownloadResponse []byte) ([]byte, error) {
	out, err := call(server, "/gsma/rsp2/es9plus/getBoundProfilePackage", map[string]string{
		"transactionId": transactionID, "prepareDownloadResponse": encode(prepareDownloadResponse)})
	if err != nil {
		return nil, err
	}
	f, err := fields(out, "boundProfilePackage")
	if err != nil {
		return nil, err
	}
	return decode(f[0])
}

func HandleNotification(server string, pendingNotification []byte) error {
	_, err := call(server, "/gsma/rsp2/es9plus/handleNotification", map[string]string{
		"pendingNotification": encode(pendingNotification)})
	return err
}

func CancelSession(server, transactionID string, cancelSessionResponse []byte) error {
	_, err := call(server, "/gsma/rsp2/es9plus/cancelSession", map[string]string{
		"transactionId": transactionID, "cancelSessionResponse": encode(cancelSessionResponse)})
	return err
}

// IsServerError reports whether err came from the server rather than the
// network.
func IsServerError(err error) bool {
	var e *Error
	return errors.As(err, &e) && e.SubjectCode != ""
}
