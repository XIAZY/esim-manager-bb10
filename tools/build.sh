#!/bin/sh
# Development build: runs tools/build-inside.sh in the BB10 builder image with
# Go with the QNX port. Both images are the ones the Dockerfile pins by
# digest. Go is copied out of the Go team's image once, to
# build/go-qnx/<digest>/go, and reused; GOQNX names a go-qnx checkout of
# one's own instead, and BB10_BUILDER_IMAGE another builder image.
# To build from source with Docker alone, use the Dockerfile (see README).
set -eu

repo=$(cd "$(dirname "$0")/.." && pwd)
pinned() { sed -n "$1" "$repo/Dockerfile"; }
goimage=$(pinned 's/^FROM \(ghcr\.io\/xiazy\/go-qnx[^ ]*\) AS go-qnx$/\1/p')
image=${BB10_BUILDER_IMAGE:-$(pinned 's/^ARG TOOLCHAIN_IMAGE=//p')}

if [ -z "${GOQNX:-}" ]; then
    dir=$repo/build/go-qnx/${goimage##*@sha256:}
    if [ ! -x "$dir/go/bin/go" ]; then
        rm -rf "$dir"
        mkdir -p "$dir"
        container=$(docker create "$goimage")
        docker cp "$container:/usr/local/go" "$dir/go"
        docker rm "$container" > /dev/null
    fi
    GOQNX=$dir/go
fi
goqnx=$GOQNX
[ -x "$goqnx/bin/go" ] || { echo "error: no Go toolchain in $goqnx" >&2; exit 1; }
mkdir -p "$repo/build"

exec docker run --rm --user "$(id -u):$(id -g)" \
    -e HOME=/tmp/home -e GOCACHE=/src/build/go-cache -e GOMODCACHE=/src/build/go-mod \
    -e PATH=/opt/go-qnx/bin:/opt/bb10-toolchain/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin \
    -v "$goqnx:/opt/go-qnx:ro" -v "$repo:/src" -w /src "$image" tools/build-inside.sh
