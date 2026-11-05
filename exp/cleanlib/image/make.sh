#!/usr/bin/env bash
# make.sh: builds the clean library's image onto .build/cleanimage/libps.exe, in
# two links: one to learn the sections' sizes, one with them packed around the
# resident entry points. Point PSX_LIBPS_IMAGE at the result to build a program
# against it. Run from the repository root, never alongside build.sh's builds
# of the same sysroot.
set -euo pipefail
HERE=exp/cleanlib/image
OUT=.build/cleanimage
SYS=.build/cleanimage-sysroot
FULL=.build/compiler/tinygo-full.wasm
T=internal/toolchain/_tinygo/targets
if [ ! -d "$SYS" ]; then
  cp -R .build/yaroze-sysroot "$SYS"
  cat "$HERE/runtime_hook.go.txt" >> "$SYS/goroot/src/runtime/runtime_yaroze.go"
fi
MOD=$SYS/mod/go.chrisrx.dev/psx
for d in exp/cleanlib "$HERE" internal/volatile; do
  mkdir -p "$MOD/$d"
  rm -f "$MOD/$d"/*.go
  cp "$d"/*.go "$MOD/$d/"
done
mkdir -p "$OUT"
rm -f "$OUT/libps.exe"
cp "$T"/*.S "$T"/*.c "$SYS/tinygo/targets/"
grep -hoE '^//export [A-Za-z_0-9]+' exp/cleanlib/*.go | awk '{print $2}' | sort -u > "$OUT/exports.txt"

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
    -- "$CWASM" -o /work/prog.elf -target=playstation-yaroze-cleanimage -opt=2 ${TGOFLAGS:-} /work/prog 2>&1 | grep -v 'MIPS-I support' || true
  [ -f "$WORK/prog.elf" ] || { echo "link failed" >&2; exit 1; }
}

cp "$HERE/clib.S" "$OUT/playstation-yaroze-cleanimage-clib.S"
python3 -I "$HERE/gen.py" . "$OUT" "$OUT/exports.txt"
# A package compiled just now can differ in size from the same one read back
# from the cache, so the sizes are learned from a link that reads the cache, as
# the packed link will.
link
link
cp "$SYS/map.txt" "$OUT/map1.txt"
python3 -I "$HERE/gen.py" . "$OUT" "$OUT/exports.txt" "$OUT/map1.txt"
link
cp "$WORK/prog.elf" "$OUT/image.elf"
go run ./tools/mips1fix "$OUT/image.elf" | tail -1
go run ./"$HERE"/imgtool "$OUT/image.elf" "$OUT/libps.exe"
