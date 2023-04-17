FROM ubuntu:20.04

ARG WABT_VERSION=1.0.24
ARG BINARYEN_VERSION=version_102

ARG DEBIAN_FRONTEND=noninteractive
RUN apt-get update && apt-get install -y curl git build-essential python

RUN bash -c 'echo -ne "deb http://apt.llvm.org/focal/ llvm-toolchain-focal-13 main\ndeb-src http://apt.llvm.org/focal/ llvm-toolchain-focal-13 main" > /etc/apt/sources.list.d/llvm.list'

RUN curl -L https://apt.llvm.org/llvm-snapshot.gpg.key | apt-key add -

RUN apt-get update && \
    apt-get install -y \
      cmake \
      ninja-build \
      clang-13 \
      clang-format-13 \
      libc++-13-dev \
      libc++abi-13-dev \
      lld-13 && \
    update-alternatives --install /usr/bin/ld ld /usr/bin/lld-13 90 && \
    update-alternatives --install /usr/bin/cc cc /usr/bin/clang-13 90 && \
    update-alternatives --install /usr/bin/cpp cpp /usr/bin/clang++-13 90 && \
    update-alternatives --install /usr/bin/c++ c++ /usr/bin/clang++-13 90

RUN ln -s /usr/bin/clang-13 /usr/bin/clang && \
    ln -s /usr/bin/clang++-13 /usr/bin/clang++ && \
    ln -s /usr/bin/clang-format-13 /usr/bin/clang-format && \
    ln -s /usr/bin/wasm-ld-13 /usr/bin/wasm-ld && \
    ln -s /usr/bin/clang-cpp-13 /usr/bin/clang-cpp

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

ENV CC=clang-13
ENV CXX=clang++-13

WORKDIR /src
