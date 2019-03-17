# Copyright 2016 The OPA Authors.  All rights reserved.
# Use of this source code is governed by an Apache2
# license that can be found in the LICENSE file.

FROM gcr.io/distroless/base

MAINTAINER Torin Sandall <torinsandall@gmail.com>

ADD opa_linux_GOARCH /opa

# Any non-zero number will do, and unfortunately a named user will not,
# as k8s pod securityContext runAsNonRoot can't resolve the user ID:
# https://github.com/kubernetes/kubernetes/issues/40958
USER 1

ENTRYPOINT ["/opa"]

CMD ["run"]
