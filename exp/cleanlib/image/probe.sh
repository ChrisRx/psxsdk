#!/usr/bin/env bash
# probe.sh NAME DIR: builds the probe in DIR for Sony's library, and runs the one
# program on Sony's image and on the clean one (make.sh builds it), onto
# .build/NAME-cimg/out and .build/NAME-cimg-clean/out, each with .tty and
# .kernel files (probetest's TestDump). A probe ends its run by printing the
# line "END". It prints the first lines that differ. SIDES=sony runs Sony's side
# only, PROBE_FRAMES caps the run (6000). Run from the repository root.
set -euo pipefail
NAME=$1 DIR=$2
IMAGE=$PWD/.build/cleanimage/libps.exe
SIDES=${SIDES:-sony clean}
BIN=$PWD/.build/$NAME-tools
mkdir -p "$BIN"
go build -o "$BIN/yaroze" ./cmd/yaroze
go test -c -o "$BIN/probetest.test" ./exp/cleanlib/image/probetest
run() {
  local dir=$PWD/.build/$1
  (cd exp/cleanlib/image/probetest && PROBE_DISC=$dir/$1.cue PROBE_OUT=$dir/out PROBE_FRAMES=${PROBE_FRAMES:-6000} PROBE_END=$'END\n' \
    "$BIN/probetest.test" -test.run 'TestDump$' >"$dir/probe.log" 2>&1) || { tail -5 "$dir/probe.log"; exit 1; }
}
for side in $SIDES; do
  case $side in
    sony)
      mkdir -p ".build/$NAME-cimg"
      "$BIN/yaroze" disc --elf ".build/$NAME-cimg/prog.elf" -o ".build/$NAME-cimg/$NAME-cimg.bin" "$DIR"
      run "$NAME-cimg"
      ;;
    clean)
      [ -f "$IMAGE" ] || { echo "no $IMAGE: run exp/cleanlib/image/make.sh" >&2; exit 1; }
      mkdir -p ".build/$NAME-cimg-clean"
      PSX_LIBPS_IMAGE=$IMAGE "$BIN/yaroze" disc --elf ".build/$NAME-cimg-clean/prog.elf" -o ".build/$NAME-cimg-clean/$NAME-cimg-clean.bin" "$DIR"
      run "$NAME-cimg-clean"
      ;;
  esac
done
if [ -f ".build/$NAME-cimg/out.tty" ] && [ -f ".build/$NAME-cimg-clean/out.tty" ]; then
  if cmp -s ".build/$NAME-cimg/out.tty" ".build/$NAME-cimg-clean/out.tty"; then
    echo "$NAME: TTY identical, $(wc -l < ".build/$NAME-cimg/out.tty") lines"
  else
    echo "$NAME: TTY differs"
    diff ".build/$NAME-cimg/out.tty" ".build/$NAME-cimg-clean/out.tty" | head -${DIFFLINES:-20} || true
  fi
fi
