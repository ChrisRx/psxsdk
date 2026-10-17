#!/usr/bin/env bash
# Builds libc++ and libc++abi for wasm32-wasi into $OUT/libcxx, with the
# threading API on, so that LLVM's headers, which use std::mutex and friends
# whether or not LLVM itself runs threads, compile. Nothing ever starts a
# thread: the pthread functions wasi-libc leaves out are stubbed when the
# compiler is linked (../glue/wasi-stubs.c). Run by build.sh in the cross
# shell; LLVM_SRC is the LLVM monorepo and OUT the output directory.
set -euxo pipefail
BUILD=$OUT/libcxx-build
PREFIX=$OUT/libcxx
# Keep the wrapper from adding nixpkgs' own libc++ headers and library.
export NIX_CXXSTDLIB_COMPILE_wasm32_unknown_wasip1=" "
export NIX_CXXSTDLIB_LINK_wasm32_unknown_wasip1=" "
mkdir -p "$BUILD"
cd "$BUILD"
cmake -G Ninja "$LLVM_SRC/runtimes" \
  -DCMAKE_BUILD_TYPE=Release \
  -DCMAKE_INSTALL_PREFIX="$PREFIX" \
  -DCMAKE_SYSTEM_NAME=WASI -DCMAKE_SYSTEM_PROCESSOR=wasm32 -DUNIX=1 \
  -DCMAKE_C_COMPILER="$CC" -DCMAKE_CXX_COMPILER="$CXX" \
  -DCMAKE_C_COMPILER_WORKS=TRUE -DCMAKE_CXX_COMPILER_WORKS=TRUE \
  -DCMAKE_AR="$(command -v "$AR")" -DCMAKE_RANLIB="$(command -v "$RANLIB")" \
  -DLLVM_ENABLE_RUNTIMES="libcxx;libcxxabi" \
  -DLIBCXX_CXX_ABI=libcxxabi \
  -DLIBCXX_ENABLE_SHARED=OFF -DLIBCXXABI_ENABLE_SHARED=OFF \
  -DLIBCXX_ENABLE_STATIC_ABI_LIBRARY=OFF \
  -DLIBCXX_STATICALLY_LINK_ABI_IN_STATIC_LIBRARY=ON \
  -DLIBCXX_HAS_MUSL_LIBC=ON \
  -DLIBCXX_USE_COMPILER_RT=ON -DLIBCXXABI_USE_COMPILER_RT=ON \
  -DLIBCXX_ENABLE_THREADS=ON -DLIBCXX_HAS_PTHREAD_API=ON \
  -DLIBCXXABI_ENABLE_THREADS=ON -DLIBCXXABI_HAS_PTHREAD_API=ON \
  -DLIBCXX_ENABLE_FILESYSTEM=OFF \
  -DLIBCXX_ENABLE_EXCEPTIONS=OFF -DLIBCXXABI_ENABLE_EXCEPTIONS=OFF \
  -DLIBCXXABI_USE_LLVM_UNWINDER=OFF \
  -DLIBCXX_INCLUDE_BENCHMARKS=OFF -DLIBCXX_INCLUDE_TESTS=OFF -DLIBCXXABI_INCLUDE_TESTS=OFF \
  -DLIBCXX_ENABLE_ABI_LINKER_SCRIPT=OFF
ninja -j "$JOBS"
ninja install
