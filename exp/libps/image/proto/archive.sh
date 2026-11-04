#!/usr/bin/env bash
# archive.sh: builds every program archive.py listed onto
# .build/archive/NAME-V/NAME-V.cue, V sony for Sony's library and go for
# the Go library's image, with the program's directory as the disc's root
# for the files it reads from the CD. Run from the worktree root.
set -uo pipefail
build() {
  name=$1 root=$2 exe=$3 loads=$4 v=$5
  o=.build/archive/$name-$v
  mkdir -p "$o"
  args=()
  [ "$loads" = - ] || for l in $loads; do args+=(--load "$l"); done
  if [ "$v" = go ]; then export PSX_LIBPS_IMAGE=$PWD/.build/image/libps.exe; else unset PSX_LIBPS_IMAGE; fi
  if ! .build/archive/eco2exe "${args[@]}" "$exe" "$o/ZBOOT.EXE" > "$o/build.log" 2>&1; then echo "$name-$v: eco2exe failed"; return; fi
  if ! .build/archive/mkdisc -o "$o/$name-$v.bin" --volume-id DISC --boot ZBOOT.EXE --file "ZBOOT.EXE=$o/ZBOOT.EXE" "$root" >> "$o/build.log" 2>&1; then echo "$name-$v: mkdisc failed: $(tail -1 "$o/build.log")"; fi
}
export -f build
while IFS=$'\t' read -r name root exe loads; do
  for v in ${ARCHIVE_SIDES:-sony go}; do printf '%s\t%s\t%s\t%s\t%s\n' "$name" "$root" "$exe" "${loads:--}" "$v"; done
done < .build/archive/programs.tsv | xargs -P "$(nproc)" -d '\n' -I{} bash -c 'IFS=$'"'"'\t'"'"' read -r a b c d e <<< "{}"; build "$a" "$b" "$c" "$d" "$e"'
