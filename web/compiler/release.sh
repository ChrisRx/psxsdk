#!/usr/bin/env bash
# Publishes the compiler in DIR (tinygo.wasm, tinygo-full.wasm and
# tinygo-debug.wasm) to the psxsdk bucket on Cloudflare R2, gzipped, under
# compiler/KEY/, the key being key.sh's, unless it is there already. It stays
# until it is deleted, so CI finds the compiler however long it has been since
# one was built, and yaroze downloads the full compiler from it to build C.
#
# With the compiler there, pushing the recipe does not have CI spend hours
# building it again, so a local build is published first.
#
# Uploading needs the R2_ACCOUNT_ID, R2_ACCESS_KEY_ID and R2_SECRET_ACCESS_KEY
# of a key that can write to the bucket, in the environment or in the file
# R2_ENV names. Finding that it is there needs neither.
#
# It also writes cmd/yaroze/wasm/full.json, unless told not to with
# NO_MANIFEST=1, which is what yaroze embeds to know what to download.
set -euo pipefail
DIR=${1:?usage: release.sh DIR}
HERE=$(cd "$(dirname "$0")" && pwd)
KEY=$("$HERE/key.sh")
BUCKET=psxsdk
PUBLIC=https://files.yaroze.dev
FILES=(tinygo-debug tinygo tinygo-full) # the last says the rest are there

WORK=$(mktemp -d)
trap 'rm -rf "$WORK"' EXIT
gzip -9 -n -c "$DIR/tinygo-full.wasm" > "$WORK/tinygo-full.wasm.gz"

if [ -z "${NO_MANIFEST:-}" ]; then
  sum=$(sha256sum "$WORK/tinygo-full.wasm.gz" | cut -d' ' -f1)
  size=$(wc -c < "$WORK/tinygo-full.wasm.gz")
  printf '{"url":"%s","sha256":"%s","size":%d}\n' \
    "$PUBLIC/compiler/$KEY/tinygo-full.wasm.gz" "$sum" "$size" > "$HERE/../../cmd/yaroze/wasm/full.json"
fi

# Cloudflare keeps a 404 for hours, so asking for the file itself before it is
# uploaded would have it missing for that long after. A query of its own asks
# without touching what the file's URL has cached.
if curl -fsI "$PUBLIC/compiler/$KEY/tinygo-full.wasm.gz?check=$(date +%s)" > /dev/null; then
  echo "release.sh: $KEY is there already"
  exit 0
fi

if [ -n "${R2_ENV:-}" ]; then
  # shellcheck disable=SC1090
  source "$R2_ENV"
fi
: "${R2_ACCOUNT_ID:?release.sh: set R2_ACCOUNT_ID, R2_ACCESS_KEY_ID and R2_SECRET_ACCESS_KEY, or R2_ENV to a file that does}"
export AWS_ACCESS_KEY_ID=$R2_ACCESS_KEY_ID AWS_SECRET_ACCESS_KEY=$R2_SECRET_ACCESS_KEY AWS_DEFAULT_REGION=auto
# R2 does not take the checksums the command sends by default.
export AWS_REQUEST_CHECKSUM_CALCULATION=when_required AWS_RESPONSE_CHECKSUM_VALIDATION=when_required
for f in "${FILES[@]}"; do
  [ -e "$WORK/$f.wasm.gz" ] || gzip -9 -n -c "$DIR/$f.wasm" > "$WORK/$f.wasm.gz"
  # A key names one build for good, so a copy never goes stale.
  aws s3 cp "$WORK/$f.wasm.gz" "s3://$BUCKET/compiler/$KEY/$f.wasm.gz" \
    --endpoint-url "https://$R2_ACCOUNT_ID.r2.cloudflarestorage.com" \
    --cache-control 'public, max-age=31536000, immutable' \
    --content-type application/gzip --no-progress
done
echo "release.sh: published $KEY at $PUBLIC/compiler/$KEY/"
