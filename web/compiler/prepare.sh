#!/usr/bin/env bash
# Makes the TinyGo tree the wasm compiler is built from, in $OUT/tinygo: a
# copy of upstream TinyGo's source at the revision the flake builds (its
# tinygo-src package), with the PlayStation targets and their runtime from
# internal/toolchain/_tinygo laid over it, tinygo-wasm.patch and
# internal/toolchain/tinygo-hot.patch applied, and
# compiler-rt's builtins, which TinyGo's tree takes from an LLVM checkout
# and nixpkgs copies in. Of upstream's submodules only wasi-libc, which
# TinyGo builds its libc from, is kept; the rest are the empty directories
# a git archive has for them.
#
#	OUT=.build/compiler NIXPKGS=github:NixOS/nixpkgs/REV web/compiler/prepare.sh
#
# Everything the compiler needs to run as a WebAssembly program is in the
# patch: the build command (cmd/tinygo-wasm), the sysroot command
# (cmd/tinygo-sysroot), and under the wasm build tag a loader that lists
# packages with go/build instead of running go list, a goenv that reads
# GOROOT and GOVERSION from the environment, a builder that runs lld
# in-process and has no clang, and a cgo package that refuses cgo. It is
# regenerated with
#
#	git -C TREE add -N . && git -C TREE diff --no-renames > web/compiler/tinygo-wasm.patch
#
# from a git checkout of TinyGo at the flake's revision, with _tinygo
# copied in and committed, and the changes applied.
set -euo pipefail
HERE=$(cd "$(dirname "$0")" && pwd)
OUT=$(realpath -m "${OUT:?set OUT to the output directory}")
NIXPKGS=${NIXPKGS:?set NIXPKGS to the nixpkgs flake reference}
TREE=$OUT/tinygo

# The tree is made again when the patch changes, the flake moves to another
# revision or the files laid over it change, which .prepared records as the
# source's store path and a hash of those files.
SRC=$(nix build --no-link --print-out-paths "$HERE/../..#tinygo-src")
OVERLAY=$HERE/../../internal/toolchain/_tinygo
STAMP="$SRC $(cd "$OVERLAY" && find . -type f | LC_ALL=C sort | xargs sha256sum | sha256sum | cut -c1-16)"
HOTPATCH=$HERE/../../internal/toolchain/tinygo-hot.patch
if [ -e "$TREE/.prepared" ] && [ "$TREE/.prepared" -nt "$HERE/tinygo-wasm.patch" ] && [ "$TREE/.prepared" -nt "$HOTPATCH" ] && [ "$TREE/.prepared" -nt "$HERE/tinygo-hot-wasm.patch" ] && [ "$(cat "$TREE/.prepared")" = "$STAMP" ]; then
  exit 0
fi
echo "==> TinyGo tree"
# The copy out of the store is read-only until the chmod below.
[ -e "$TREE" ] && chmod -R u+w "$TREE"
rm -rf "$TREE"
mkdir -p "$TREE"
excludes=()
for p in $(sed -n 's/^[[:space:]]*path = //p' "$SRC/.gitmodules"); do
  [ "$p" = lib/wasi-libc ] || excludes+=(--exclude="./$p")
done
tar -C "$SRC" "${excludes[@]}" --exclude=.git -cf - . | tar -x -C "$TREE"
chmod -R u+w "$TREE"
# The other submodules are left as the empty directories a git archive
# has for them.
for p in $(sed -n 's/^[[:space:]]*path = //p' "$SRC/.gitmodules"); do
  mkdir -p "$TREE/$p"
done
cp -r "$OVERLAY"/. "$TREE"

CRT=$(nix build --no-link --print-out-paths "$NIXPKGS#llvmPackages_22.compiler-rt.src")
mkdir -p "$TREE/lib/compiler-rt-builtins"
cp -a "$CRT"/compiler-rt/lib/builtins/* "$TREE/lib/compiler-rt-builtins/"
chmod -R u+w "$TREE"

patch -p1 -d "$TREE" < "$HERE/tinygo-wasm.patch"
# The hot reload flags and hooks, which the flake applies to its TinyGo too,
# and the same flags on the wasm build command, which only this tree has.
patch -p1 -d "$TREE" < "$HOTPATCH"
patch -p1 -d "$TREE" < "$HERE/tinygo-hot-wasm.patch"
echo "$STAMP" > "$TREE/.prepared"
