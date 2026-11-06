#!/usr/bin/env bash
# regress.sh [NAME...]: builds each example for Sony's library twice, onto
# .build/NAME-cimg with Sony's image and .build/NAME-cimg-clean with the clean
# one (make.sh builds it), and runs TestClean on the pair, several at once,
# printing a line per run as ../regress.sh does. Sony's side carries the clean
# font (sonyfont.py), and both run on PSX_BIOS, the retail BIOS unless set,
# since OpenBIOS prints a line inside Sony's first GsInitGraph. A name may carry
# a pad mask, as quest:0410. With no names it runs ../regress.sh's list. Logs
# land in .build/cimg-regress/. Run from the repository root.
set -euo pipefail
IMAGE=$PWD/.build/cleanimage/libps.exe
[ -f "$IMAGE" ] || { echo "no $IMAGE: run exp/cleanlib/image/make.sh" >&2; exit 1; }
export PSX_SONY_LIBPS=${PSX_SONY_LIBPS:-$HOME/src/ChrisRx/psxsdk-sony/libps.exe.gz}
[ -f "$PSX_SONY_LIBPS" ] || { echo "no $PSX_SONY_LIBPS: comparing needs a private copy of Sony's image, spec 15" >&2; exit 1; }
SONY=$PWD/.build/cleanimage/sony-cleanfont.exe
python3 -I exp/cleanlib/image/sonyfont.py "$SONY"
export PSX_BIOS=${PSX_BIOS:-$HOME/.mednafen/firmware/scph1001.bin}
if [ $# -gt 0 ]; then
  LIST=("$@")
else
  eval "$(sed -n '/^LIST=(/,/^)/p' exp/cleanlib/regress.sh)"
fi
FRAMES=${PSX_CLEAN_FRAMES:-300}
OUT=.build/cimg-regress
mkdir -p "$OUT"
go build -o .build/yaroze-cimg ./cmd/yaroze
go test -c -o .build/cleantest ./tests

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
  mkdir -p ".build/$n-cimg" ".build/$n-cimg-clean"
  PSX_LIBPS_IMAGE=$SONY .build/yaroze-cimg disc --elf ".build/$n-cimg/prog.elf" -o ".build/$n-cimg/$n-cimg.bin" "$d" >".build/$n-cimg/build.log" 2>&1 || {
    echo "FAILED $n-cimg"; tail -3 ".build/$n-cimg/build.log"; return 1
  }
  PSX_LIBPS_IMAGE=$IMAGE .build/yaroze-cimg disc --elf ".build/$n-cimg-clean/prog.elf" -o ".build/$n-cimg-clean/$n-cimg-clean.bin" "$d" >".build/$n-cimg-clean/build.log" 2>&1 || {
    echo "FAILED $n-cimg-clean"; tail -3 ".build/$n-cimg-clean/build.log"; return 1
  }
}
run() {
  local item=$1 name=${1%%:*} pad=
  [[ $item == *:* ]] && pad=${item#*:}
  local log=$PWD/$OUT/${item/:/-}.log
  (cd tests && PSX_CLEAN=$name PSX_CLEAN_REF=cimg PSX_CLEAN_PAD=$pad PSX_CLEAN_FRAMES=$FRAMES \
    ../.build/cleantest -test.run 'TestClean$' -test.v >"$log" 2>&1) && res=PASS || res=FAIL
  local swaps diff
  swaps=$(grep -o 'swaps: reference [0-9]*, clean [0-9]*' "$log" | head -1 || true)
  diff=; [ $res = FAIL ] && diff=$(grep -m1 -E 'differ|stopped|different numbers|fault|panic' "$log" | sed 's/^ *//' || true)
  if grep -q -- '--- SKIP' "$log"; then
    res=TIME
    diff=$(grep -m1 'timing only:' "$log" | sed 's/^.*timing only: //' || true)
  fi
  printf '%-14s %s  %s  %s\n' "$item" "$res" "${swaps#swaps: }" "$diff"
}
export -f dir build run
export IMAGE SONY OUT FRAMES
printf '%s\n' "${LIST[@]}" | sed 's/:.*//' | sort -u | xargs -P 4 -I{} bash -c 'build {}'
printf '%s\n' "${LIST[@]}" | xargs -P "${JOBS:-8}" -I{} bash -c 'run {}' | sort
