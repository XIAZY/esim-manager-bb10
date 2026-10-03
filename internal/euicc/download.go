package euicc

import (
	"crypto/sha256"
	"errors"
	"fmt"

	"esimmanager/internal/tlv"
)

// Challenge returns a fresh eUICC challenge (GetEuiccChallenge).
func (e *EUICC) Challenge() ([]byte, error) {
	v, err := e.call(tlv.Encode(0xBF2E), 0xBF2E)
	if err != nil {
		return nil, err
	}
	c, ok := tlv.Find(v, 0x80)
	if !ok {
		return nil, errors.New("euicc: no challenge in the response")
	}
	return c.Value, nil
}

// Info1 returns the encoded EUICCInfo1 (GetEuiccInfo1).
func (e *EUICC) Info1() ([]byte, error) {
	resp, err := e.command(tlv.Encode(0xBF20))
	if err != nil {
		return nil, err
	}
	t, ok := tlv.Find(resp, 0xBF20)
	if !ok {
		return nil, errors.New("euicc: no EUICCInfo1 in the response")
	}
	return t.Raw, nil
}

// element decodes one TLV with the given tag from DER bytes sent by the server.
func element(der []byte, tag uint16, name string) (tlv.TLV, error) {
	t, ok := tlv.Find(der, tag)
	if !ok {
		return tlv.TLV{}, fmt.Errorf("euicc: the server's %s is malformed", name)
	}
	return t, nil
}

// AuthenticateServer passes the SM-DP+'s InitiateAuthentication response to the
// eUICC. It returns the encoded AuthenticateServerResponse and the transaction
// ID from serverSigned1.
func (e *EUICC) AuthenticateServer(serverSigned1, serverSignature1, ciPKID, serverCert []byte,
	matchingID string) (resp, transactionID []byte, err error) {
	signed1, err := element(serverSigned1, 0x30, "serverSigned1")
	if err != nil {
		return nil, nil, err
	}
	txid, ok := tlv.Find(signed1.Value, 0x80)
	if !ok {
		return nil, nil, errors.New("euicc: serverSigned1 has no transaction ID")
	}
	sig1, err := element(serverSignature1, 0x5F37, "serverSignature1")
	if err != nil {
		return nil, nil, err
	}
	pkid, err := element(ciPKID, 0x04, "euiccCiPKIdToBeUsed")
	if err != nil {
		return nil, nil, err
	}
	cert, err := element(serverCert, 0x30, "serverCertificate")
	if err != nil {
		return nil, nil, err
	}

	// CtxParams1: matchingId and DeviceInfo, with the same TAC lpac sends and
	// empty DeviceCapabilities.
	var mid []byte
	if matchingID != "" {
		mid = tlv.Encode(0x80, []byte(matchingID))
	}
	deviceInfo := tlv.Encode(0xA1, tlv.Encode(0x80, []byte{0x35, 0x29, 0x06, 0x11}), tlv.Encode(0xA1))
	req := tlv.Encode(0xBF38, signed1.Raw, sig1.Raw, pkid.Raw, cert.Raw, tlv.Encode(0xA0, mid, deviceInfo))

	resp, err = e.command(req)
	if err != nil {
		return nil, nil, err
	}
	return resp, append([]byte(nil), txid.Value...), nil
}

// ErrConfirmationCodeRequired means the profile needs a confirmation code.
var ErrConfirmationCodeRequired = errors.New("this profile needs a confirmation code")

// ConfirmationCodeRequired reads smdpSigned2.ccRequiredFlag.
func ConfirmationCodeRequired(smdpSigned2 []byte) (bool, error) {
	signed2, err := element(smdpSigned2, 0x30, "smdpSigned2")
	if err != nil {
		return false, err
	}
	f, ok := tlv.Find(signed2.Value, 0x01)
	if !ok {
		return false, errors.New("euicc: smdpSigned2 has no ccRequiredFlag")
	}
	return tlv.Int(f.Value) != 0, nil
}

