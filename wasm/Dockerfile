FROM ubuntu:20.04

RUN apt-get update && apt-get install -y curl git build-essential python

RUN bash -c 'echo -ne "deb http://apt.llvm.org/focal/ llvm-toolchain-focal-11 main\ndeb-src http://apt.llvm.org/focal/ llvm-toolchain-focal-11 main" > /etc/apt/sources.list.d/llvm.list'

RUN curl -L https://apt.llvm.org/llvm-snapshot.gpg.key | apt-key add -

ARG DEBIAN_FRONTEND=noninteractive
RUN apt-get update && \
    apt-get install -y \
    cmake \
    clang-11 \
    libc++-11-dev libc++abi-11-dev \
    lld-11

ENV CC=clang-11
ENV CXX=clang-11
ENV LLD=wasm-ld-11
ENV AR=llvm-ar-11
ENV RANLIB=llvm-ranlib-11

RUN ln -s /usr/bin/clang-11 /usr/bin/clang && \
    ln -s /usr/bin/clang++-11 /usr/bin/clang++ && \
    ln -s /usr/bin/clang-cpp-11 /usr/bin/clang-cpp

RUN git clone https://github.com/WebAssembly/wabt && \
    cd wabt && \
    git checkout 1.0.17 && \
    git submodule update --init && \
    make

ENV PATH="/wabt/out/clang/Debug:${PATH}"

WORKDIR /src