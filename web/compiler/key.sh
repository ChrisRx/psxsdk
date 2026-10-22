#!/usr/bin/env bash
# Prints the compiler's key: TinyGo's revision and a hash of what build.sh
# reads: this directory's files, less those git ignores, the Markdown, the
# scripts that name and publish a build, and mkcache.sh, which runs after
# the build over committed sources and so changes nothing in it, the
# PlayStation files laid over TinyGo, and the flake's lock. The cache entry
# and the release the compiler is kept in are both named by it, and it
# hashes the files as they are in the working tree, so a local run names
# the release CI looks for once the change is committed.
set -euo pipefail
cd "$(dirname "$0")/../.."
rev=$(sed -n 's/.*rev = "\([0-9a-f]\{40\}\)".*/\1/p' flake.nix | head -1)
sum=$(git ls-files -z --cached --others --exclude-standard -- web/compiler ':!*.md' ':!web/compiler/key.sh' ':!web/compiler/release.sh' ':!web/compiler/mkcache.sh' internal/toolchain/_tinygo internal/toolchain/tinygo-hot.patch flake.lock | LC_ALL=C sort -z | xargs -0 sha256sum | sha256sum | cut -c1-16)
echo "compiler-${rev:0:12}-$sum"
