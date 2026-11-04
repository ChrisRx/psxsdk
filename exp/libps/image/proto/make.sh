#!/usr/bin/env bash
# make.sh: builds the image ELF, removes its traps and writes .build/image/libps.exe.
set -euo pipefail
HERE=$(dirname "$(realpath "$0")")
"$HERE/build.sh" | grep -E 'packed|error|UNPLACED|undefined' || true
go run ./tools/mips1fix .build/image/image.elf | tail -1
go run ./exp/libps/image/proto/imgtool .build/image/image.elf .build/image/libps.exe
