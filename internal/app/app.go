// Package app is eSIM Manager's logic: one goroutine owns the eUICC, runs the
// commands the UI sends, and publishes state snapshots and events for the UI.
package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"esimmanager/internal/es9p"
	"esimmanager/internal/euicc"
)

// Card is an open channel to the ISD-R.
type Card interface {
	euicc.Transport
	Close()
}

// Publisher receives what the UI shows. Both calls may come from any
// goroutine; the payload is JSON.
type Publisher interface {
	State(json string)
	Event(json string)
}

// Command is one request from the UI.
type Command struct {
	Name string            `json:"name"`
	Args map[string]string `json:"args"`
}

// State is everything the UI renders.
type State struct {
	Busy          bool           `json:"busy"`
	Progress      string         `json:"progress"`
	Conn          string         `json:"conn"` // connecting, ready, noAccess, notReady, noEuicc, error
	ConnDetail    string         `json:"connDetail"`
	Previewing    bool           `json:"previewing"`
	Chip          *Chip          `json:"chip"`
	Profiles      []Profile      `json:"profiles"`
	Notifications []Notification `json:"notifications"`
}

type Chip struct {
	EID             string   `json:"eid"`
	DefaultSMDP     string   `json:"defaultSmdp"`
	RootSMDS        string   `json:"rootSmds"`
	ProfileVersion  string   `json:"profileVersion"`
	SVN             string   `json:"svn"`
	FirmwareVersion string   `json:"firmwareVersion"`
	FreeNVM         uint64   `json:"freeNvm"`
	FreeVM          uint64   `json:"freeVm"`
	Category        string   `json:"category"`
	SAS             string   `json:"sas"`
	CIPKIDs         []string `json:"ciPkids"`
	RSPCapabilities []string `json:"rspCapabilities"`
}

type Profile struct {
	ICCID    string `json:"iccid"`
	ISDPAID  string `json:"isdpAid"`
	Enabled  bool   `json:"enabled"`
	Nickname string `json:"nickname"`
	Provider string `json:"provider"`
	Name     string `json:"name"`
	Class    string `json:"profileClass"`
}

// Title is how the UI names a profile.
func (p Profile) Title() string {
	for _, s := range []string{p.Nickname, p.Name, p.Provider} {
		if s != "" {
			return s
		}
	}
	return p.ICCID
}

type Notification struct {
	Seq       uint64 `json:"seq"`
	Operation string `json:"operation"`
	ICCID     string `json:"iccid"`
}

// download is a session between startDownload and finishDownload.
type download struct {
	server    string
	txid      string // as the SM-DP+ names it
	txidBin   []byte // as the eUICC names it
	auth      *es9p.ClientAuthentication
	title     string
	needsCode bool
}

// App runs commands on the eUICC. Create it with New and call Run on its own
// goroutine.
type App struct {
	pub   Publisher
	open  func() (Card, error)
	cmds  chan Command
	done  chan struct{}
	state State

	card Card
	e    *euicc.EUICC
	dl   *download
}

// New returns an App that opens a channel to the ISD-R with open.
func New(pub Publisher, open func() (Card, error)) *App {
	return &App{pub: pub, open: open, cmds: make(chan Command, 16), done: make(chan struct{}),
		state: State{Conn: "connecting"}}
}

// Send queues a command; it never blocks the UI thread.
func (a *App) Send(c Command) {
	select {
	case a.cmds <- c:
	default:
		log.Printf("app: command queue full, dropping %s", c.Name)
	}
}

// Run handles commands until Stop. The UI sends the first refresh once it
// can show the result.
func (a *App) Run() {
	defer close(a.done)
	for c := range a.cmds {
		if c.Name == "quit" {
			break
		}
		a.handle(c)
		// Hold a logical channel only while working: qct_rrm never frees one
		// whose owner died. A pending download keeps its session open.
		if a.dl == nil {
			a.closeCard()
		}
	}
	if a.dl != nil {
		a.cancelDownload(a.dl, euicc.CancelEndUserRejection)
	}
	a.closeCard()
}

// Stop asks Run to finish (cancelling a pending download and closing the
// channel) and waits up to timeout for it, so the channel is not leaked when
// the process exits.
func (a *App) Stop(timeout time.Duration) {
	select {
	case a.cmds <- Command{Name: "quit"}:
	default:
	}
	select {
	case <-a.done:
	case <-time.After(timeout):
		log.Printf("app: still busy at exit; the SIM channel may stay open")
	}
}

func (a *App) publish() {
	b, _ := json.Marshal(a.state)
	a.pub.State(string(b))
}

func (a *App) event(v map[string]any) {
	b, _ := json.Marshal(v)
	a.pub.Event(string(b))
}

