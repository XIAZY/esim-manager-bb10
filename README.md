# eSIM Manager

An eSIM profile manager for BlackBerry 10 with a removable eUICC card, written in
Go with a Cascades UI. The SGP.22 logic is a Go port of the parts of
[lpac](https://github.com/estkme-group/lpac)'s libeuicc it needs. It talks to
the card through the modem's SATSA logical-channel service
(`/radio/lib/libsatsa.so`), so the SIM stays connected to the network while it
works.

It can:

- list profiles and show their details
- enable and disable profiles (the SIM restarts, and the app reconnects)
- rename and delete profiles
- download profiles from an activation code (typed or scanned from the QR code)
  or an SM-DP+ address, after showing the profile's name and operator
- send or discard pending notifications; they are sent automatically after
  downloads and deletes, and on refresh
- show the eUICC's EID, versions, free memory and certificates

## Layout

All logic is Go. C is limited to two thin cgo layers:

- `internal/satsa`: the APDU transport. libsatsa has no header; its
  declarations were rebuilt from the disassembly of both the client and
  qct_rrm's `satsa_*` handlers. One cgo call per APDU.
- `internal/ui`: the Cascades bridge, one C++ `QObject`. Go posts JSON state
  snapshots and events to it from any goroutine (queued to the UI thread), and
  QML sends commands back as a name and JSON arguments.

The rest:

- `internal/tlv`: SGP.22 BER-TLV.
- `internal/euicc`: ES10a/b/c on the ISD-R: chip info, profiles, profile
  operations, notifications, and the download steps (AuthenticateServer,
  PrepareDownload, LoadBoundProfilePackage in its SGP.22 segments).
- `internal/es9p`: ES9+ to the SM-DP+ over HTTPS, with lpac's error messages.
- `internal/app`: one goroutine owns the eUICC and runs the UI's commands in
  order; it publishes the state the pages render.
- `main.go`: keeps the main goroutine on the process's main thread for the UI
  and starts the app goroutine.
- `assets/*.qml`: the pages. `main.qml` holds the state and the actions.

## Device setup (root)

libsatsa opens `/dev/radio/cellular/uicc`, which only `rrm`, group `radio` and
user `nfc` may use, so eSIM Manager's group needs an ACL entry. The entry is lost on
every reboot.

**On a phone rooted with bb10mt, eSIM Manager adds it itself.** When the open fails
with "permission denied", it pipes one fixed command into bb10mt's
`/base/bin/__rrm` (a shell running as `rrm`, the node's owner):
`setfacl -m g:<its numeric gid>:rw /dev/radio/cellular/uicc`, then retries once.

On a phone without bb10mt's helper, run that command as root after each boot;
the app's group is in `ls -ld /apps/io.github.xiazy.esimmanager.*`.

The app also asks for the camera, for scanning activation codes. Everything else
works without it.

## Security notes

- The grant gives the app's group the whole radio `uicc` service, not just the
  eUICC's profile manager: the same interface can also verify or change SIM PINs. eSIM Manager
  only opens a logical channel to the ISD-R (qct_rrm refuses the USIM/ISIM
  AIDs, and SELECT and MANAGE CHANNEL on SATSA channels) and calls nothing
  else, which is why the app is kept small.
- bb10mt's helpers (`/base/bin/__root`, `__rrm`, … and `g_*`) are setuid and
  executable by everyone, and give the shell they start every QNX ability. Any
  process on such a phone, any app included, can become root this way. eSIM Manager
  only uses `__rrm` for the one `setfacl` above, but the exposure exists
  whether or not eSIM Manager is installed.
- Like lpac, the HTTPS client does not verify the SM-DP+ certificate (SM-DP+
  certificates chain to the GSMA CI, not a web root). Someone on the network
  path could read the activation code, but cannot install a profile: the eUICC
  checks the server's GSMA certificate chain and the profile package's
  signature itself. Responses are capped at 4 MB.
- Nothing is logged except errors. Activation and confirmation codes are not
  stored.

## Build and install

Needs the builder image from `../blackberry10-toolchain` (for `bb10-cc`, moc and
the BB10 SDK) and the qnx/arm Go toolchain from `~/go-qnx` (built once with
`cd ~/go-qnx/src && ./make.bash`; set `GOQNX` for another path).

```sh
tools/build.sh                 # -> build/io.github.xiazy.esimmanager.bar
tools/install.sh root@<phone>  # phones rooted with bb10mt
```

`go test ./internal/tlv ./internal/euicc ./internal/es9p` runs the tests on the
host (the cgo packages only build for the phone). They use dummy identifiers.

## License

LGPL-2.1-only, see `LICENSE`. `internal/euicc`, `internal/es9p` and
`internal/tlv` are translated from lpac's libeuicc (Copyright 2023-2025 ESTKME
TECHNOLOGY LIMITED, Hong Kong), which is LGPL-2.1-only; see
`THIRD_PARTY_NOTICES.md`, which also covers the Go runtime in the binaries.
Licensing per file is declared in `REUSE.toml`.

Binaries link the LGPL code statically. The complete source and the build
scripts here let anyone rebuild the app with a modified version of it.
