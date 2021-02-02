FROM ubuntu:20.04

ARG WABT_VERSION=1.0.20
ARG BINARYEN_VERSION=version_99

RUN apt-get update && apt-get install -y curl git build-essential python

RUN bash -c 'echo -ne "deb http://apt.llvm.org/focal/ llvm-toolchain-focal-11 main\ndeb-src http://apt.llvm.org/focal/ llvm-toolchain-focal-11 main" > /etc/apt/sources.list.d/llvm.list'

RUN curl -L https://apt.llvm.org/llvm-snapshot.gpg.key | apt-key add -

ARG DEBIAN_FRONTEND=noninteractive
RUN apt-get update && \
    apt-get install -y \
      cmake \
      clang-11 \
      libc++-11-dev \
      libc++abi-11-dev \
      lld-11 && \
    update-alternatives --install /usr/bin/ld ld /usr/bin/lld-11 90 && \
    update-alternatives --install /usr/bin/cc cc /usr/bin/clang-11 90 && \
    update-alternatives --install /usr/bin/cpp cpp /usr/bin/clang++-11 90 && \
    update-alternatives --install /usr/bin/c++ c++ /usr/bin/clang++-11 90

RUN ln -s /usr/bin/clang-11 /usr/bin/clang && \
    ln -s /usr/bin/clang++-11 /usr/bin/clang++ && \
    ln -s /usr/bin/clang-cpp-11 /usr/bin/clang-cpp

RUN git clone https://github.com/WebAssembly/wabt && \
    cd wabt && \
    git checkout $WABT_VERSION && \
    git submodule update --init && \
    make

RUN git clone https://github.com/WebAssembly/binaryen && \
    cd binaryen && \
    git checkout $BINARYEN_VERSION && \
    cmake . && \
    make

ENV PATH="/binaryen/bin:/wabt/out/clang/Debug:${PATH}"

ENV CC=clang-11
ENV CXX=clang++-11

WORKDIR /src
