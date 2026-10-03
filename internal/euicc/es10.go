package euicc

import (
	"encoding/hex"
	"fmt"

	"esimmanager/internal/tlv"
)

// EID returns the eUICC's identifier.
func (e *EUICC) EID() (string, error) {
	v, err := e.call(tlv.Encode(0xBF3E, tlv.Encode(0x5C, []byte{0x5A})), 0xBF3E)
	if err != nil {
		return "", err
	}
	eid, ok := tlv.Find(v, 0x5A)
	if !ok {
		return "", fmt.Errorf("euicc: no EID in the response")
	}
	return hex.EncodeToString(eid.Value), nil
}

// Addresses are the SM-DP+ and SM-DS addresses configured on the eUICC.
type Addresses struct {
	DefaultSMDP string
	RootSMDS    string
}

func (e *EUICC) Addresses() (Addresses, error) {
	v, err := e.call(tlv.Encode(0xBF3C), 0xBF3C)
	if err != nil {
		return Addresses{}, err
	}
	var a Addresses
	if t, ok := tlv.Find(v, 0x80); ok {
		a.DefaultSMDP = string(t.Value)
	}
	if t, ok := tlv.Find(v, 0x81); ok {
		a.RootSMDS = string(t.Value)
	}
	return a, nil
}

// Info is the part of EUICCInfo2 the app shows.
type Info struct {
	ProfileVersion   string
	SVN              string
	FirmwareVersion  string
	FreeNVM          uint64
	FreeVM           uint64
	Category         string
	SASAccreditation string
	CIPKIDs          []string
	RSPCapabilities  []string
}

var rspCapabilityNames = []string{"additionalProfile", "crlSupport", "rpmSupport", "testProfileSupport",
	"deviceInfoExtensibilitySupport"}

func version(v []byte) string {
	if len(v) != 3 {
		return ""
	}
	return fmt.Sprintf("%d.%d.%d", v[0], v[1], v[2])
}

func (e *EUICC) Info() (Info, error) {
	v, err := e.call(tlv.Encode(0xBF22), 0xBF22)
	if err != nil {
		return Info{}, err
	}
	items, err := tlv.Children(v)
	if err != nil {
		return Info{}, err
	}
	var in Info
	for _, t := range items {
		switch t.Tag {
		case 0x81:
			in.ProfileVersion = version(t.Value)
		case 0x82:
			in.SVN = version(t.Value)
		case 0x83:
			in.FirmwareVersion = version(t.Value)
		case 0x84: // extCardResource
			if r, ok := tlv.Find(t.Value, 0x82); ok {
				in.FreeNVM = tlv.Int(r.Value)
			}
			if r, ok := tlv.Find(t.Value, 0x83); ok {
				in.FreeVM = tlv.Int(r.Value)
			}
		case 0x88:
			in.RSPCapabilities = tlv.Bits(t.Value, rspCapabilityNames)
		case 0xAA: // euiccCiPKIdListForSigning
			ids, _ := tlv.Children(t.Value)
			for _, id := range ids {
				in.CIPKIDs = append(in.CIPKIDs, hex.EncodeToString(id.Value))
			}
		case 0xAB:
			in.Category = "other"
			if c := tlv.Int(t.Value); c <= 3 {
				in.Category = [...]string{"other", "basicEuicc", "mediumEuicc", "contactlessEuicc"}[c]
			}
		case 0x0C:
			in.SASAccreditation = string(t.Value)
		}
	}
	return in, nil
}

// Profile is one entry of the eUICC's profile list.
type Profile struct {
	ICCID    string
	ISDPAID  string
	Enabled  bool
	Nickname string
	Provider string
	Name     string
	Class    string
}

var profileClasses = map[uint64]string{0: "test", 1: "provisioning", 2: "operational"}

func (e *EUICC) Profiles() ([]Profile, error) {
	v, err := e.call(tlv.Encode(0xBF2D), 0xBF2D)
	if err != nil {
		return nil, err
	}
	list, ok := tlv.Find(v, 0xA0)
	if !ok {
		return nil, fmt.Errorf("euicc: no profile list in the response")
	}
	items, err := tlv.Children(list.Value)
	if err != nil {
		return nil, err
	}
	var out []Profile
	for _, item := range items {
		if item.Tag != 0xE3 {
			continue
		}
		fields, err := tlv.Children(item.Value)
		if err != nil {
			return nil, err
		}
		var p Profile
		for _, f := range fields {
			switch f.Tag {
			case 0x5A:
				p.ICCID = decodeBCD(f.Value)
			case 0x4F:
				p.ISDPAID = hex.EncodeToString(f.Value)
			case 0x9F70:
				p.Enabled = tlv.Int(f.Value) == 1
			case 0x90:
				p.Nickname = string(f.Value)
			case 0x91:
				p.Provider = string(f.Value)
			case 0x92:
				p.Name = string(f.Value)
			case 0x95:
				p.Class = profileClasses[tlv.Int(f.Value)]
			}
		}
		out = append(out, p)
	}
	return out, nil
}

