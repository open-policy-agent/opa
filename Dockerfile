# Copyright 2019 The OPA Authors.  All rights reserved.
# Use of this source code is governed by an Apache2
# license that can be found in the LICENSE file.
ARG BUILD_COMMIT
# we cant use build-args in `COPY --from=...` below, so work around this
# see: https://medium.com/@tonistiigi/advanced-multi-stage-build-patterns-6f741b852fae
FROM build-${BUILD_COMMIT} AS copy-src

FROM gcr.io/distroless/base${VARIANT}
# make root (uid 0) default when not specified
ARG USER=0
MAINTAINER Torin Sandall <torinsandall@gmail.com>
COPY --from=copy-src /go/src/github.com/open-policy-agent/opa/opa_linux_amd64 /opa

# Any non-zero number will do, and unfortunately a named user will not,
# as k8s pod securityContext runAsNonRoot can't resolve the user ID:
# https://github.com/kubernetes/kubernetes/issues/40958
USER ${USER}

ENTRYPOINT ["/opa"]
CMD ["run"]
