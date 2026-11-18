#!/usr/bin/env bash
# terra.sh: builds Team Fatal's original Terra Incognita, a program no one can
# rebuild, on Sony's image and on the clean one (make.sh builds it), onto
# .build/terra-cimg and .build/terra-cimg-clean, and compares them for 4,500
# frames with probetest's TestTerra and TestTerraStack. It needs the web archive
# and the retail kernel in the places below. Run from the repository root.
set -euo pipefail
A=$HOME/src/ChrisRx/yaroze_web_archive/site/yaroze_scea/showcase/terrai_n.zip.x
export PSX_BIOS=${PSX_BIOS:-$HOME/.mednafen/firmware/scph1001.bin}
export PSX_SONY_LIBPS=${PSX_SONY_LIBPS:-$HOME/src/ChrisRx/psxsdk-sony/libps.exe.gz}
[ -f "$PSX_SONY_LIBPS" ] || { echo "no $PSX_SONY_LIBPS: comparing needs a private copy of Sony's image, spec 15" >&2; exit 1; }
SONY=$PWD/.build/cleanimage/sony.exe
gunzip -c "$PSX_SONY_LIBPS" >"$SONY"
args=()
for f in examples/terra/assets/SOUND/*; do args+=(--file "DATA/SOUND/$(basename "$f")=$f"); done
build() {
  local o=.build/$1
  mkdir -p "$o"
  go run ./tools/eco2exe --load "$A/DATA1.ARC@80090000" --load "$A/DATA2.ARC@80098000" --load "$A/DATA3.ARC@800e0000" "$A/TERRA.EXE" "$o/MAIN.EXE" | tail -1
  go run ./tools/mkdisc -o "$o/$1.bin" --volume-id TERRA --boot MAIN.EXE --file "MAIN.EXE=$o/MAIN.EXE" "${args[@]}" | tail -1
}
PSX_LIBPS_IMAGE=$SONY build terra-cimg
PSX_LIBPS_IMAGE=$PWD/.build/cleanimage/libps.exe build terra-cimg-clean
go test -count=1 -v -timeout 40m -run 'TestTerra' ./internal/libps/image/probetest -args -terra 2>&1 | grep -vE '^=== |^ +[0-9a-f]{8}: ' | cut -c1-200 | tail -${TAIL:-200}
