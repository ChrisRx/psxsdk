#!/usr/bin/env bash
# Builds the TinyGo compiler as a WebAssembly program, for the playground in
# the player, into OUTDIR as tinygo.wasm:
#
#	web/compiler/build.sh .build/compiler
#
# TinyGo is upstream's source at the revision the flake builds, with the
# PlayStation targets and runtime from internal/toolchain/_tinygo laid over
# it. The changes that make the compiler itself run as a WebAssembly
# program are in tinygo-wasm.patch, which prepare.sh applies to that tree
# in OUTDIR/tinygo. Everything else comes from
# nixpkgs, at the revision the flake pins: the LLVM sources, the wasm32-wasi
# cross toolchain, and the shell nixpkgs builds TinyGo in.
#
# The steps, each skipped when its output is already there:
#
#  0. The TinyGo tree, as prepare.sh makes it.
#  1. An overlay of the LLVM source tree, symlinks into the nix store with
#     the files llvm/llvm-wasi.patch changes copied and patched: WASI has no
#     signals, processes or sockets, and LLVM's Support library assumes them.
#  2. libc++ and libc++abi for wasm32-wasi with their threading API on, so
#     that LLVM's headers, which use std::mutex and friends whatever
#     LLVM_ENABLE_THREADS says, compile. Nothing starts a thread.
#  3. LLVM, clang and lld for wasm32-wasi, as static libraries, with only
#     the Mips backend (llvm/configure.sh).
#  4. The glue: go-llvm's C++ bindings, an lld driver, TinyGo's in-process
#     clang driver, and stubs for the POSIX functions LLVM links against
#     and WASI does not have (glue/).
#  5. Two TinyGo target definitions that link that into a wasip1 program
#     (target/gen.sh): one with clang and one without.
#  6. TinyGo compiling its own compiler twice: tinygo-full.wasm, with
#     clang, which mkcache.sh runs to fill the compiler's cache with the
#     target's C and the standard library; and tinygo.wasm, without, which
#     is what ships and relies on that cache. Clang is half the module.
#     The tree's tinygo-sysroot command is built natively alongside, for
#     the playground task.
#
# The whole thing takes about 50 minutes on 20 cores, most of it LLVM, and
# tinygo.wasm comes out around 75MB, 22MB compressed. The last steps, with
# wasm-opt, need over 10GB of memory.
set -euo pipefail
HERE=$(cd "$(dirname "$0")" && pwd)
OUT=$(realpath -m "${1:?usage: build.sh OUTDIR}")
JOBS=${JOBS:-$(nproc)}
if [ -z "${NIXPKGS:-}" ]; then
  rev=$(nix flake metadata --json "$HERE/../.." | python3 -c 'import json, sys; print(json.load(sys.stdin)["locks"]["nodes"]["nixpkgs"]["locked"]["rev"])')
  NIXPKGS="github:NixOS/nixpkgs/$rev"
fi
export HERE OUT JOBS NIXPKGS
mkdir -p "$OUT"

echo "==> nix store paths"
LLVM_SRC=$(nix build --no-link --print-out-paths "$NIXPKGS#llvmPackages_22.llvm.monorepoSrc")
TBLGEN=$(nix build --no-link --print-out-paths "$NIXPKGS#llvmPackages_22.tblgen")/bin
CLANG_LIB=$(nix build --no-link --print-out-paths "$NIXPKGS#llvmPackages_22.clang.cc.lib")
CLANG_BIN=$(nix build --no-link --print-out-paths "$NIXPKGS#llvmPackages_22.clang-unwrapped")/bin
export LLVM_SRC TBLGEN

# cross runs a command in a wasm32-wasi cross shell: $CC and $CXX are the
# wasi clang wrappers, with wasi-libc and libc++, and cmake and ninja are
# at hand. dev runs one in the shell nixpkgs builds TinyGo in, which has the
# Go toolchain and LLVM TinyGo's native build needs.
# The cross shell asks for static linking through NIX_CFLAGS_LINK, which
# the build machine's own gcc wrapper reads too, and glibc has no static
# library there; the native tablegen LLVM builds for itself has to link, so
# the variable goes. wasm-ld links statically regardless.
cross() {
  nix develop --impure --expr "let pkgs = import (builtins.getFlake \"$NIXPKGS\") { system = builtins.currentSystem; }; in pkgs.pkgsCross.wasi32.mkShell { nativeBuildInputs = [ pkgs.cmake pkgs.ninja pkgs.python3 ]; }" --command bash -c "unset NIX_CFLAGS_LINK; $1"
}
dev() {
  nix develop "$NIXPKGS#tinygo" --command bash -c "export GOFLAGS=-mod=mod GOPROXY=https://proxy.golang.org,direct; $1"
}

