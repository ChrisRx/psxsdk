#!/usr/bin/env bash
# build.sh: syncs exp/libps and the image program into .build/image-sysroot,
# regenerates the target files, and builds the image ELF. Run from the
# worktree root.
set -euo pipefail
HERE=$(dirname "$(realpath "$0")")
SYS=.build/image-sysroot
if [ ! -d "$SYS" ]; then
  cp -R .build/yaroze-sysroot "$SYS"
  cat "$HERE/runtime_hook.go.txt" >> "$SYS/goroot/src/runtime/runtime_yaroze.go"
fi
MOD=$SYS/mod/go.chrisrx.dev/psx
if [ ! -d "$SYS/tinygo/lib/compiler-rt-builtins" ]; then
  cp -R .build/compiler/tinygo/lib/compiler-rt-builtins "$SYS/tinygo/lib/"
  # The sysroot's clang headers are the few its C needs; compiler-rt's
  # float.h wants one more of clang's own.
  inc=$(ls -d /nix/store/*-clang-22*-lib/lib/clang/22/include | head -1)
  cp -n "$inc"/*.h "$SYS/tinygo/lib/clang/include/"
fi
mkdir -p .build/image
for d in exp/libps exp/libps/image internal/volatile yaroze; do
  mkdir -p "$MOD/$d"
  rm -f "$MOD/$d"/*.go
  cp "$d"/*.go "$MOD/$d/"
done
grep -rhoE '^//export [A-Za-z_0-9]+' exp/libps/*.go | awk '{print $2}' | sort -u > .build/image/goexports.txt
build() {
  cp .build/image/playstation-yaroze-image* "$SYS/tinygo/targets/"
  "$HERE/tgo.sh" "$SYS" mod/go.chrisrx.dev/psx/exp/libps/image .build/image/image.elf -target=playstation-yaroze-image -opt=2 -tags=sonydata "$@" 2>&1 | grep -v 'MIPS-I support'
}
# musl's math sources, their <math.h> and <endian.h> made this
# directory's own, beside the headers that stand in for musl's.
m=.build/compiler/tinygo/lib/wasi-libc/libc-top-half/musl/src
mkdir -p "$SYS/tinygo/lib/musl-math"
for f in "$m"/math/*.c "$m"/math/*_data.h "$m"/internal/libm.h; do
  sed -e 's|#include <math.h>|#include "math.h"|; s|#include <endian.h>|#include "endian.h"|; s|#include <features.h>|#include "features.h"|' "$f" > "$SYS/tinygo/lib/musl-math/$(basename "$f")"
done
cp "$HERE"/libm/*.h "$SYS/tinygo/lib/musl-math/"
cp internal/toolchain/_tinygo/targets/*.S internal/toolchain/_tinygo/targets/*.c "$SYS/tinygo/targets/"
python3 "$HERE/gen.py" . .build/image .build/image/goexports.txt
build "$@"
cp "$SYS/map.txt" .build/image/map1.txt
python3 "$HERE/gen.py" . .build/image .build/image/goexports.txt .build/image/map1.txt
build "$@"
