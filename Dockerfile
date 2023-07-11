# Copyright 2019 The OPA Authors.  All rights reserved.
# Use of this source code is governed by an Apache2
# license that can be found in the LICENSE file.

ARG BASE

FROM ${BASE}

LABEL org.opencontainers.image.authors="Torin Sandall <torinsandall@gmail.com>"
LABEL org.opencontainers.image.source="https://github.com/open-policy-agent/opa"

# Note: this is replicated from our base images for clarity. The following all
# use 65532:65532
# * cgr.dev/chainguard/busybox
# * cgr.dev/chainguard/glibc-dynamic
# * cgr.dev/chainguard/static
USER 65532:65532

# TARGETOS and TARGETARCH are automatic platform args injected by BuildKit
# https://docs.docker.com/engine/reference/builder/#automatic-platform-args-in-the-global-scope
ARG TARGETOS
ARG TARGETARCH
ARG BIN_DIR=.
ARG BIN_SUFFIX=
COPY ${BIN_DIR}/opa_${TARGETOS}_${TARGETARCH}${BIN_SUFFIX} /opa
ENV PATH=${PATH}:/

ENTRYPOINT ["/opa"]
CMD ["run"]
