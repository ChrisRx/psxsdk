#!/usr/bin/env python3
# sonyfont.py OUT: writes Sony's resident image, from the private copy
# PSX_SONY_LIBPS names (spec 15), with its font replaced by the clean library's,
# so that comparing pictures with the clean image shows what the library does and
# not whose font it draws. The output is Sony's bytes: it stays in .build. Run from
# the repository root.
import gzip, os, re, struct, sys

FONT = 0x80047A60  # Sony's font, 512 words
WORDS = 512

src = open("exp/cleanlib/fontdata.go").read()
font = [int(x, 16) for x in re.findall(r"0x[0-9a-f]{8}", src[src.index("[512]uint32"):])]
if len(font) != WORDS:
    sys.exit(f"exp/cleanlib/fontdata.go: {len(font)} words, want {WORDS}")
image = bytearray(gzip.open(os.environ["PSX_SONY_LIBPS"]).read())
load = struct.unpack_from("<I", image, 0x18)[0]
struct.pack_into(f"<{WORDS}I", image, 0x800 + FONT - load, *font)
open(sys.argv[1], "wb").write(image)