"$HERE/prepare.sh"
TREE=$OUT/tinygo

SRC=$OUT/llvm-src
export SRC
if [ ! -e "$SRC/.patched" ]; then
  echo "==> LLVM source overlay"
  rm -rf "$SRC"
  mkdir -p "$SRC"
  for d in llvm clang lld cmake third-party libunwind; do
    cp -rs "$LLVM_SRC/$d" "$SRC/$d"
  done
  chmod -R u+w "$SRC"
  # The files the patch changes become real files, since a symlink into
  # the store cannot be written through.
  (cd "$SRC" && for f in $(sed -n 's#^+++ b/##p' "$HERE/llvm/llvm-wasi.patch"); do
    cp --remove-destination "$(readlink -f "$f")" "$f"
    chmod u+w "$f"
  done && patch -p1 < "$HERE/llvm/llvm-wasi.patch")
  touch "$SRC/.patched"
fi

if [ ! -e "$OUT/libcxx/lib/libc++.a" ]; then
  echo "==> libc++"
  cross "$HERE/llvm/libcxx.sh"
fi

if [ ! -e "$OUT/llvm-build/build.ninja" ]; then
  echo "==> configure LLVM"
  cross "$HERE/llvm/configure.sh"
fi
if [ ! -e "$OUT/llvm-build/lib/liblldELF.a" ]; then
  echo "==> build LLVM"
  # The libraries only, and keep going past failures: the executables that
  # the libraries' dependencies drag in, the target's own tablegen among
  # them, have no reason to link for wasi, and the libraries are checked
  # for afterwards.
  cross "cd '$OUT/llvm-build' && ninja -k 0 -j $JOBS \$(ninja -t targets all | sed -n 's#^\(lib/lib[^:]*\.a\):.*#\1#p') || true"
  for lib in libLLVMMipsCodeGen libLLVMPasses libclangCodeGen liblldELF; do
    if [ ! -e "$OUT/llvm-build/lib/$lib.a" ]; then
      echo "build.sh: LLVM did not build $lib.a" >&2
      exit 1
    fi
  done
fi

echo "==> glue"
# go list -m leaves Dir empty for a module not in the module cache yet, as
# on a fresh machine, so the module is downloaded first.
GOLLVM=$(dev "cd '$TREE' && go mod download tinygo.org/x/go-llvm && go list -m -f '{{.Dir}}' tinygo.org/x/go-llvm")
export GOLLVM
cross "$HERE/glue/build.sh"

echo "==> targets"
cross "$HERE/target/gen.sh && FULL=1 $HERE/target/gen.sh"

echo "==> tinygo-full.wasm and tinygo.wasm"
dev "cd '$TREE' \
  && go build -tags llvm22 -ldflags '-X github.com/tinygo-org/tinygo/goenv.clangResourceDir=$CLANG_LIB/lib/clang/22' -o build/tinygo . \
  && go build -tags llvm22 -ldflags '-X github.com/tinygo-org/tinygo/goenv.clangResourceDir=$CLANG_LIB/lib/clang/22' -o build/tinygo-sysroot ./cmd/tinygo-sysroot \
  && export PATH=\"\$PWD/build:$CLANG_BIN:\$PATH\" \
  && export CGO_CFLAGS='-I$OUT/llvm-build/include -I$SRC/llvm/include' \
  && tinygo build -target='$OUT/tinygo-wasm-full.json' -tags 'byollvm llvm22' -o '$OUT/tinygo-full.wasm' ./cmd/tinygo-wasm \
  && tinygo build -target='$OUT/tinygo-wasm.json' -tags 'byollvm llvm22' -o '$OUT/tinygo-debug.wasm' ./cmd/tinygo-wasm \
  && wasm-opt --strip-debug --strip-producers '$OUT/tinygo-debug.wasm' -o '$OUT/tinygo.wasm'"
# The debug information is stripped afterwards rather than with -no-debug,
# which sends wasm-opt's asyncify pass into a much longer and hungrier run.
ls -la "$OUT/tinygo-full.wasm" "$OUT/tinygo.wasm"
# The key of the recipe this build is of, for task compiler:release to
# check before it publishes the build under the recipe's current key.
"$HERE/key.sh" > "$OUT/key"
