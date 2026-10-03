#!/bin/sh
# Builds build/io.github.xiazy.esimmanager.bar. Runs inside the BB10 builder
# image (blackberry10-toolchain) with the qnx/arm Go toolchain (go-qnx) on
# PATH; tools/build.sh and the Dockerfile both call it.
set -eu
cd "$(dirname "$0")/.."

export GOOS=qnx GOARCH=arm GOARM=7
export CGO_ENABLED=1 CC=bb10-cc CXX=bb10-c++

mkdir -p build
/opt/bb10-qt4/bin/moc -nw -o internal/ui/moc_bridge.cpp internal/ui/bridge.h
# go build skips linking when the old binary looks up to date, even if the
# external linker (bb10-cc) changed, so always relink.
rm -f build/esimmanager
# -s -w: no symbol table or DWARF; nethttpomithttp2: SM-DP+ servers speak
# HTTP/1.1, so leave HTTP/2 out of net/http.
go build -trimpath -tags nethttpomithttp2 -ldflags="-s -w" -o build/esimmanager .
blackberry-nativepackager -devMode -package build/io.github.xiazy.esimmanager.bar \
    bar-descriptor.xml -configuration Device-Debug 2>&1 | grep -v -e "^WARNING" -e JAVA_TOOL || true
test -f build/io.github.xiazy.esimmanager.bar
echo "packaged: build/io.github.xiazy.esimmanager.bar"
