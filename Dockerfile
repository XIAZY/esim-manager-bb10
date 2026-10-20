# syntax=docker/dockerfile:1.7
#
# Builds eSIM Manager from source:
#
#   docker build --output build .     # -> build/io.github.xiazy.esimmanager.bar
#
# The BB10 builder image comes from https://github.com/XIAZY/blackberry10-toolchain
# (set TOOLCHAIN_IMAGE to use a locally built one); Go with the QNX port comes
# from the Go team's image, https://github.com/XIAZY/go-qnx (Go in
# /usr/local/go, as in the official golang image). Both are pinned by digest,
# each the multi-arch index, so the same build runs on amd64 and arm64.

ARG TOOLCHAIN_IMAGE=ghcr.io/xiazy/blackberry10-toolchain@sha256:60ce49510a37ed92368cca010bbaabf97b12d209b9fab179e8491b266bb2578f

# go-qnx 1.27.2: upstream Go 1.27.2 with the QNX port.
FROM ghcr.io/xiazy/go-qnx:1.27.2@sha256:1915ce0568acda8de04c82be016a47fa267858aef4b7c7b77402363770c04669 AS go-qnx

FROM ${TOOLCHAIN_IMAGE} AS build
COPY --from=go-qnx /usr/local/go /opt/go-qnx
ENV PATH=/opt/go-qnx/bin:$PATH \
    HOME=/tmp/home \
    GOCACHE=/tmp/go-cache
WORKDIR /src
COPY . .
RUN tools/build-inside.sh

FROM scratch
COPY --from=build /src/build/io.github.xiazy.esimmanager.bar /
