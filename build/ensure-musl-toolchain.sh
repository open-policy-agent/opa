#!/usr/bin/env bash

set -exo pipefail
CC=musl-gcc
PKG=musl-tools

type -f ${CC} 2>/dev/null && exit 0

apt-get update && \
  apt-get install --no-install-recommends -y ${PKG}
