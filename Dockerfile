# Copyright 2019 The OPA Authors.  All rights reserved.
# Use of this source code is governed by an Apache2
# license that can be found in the LICENSE file.

ARG BASE

FROM ${BASE}

# Any non-zero number will do, and unfortunately a named user will not, as k8s
# pod securityContext runAsNonRoot can't resolve the user ID:
# https://github.com/kubernetes/kubernetes/issues/40958. Make root (uid 0) when
# not specified.
ARG USER=0

MAINTAINER Torin Sandall <torinsandall@gmail.com>

# Hack.. https://github.com/moby/moby/issues/37965
# _Something_ needs to be between the two COPY steps.
USER ${USER}

ARG BIN_DIR=.
COPY ${BIN_DIR}/opa_docker_amd64 /opa
COPY ./vendor/github.com/wasmerio/go-ext-wasm/wasmer/libwasmer.so /libwasmer.so

ENTRYPOINT ["/opa"]
CMD ["run"]
