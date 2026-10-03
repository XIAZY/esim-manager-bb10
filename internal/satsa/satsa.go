// Package satsa sends APDUs to the SIM through the modem's SATSA (JSR-177)
// logical-channel service. /radio/lib/libsatsa.so is a thin client for
// qct_rrm, which maps the calls onto QMI UIM logical channels; unlike SAP, the
// SIM stays attached to the modem.
//
// The library ships without a header. The declarations in the C preamble were
// rebuilt from the disassembly of both libsatsa.so and /radio/bin/qct_rrm's
// satsa_* handlers (BB10 10.3.3):
//   - Calls return 0 on success and -1 on a transport or encoding error. The
//     service's own result is in the status outputs (0 = OK).
//   - Outputs are written only on success.
//   - tag is only read on channel 0 (the SIM toolkit channel). Ordinary
//     logical channels ignore it, so it is always 0.
//   - qct_rrm only exchanges on channels SATSA opened, and refuses SELECT and
//     MANAGE CHANNEL on them, so a channel stays on the application it opened.
//
// Each Go call is a single cgo call; exchange and get-data are combined in C.
package satsa

/*
#include <dlfcn.h>
#include <errno.h>
#include <stdint.h>
#include <string.h>
#include <unistd.h>

#define SATSA_LIB "/radio/lib/libsatsa.so"
#define SATSA_NODE "/dev/radio/cellular/uicc"

// Opens /dev/radio/<provider>/uicc; returns the fd or -1.
typedef int (*attach_fn)(const char *provider, char *name_out);
// close(fd).
typedef int (*detach_fn)(int fd);
// MANAGE CHANNEL + SELECT. aid is always read as 16 bytes; aid_len 5..16.
// AIDs starting A000000087 (USIM/ISIM) are refused. fcp 0..2 selects the
// SELECT response type. status: 0 OK, 1 bad argument, 3 bad AID, 4 no channel
// left, 5 UICC not ready, otherwise a QMI error.
typedef int (*open_fn)(int fd, const uint8_t aid[16], uint8_t aid_len, uint8_t tag, uint32_t fcp,
                       uint32_t *status, uint8_t *channel, uint32_t *select_len);
// Sends one C-APDU (at least 4 bytes). Outputs status and the R-APDU length.
typedef int (*exchange_fn)(int fd, const uint8_t *apdu, uint32_t len, uint8_t channel, uint8_t tag,
                           uint32_t *status, uint32_t *resp_len);
// Copies the pending response; *len is the buffer size in, bytes written out.
typedef int (*get_data_fn)(int fd, uint8_t channel, uint8_t tag, uint32_t *len, uint8_t *buf);
// Closes a channel SATSA opened.
typedef int (*close_fn)(int fd, uint8_t channel, uint8_t tag, uint32_t *status);

static attach_fn s_attach;
static detach_fn s_detach;
static open_fn s_open;
static exchange_fn s_exchange;
static get_data_fn s_get_data;
static close_fn s_close;

static int satsa_load(void) {
    void *lib;
    if (s_attach)
        return 0;
    lib = dlopen(SATSA_LIB, RTLD_NOW);
    if (lib == NULL)
        return -1;
    s_detach = (detach_fn)dlsym(lib, "SATSADetachProvider");
    s_open = (open_fn)dlsym(lib, "SATSAOpenConnection");
    s_exchange = (exchange_fn)dlsym(lib, "SATSAExchangeApdu");
    s_get_data = (get_data_fn)dlsym(lib, "SATSAGetExchangeData");
    s_close = (close_fn)dlsym(lib, "SATSACloseChannel");
    attach_fn attach = (attach_fn)dlsym(lib, "SATSAAttachProvider");
    if (!attach || !s_detach || !s_open || !s_exchange || !s_get_data || !s_close) {
        dlclose(lib);
        return -1;
    }
    s_attach = attach;
    return 0;
}

// Returns the fd, or -errno. libsatsa logs after open() fails, which can
// clobber errno, so the reason comes from access().
static int satsa_attach(void) {
    int fd;
    if (satsa_load() != 0)
        return -ENOENT;
    fd = s_attach("cellular", NULL);
    if (fd >= 0)
        return fd;
    return access(SATSA_NODE, R_OK | W_OK) != 0 ? -errno : -EIO;
}

static void satsa_detach(int fd) {
    s_detach(fd);
}

// Returns the channel (1..19), or -1 with *status set (-1: request failed).
static int satsa_open(int fd, const uint8_t *aid, int aid_len, int *status) {
    uint8_t aid16[16] = {0};
    uint32_t st = 0, select_len = 0;
    uint8_t channel = 0;
    if (aid_len < 5 || aid_len > 16) {
        *status = 1;
        return -1;
    }
    memcpy(aid16, aid, aid_len);
    if (s_open(fd, aid16, aid_len, 0, 0, &st, &channel, &select_len) != 0) {
        *status = -1;
        return -1;
    }
    *status = (int)st;
    if (st != 0 || channel == 0 || channel > 19) {
        // Status 1 can come after qct_rrm opened the channel ("Couldn't store
        // data"). Other failures leave channel unassigned, and closing it
        // could close a channel someone else owns.
        if (st == 1 && channel != 0 && channel <= 19)
            s_close(fd, channel, 0, &st);
        return -1;
    }
    return channel;
}

static void satsa_close(int fd, int channel) {
    uint32_t st;
    s_close(fd, (uint8_t)channel, 0, &st);
}

// One APDU round trip. Returns the R-APDU length (data + SW1 SW2), or -1.
static int satsa_transmit(int fd, int channel, const uint8_t *apdu, int len, uint8_t *out, int cap) {
    uint32_t st = 0, n = 0;
    if (s_exchange(fd, apdu, (uint32_t)len, (uint8_t)channel, 0, &st, &n) != 0 || st != 0)
        return -1;
    if (n < 2 || n > (uint32_t)cap)
        return -1;
    if (s_get_data(fd, (uint8_t)channel, 0, &n, out) != 0 || n < 2)
        return -1;
    return (int)n;
}
*/
import "C"

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"unsafe"
)

