#!/usr/bin/env bash
# refs.sh: rebuilds the reference discs (.build/NAME-ref) of regress.sh's
# list against exp/libps, four at a time. exp/libps left the tree, so run it in a
# worktree of the branch sony-reference (spec 15), and copy .build/NAME-ref back,
# and never alongside regress.sh. A disc
# that fails to build is named with the end of its log, and the script exits
# nonzero once the others are done.
set -euo pipefail
NAMES="hello pads isbg isbg1 flush split jumpcube quest car scene cdda timers flame morph offscr oden dungeon tunnel clone breakout invaders square dogtale check dino terra card"
dir() {
  case $1 in
    hello) echo docs/tutorial/hello ;;
    isbg|isbg1|flush) echo exp/cleanlib/spec/probes/$1 ;;
    *) echo examples/$1 ;;
  esac
}
build() {
  local n=$1 d
  d=$(dir "$n")
  mkdir -p ".build/$n-ref"
  if go run ./cmd/yaroze disc --libps go --elf ".build/$n-ref/prog.elf" -o ".build/$n-ref/$n-ref.bin" "$d" >".build/$n-ref/build.log" 2>&1; then
    echo "built $n-ref"
  else
    echo "FAILED $n-ref"; tail -3 ".build/$n-ref/build.log"
    return 1
  fi
}
export -f build dir
printf '%s\n' $NAMES | xargs -P 4 -I{} bash -c 'build {}'
