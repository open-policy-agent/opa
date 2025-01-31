#!/usr/bin/env bash

set -exo pipefail
CC=x86_64-w64-mingw32-gcc
PKG=gcc-mingw-w64-x86-64

type -f ${CC} 2>/dev/null && exit 0
echo "deb https://deb.debian.org/debian experimental main" >> /etc/apt/sources.list

apt-get update && \
  apt-get install --no-install-recommends -y ${PKG} && \
  apt-get install --no-install-recommends -y binutils-mingw-w64-x86-64