// ConnError explains why Open failed. ConnState is one of noAccess,
// notReady and noApplet.
type ConnError struct {
	State string
	Msg   string
}

func (e *ConnError) Error() string     { return e.Msg }
func (e *ConnError) ConnState() string { return e.State }

// maxResponse bounds one R-APDU. qct_rrm caps responses at 65537 bytes.
const maxResponse = 65537

// Channel is an open logical channel to one application on the SIM.
//
// qct_rrm does not free a channel when the process that opened it exits, and
// a card has only a few (often 3 besides the basic channel), so every Channel
// must be closed, including on exit.
type Channel struct {
	fd      C.int
	channel int
	buf     []byte // response buffer, reused
}

// Open opens a logical channel to the application with the given AID.
func Open(aid []byte) (*Channel, error) {
	if len(aid) < 5 || len(aid) > 16 {
		return nil, fmt.Errorf("satsa: bad AID length %d", len(aid))
	}
	fd := C.satsa_attach()
	if fd == -C.EACCES || fd == -C.EPERM {
		// The access grant is lost on every reboot; restore it if we can.
		if restoreAccess() == nil {
			fd = C.satsa_attach()
		}
	}
	if fd < 0 {
		if fd == -C.EACCES || fd == -C.EPERM {
			return nil, &ConnError{"noAccess", "no access to the SIM interface"}
		}
		return nil, fmt.Errorf("opening the SIM interface: %w", syscall.Errno(-fd))
	}
	var status C.int
	ch := C.satsa_open(fd, (*C.uint8_t)(unsafe.Pointer(&aid[0])), C.int(len(aid)), &status)
	if ch < 0 {
		C.satsa_detach(fd)
		switch status {
		case 4:
			return nil, &ConnError{"noChannel", "all of the SIM's logical channels are in use"}
		case 5:
			return nil, &ConnError{"notReady", "the SIM is not ready"}
		case -1:
			return nil, errors.New("the SIM interface did not answer")
		default:
			return nil, &ConnError{"noApplet", fmt.Sprintf("the application is not on the SIM (status %d)", int(status))}
		}
	}
	return &Channel{fd: fd, channel: int(ch), buf: make([]byte, maxResponse)}, nil
}

// Transmit sends one command APDU and returns the response including SW1 SW2.
// The channel number is encoded into CLA here (ISO 7816-4 5.4.1).
func (c *Channel) Transmit(apdu []byte) ([]byte, error) {
	if len(apdu) < 4 || len(apdu) > 5+255+1 {
		return nil, fmt.Errorf("satsa: bad APDU length %d", len(apdu))
	}
	cmd := append([]byte(nil), apdu...)
	if c.channel < 4 {
		cmd[0] = cmd[0]&0xF0 | byte(c.channel)
	} else {
		cmd[0] = cmd[0]&0x80 | 0x40 | byte(c.channel-4)
	}
	n := C.satsa_transmit(c.fd, C.int(c.channel), (*C.uint8_t)(unsafe.Pointer(&cmd[0])), C.int(len(cmd)),
		(*C.uint8_t)(unsafe.Pointer(&c.buf[0])), C.int(len(c.buf)))
	if n < 0 {
		return nil, errors.New("satsa: exchange failed")
	}
	return append([]byte(nil), c.buf[:n]...), nil
}

// Close closes the channel and the connection to the SIM interface.
func (c *Channel) Close() {
	if c.fd < 0 {
		return
	}
	C.satsa_close(c.fd, C.int(c.channel))
	C.satsa_detach(c.fd)
	c.fd = -1
}

// rrmShell is bb10mt's helper: a shell running as rrm, the owner of the SIM
// interface node, reading commands from stdin. It ignores arguments.
const rrmShell = "/base/bin/__rrm"

// restoreAccess grants our own group rw on the node through rrmShell.
func restoreAccess() error { return setfacl("-m", ":rw") }

// RevokeAccess removes the grant again. Call it on exit, so the grant exists
// only while the app runs: BB10 has no uninstall hook, but the installer
// closes a running app before removing it, and the group number can later go
// to another app.
func RevokeAccess() error { return setfacl("-x", "") }

// setfacl runs "setfacl <op> g:<our gid><perm> <node>" through rrmShell. The
// command is fixed apart from our numeric gid, so nothing else reaches that
// shell.
func setfacl(op, perm string) error {
	if _, err := os.Stat(rrmShell); err != nil {
		return err
	}
	cmd := exec.Command(rrmShell)
	cmd.Stdin = strings.NewReader(fmt.Sprintf("setfacl %s g:%d%s /dev/radio/cellular/uicc\n", op, os.Getgid(), perm))
	return cmd.Run()
}
