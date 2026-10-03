# syntax=docker/dockerfile:1.7
#
# Builds eSIM Manager from source:
#
#   docker build --output build .     # -> build/io.github.xiazy.esimmanager.bar
#
# The BB10 builder image comes from https://github.com/XIAZY/blackberry10-toolchain
# (set TOOLCHAIN_IMAGE to use a locally built one); the qnx/arm Go toolchain is
# built from https://github.com/XIAZY/go-qnx at a pinned commit.

ARG TOOLCHAIN_IMAGE=ghcr.io/xiazy/blackberry10-toolchain:latest
ARG GO_QNX_COMMIT=83c87d631cd303e993352727ede4a135475597e7

# go-qnx: Go with the QNX port, bootstrapped with upstream Go.
FROM golang:1.27-bookworm AS go-qnx
ARG GO_QNX_COMMIT
RUN git init -q /go-qnx && cd /go-qnx && \
    git fetch -q --depth 1 https://github.com/XIAZY/go-qnx.git "$GO_QNX_COMMIT" && \
    git checkout -q FETCH_HEAD && \
    cd src && GOROOT_BOOTSTRAP=/usr/local/go ./make.bash && \
    rm -rf /go-qnx/.git /go-qnx/pkg/obj /go-qnx/test

FROM ${TOOLCHAIN_IMAGE} AS build
COPY --from=go-qnx /go-qnx /opt/go-qnx
ENV PATH=/opt/go-qnx/bin:$PATH \
    HOME=/tmp/home \
    GOCACHE=/tmp/go-cache
WORKDIR /src
COPY . .
RUN tools/build-inside.sh

FROM scratch
COPY --from=build /src/build/io.github.xiazy.esimmanager.bar /
