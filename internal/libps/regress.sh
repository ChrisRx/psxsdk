#!/usr/bin/env bash
# regress.sh [NAME...]: builds each example against the clean library and
# runs TestClean on it next to its reference (.build/NAME-ref, which refs.sh
# builds), several at once, printing a line per run: PASS, FAIL or TIME (a
# run TestClean lists as differing only in timing, which differed), the swap
# counts, and the first difference or the reason. With no names it runs the
# whole list below. A name may carry a pad mask, as quest:0410, or a comma list
# of them (PSX_CLEAN_PAD). The card runs format, save and then list, load,
# dump or delete.
# Logs land in .build/regress/. Run from the repository root.
set -euo pipefail
LIST=(
  blank pads pads:4010 isbg isbg1 flush split split:9031 jumpcube jumpcube:4020
  quest quest:0410 quest:4010 quest:0020 car car:9410 scene scene:4038 cdda
  timers flame morph offscr oden dungeon tunnel tunnel:0800 clone
  breakout invaders square dogtale check dino terra cdda:4000
  card card:0010,0010,4000,0040,0040,4000,0040,0040,4000
  card:0010,0010,4000,0040,0040,4000,0040,4000
  card:0010,0010,4000,0040,0040,4000,0010,0010,0010,4000,4000
  card:0010,0010,4000,0040,0040,4000,0040,0040,0040,4000,4000
)
[ $# -gt 0 ] && LIST=("$@")
declare -A DIRS=(
  [isbg]=internal/libps/spec/probes/isbg
  [isbg1]=internal/libps/spec/probes/isbg1
  [flush]=internal/libps/spec/probes/flush
)
FRAMES=${PSX_CLEAN_FRAMES:-300}
OUT=.build/regress
mkdir -p "$OUT"

declare -A built
for item in "${LIST[@]}"; do
  name=${item%%:*}
  [ -n "${built[$name]:-}" ] && continue
  built[$name]=1
  internal/libps/build.sh "$name-clean" "${DIRS[$name]:-examples/$name}" >/dev/null
done
go test -c -o .build/cleantest ./tests

run() {
  local item=$1 name=${1%%:*} pad=
  [[ $item == *:* ]] && pad=${item#*:}
  local log=$PWD/$OUT/${item/:/-}.log
  (cd tests && PSX_CLEAN=$name PSX_CLEAN_PAD=$pad PSX_CLEAN_FRAMES=$FRAMES \
    ../.build/cleantest -test.run 'TestClean$' -test.v >"$log" 2>&1) && res=PASS || res=FAIL
  local swaps diff
  swaps=$(grep -o 'swaps: reference [0-9]*, clean [0-9]*' "$log" | head -1 || true)
  diff=; [ $res = FAIL ] && diff=$(grep -m1 -E 'differ|stopped|different numbers' "$log" | sed 's/^ *//' || true)
  if grep -q -- '--- SKIP' "$log"; then
    res=TIME
    diff=$(grep -m1 'timing only:' "$log" | sed 's/^.*timing only: //' || true)
  fi
  printf '%-14s %s  %s  %s\n' "$item" "$res" "${swaps#swaps: }" "$diff"
}
export -f run
export OUT FRAMES
printf '%s\n' "${LIST[@]}" | xargs -P "${JOBS:-8}" -I{} bash -c 'run {}' | sort