// PrepareDownload passes the SM-DP+'s AuthenticateClient response to the eUICC
// and returns the encoded PrepareDownloadResponse.
func (e *EUICC) PrepareDownload(smdpSigned2, smdpSignature2, smdpCert []byte, confirmationCode string) ([]byte, error) {
	signed2, err := element(smdpSigned2, 0x30, "smdpSigned2")
	if err != nil {
		return nil, err
	}
	sig2, err := element(smdpSignature2, 0x5F37, "smdpSignature2")
	if err != nil {
		return nil, err
	}
	cert, err := element(smdpCert, 0x30, "smdpCertificate")
	if err != nil {
		return nil, err
	}
	txid, ok := tlv.Find(signed2.Value, 0x80)
	if !ok {
		return nil, errors.New("euicc: smdpSigned2 has no transaction ID")
	}
	required, err := ConfirmationCodeRequired(smdpSigned2)
	if err != nil {
		return nil, err
	}

	parts := [][]byte{signed2.Raw, sig2.Raw}
	if required {
		if confirmationCode == "" {
			return nil, ErrConfirmationCodeRequired
		}
		// hashCc = SHA256(SHA256(code) | transactionId)
		h := sha256.Sum256([]byte(confirmationCode))
		hashCc := sha256.Sum256(append(h[:], txid.Value...))
		parts = append(parts, tlv.Encode(0x04, hashCc[:]))
	}
	parts = append(parts, cert.Raw)
	return e.command(tlv.Encode(0xBF21, parts...))
}

// InstallResult is the outcome of LoadBoundProfilePackage.
type InstallResult struct {
	Seq       uint64
	ICCID     string
	CommandID string // set on failure: the BPP command that failed
	Reason    string // set on failure
}

var bppCommands = map[uint64]string{0: "initialise_secure_channel", 1: "configure_isdp", 2: "store_metadata",
	3: "store_metadata2", 4: "replace_session_keys", 5: "load_profile_elements"}

var installErrors = map[uint64]string{1: "incorrect_input_values", 2: "invalid_signature",
	3: "invalid_transaction_id", 4: "unsupported_crt_values", 5: "unsupported_remote_operation_type",
	6: "unsupported_profile_class", 7: "scp03t_structure_error", 8: "scp03t_security_error",
	9:  "install_failed_due_to_iccid_already_exists_on_euicc",
	10: "install_failed_due_to_insufficient_memory_for_profile", 11: "install_failed_due_to_interruption",
	12: "install_failed_due_to_pe_processing_error", 13: "install_failed_due_to_data_mismatch",
	14: "test_profile_install_failed_due_to_invalid_naa_key", 15: "ppr_not_allowed",
	127: "install_failed_due_to_unknown_error"}

// InstallError is a failed installation reported by the eUICC.
type InstallError struct{ InstallResult }

func (e InstallError) Error() string {
	return fmt.Sprintf("installation failed at %s: %s", e.CommandID, e.Reason)
}

func name(m map[uint64]string, v []byte) string {
	if s, ok := m[tlv.Int(v)]; ok {
		return s
	}
	return "unknown"
}

// header returns the tag and length of an element, without its value.
func header(t tlv.TLV) []byte { return t.Raw[:len(t.Raw)-len(t.Value)] }

