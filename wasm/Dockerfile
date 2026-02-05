FROM ubuntu:24.04@sha256:80dd3c3b9c6cecb9f1667e9290b3bc61b78c2678c02cbdae5f0fea92cc6734ab

ARG WABT_VERSION=1.0.39
ARG BINARYEN_VERSION=version_125

ARG DEBIAN_FRONTEND=noninteractive
RUN apt-get update && apt-get install -y curl git build-essential python3 python-is-python3

RUN bash -c 'echo -ne "deb http://apt.llvm.org/noble/ llvm-toolchain-noble-21 main\ndeb-src http://apt.llvm.org/noble/ llvm-toolchain-noble-21 main" > /etc/apt/sources.list.d/llvm.list'

RUN curl -L https://apt.llvm.org/llvm-snapshot.gpg.key | apt-key add -

RUN apt-get update && \
    apt-get install -y \
      cmake \
      ninja-build \
      clang-21 \
      clang-format-21 \
      libc++-21-dev \
      libc++abi-21-dev \
      lld-21 && \
    update-alternatives --install /usr/bin/ld ld /usr/bin/lld-21 90 && \
    update-alternatives --install /usr/bin/cc cc /usr/bin/clang-21 90 && \
    update-alternatives --install /usr/bin/cpp cpp /usr/bin/clang++-21 90 && \
    update-alternatives --install /usr/bin/c++ c++ /usr/bin/clang++-21 90

RUN ln -s /usr/bin/clang-21 /usr/bin/clang && \
    ln -s /usr/bin/clang++-21 /usr/bin/clang++ && \
    ln -s /usr/bin/clang-format-21 /usr/bin/clang-format && \
    ln -s /usr/bin/wasm-ld-21 /usr/bin/wasm-ld && \
    ln -s /usr/bin/clang-cpp-21 /usr/bin/clang-cpp

RUN git clone https://github.com/WebAssembly/wabt && \
    cd wabt && \
    git checkout $WABT_VERSION && \
    git submodule update --init && \
    make

RUN git clone https://github.com/WebAssembly/binaryen && \
    cd binaryen && \
    git checkout $BINARYEN_VERSION && \
    git submodule update --init && \
    cmake . -DBUILD_TESTS=OFF && \
    make wasm-opt wasm-dis wasm-as

ENV PATH="/binaryen/bin:/wabt/out/clang/Debug:${PATH}"

ENV CC=clang-21
ENV CXX=clang++-21

WORKDIR /src
