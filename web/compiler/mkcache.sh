#!/usr/bin/env bash
# Fills the wasm compiler's cache by running a build of it under wasmtime
# over programs in the sysroot's modules, and stamps the sysroot with the
# shipped compiler's hash:
#
#	web/compiler/mkcache.sh COMPILER.wasm TINYGO.wasm SYSROOT PROGRAM...
#
# SYSROOT is a directory tinygo-sysroot wrote; the cache goes into
# SYSROOT/cache and TINYGO.wasm's hash into its env.json as buildID. Each
# PROGRAM is the directory of a main package within one of the sysroot's
# modules, given as a path inside the sysroot (mod/go.chrisrx.dev/psx/examples/check).
#
# The compiler in the browser has no clang, so the C and assembly files the
# target needs have to be compiled here, against the same paths, and found
# in the cache; the entries for them, obj-*.bc, are keyed by file and
# flags, not by the compiler. The standard library's and the modules'
# packages come along, which is what makes a first build in the browser
# quick; their entries are keyed by the compiler's hash, so a new compiler
# never picks up an old cache.
#
# COMPILER is either the full build of the compiler, the one with clang,
# which makes the C and assembly entries itself, or the shipped compiler,
# TINYGO.wasm itself, which cannot: then OBJ_CACHE names a directory
# holding the obj-*.bc of an earlier run and the dep-*.json beside them,
# which say what each was compiled from and are how the compiler finds
# one, and they are copied in first. The sysroot yaroze embeds,
# cmd/yaroze/wasm/sysroot.tar.gz, holds them, which is how the site is
# built from a checkout with no compiler built. A C or assembly file the
# seeded entries do not cover fails the build, since there is no clang to
# compile it, which says the embedded sysroot is due a refresh with task
# yaroze.
#
# The C and assembly entries are keyed by the optimization level too, so
# each program is built at every level in OPTS: z by default, TinyGo's own
# and the playground's. task yaroze runs it again over a copy of the
# playground's sysroot with OPTS=2, the level yaroze builds at by default.
# env.json's opts lists the levels the cache holds.
set -euo pipefail
usage="usage: mkcache.sh COMPILER.wasm TINYGO.wasm SYSROOT PROGRAM..."
FULL=$(realpath "${1:?$usage}")
TINYGO=$(realpath "${2:?$usage}")
SYSROOT=$(realpath "${3:?$usage}")
shift 3
[ $# -gt 0 ] || { echo "mkcache.sh: no programs given" >&2; exit 2; }
mkdir -p "$SYSROOT/cache"
if [ -n "${OBJ_CACHE:-}" ]; then
  n=$(ls "$OBJ_CACHE"/obj-*.bc "$OBJ_CACHE"/dep-*.json 2>/dev/null | wc -l)
  [ "$n" -gt 0 ] || { echo "mkcache.sh: OBJ_CACHE=$OBJ_CACHE holds no obj-*.bc and dep-*.json entries" >&2; exit 2; }
  cp "$OBJ_CACHE"/obj-*.bc "$OBJ_CACHE"/dep-*.json "$SYSROOT/cache/"
  echo "mkcache: seeded $n compiled C and assembly entries, with their dependency lists, from $OBJ_CACHE"
fi

BUILDID=$(sha256sum "$TINYGO" | cut -c1-16)
python3 - "$SYSROOT/env.json" "$BUILDID" "${OPTS:-z}" <<'PY'
import json, sys
p, id, opts = sys.argv[1], sys.argv[2], sys.argv[3].split()
env = json.load(open(p))
env["buildID"] = id
# The levels the cache holds, those of an earlier run on the same sysroot
# included, which yaroze reads to know what it can build at.
env["opts"] = env.get("opts", []) + [o for o in opts if o not in env.get("opts", [])]
json.dump(env, open(p, "w"), indent="\t")
open(p, "a").write("\n")
PY
eval "$(python3 - "$SYSROOT/env.json" <<'PY'
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
mkdir -p "$WORK/tmp" "$SYSROOT/cache"
printf 'module play\n\ngo %s\n\n%s' "$(echo "$GOVERSION" | sed -E 's/^go([0-9]+\.[0-9]+).*/\1/')" "$GOMOD" > "$WORK/go.mod"

# wasmtime compiles the module once, since that takes a while.
wasmtime compile -o "$WORK/tinygo.cwasm" "$FULL"
for prog in "$@"; do
  rm -rf "$WORK/prog"
  mkdir -p "$WORK/prog"
  cp "$SYSROOT/$prog"/*.go "$WORK/prog/"
  for opt in ${OPTS:-z}; do
    echo "mkcache: $prog -opt=$opt"
    args=(-target="$target" -opt="$opt" -o /work/prog.elf /work/prog)
    # The full build is a command and takes its arguments as one. The
    # shipped compiler is a reactor, driven through its exports, which
    # the wasmtime command can call but not write to: its tinygo_main
    # takes the arguments from TINYGO_ARGS, separated by unit separators,
    # as yaroze gives them under wasmtime.
    if [ "$FULL" = "$TINYGO" ]; then
      run=(--invoke tinygo_main --env "TINYGO_ARGS=$(IFS=$'\x1f'; echo "${args[*]}")" -- "$WORK/tinygo.cwasm")
    else
      run=(-- "$WORK/tinygo.cwasm" "${args[@]}")
    fi
    wasmtime run --allow-precompiled \
      --dir "$SYSROOT::/sysroot" --dir "$WORK::/work" --dir "$WORK/tmp::/tmp" --dir "$SYSROOT/cache::/tmp/cache/tinygo" \
      --env GOROOT="$GOROOT" --env GOVERSION="$GOVERSION" --env GOTOOLTAGS="$GOTOOLTAGS" \
      --env TINYGOROOT="$TINYGOROOT" --env GOMODCACHE="$GOMODCACHE" --env TINYGO_BUILD_ID="$BUILDID" \
      --env HOME=/tmp --env XDG_CACHE_HOME=/tmp/cache --env TMPDIR=/tmp --env PWD=/work \
      "${run[@]}"
    test -s "$WORK/prog.elf" || { echo "mkcache.sh: $prog produced no executable" >&2; exit 1; }
    rm -f "$WORK/prog.elf"
  done
done
echo "mkcache: $(ls "$SYSROOT/cache" | wc -l) entries, $(du -sh "$SYSROOT/cache" | cut -f1), build id $BUILDID"