// LoadBoundProfilePackage installs a bound profile package, sending it in the
// segments SGP.22 defines: the BF36 header with InitialiseSecureChannel,
// ConfigureISDP, the StoreMetadata header and each of its parts, the optional
// ReplaceSessionKeys, and the profile elements header and each element.
func (e *EUICC) LoadBoundProfilePackage(bpp []byte) (InstallResult, error) {
	pkg, ok := tlv.Find(bpp, 0xBF36)
	if !ok {
		return InstallResult{}, errors.New("euicc: not a bound profile package")
	}
	items, err := tlv.Children(pkg.Value)
	if err != nil {
		return InstallResult{}, err
	}
	find := func(tag uint16) (tlv.TLV, bool) {
		for _, t := range items {
			if t.Tag == tag {
				return t, true
			}
		}
		return tlv.TLV{}, false
	}

	var segments [][]byte
	first := append([]byte(nil), header(pkg)...)
	found := false
	for _, t := range items {
		first = append(first, t.Raw...)
		if t.Tag == 0xBF23 {
			found = true
			break
		}
	}
	if !found {
		return InstallResult{}, errors.New("euicc: the package has no InitialiseSecureChannel")
	}
	segments = append(segments, first)
	configure, ok := find(0xA0)
	if !ok {
		return InstallResult{}, errors.New("euicc: the package has no ConfigureISDP")
	}
	segments = append(segments, configure.Raw)
	for _, tag := range []uint16{0xA1, 0xA2, 0xA3} {
		t, ok := find(tag)
		if !ok {
			if tag == 0xA2 {
				continue // ReplaceSessionKeys is optional
			}
			return InstallResult{}, fmt.Errorf("euicc: the package has no %X", tag)
		}
		if tag == 0xA2 {
			segments = append(segments, t.Raw)
			continue
		}
		segments = append(segments, header(t))
		parts, err := tlv.Children(t.Value)
		if err != nil {
			return InstallResult{}, err
		}
		for _, p := range parts {
			segments = append(segments, p.Raw)
		}
	}

	var result InstallResult
	for _, seg := range segments {
		resp, err := e.command(seg)
		if err != nil {
			return result, err
		}
		if len(resp) == 0 {
			continue
		}
		data, ok := tlv.Path(resp, 0xBF37, 0xBF27)
		if !ok {
			return result, errors.New("euicc: malformed installation result")
		}
		if meta, ok := tlv.Find(data.Value, 0xBF2F); ok {
			n := parseNotificationMetadata(meta.Value)
			result.Seq, result.ICCID = n.Seq, n.ICCID
		}
		final, ok := tlv.Find(data.Value, 0xA2)
		if !ok {
			return result, errors.New("euicc: installation result has no outcome")
		}
		outcome, _, err := tlv.Parse(final.Value)
		if err != nil {
			return result, err
		}
		if outcome.Tag == 0xA0 {
			continue // success
		}
		result.CommandID, result.Reason = "unknown", "unknown"
		if c, ok := tlv.Find(outcome.Value, 0x80); ok {
			result.CommandID = name(bppCommands, c.Value)
		}
		if r, ok := tlv.Find(outcome.Value, 0x81); ok {
			result.Reason = name(installErrors, r.Value)
		}
		return result, InstallError{result}
	}
	return result, nil
}

// Cancel-session reasons (SGP.22 CancelSessionReason).
const (
	CancelEndUserRejection = 0
	CancelPostponed        = 1
	CancelTimeout          = 2
)

// CancelSession ends a download session on the eUICC and returns the encoded
// CancelSessionResponse, for the SM-DP+.
func (e *EUICC) CancelSession(transactionID []byte, reason uint64) ([]byte, error) {
	resp, err := e.command(tlv.Encode(0xBF41, tlv.Encode(0x80, transactionID), tlv.Encode(0x81, tlv.EncodeInt(reason))))
	if err != nil {
		return nil, err
	}
	t, ok := tlv.Find(resp, 0xBF41)
	if !ok {
		return nil, errors.New("euicc: no CancelSessionResponse")
	}
	return t.Raw, nil
}

// Metadata is the part of a profile's StoreMetadataRequest shown before
// installing it.
type Metadata struct {
	ICCID    string
	Provider string
	Name     string
	Class    string
}

// ParseMetadata decodes the profileMetadata an SM-DP+ sends.
func ParseMetadata(der []byte) (Metadata, error) {
	t, ok := tlv.Find(der, 0xBF25)
	if !ok {
		return Metadata{}, errors.New("euicc: malformed profile metadata")
	}
	var m Metadata
	fields, err := tlv.Children(t.Value)
	if err != nil {
		return Metadata{}, err
	}
	for _, f := range fields {
		switch f.Tag {
		case 0x5A:
			m.ICCID = decodeBCD(f.Value)
		case 0x91:
			m.Provider = string(f.Value)
		case 0x92:
			m.Name = string(f.Value)
		case 0x95:
			m.Class = profileClasses[tlv.Int(f.Value)]
		}
	}
	return m, nil
}
