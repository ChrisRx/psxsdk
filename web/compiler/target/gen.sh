#!/usr/bin/env bash
# Writes $OUT/tinygo-wasm.json: the TinyGo target that links the compiler
# itself for wasm32-wasi; with FULL=1, tinygo-wasm-full.json, the variant
# with clang that fills the cache. It is wasip1 plus everything LLVM needs linked in:
# the LLVM, clang and lld libraries built for wasi, the glue, libc++, the
# wasi-libc emulation libraries and the full wasi-libc, since TinyGo builds a
# subset, and stacks deep enough for LLVM. Run by build.sh in the cross
# shell.
set -euo pipefail
SUFFIX=${FULL:+-full}
WASILIB=$(dirname "$($CC -print-file-name=libwasi-emulated-signal.a)")
{
  echo '{'
  echo '	"inherits": ["wasip1"],'
  # Goroutines run on heap-allocated stacks of this size, the main one
  # included, and LLVM's recursion needs far more than the 64K default.
  echo '	"default-stack-size": 33554432,'
  echo '	"ldflags": ['
  echo '		"-z", "stack-size=33554432",'
  echo '		"--error-limit=0",'
  echo "		\"$OUT/glue/libtinygoglue${SUFFIX}.a\","
  for lib in "$OUT"/llvm-build/lib/lib*.a; do
    # Only the ELF flavor of lld is driven, and clang goes into the full
    # variant alone.
    case $(basename "$lib") in liblldCOFF.a|liblldMachO.a|liblldMinGW.a|liblldWasm.a) continue;; esac
    case ${FULL:-} in "") case $(basename "$lib") in libclang*) continue;; esac;; esac
    echo "		\"$lib\","
  done
  echo "		\"$OUT/libcxx/lib/libc++.a\","
  echo "		\"$OUT/libcxx/lib/libc++abi.a\","
  echo "		\"$WASILIB/libwasi-emulated-signal.a\","
  echo "		\"$WASILIB/libwasi-emulated-mman.a\","
  echo "		\"$WASILIB/libwasi-emulated-process-clocks.a\","
  echo "		\"$WASILIB/libwasi-emulated-getpid.a\","
  echo "		\"$WASILIB/libc.a\""
  echo '	]'
  echo '}'
} > "$OUT/tinygo-wasm${SUFFIX}.json"
echo "wrote $OUT/tinygo-wasm${SUFFIX}.json"
