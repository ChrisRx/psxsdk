#!/usr/bin/env bash
# Makes the TinyGo tree the wasm compiler is built from, in $OUT/tinygo: a
# copy of the fork's committed tree with tinygo-wasm.patch applied, and the
# two things a checkout of it lacks, the wasi-libc submodule that TinyGo
# builds its libc from and compiler-rt's builtins, which TinyGo's tree
# takes from an LLVM checkout and nixpkgs copies in.
#
#	TINYGO_SRC=~/src/tinygo OUT=.build/compiler web/compiler/prepare.sh
#
# The fork itself carries only the PlayStation targets, which is what a
# pull request to TinyGo would hold. Everything the compiler needs to run
# as a WebAssembly program is in the patch: the build command
# (cmd/tinygo-wasm), the sysroot command (cmd/tinygo-sysroot), and under
# the wasm build tag a loader that lists packages with go/build instead of
# running go list, a goenv that reads GOROOT and GOVERSION from the
# environment, a builder that runs lld in-process and has no clang, and a
# cgo package that refuses cgo. It is regenerated with
#
#	git -C $TINYGO_SRC add -N . && git -C $TINYGO_SRC diff --no-renames > web/compiler/tinygo-wasm.patch
#
# from a fork tree with the changes applied.
set -euo pipefail
HERE=$(cd "$(dirname "$0")" && pwd)
TINYGO_SRC=$(realpath "${TINYGO_SRC:?set TINYGO_SRC to a checkout of the TinyGo fork}")
OUT=$(realpath -m "${OUT:?set OUT to the output directory}")
NIXPKGS=${NIXPKGS:?set NIXPKGS to the nixpkgs flake reference}
TREE=$OUT/tinygo

if [ -e "$TREE/.prepared" ] && [ "$TREE/.prepared" -nt "$HERE/tinygo-wasm.patch" ]; then
  exit 0
fi
echo "==> TinyGo tree"
rm -rf "$TREE"
mkdir -p "$TREE"
git -C "$TINYGO_SRC" archive --format=tar HEAD | tar -x -C "$TREE"

# Submodules are not in an archive.
if [ ! -e "$TINYGO_SRC/lib/wasi-libc/Makefile" ]; then
  echo "prepare.sh: $TINYGO_SRC/lib/wasi-libc is empty; run: git -C $TINYGO_SRC submodule update --init lib/wasi-libc" >&2
  exit 1
fi
rm -rf "$TREE/lib/wasi-libc"
cp -a "$TINYGO_SRC/lib/wasi-libc" "$TREE/lib/wasi-libc"

CRT=$(nix build --no-link --print-out-paths "$NIXPKGS#llvmPackages_22.compiler-rt.src")
mkdir -p "$TREE/lib/compiler-rt-builtins"
cp -a "$CRT"/compiler-rt/lib/builtins/* "$TREE/lib/compiler-rt-builtins/"
chmod -R u+w "$TREE"

patch -p1 -d "$TREE" < "$HERE/tinygo-wasm.patch"
touch "$TREE/.prepared"
