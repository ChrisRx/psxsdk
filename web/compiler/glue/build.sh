#!/usr/bin/env bash
# Compiles the C and C++ the TinyGo compiler links against besides LLVM
# itself, for wasm32-wasi, into $OUT/glue/libtinygoglue.a: go-llvm's own
# bindings, an lld driver with only the ELF linker in it, and the POSIX
# stubs; and as libtinygoglue-full.a, the same with TinyGo's in-process
# clang driver, for the full build that fills the cache.
# Run by build.sh in the cross shell, with GOLLVM the go-llvm module's
# directory; TinyGo's own files come from the prepared tree in $OUT/tinygo.
set -euxo pipefail
: "${GOLLVM:?GOLLVM is empty: go-llvm module directory not found}"
OBJ=$OUT/glue/obj
mkdir -p "$OBJ"
export NIX_CXXSTDLIB_COMPILE_wasm32_unknown_wasip1=" "
export NIX_CXXSTDLIB_LINK_wasm32_unknown_wasip1=" "
INC="-I$OUT/llvm-build/include -I$SRC/llvm/include -I$SRC/lld/include"
COMMON="-O2 -D_WASI_EMULATED_SIGNAL -D_WASI_EMULATED_PROCESS_CLOCKS -D_WASI_EMULATED_MMAN -D_WASI_EMULATED_GETPID -D__STDC_CONSTANT_MACROS -D__STDC_FORMAT_MACROS -D__STDC_LIMIT_MACROS -include endian.h -isystem $HERE/llvm/wasi-compat/include -include wasi-compat.h"
CXXFLAGS="$COMMON -std=c++17 -fno-rtti -fno-exceptions -nostdinc++ -isystem $OUT/libcxx/include/c++/v1 $INC"
for f in IRBindings SupportBindings backports; do
  $CXX $CXXFLAGS -c "$GOLLVM/$f.cpp" -o "$OBJ/gollvm-$f.o"
done
$CXX $CXXFLAGS -c "$HERE/glue/lld-elf.cpp" -o "$OBJ/tinygo-lld.o"
$CC $COMMON -c "$HERE/glue/wasi-stubs.c" -o "$OBJ/wasi-stubs.o"
rm -f "$OUT/glue/libtinygoglue.a"
"$AR" rcs "$OUT/glue/libtinygoglue.a" "$OBJ"/gollvm-*.o "$OBJ/tinygo-lld.o" "$OBJ/wasi-stubs.o"
# The full variant, with TinyGo's clang driver, for the compiler that fills
# the cache.
for f in clang cc1as; do
  $CXX $CXXFLAGS -I"$SRC/clang/include" -I"$OUT/llvm-build/tools/clang/include" -I"$OUT/tinygo/builder" -c "$OUT/tinygo/builder/$f.cpp" -o "$OBJ/tinygo-$f.o"
done
$CC $COMMON -DTINYGO_HAVE_CLANG -c "$HERE/glue/wasi-stubs.c" -o "$OBJ/wasi-stubs-full.o"
rm -f "$OUT/glue/libtinygoglue-full.a"
"$AR" rcs "$OUT/glue/libtinygoglue-full.a" "$OBJ"/gollvm-*.o "$OBJ/tinygo-lld.o" "$OBJ/tinygo-clang.o" "$OBJ/tinygo-cc1as.o" "$OBJ/wasi-stubs-full.o"
