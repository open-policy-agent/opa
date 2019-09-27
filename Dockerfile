# Copyright 2019 The OPA Authors.  All rights reserved.
# Use of this source code is governed by an Apache2
# license that can be found in the LICENSE file.

ARG VARIANT

# Any non-zero number will do, and unfortunately a named user will not, as k8s
# pod securityContext runAsNonRoot can't resolve the user ID:
# https://github.com/kubernetes/kubernetes/issues/40958. Make root (uid 0) when
# not specified.
ARG USER=0

FROM gcr.io/distroless/base${VARIANT}

MAINTAINER Torin Sandall <torinsandall@gmail.com>
COPY opa_linux_amd64 /opa
USER ${USER}
ENTRYPOINT ["/opa"]
CMD ["run"]
