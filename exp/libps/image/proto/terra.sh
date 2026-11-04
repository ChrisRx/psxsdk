#!/usr/bin/env bash
# terra.sh V: builds Team Fatal's TERRA.EXE onto .build/terra-orig-V, on
# Sony's library (sony) or the Go library's image (go), with its sound files.
set -euo pipefail
v=$1
A=$HOME/src/ChrisRx/yaroze_web_archive/site/yaroze_scea/showcase/terrai_n.zip.x
o=.build/terra-orig-$v
mkdir -p "$o"
if [ "$v" = go ]; then export PSX_LIBPS_IMAGE=$PWD/.build/image/libps.exe; fi
go run ./tools/eco2exe --load "$A/DATA1.ARC@80090000" --load "$A/DATA2.ARC@80098000" --load "$A/DATA3.ARC@800e0000" "$A/TERRA.EXE" "$o/MAIN.EXE" | tail -1
args=()
for f in examples/terra/assets/SOUND/*; do args+=(--file "DATA/SOUND/$(basename "$f")=$f"); done
go run ./tools/mkdisc -o "$o/terra-orig-$v.bin" --volume-id TERRA --boot MAIN.EXE --file "MAIN.EXE=$o/MAIN.EXE" "${args[@]}" | tail -1
