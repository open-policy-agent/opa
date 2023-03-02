# Copyright 2019 The OPA Authors.  All rights reserved.
# Use of this source code is governed by an Apache2
# license that can be found in the LICENSE file.

ARG BASE

FROM ${BASE}

LABEL org.opencontainers.image.authors="Torin Sandall <torinsandall@gmail.com>"
LABEL org.opencontainers.image.source="https://github.com/open-policy-agent/opa"


# Temporarily allow us to identify whether running from within an offical
# Docker image with a "rootless" tag, so that we may print a warning that this image tag
# will not be published after 0.50.0. Remove after 0.50.0 release.
ARG OPA_DOCKER_IMAGE_TAG
ENV OPA_DOCKER_IMAGE_TAG=${OPA_DOCKER_IMAGE_TAG}

# Any non-zero number will do, and unfortunately a named user will not, as k8s
# pod securityContext runAsNonRoot can't resolve the user ID:
# https://github.com/kubernetes/kubernetes/issues/40958.
ARG USER=1000:1000
USER ${USER}

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
