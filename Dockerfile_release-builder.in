FROM golang:GOVERSION

RUN mkdir /src \
    && cd /src \
    && git clone --branch v0.53 https://github.com/gohugoio/hugo.git \
    && cd hugo \
    && go install --tags extended

RUN echo 'deb http://deb.nodesource.com/node_6.x jessie main' > /etc/apt/sources.list.d/nodesource.list
RUN echo 'deb-src http://deb.nodesource.com/node_6.x jessie main' >> /etc/apt/sources.list.d/nodesource.list

RUN apt-get update -y \
    && apt-get install -y -q --force-yes --no-install-recommends \
        nodejs \
        locales \
    && rm -fr /var/lib/apt/lists/* /var/cache/*

RUN echo en_US.UTF-8 UTF-8 > /etc/locale.gen && locale-gen
ENV LANG en_US.UTF-8
ENV LANGUAGE en_US:en
ENV LC_ALL en_US.UTF-8