func (a *App) toast(format string, args ...any) {
	a.event(map[string]any{"type": "toast", "text": fmt.Sprintf(format, args...)})
}

func (a *App) progress(s string) {
	a.state.Progress = s
	a.publish()
}

func (a *App) handle(c Command) {
	arg := func(k string) string { return strings.TrimSpace(c.Args[k]) }
	if a.dl != nil && c.Name != "accept" && c.Name != "reject" {
		return // a download is waiting for the user's decision
	}
	a.state.Busy = true
	a.publish()
	defer func() {
		a.state.Busy, a.state.Progress = false, ""
		a.publish()
	}()

	switch c.Name {
	case "refresh":
		if a.refresh(1) && len(a.state.Notifications) > 0 {
			a.sendNotifications(false)
		}
	case "enable", "disable":
		a.switchProfile(arg("iccid"), c.Name == "enable")
	case "delete":
		a.deleteProfile(arg("iccid"))
	case "nickname":
		a.setNickname(arg("iccid"), c.Args["name"])
	case "download":
		a.startDownload(arg("smdp"), arg("matchingId"))
	case "accept":
		a.finishDownload(true, arg("code"))
	case "reject":
		a.finishDownload(false, "")
	case "sendNotifications":
		a.sendNotifications(true)
	case "removeNotification":
		a.removeNotification(arg("seq"))
	default:
		log.Printf("app: unknown command %q", c.Name)
	}
}

// connect opens the card if needed, trying up to attempts times while the SIM
// restarts. It records why it failed in the state.
func (a *App) connect(attempts int) bool {
	if a.card != nil {
		return true
	}
	var err error
	for i := 0; i < attempts; i++ {
		if i > 0 {
			a.progress("Waiting for the SIM…")
			time.Sleep(2 * time.Second)
		}
		var card Card
		if card, err = a.open(); err == nil {
			a.card, a.e = card, euicc.New(card)
			a.state.Conn, a.state.ConnDetail = "ready", ""
			return true
		}
	}
	var ce interface{ ConnState() string }
	state := "error"
	if errors.As(err, &ce) {
		state = ce.ConnState()
	}
	switch state {
	case "noAccess":
		a.state.Conn, a.state.ConnDetail = "noAccess", "eSIM Manager could not get access to the SIM. It needs a phone rooted with bb10mt."
	case "notReady":
		a.state.Conn, a.state.ConnDetail = "notReady", "The SIM is not ready."
	case "noApplet":
		a.state.Conn, a.state.ConnDetail = "noEuicc", "No eUICC answered. Is a removable eSIM card inserted?"
	case "noChannel":
		a.state.Conn, a.state.ConnDetail = "noChannel", "All of the SIM's logical channels are in use. Restarting the phone frees them."
	default:
		a.state.Conn, a.state.ConnDetail = "error", err.Error()
	}
	return false
}

func (a *App) closeCard() {
	if a.card != nil {
		a.card.Close()
		a.card, a.e = nil, nil
	}
}

// fail reports an eUICC error and drops the channel, which may be stale.
func (a *App) fail(what string, err error) {
	a.closeCard()
	a.toast("%s: %v", what, err)
}

func (a *App) refresh(attempts int) bool {
	a.progress("Reading the eUICC…")
	if !a.connect(attempts) {
		return false
	}
	if err := a.readAll(); err != nil {
		a.fail("Could not read the eUICC", err)
		return false
	}
	return true
}

func (a *App) readAll() error {
	eid, err := a.e.EID()
	if err != nil {
		return err
	}
	addr, err := a.e.Addresses()
	if err != nil {
		return err
	}
	info, err := a.e.Info()
	if err != nil {
		return err
	}
	a.state.Chip = &Chip{EID: eid, DefaultSMDP: addr.DefaultSMDP, RootSMDS: addr.RootSMDS,
		ProfileVersion: info.ProfileVersion, SVN: info.SVN, FirmwareVersion: info.FirmwareVersion,
		FreeNVM: info.FreeNVM, FreeVM: info.FreeVM, Category: info.Category, SAS: info.SASAccreditation,
		CIPKIDs: info.CIPKIDs, RSPCapabilities: info.RSPCapabilities}
	if err := a.readProfiles(); err != nil {
		return err
	}
	return a.readNotifications()
}

func (a *App) readProfiles() error {
	ps, err := a.e.Profiles()
	if err != nil {
		return err
	}
	a.state.Profiles = make([]Profile, len(ps))
	for i, p := range ps {
		a.state.Profiles[i] = Profile{ICCID: p.ICCID, ISDPAID: p.ISDPAID, Enabled: p.Enabled,
			Nickname: p.Nickname, Provider: p.Provider, Name: p.Name, Class: p.Class}
	}
	return nil
}

