#!/usr/bin/env bash

set -eux

export GOPATH="$(pwd)/.gobuild"
SRCDIR="${GOPATH}/src/github.com/szabado/mssh"

[[ -d "${GOPATH}" ]] && rm -rf ${GOPATH}
tar --exclude="archive.tar.gz" -czf archive.tar.gz .

mkdir -p ${GOPATH}/{src,pkg,bin}
mkdir -p ${SRCDIR}
cp archive.tar.gz  ${SRCDIR}
(
	cd ${SRCDIR}
	tar -xzf archive.tar.gz --strip-components 1
	export GO111MODULE=on
    go mod vendor
	go install .
)
