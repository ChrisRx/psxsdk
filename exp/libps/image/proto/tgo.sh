#!/usr/bin/env bash
# tgo.sh SYSROOT PROG OUT.elf [tinygo args...]: builds one program of the
# sysroot with the full compiler (clang included), as mkcache.sh does, with
# extra arguments: a diagnosis loop for target changes that needs no cache
# rebuild. Run from the worktree root.
set -euo pipefail
SYSROOT=$(realpath "$1"); PROG=$2; OUT=$(realpath -m "$3"); shift 3
FULL=.build/compiler/tinygo-full.wasm
eval "$(python3 -I - "$SYSROOT/env.json" <<'PY'
import json, shlex, sys
env = json.load(open(sys.argv[1]))
for k in ("GOROOT", "TINYGOROOT", "GOMODCACHE"):
    print(f"export {k}=/sysroot/{env[k]}")
for k in ("GOVERSION", "GOTOOLTAGS", "target"):
    print(f"export {k}={shlex.quote(env[k])}")
requires = "".join(f"require {m['path']} v0.0.0\n" for m in env["modules"])
replaces = "".join(f"replace {m['path']} => /sysroot/{m['dir']}\n" for m in env["modules"])
print(f"GOMOD={shlex.quote(requires + chr(10) + replaces)}")
PY
)"
WORK=$(mktemp -d)
trap 'rm -rf "$WORK"' EXIT
mkdir -p "$WORK/tmp" "$WORK/prog" "$SYSROOT/cache"
printf 'module play\n\ngo %s\n\n%s' "$(echo "$GOVERSION" | sed -E 's/^go([0-9]+\.[0-9]+).*/\1/')" "$GOMOD" > "$WORK/go.mod"
cp "$SYSROOT/$PROG"/*.go "$WORK/prog/"
if [ ! -f .build/tgo-full.cwasm ]; then wasmtime compile -o .build/tgo-full.cwasm "$FULL"; fi
wasmtime run --allow-precompiled \
  --dir "$SYSROOT::/sysroot" --dir "$WORK::/work" --dir "$WORK/tmp::/tmp" --dir "$SYSROOT/cache::/tmp/cache/tinygo" \
  --env GOROOT="$GOROOT" --env GOVERSION="$GOVERSION" --env GOTOOLTAGS="$GOTOOLTAGS" \
  --env TINYGOROOT="$TINYGOROOT" --env GOMODCACHE="$GOMODCACHE" --env TINYGO_BUILD_ID="dev" \
  --env HOME=/tmp --env XDG_CACHE_HOME=/tmp/cache --env TMPDIR=/tmp --env PWD=/work \
  -- .build/tgo-full.cwasm -o /work/prog.elf "$@" /work/prog
cp "$WORK/prog.elf" "$OUT"
echo "wrote $OUT"
