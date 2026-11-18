#!/usr/bin/env bash
# make.sh: builds the clean library's image onto .build/cleanimage/libps.exe, in
# two links: one to learn the sections' sizes, one with them packed around the
# resident entry points. Point PSX_LIBPS_IMAGE at the result to build a program
# against it. Run from the repository root, never alongside build.sh's builds
# of the same sysroot.
#
# OPT sets the compiler's optimization level (0, 1, 2, s or z), 2 unless set. A
# level other than 2 is for TestNoHeapAnyOpt alone (spec 59): a program that
# links the library statically picks its own level, so the library must not
# allocate at any of them. Such a build stops after the first link and leaves
# .build/cleanimage-optN/image.elf, the whole library unpacked, and no libps.exe:
# the pins hold level 2's sizes, and the packed link script cannot name some of
# level 0's sections.
set -euo pipefail
HERE=internal/libps/image
OPT=${OPT:-2}
OUT=.build/cleanimage
if [ "$OPT" != 2 ]; then OUT=.build/cleanimage-opt$OPT; fi
SYS=.build/cleanimage-sysroot
FULL=.build/compiler/tinygo-full.wasm
T=internal/toolchain/_tinygo/targets
if [ ! -d "$SYS" ]; then
  cp -R .build/yaroze-sysroot "$SYS"
  cat "$HERE/runtime_hook.go.txt" >> "$SYS/goroot/src/runtime/runtime_yaroze.go"
fi
MOD=$SYS/mod/go.chrisrx.dev/psx
for d in internal/libps "$HERE" internal/volatile; do
  mkdir -p "$MOD/$d"
  rm -f "$MOD/$d"/*.go
  cp "$d"/*.go "$MOD/$d/"
done
mkdir -p "$OUT"
rm -f "$OUT/libps.exe"
cp "$T"/*.S "$T"/*.c "$SYS/tinygo/targets/"
grep -hoE '^//export [A-Za-z_0-9]+' internal/libps/*.go | awk '{print $2}' | sort -u > "$OUT/exports.txt"

eval "$(python3 -I - "$SYS/env.json" <<'PY'
import json, shlex, sys
env = json.load(open(sys.argv[1]))
for k in ("GOROOT", "TINYGOROOT", "GOMODCACHE"):
    print(f"{k}=/sysroot/{env[k]}")
for k in ("GOVERSION", "GOTOOLTAGS"):
    print(f"{k}={shlex.quote(env[k])}")
requires = "".join(f"require {m['path']} v0.0.0\n" for m in env["modules"])
replaces = "".join(f"replace {m['path']} => /sysroot/{m['dir']}\n" for m in env["modules"])
print(f"GOMOD={shlex.quote(requires + chr(10) + replaces)}")
PY
)"
WORK=$(mktemp -d)
trap 'rm -rf "$WORK"' EXIT
mkdir -p "$WORK/tmp" "$WORK/prog" "$SYS/cache"
printf 'module play\n\ngo %s\n\n%s' "$(echo "$GOVERSION" | sed -E 's/^go([0-9]+\.[0-9]+).*/\1/')" "$GOMOD" > "$WORK/go.mod"
cp "$HERE"/*.go "$WORK/prog/"
CWASM=.build/tgo-full.cwasm
if [ ! -f "$CWASM" ]; then wasmtime compile -o "$CWASM" "$FULL"; fi

link() {
  cp "$OUT"/playstation-yaroze-cleanimage* "$SYS/tinygo/targets/"
  rm -f "$WORK/prog.elf"
  wasmtime run --allow-precompiled \
    --dir "$SYS::/sysroot" --dir "$WORK::/work" --dir "$WORK/tmp::/tmp" --dir "$SYS/cache::/tmp/cache/tinygo" \
    --env GOROOT="$GOROOT" --env GOVERSION="$GOVERSION" --env GOTOOLTAGS="$GOTOOLTAGS" \
    --env TINYGOROOT="$TINYGOROOT" --env GOMODCACHE="$GOMODCACHE" --env TINYGO_BUILD_ID="dev" \
    --env HOME=/tmp --env XDG_CACHE_HOME=/tmp/cache --env TMPDIR=/tmp --env PWD=/work \
    -- "$CWASM" -o /work/prog.elf -target=playstation-yaroze-cleanimage -opt="$OPT" ${TGOFLAGS:-} /work/prog 2>&1 | grep -v 'MIPS-I support' || true
  [ -f "$WORK/prog.elf" ] || { echo "link failed" >&2; exit 1; }
}

cp "$HERE/clib.S" "$OUT/playstation-yaroze-cleanimage-clib.S"
if [ "$OPT" = 0 ]; then
  # Level 0 keeps the runtime's float32 printing and its unwinding, which
  # nothing in the library calls, and the image has no single-precision
  # routines or longjmp for them. This image is only ever checked for
  # allocations, never run, so each of those names is a break instruction.
  for f in tinygo_longjmp __nesf2 __gtsf2 __ltsf2 __gesf2 __lesf2 __fixsfsi __floatsisf __subsf3; do
    printf '.section .text.%s\n.global %s\n.type %s, @function\n%s:\n\tbreak\n' "$f" "$f" "$f" "$f"
  done >> "$OUT/playstation-yaroze-cleanimage-clib.S"
fi
python3 -I "$HERE/gen.py" . "$OUT" "$OUT/exports.txt"
# A package compiled just now can differ in size from the same one read back
# from the cache, so the sizes are learned from a link that reads the cache, as
# the packed link will.
link
if [ "$OPT" != 2 ]; then
  cp "$WORK/prog.elf" "$OUT/image.elf"
  echo "wrote $OUT/image.elf, unpacked, for TestNoHeapAnyOpt"
  exit 0
fi
link
cp "$SYS/map.txt" "$OUT/map1.txt"
# The public entry points alone, for the comparison tools to name calls by: tools
# on the implementation side read this rather than the map.
awk 'NF == 5 && $5 ~ /^\.fix\./ { print $1, substr($5, 6) }' "$OUT/map1.txt" > "$OUT/entries.txt"
python3 -I "$HERE/gen.py" . "$OUT" "$OUT/exports.txt" "$OUT/map1.txt"
link
# One more link, for the first instructions that go in delay slots: the stubs' sizes
# don't change, so nothing moves, and generating again has to give the same stubs.
heads() {
  cp "$SYS/map.txt" "$OUT/map2.txt"
  go run ./"$HERE"/imgtool "$WORK/prog.elf" "$WORK/flat.exe" >/dev/null
  python3 -I "$HERE/gen.py" . "$OUT" "$OUT/exports.txt" "$OUT/map1.txt" "$OUT/map2.txt" "$WORK/flat.exe" >/dev/null
}
heads
cp "$OUT/playstation-yaroze-cleanimage.S" "$WORK/stubs.S"
link
heads
cmp "$WORK/stubs.S" "$OUT/playstation-yaroze-cleanimage.S"
cp "$WORK/prog.elf" "$OUT/image.elf"
# The image's Go heap is 8 bytes (gen.py): the library must allocate nothing,
# and an allocation would stop it at run time with no message. The runtime's
# alloc is never inlined, so it is linked in exactly when something allocates.
if llvm-nm "$OUT/image.elf" | grep -q ' runtime\.alloc$'; then
  echo "make.sh: the library allocates (runtime.alloc is linked in), and the image has no heap for it" >&2
  rm -f "$OUT/libps.exe"
  exit 1
fi
go run ./tools/mips1fix "$OUT/image.elf" | tail -1
go run ./"$HERE"/imgtool "$OUT/image.elf" "$OUT/libps.exe"
