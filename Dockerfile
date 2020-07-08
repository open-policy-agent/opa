# Copyright 2019 The OPA Authors.  All rights reserved.
# Use of this source code is governed by an Apache2
# license that can be found in the LICENSE file.

ARG BASE

FROM gcr.io/distroless/base as certs

FROM ${BASE}

# Any non-zero number will do, and unfortunately a named user will not, as k8s
# pod securityContext runAsNonRoot can't resolve the user ID:
# https://github.com/kubernetes/kubernetes/issues/40958. Make root (uid 0) when
# not specified.
ARG USER=0

MAINTAINER Torin Sandall <torinsandall@gmail.com>
COPY --from=certs /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt

# Hack.. https://github.com/moby/moby/issues/37965
# _Something_ needs to be between the two COPY steps.
USER ${USER}

ARG BIN_DIR=.
COPY ${BIN_DIR}/opa_linux_amd64 /opa

ENTRYPOINT ["/opa"]
CMD ["run"]
