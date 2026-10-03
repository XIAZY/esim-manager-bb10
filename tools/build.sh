#!/bin/sh
# Builds build/io.github.xiazy.esimmanager.bar in the BB10 builder image
# (../blackberry10-toolchain), with the qnx/arm Go toolchain from $GOQNX
# (default ~/go-qnx, built with src/make.bash).
set -eu

repo=$(cd "$(dirname "$0")/.." && pwd)
goqnx=${GOQNX:-$HOME/go-qnx}
image=${BB10_BUILDER_IMAGE:-bb10-builder:latest}
[ -x "$goqnx/bin/go" ] || { echo "error: no Go toolchain in $goqnx (run src/make.bash there)" >&2; exit 1; }
mkdir -p "$repo/build"

exec docker run --rm --user "$(id -u):$(id -g)" \
    -e HOME=/tmp/home -e GOCACHE=/src/build/go-cache -e GOMODCACHE=/src/build/go-mod \
    -v "$goqnx:/opt/go-qnx:ro" -v "$repo:/src" -w /src "$image" sh -ec '
        export PATH=/opt/go-qnx/bin:$PATH GOOS=qnx GOARCH=arm GOARM=7
        export CGO_ENABLED=1 CC=bb10-cc CXX=bb10-c++
        /opt/bb10-qt4/bin/moc -nw -o internal/ui/moc_bridge.cpp internal/ui/bridge.h
        # -s -w: no symbol table or DWARF; nethttpomithttp2: SM-DP+ servers
        # speak HTTP/1.1, so leave HTTP/2 out of net/http.
        # go build skips linking when the old binary looks up to date, even
        # if the external linker (bb10-cc) changed, so always relink.
        rm -f build/esimmanager
        go build -trimpath -tags nethttpomithttp2 -ldflags="-s -w" -o build/esimmanager .
        blackberry-nativepackager -devMode -package build/io.github.xiazy.esimmanager.bar \
            bar-descriptor.xml -configuration Device-Debug 2>&1 | grep -v -e "^WARNING" -e JAVA_TOOL
        echo "packaged: build/io.github.xiazy.esimmanager.bar"'
