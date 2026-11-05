#!/usr/bin/env bash
# archive.sh [NAME...]: builds the web archive's original programs on Sony's image
# (with the clean font, as regress.sh does) and on the clean one (make.sh builds
# it), onto .build/carchive/NAME-sony and NAME-clean, with the program's directory
# as the disc's root, and compares them with probetest's TestArchive on PSX_BIOS,
# the retail BIOS unless set. It writes .build/carchive/summary.txt, a line a
# program. With names it runs those programs and prints their lines. FRAMES (1200)
# sets the length of a run. FIRST=1 also writes .build/carchive/first.txt and first/,
# where each program first differs, with its GPU words. Run from the repository root.
set -uo pipefail
IMAGE=$PWD/.build/cleanimage/libps.exe
[ -f "$IMAGE" ] || { echo "no $IMAGE: run exp/cleanlib/image/make.sh" >&2; exit 1; }
SONY=$PWD/.build/cleanimage/sony-cleanfont.exe
python3 -I exp/cleanlib/image/sonyfont.py "$SONY"
export PSX_BIOS=${PSX_BIOS:-$HOME/.mednafen/firmware/scph1001.bin}
OUT=.build/carchive
mkdir -p "$OUT"
python3 -I exp/cleanlib/image/archive.py "$HOME/src/ChrisRx/yaroze_web_archive/site" "$OUT"
go build -o "$OUT/eco2exe" ./tools/eco2exe
go build -o "$OUT/mkdisc" ./tools/mkdisc
go test -c -o "$OUT/probetest.test" ./exp/cleanlib/image/probetest

build() {
  local name=$1 root=$2 exe=$3 loads=$4 side=$5 o args=() l
  o=$OUT/$name-$side
  mkdir -p "$o"
  [ "$loads" = - ] || for l in $loads; do args+=(--load "$l"); done
  if [ "$side" = clean ]; then export PSX_LIBPS_IMAGE=$IMAGE; else export PSX_LIBPS_IMAGE=$SONY; fi
  "$OUT/eco2exe" "${args[@]}" "$exe" "$o/ZBOOT.EXE" >"$o/build.log" 2>&1 || { echo "$name-$side: eco2exe failed"; return; }
  "$OUT/mkdisc" -o "$o/$name-$side.bin" --volume-id DISC --boot ZBOOT.EXE --file "ZBOOT.EXE=$o/ZBOOT.EXE" "$root" >>"$o/build.log" 2>&1 ||
    echo "$name-$side: mkdisc failed: $(tail -1 "$o/build.log")"
}
export -f build
export IMAGE SONY OUT
wanted=" $* "
while IFS=$'\t' read -r name root exe loads; do
  [ $# -eq 0 ] || [[ $wanted == *" $name "* ]] || continue
  for side in sony clean; do printf '%s\t%s\t%s\t%s\t%s\n' "$name" "$root" "$exe" "${loads:--}" "$side"; done
done <"$OUT/programs.tsv" | xargs -P "$(nproc)" -d '\n' -I{} bash -c 'IFS=$'"'"'\t'"'"' read -r a b c d e <<< "{}"; build "$a" "$b" "$c" "$d" "$e"'

run=TestArchive
[ $# -eq 0 ] || run="TestArchive/programs/($(IFS='|'; echo "$*"))\$"
(cd exp/cleanlib/image/probetest && ARCHIVE_FRAMES=${FRAMES:-1200} ARCHIVE_ONLY="$*" "$PWD/../../../../$OUT/probetest.test" \
  -test.run "$run" -test.timeout 60m -test.parallel "${JOBS:-20}" -archive) >"$OUT/test.log" 2>&1
tail -1 "$OUT/test.log" | cut -c1-120
if [ -n "${FIRST:-}" ]; then
  (cd exp/cleanlib/image/probetest && ARCHIVE_FIRST="$*" "$PWD/../../../../$OUT/probetest.test" \
    -test.run 'TestArchiveFirst$' -test.timeout 30m -test.parallel "${JOBS:-20}" -archive) >>"$OUT/test.log" 2>&1
fi
if [ $# -gt 0 ]; then sort "$OUT/summary-only.txt"; else sort -o "$OUT/summary.txt" "$OUT/summary.txt"; awk '{print $2}' "$OUT/summary.txt" | sort | uniq -c; fi
