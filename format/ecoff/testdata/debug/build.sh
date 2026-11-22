#!/bin/sh
# Builds steps and vars, the ECOFFs the debug information tests read, with GCC 2.95.3
# for mipsel-ecoff: yaroze-toolchain's legacy toolchain, or its archive from
# https://files.yaroze.dev/toolchains/, on PATH. GCC records the directory
# it compiled in, so the build is in a fixed one, /tmp/ecoff-debug, for
# the same file from any checkout.
set -e
here=$(cd "$(dirname "$0")" && pwd)
dir=/tmp/ecoff-debug
rm -rf "$dir"
mkdir -p "$dir"
cp "$here/start.s" "$here/steps.c" "$here/scale.c" "$here/vars.c" "$dir/"
cd "$dir"
mipsel-ecoff-gcc -c -o start.o start.s
mipsel-ecoff-gcc -O2 -msoft-float -G0 -g -c -o steps.o steps.c
mipsel-ecoff-gcc -O2 -msoft-float -G0 -c -o scale.o scale.c
mipsel-ecoff-gcc -nostdlib -msoft-float -Wl,-Ttext,0x80140000,-e,_start -o steps start.o steps.o scale.o
cp steps "$here/steps"
mipsel-ecoff-gcc -O0 -msoft-float -G0 -g -c -o vars.o vars.c
mipsel-ecoff-gcc -nostdlib -msoft-float -Wl,-Ttext,0x80140000,-e,_start -o vars start.o vars.o
cp vars "$here/vars"
rm -rf "$dir"
