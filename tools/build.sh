#!/bin/sh
# Development build: runs tools/build-inside.sh in the BB10 builder image with
# a local go-qnx checkout (default ~/go-qnx, built with src/make.bash).
# To build from source without either, use the Dockerfile (see README).
set -eu

repo=$(cd "$(dirname "$0")/.." && pwd)
goqnx=${GOQNX:-$HOME/go-qnx}
image=${BB10_BUILDER_IMAGE:-ghcr.io/xiazy/blackberry10-toolchain:latest}
[ -x "$goqnx/bin/go" ] || { echo "error: no Go toolchain in $goqnx (run src/make.bash there)" >&2; exit 1; }
mkdir -p "$repo/build"

exec docker run --rm --user "$(id -u):$(id -g)" \
    -e HOME=/tmp/home -e GOCACHE=/src/build/go-cache -e GOMODCACHE=/src/build/go-mod \
    -e PATH=/opt/go-qnx/bin:/opt/bb10-toolchain/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin \
    -v "$goqnx:/opt/go-qnx:ro" -v "$repo:/src" -w /src "$image" tools/build-inside.sh