// Result codes of EnableProfile, DisableProfile and DeleteProfile (SGP.22).
const (
	ResultOK            = 0
	ResultNotFound      = 1
	ResultWrongState    = 2
	ResultPolicy        = 3
	ResultWrongReenable = 4
	ResultCATBusy       = 5
)

// Enable enables a profile. With refresh, the eUICC asks the device to restart
// the SIM session so the modem picks up the profile; that also ends every
// logical channel.
func (e *EUICC) Enable(id string, refresh bool) (int, error) {
	return e.switchProfile(0xBF31, id, refresh)
}

// Disable disables a profile; see Enable for refresh.
func (e *EUICC) Disable(id string, refresh bool) (int, error) {
	return e.switchProfile(0xBF32, id, refresh)
}

func (e *EUICC) switchProfile(tag uint16, id string, refresh bool) (int, error) {
	pid, err := profileID(id)
	if err != nil {
		return 0, err
	}
	flag := byte(0x00)
	if refresh {
		flag = 0xFF
	}
	return e.resultCode(tlv.Encode(tag, tlv.Encode(0xA0, pid, tlv.Encode(0x81, []byte{flag}))), tag)
}

// Delete deletes a disabled profile.
func (e *EUICC) Delete(id string) (int, error) {
	pid, err := profileID(id)
	if err != nil {
		return 0, err
	}
	return e.resultCode(tlv.Encode(0xBF33, pid), 0xBF33)
}

// SetNickname sets or, with an empty name, clears a profile's nickname.
func (e *EUICC) SetNickname(iccid, name string) (int, error) {
	b, err := encodeBCD(iccid, 10)
	if err != nil {
		return 0, err
	}
	return e.resultCode(tlv.Encode(0xBF29, tlv.Encode(0x5A, b), tlv.Encode(0x90, []byte(name))), 0xBF29)
}

// Notification is a pending notification's metadata.
type Notification struct {
	Seq       uint64
	Operation string
	Address   string
	ICCID     string
}

var operations = map[byte]string{0x80: "install", 0x40: "enable", 0x20: "disable", 0x10: "delete"}

func (e *EUICC) Notifications() ([]Notification, error) {
	v, err := e.call(tlv.Encode(0xBF28), 0xBF28)
	if err != nil {
		return nil, err
	}
	list, ok := tlv.Find(v, 0xA0)
	if !ok {
		return nil, nil // listNotificationsResultError: nothing to list
	}
	items, err := tlv.Children(list.Value)
	if err != nil {
		return nil, err
	}
	var out []Notification
	for _, item := range items {
		if item.Tag != 0xBF2F {
			continue
		}
		out = append(out, parseNotificationMetadata(item.Value))
	}
	return out, nil
}

func parseNotificationMetadata(v []byte) Notification {
	var n Notification
	fields, _ := tlv.Children(v)
	for _, f := range fields {
		switch f.Tag {
		case 0x80:
			n.Seq = tlv.Int(f.Value)
		case 0x81:
			if len(f.Value) >= 2 {
				n.Operation = operations[f.Value[1]]
			}
		case 0x0C:
			n.Address = string(f.Value)
		case 0x5A:
			n.ICCID = decodeBCD(f.Value)
		}
	}
	return n
}

// PendingNotification returns a notification's server address and its
// encoded form, to send with ES9+ HandleNotification.
func (e *EUICC) PendingNotification(seq uint64) (address string, raw []byte, err error) {
	req := tlv.Encode(0xBF2B, tlv.Encode(0xA0, tlv.Encode(0x80, tlv.EncodeInt(seq))))
	v, err := e.call(req, 0xBF2B)
	if err != nil {
		return "", nil, err
	}
	list, ok := tlv.Find(v, 0xA0)
	if !ok {
		return "", nil, fmt.Errorf("euicc: notification %d not found", seq)
	}
	items, err := tlv.Children(list.Value)
	if err != nil || len(items) == 0 {
		return "", nil, fmt.Errorf("euicc: notification %d not found", seq)
	}
	pending := items[0]
	var meta tlv.TLV
	switch pending.Tag {
	case 0xBF37: // profileInstallationResult
		meta, ok = tlv.Path(pending.Value, 0xBF27, 0xBF2F)
	case 0x30: // otherSignedNotification
		meta, ok = tlv.Find(pending.Value, 0xBF2F)
	default:
		ok = false
	}
	if !ok {
		return "", nil, fmt.Errorf("euicc: malformed notification %d", seq)
	}
	n := parseNotificationMetadata(meta.Value)
	if n.Address == "" {
		return "", nil, fmt.Errorf("euicc: notification %d has no address", seq)
	}
	return n.Address, pending.Raw, nil
}

// RemoveNotification deletes a notification from the eUICC's list.
func (e *EUICC) RemoveNotification(seq uint64) (int, error) {
	return e.resultCode(tlv.Encode(0xBF30, tlv.Encode(0x80, tlv.EncodeInt(seq))), 0xBF30)
}
