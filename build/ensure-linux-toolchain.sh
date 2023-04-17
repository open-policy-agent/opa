#!/usr/bin/env bash
set -eo pipefail

case "$(uname -m | tr '[:upper:]' '[:lower:]')" in
  amd64 | x86_64 | x64)
    HOST_ARCH=amd64
    ;;
  arm64 | aarch64)
    HOST_ARCH=arm64
    ;;
  *)
    echo "Error: Host architecture not supported." >&2
    exit 1
    ;;
esac

# Native build
if [ "${GOARCH}" = "${HOST_ARCH}" ]; then
  if ! [ -x "$(command -v gcc)" ]; then
    echo "Error: gcc not found." >&2
    exit 1
  fi
  exit 0
fi

# Cross-compile
case "${GOARCH}" in
  amd64)
    PKG=gcc-x86-64-linux-gnu
    CC=x86_64-linux-gnu-gcc
    ;;
  arm64)
    PKG=gcc-aarch64-linux-gnu
    CC=aarch64-linux-gnu-gcc
    ;;
  *)
    echo "Error: Target architecture ${GOARCH} not supported." >&2
    exit 1
    ;;
esac

type -f ${CC} 2>/dev/null && exit 0

if ! [ -x "$(command -v apt-get)" ]; then
  echo "Error: apt-get not found. Could not install missing toolchain." >&2
  exit 1
fi

apt-get update >/dev/null && \
  apt-get install -y ${PKG} >/dev/null

echo ${CC}
