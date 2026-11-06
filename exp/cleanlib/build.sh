#!/usr/bin/env bash
# build.sh NAME DIR: builds the main package in DIR (a path in this module,
# as docs/tutorial/hello) against the clean library, exp/cleanlib, onto
# .build/NAME/NAME.cue, with prog.elf beside it. Run from the repository
# root.
#
# The embedded compiler's file store does not hold exp/cleanlib, so this
# builds with the full compiler, which has clang, over a copy of its
# sysroot with this module's packages synced in.
set -euo pipefail
NAME=$1 DIR=$2
SYS=.build/clean-sysroot
FULL=.build/compiler/tinygo-full.wasm
if [ ! -d "$SYS" ]; then
  cp -R .build/yaroze-sysroot "$SYS"
fi
MOD=$SYS/mod/go.chrisrx.dev/psx
sync() {
  mkdir -p "$MOD/$1"
  rm -f "$MOD/$1"/*.go
  cp "$1"/*.go "$MOD/$1/" 2>/dev/null || true
}
for d in yaroze exp/cleanlib internal/volatile disc "$DIR"; do
  sync "$d"
done
for d in $(cd exp/cleanlib && find . -mindepth 1 -type d -not -path './spec*' -not -path './gen*'); do
  sync "exp/cleanlib/${d#./}"
done
cp internal/toolchain/_tinygo/targets/*.json internal/toolchain/_tinygo/targets/*.S internal/toolchain/_tinygo/targets/*.c internal/toolchain/_tinygo/targets/*.ld "$SYS/tinygo/targets/"

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
cp "$DIR"/*.go "$WORK/prog/"
CWASM=.build/tgo-full.cwasm
if [ ! -f "$CWASM" ]; then wasmtime compile -o "$CWASM" "$FULL"; fi
OUT=.build/$NAME
mkdir -p "$OUT"
wasmtime run --allow-precompiled \
  --dir "$SYS::/sysroot" --dir "$WORK::/work" --dir "$WORK/tmp::/tmp" --dir "$SYS/cache::/tmp/cache/tinygo" \
  --env GOROOT="$GOROOT" --env GOVERSION="$GOVERSION" --env GOTOOLTAGS="$GOTOOLTAGS" \
  --env TINYGOROOT="$TINYGOROOT" --env GOMODCACHE="$GOMODCACHE" --env TINYGO_BUILD_ID="dev" \
  --env HOME=/tmp --env XDG_CACHE_HOME=/tmp/cache --env TMPDIR=/tmp --env PWD=/work \
  -- "$CWASM" -o /work/prog.elf -target=playstation-yaroze-clean-fidelity -opt=2 ${TGOFLAGS:-} /work/prog 2>&1 | grep -v 'MIPS-I support' || true
[ -f "$WORK/prog.elf" ] || { echo "build failed" >&2; exit 1; }
cp "$WORK/prog.elf" "$OUT/prog.elf"
go run ./tools/mips1fix "$OUT/prog.elf" | tail -1
go run ./tools/elf2exe --stack-addr 0x801fff00 "$OUT/prog.elf" "$OUT/MAIN.EXE" | tail -1
# Unless PAD_TO_REF=0, the executable is padded with zeros to the size of
# the reference disc's (.build/BASE-ref, BASE being NAME less "-clean"), so
# that the BIOS takes as long to load it and the files and tracks after it
# sit where the reference's do. The reference's library is inside its
# executable, where the real one is resident, so the sizes differ for no
# reason of the library's; the padding makes boot timing and disc layout
# comparable in TestClean. PAD_TO_REF may also name another disc's .bin to
# pad to, as a build against Sony's library (.build/BASE-sony).
REF=.build/${NAME%-clean}-ref/${NAME%-clean}-ref.bin
case "${PAD_TO_REF:-1}" in *.bin) REF=$PAD_TO_REF ;; esac
if [ "${PAD_TO_REF:-1}" != 0 ] && [ -f "$REF" ]; then
  python3 -I - "$REF" "$OUT/MAIN.EXE" <<'PY'
import struct, sys
def sector(f, n):
    f.seek(n * 2352)
    raw = f.read(2352)
    return raw[16:2064] if raw[15] == 1 else raw[24:2072]
ref, exe = sys.argv[1], sys.argv[2]
with open(ref, 'rb') as f:
    pvd = sector(f, 16)
    lba = struct.unpack('<I', pvd[158:162])[0]
    d = sector(f, lba)
    size = 0
    i = 0
    while i < len(d) and d[i]:
        rec = d[i:i + d[i]]
        if rec[33:33 + rec[32]].startswith(b'MAIN.EXE'):
            size = struct.unpack('<I', rec[10:14])[0]
        i += d[i]
b = bytearray(open(exe, 'rb').read())
if size > len(b):
    b += bytes(size - len(b))
    struct.pack_into('<I', b, 0x1c, size - 2048)
    open(exe, 'wb').write(b)
    print("padded MAIN.EXE to", size, "bytes")
PY
fi
go run ./tools/mkdisc -o "$OUT/$NAME.bin" --volume-id "$NAME" --boot MAIN.EXE --file "MAIN.EXE=$OUT/MAIN.EXE" --src "$DIR" | tail -1