func (a *App) readNotifications() error {
	ns, err := a.e.Notifications()
	if err != nil {
		return err
	}
	a.state.Notifications = make([]Notification, len(ns))
	for i, n := range ns {
		a.state.Notifications[i] = Notification{Seq: n.Seq, Operation: n.Operation, ICCID: n.ICCID}
	}
	return nil
}

func (a *App) title(iccid string) string {
	for _, p := range a.state.Profiles {
		if p.ICCID == iccid {
			return p.Title()
		}
	}
	return iccid
}

// resultMessage explains an SGP.22 EnableProfile/DisableProfile/DeleteProfile
// result code.
func resultMessage(code int, enabling bool) string {
	switch code {
	case euicc.ResultNotFound:
		return "The profile was not found on the eUICC."
	case euicc.ResultWrongState:
		if enabling {
			return "The profile is not disabled."
		}
		return "The profile is not enabled."
	case euicc.ResultPolicy:
		return "The profile's policy rules do not allow this."
	case euicc.ResultWrongReenable:
		return "Re-enabling this profile is not allowed."
	case euicc.ResultCATBusy:
		return "The SIM is busy. Try again in a moment."
	}
	return fmt.Sprintf("The eUICC refused the request (code %d).", code)
}

func (a *App) switchProfile(iccid string, enable bool) {
	if !a.connect(1) {
		return
	}
	if enable {
		a.progress("Enabling the profile…")
	} else {
		a.progress("Disabling the profile…")
	}
	var code int
	var err error
	if enable {
		code, err = a.e.Enable(iccid, true)
	} else {
		code, err = a.e.Disable(iccid, true)
	}
	// With the refresh flag the eUICC restarts the SIM session, which ends our
	// channel either way.
	a.closeCard()
	switch {
	case err != nil:
		a.toast("The eUICC did not answer: %v", err)
		a.refresh(3)
		return
	case code != euicc.ResultOK:
		a.toast("%s", resultMessage(code, enable))
		a.refresh(3)
		return
	}
	if enable {
		a.toast("Profile enabled. The phone reconnects to the network shortly.")
	} else {
		a.toast("Profile disabled.")
	}
	a.progress("Waiting for the SIM to restart…")
	time.Sleep(4 * time.Second)
	a.refresh(15)
}

func (a *App) deleteProfile(iccid string) {
	if !a.connect(1) {
		return
	}
	title := a.title(iccid)
	a.progress("Deleting the profile…")
	code, err := a.e.Delete(iccid)
	switch {
	case err != nil:
		a.fail("Could not delete the profile", err)
	case code != euicc.ResultOK:
		a.toast("%s", resultMessage(code, true))
	default:
		a.toast("“%s” deleted.", title)
		a.event(map[string]any{"type": "deleted", "iccid": iccid})
		if a.refresh(1) {
			a.sendNotifications(false)
		}
		return
	}
	a.refresh(1)
}

func (a *App) setNickname(iccid, name string) {
	if len(name) > 64 {
		a.toast("Nicknames can be at most 64 bytes.")
		return
	}
	if !a.connect(1) {
		return
	}
	code, err := a.e.SetNickname(iccid, name)
	switch {
	case err != nil:
		a.fail("Could not save the nickname", err)
	case code != euicc.ResultOK:
		a.toast("The eUICC rejected the nickname.")
	default:
		a.toast("Nickname saved.")
	}
	a.refresh(1)
}

// sendNotifications sends every pending notification to its server and
// removes the ones that were delivered. Automatic sends stay quiet unless
// something fails; whatever fails stays pending.
func (a *App) sendNotifications(user bool) {
	if !a.connect(1) {
		return
	}
	sent, failed := 0, 0
	for _, n := range a.state.Notifications {
		a.progress(fmt.Sprintf("Sending notification %d…", n.Seq))
		addr, raw, err := a.e.PendingNotification(n.Seq)
		if err == nil {
			err = es9p.HandleNotification(addr, raw)
		}
		if err == nil {
			_, err = a.e.RemoveNotification(n.Seq)
		}
		if err != nil {
			log.Printf("app: notification %d: %v", n.Seq, err)
			failed++
		} else {
			sent++
		}
	}
	if err := a.readNotifications(); err != nil {
		a.fail("Could not read the notifications", err)
		return
	}
	switch {
	case failed > 0 && user:
		a.toast("%d sent, %d could not be sent.", sent, failed)
	case failed > 0:
		a.toast("Some notifications could not be sent yet.")
	case user:
		a.toast("%d sent.", sent)
	}
}

