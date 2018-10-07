FROM ubuntu:18.04

RUN apt-get update && apt-get install -y curl git build-essential

RUN bash -c 'echo -ne "deb http://apt.llvm.org/bionic/ llvm-toolchain-bionic main\ndeb-src http://apt.llvm.org/bionic/ llvm-toolchain-bionic main" > /etc/apt/sources.list.d/llvm.list'

RUN curl -L https://apt.llvm.org/llvm-snapshot.gpg.key | apt-key add -

RUN apt-get update && \
    apt-get install -y \
    cmake \
    clang-8 \
    lld-8

ENV CC=clang-8
ENV CXX=clang-8
ENV LLD=wasm-ld-8
ENV AR=llvm-ar-8
ENV RANLIB=llvm-ranlib-8

RUN ln -s /usr/bin/clang-8 /usr/bin/clang && \
    ln -s /usr/bin/clang++-8 /usr/bin/clang++ && \
    ln -s /usr/bin/clang-cpp-8 /usr/bin/clang-cpp

RUN git clone https://github.com/WebAssembly/wabt && \
    cd wabt && \
    git checkout 1.0.5 && \
    git submodule update --init && \
    make

ENV PATH="/wabt/out/clang/Debug:${PATH}"

WORKDIR /src