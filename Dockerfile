# Copyright 2016 The OPA Authors.  All rights reserved.
# Use of this source code is governed by an Apache2
# license that can be found in the LICENSE file.

FROM alpine:latest

MAINTAINER Torin Sandall <torinsandall@gmail.com>

ENV BIN /usr/bin/opa

RUN apk add --update ca-certificates &&\
    rm -rf /var/cache/apk/* &&\
    update-ca-certificates

COPY opa $BIN

EXPOSE 8181

ENTRYPOINT ["opa"]
