# Copyright 2018 The OPA Authors.  All rights reserved.
# Use of this source code is governed by an Apache2
# license that can be found in the LICENSE file.

FROM gcr.io/distroless/base:debug

MAINTAINER Torin Sandall <torinsandall@gmail.com>

ADD opa_linux_GOARCH /opa

ENTRYPOINT ["/opa"]

CMD ["run"]
