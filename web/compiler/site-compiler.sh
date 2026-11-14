#!/usr/bin/env bash
# Writes PLAY/compiler.json, which tells the playground where to fetch its
# compiler from: R2, by the key of the compiler yaroze embeds, which
# cmd/yaroze/wasm/full.json names, so that the site serves no 44MB module of
# its own and a page always loads the compiler it was built with. The copy on
# R2 has to be the embedded one byte for byte, which is checked, since the
# site's discs are built by the embedded compiler.
#
# A compiler the compiler task built in .build/compiler, which has the full
# compiler beside it, is a local one that need not be on R2 yet: it is
# copied beside the page instead, gzipped, for task serve.
set -euo pipefail
PLAY=${1:?usage: site-compiler.sh PLAY}
HERE=$(cd "$(dirname "$0")" && pwd)
ROOT=$HERE/../..
EMBEDDED=$ROOT/cmd/yaroze/wasm/tinygo.wasm.gz
LOCAL=$ROOT/.build/compiler
mkdir -p "$PLAY"
rm -f "$PLAY/tinygo.wasm" "$PLAY/tinygo.wasm.gz"

if [ -f "$LOCAL/tinygo-full.wasm" ]; then
  gzip -9 -n -c "$LOCAL/tinygo.wasm" > "$PLAY/tinygo.wasm.gz"
  printf '{"url":"tinygo.wasm.gz"}\n' > "$PLAY/compiler.json"
  echo "site-compiler.sh: serving the compiler in .build/compiler from the site"
  exit 0
fi

# The full compiler's URL names the release the embedded compiler is from.
url=$(sed -n 's/.*"url":"\([^"]*\)\/tinygo-full\.wasm\.gz".*/\1/p' "$ROOT/cmd/yaroze/wasm/full.json")
[ -n "$url" ] || { echo "site-compiler.sh: no release in cmd/yaroze/wasm/full.json" >&2; exit 1; }
url=$url/tinygo.wasm.gz

# Cloudflare keeps a 404 for hours, so the file's own URL is asked for only
# once it is known to be there, with a query of its own: see release.sh.
if ! curl -fsI "$url?check=$(date +%s)" > /dev/null; then
  echo "site-compiler.sh: $url is not on R2; publish it with task compiler:release" >&2
  exit 1
fi
want=$(sha256sum < "$EMBEDDED" | cut -d' ' -f1)
have=$(curl -fsS --retry 3 "$url?check=$(date +%s)" | sha256sum | cut -d' ' -f1)
if [ "$have" != "$want" ]; then
  echo "site-compiler.sh: $url is not the compiler yaroze embeds" >&2
  exit 1
fi
printf '{"url":"%s","sha256":"%s"}\n' "$url" "$want" > "$PLAY/compiler.json"
echo "site-compiler.sh: the playground fetches $url"
