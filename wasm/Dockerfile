FROM ubuntu:20.04

ARG WABT_VERSION=1.0.23
ARG BINARYEN_VERSION=version_101

ARG DEBIAN_FRONTEND=noninteractive
RUN apt-get update && apt-get install -y curl git build-essential python

RUN bash -c 'echo -ne "deb http://apt.llvm.org/focal/ llvm-toolchain-focal-12 main\ndeb-src http://apt.llvm.org/focal/ llvm-toolchain-focal-12 main" > /etc/apt/sources.list.d/llvm.list'

RUN curl -L https://apt.llvm.org/llvm-snapshot.gpg.key | apt-key add -

RUN apt-get update && \
    apt-get install -y \
      cmake \
      clang-12 \
      clang-format-12 \
      libc++-12-dev \
      libc++abi-12-dev \
      lld-12 && \
    update-alternatives --install /usr/bin/ld ld /usr/bin/lld-12 90 && \
    update-alternatives --install /usr/bin/cc cc /usr/bin/clang-12 90 && \
    update-alternatives --install /usr/bin/cpp cpp /usr/bin/clang++-12 90 && \
    update-alternatives --install /usr/bin/c++ c++ /usr/bin/clang++-12 90

RUN ln -s /usr/bin/clang-12 /usr/bin/clang && \
    ln -s /usr/bin/clang++-12 /usr/bin/clang++ && \
    ln -s /usr/bin/clang-format-12 /usr/bin/clang-format && \
    ln -s /usr/bin/clang-cpp-12 /usr/bin/clang-cpp

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

ENV CC=clang-12
ENV CXX=clang++-12

WORKDIR /src
