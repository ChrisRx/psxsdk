#!/usr/bin/env bash
# Publishes the compiler in DIR (tinygo.wasm, tinygo-full.wasm and
# tinygo-debug.wasm) as a release named by its key (key.sh), unless that
# release exists already. A release, unlike the cache or a workflow's
# artifact, is kept until it is deleted, so the Pages workflow finds the
# compiler however long it has been since one was built. The release is a
# prerelease, which GitHub never marks latest, so latest stays the SDK's:
# --latest=false alone does not keep it from being. Needs gh, logged in or
# with GH_TOKEN set, and TARGET, the commit the release's tag is made at
# when it is new (main by default).
set -euo pipefail
DIR=${1:?usage: release.sh DIR}
HERE=$(cd "$(dirname "$0")" && pwd)
KEY=$("$HERE/key.sh")
# gh release create makes a draft, uploads the files to it and then
# publishes it, so an interrupted run leaves a draft, which is deleted and
# made again rather than taken for the release.
if draft=$(gh release view "$KEY" --json isDraft --jq .isDraft 2>/dev/null); then
  if [ "$draft" != true ]; then
    echo "release.sh: $KEY exists already"
    exit 0
  fi
  echo "release.sh: deleting the draft $KEY an interrupted run left"
  gh release delete "$KEY" --yes
fi
gh release create "$KEY" "$DIR"/tinygo.wasm "$DIR"/tinygo-full.wasm "$DIR"/tinygo-debug.wasm \
  --target "${TARGET:-main}" --prerelease --latest=false \
  --title "Playground compiler $KEY" \
  --notes "The playground's compiler, TinyGo compiled to WebAssembly, for the recipe in web/compiler that key.sh names $KEY. The Pages workflow downloads it when its cache misses."