func (a *App) removeNotification(seq string) {
	n, err := strconv.ParseUint(seq, 10, 64)
	if err != nil || !a.connect(1) {
		return
	}
	if _, err := a.e.RemoveNotification(n); err != nil {
		a.fail("Could not remove the notification", err)
		return
	}
	if err := a.readNotifications(); err != nil {
		a.fail("Could not read the notifications", err)
	}
}

func (a *App) startDownload(server, matchingID string) {
	if !a.connect(1) {
		return
	}
	if server == "" {
		addr, err := a.e.Addresses()
		if err != nil {
			a.fail("Could not read the eUICC", err)
			return
		}
		if server = addr.DefaultSMDP; server == "" {
			a.toast("No SM-DP+ address was given and the eUICC has no default.")
			return
		}
	}
	dl := &download{server: server}
	a.progress("Contacting " + server + "…")
	challenge, err := a.e.Challenge()
	if err != nil {
		a.fail("The eUICC did not answer", err)
		return
	}
	info1, err := a.e.Info1()
	if err != nil {
		a.fail("The eUICC did not answer", err)
		return
	}
	auth, err := es9p.InitiateAuthentication(server, challenge, info1)
	if err != nil {
		a.toast("%v", err)
		return
	}
	dl.txid = auth.TransactionID
	a.progress("Authenticating…")
	resp, txid, err := a.e.AuthenticateServer(auth.ServerSigned1, auth.ServerSignature1, auth.EuiccCiPKIDToUse,
		auth.ServerCertificate, matchingID)
	if err != nil {
		a.fail("The eUICC rejected the server", err)
		return
	}
	dl.txidBin = txid
	if dl.auth, err = es9p.AuthenticateClient(server, dl.txid, resp); err != nil {
		a.cancelDownload(dl, euicc.CancelEndUserRejection)
		a.toast("%v", err)
		return
	}
	meta, err := euicc.ParseMetadata(dl.auth.ProfileMetadata)
	if err == nil {
		dl.needsCode, err = euicc.ConfirmationCodeRequired(dl.auth.SMDPSigned2)
	}
	if err != nil {
		a.cancelDownload(dl, euicc.CancelEndUserRejection)
		a.toast("%v", err)
		return
	}
	dl.title = meta.Name
	if dl.title == "" {
		dl.title = meta.Provider
	}
	a.dl = dl
	a.state.Previewing = true
	a.event(map[string]any{"type": "preview", "iccid": meta.ICCID, "provider": meta.Provider, "name": meta.Name,
		"profileClass": meta.Class, "smdp": server, "codeRequired": dl.needsCode})
}

// cancelDownload ends the session on the eUICC and tells the SM-DP+.
// Failures are only logged: the session times out on both sides anyway.
func (a *App) cancelDownload(dl *download, reason uint64) {
	if a.e != nil && dl.txidBin != nil {
		resp, err := a.e.CancelSession(dl.txidBin, reason)
		if err == nil {
			err = es9p.CancelSession(dl.server, dl.txid, resp)
		}
		if err != nil {
			log.Printf("app: cancelling the download: %v", err)
		}
	}
}

func (a *App) finishDownload(accept bool, code string) {
	dl := a.dl
	if dl == nil {
		return
	}
	a.dl = nil
	a.state.Previewing = false
	if !a.connect(1) {
		return
	}
	if !accept {
		a.progress("Cancelling…")
		a.cancelDownload(dl, euicc.CancelEndUserRejection)
		a.toast("Download cancelled.")
		return
	}

	a.progress("Preparing the download…")
	pdr, err := a.e.PrepareDownload(dl.auth.SMDPSigned2, dl.auth.SMDPSignature2, dl.auth.SMDPCertificate, code)
	if err != nil {
		a.cancelDownload(dl, euicc.CancelEndUserRejection)
		a.toast("The eUICC could not prepare the download: %v", err)
		return
	}
	a.progress("Downloading the profile…")
	bpp, err := es9p.GetBoundProfilePackage(dl.server, dl.txid, pdr)
	if err != nil {
		a.cancelDownload(dl, euicc.CancelEndUserRejection)
		a.toast("%v", err)
		return
	}
	a.progress("Installing the profile…")
	if _, err := a.e.LoadBoundProfilePackage(bpp); err != nil {
		var ie euicc.InstallError
		if !errors.As(err, &ie) {
			a.cancelDownload(dl, euicc.CancelEndUserRejection)
		}
		a.toast("%v", err)
		a.refresh(1)
		return
	}
	a.toast("“%s” installed.", dl.title)
	a.event(map[string]any{"type": "installed"})
	if a.refresh(1) {
		a.sendNotifications(false)
	}
}
