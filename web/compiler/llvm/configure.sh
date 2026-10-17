#!/usr/bin/env bash
# Configures LLVM, clang and lld for wasm32-wasi in $OUT/llvm-build, as
# static libraries, with the flags TinyGo's own make/llvm.mk uses, trimmed to
# what building for the PlayStation needs: the Mips backend, no threads, no
# tools. Clang goes only into the full build of the compiler that fills the
# cache; the shipped one leaves it out. The tablegens are named outright so
# that the minimal one, which LLVM otherwise builds for itself, is the same
# native llvm-tblgen: named through LLVM_NATIVE_TOOL_DIR alone it is not
# known yet when the minimal one is set up. Run by build.sh in the cross shell, with SRC the patched source
# overlay, TBLGEN the directory of the native llvm-tblgen and clang-tblgen,
# and HERE this directory's parent.
set -euxo pipefail
BUILD=$OUT/llvm-build
# Use the libc++ built by libcxx.sh rather than the one the wrapper would add.
export NIX_CXXSTDLIB_COMPILE_wasm32_unknown_wasip1=" "
export NIX_CXXSTDLIB_LINK_wasm32_unknown_wasip1=" "
CXXSTD="-nostdinc++ -isystem $OUT/libcxx/include/c++/v1"
# Declarations WASI lacks, and endian.h, which LLVM's bit.h expects to find
# as machine/endian.h otherwise.
COMPAT="-isystem $HERE/llvm/wasi-compat/include -include wasi-compat.h -include endian.h"
EMULATED="-D_WASI_EMULATED_SIGNAL -D_WASI_EMULATED_PROCESS_CLOCKS -D_WASI_EMULATED_MMAN -D_WASI_EMULATED_GETPID"
mkdir -p "$BUILD"
cd "$BUILD"
cmake -G Ninja "$SRC/llvm" \
  -DCMAKE_BUILD_TYPE=Release \
  -DCMAKE_SYSTEM_NAME=WASI -DCMAKE_SYSTEM_PROCESSOR=wasm32 \
  -DCMAKE_HOST_SYSTEM_NAME=Linux -DCMAKE_HOST_SYSTEM_PROCESSOR=x86_64 \
  -DCMAKE_CROSSCOMPILING=ON -DUNIX=1 \
  -DCMAKE_C_COMPILER="$CC" -DCMAKE_CXX_COMPILER="$CXX" \
  -DCMAKE_AR="$(which llvm-ar)" -DCMAKE_RANLIB="$(which llvm-ranlib)" \
  -DCMAKE_C_FLAGS="$COMPAT $EMULATED" \
  -DCMAKE_CXX_FLAGS="$CXXSTD $COMPAT $EMULATED" \
  -DLLVM_HOST_TRIPLE=wasm32-unknown-wasi \
  -DLLVM_DEFAULT_TARGET_TRIPLE=wasm32-unknown-wasi \
  -DLLVM_NATIVE_TOOL_DIR="$TBLGEN" \
  -DLLVM_TABLEGEN="$TBLGEN/llvm-tblgen" -DCLANG_TABLEGEN="$TBLGEN/clang-tblgen" \
  -DCROSS_TOOLCHAIN_FLAGS_NATIVE="-DCMAKE_C_COMPILER=$(which gcc);-DCMAKE_CXX_COMPILER=$(which g++);-DLLVM_ENABLE_PROJECTS=clang" \
  -DLLVM_TARGETS_TO_BUILD=Mips \
  -DLLVM_ENABLE_PROJECTS="clang;lld" \
  -DLLVM_ENABLE_THREADS=OFF \
  -DLLVM_ENABLE_PIC=OFF \
  -DLLVM_BUILD_STATIC=ON \
  -DBUILD_SHARED_LIBS=OFF \
  -DLLVM_ENABLE_RTTI=OFF -DLLVM_ENABLE_EH=OFF \
  -DLLVM_ENABLE_ZLIB=OFF -DLLVM_ENABLE_ZSTD=OFF -DLLVM_ENABLE_TERMINFO=OFF \
  -DLLVM_ENABLE_LIBXML2=OFF -DLLVM_ENABLE_LIBEDIT=OFF -DLLVM_ENABLE_FFI=OFF \
  -DLLVM_ENABLE_LIBPFM=OFF -DLLVM_ENABLE_Z3_SOLVER=OFF -DLLVM_ENABLE_OCAMLDOC=OFF \
  -DLLVM_ENABLE_BACKTRACES=OFF -DLLVM_ENABLE_CRASH_OVERRIDES=OFF \
  -DLLVM_ENABLE_UNWIND_TABLES=OFF -DLLVM_ENABLE_BINDINGS=OFF \
  -DLLVM_INCLUDE_TESTS=OFF -DLLVM_INCLUDE_BENCHMARKS=OFF -DLLVM_INCLUDE_EXAMPLES=OFF \
  -DLLVM_INCLUDE_DOCS=OFF -DLLVM_INCLUDE_UTILS=OFF \
  -DLLVM_BUILD_TOOLS=OFF -DLLVM_BUILD_UTILS=OFF -DLLVM_INCLUDE_TOOLS=ON \
  -DLLVM_TOOL_CLANG_TOOLS_EXTRA_BUILD=OFF \
  -DCLANG_BUILD_TOOLS=OFF -DCLANG_ENABLE_STATIC_ANALYZER=OFF -DCLANG_ENABLE_ARCMT=OFF \
  -DCLANG_INCLUDE_TESTS=OFF -DCLANG_INCLUDE_DOCS=OFF \
  -DLLD_BUILD_TOOLS=OFF \
  -DLLVM_ENABLE_DUMP=ON \
  -DLLVM_ENABLE_ASSERTIONS=OFF
