---
title: Making data
weight: 3
description: Turning pictures, sounds, MIDI files and models into the files a program reads off its disc, with the converters, and planning video and sound memory for them.
---

A program's pictures, sounds, music and models are files on its disc in the console's own formats. The converters here make each of them from the files a modern editor saves, and every one is tested against what Sony's own tools made of the same input:

| What | From | With | In the program |
| --- | --- | --- | --- |
| A sprite, texture or background | PNG, BMP, GIF | [`png2tim`](../tools/png2tim.md) | `yaroze.TIM(f.Data()).Load()`, and `.Sprite()` |
| A model | OBJ and MTL | [`obj2tmd`](../tools/obj2tmd.md) | `yaroze.TMD`, `Map`, `Object.Link` |
| One of Sony's RSD models | RSD, PLY, MAT | [`rsdlink`](../tools/rsdlink.md) | as above |
| A sound | WAV, AIFF | [`wav2vag`](../tools/wav2vag.md) | in a bank |
| A bank of sounds | VAGs and a definition | [`mkvab`](../tools/mkvab.md) | `yaroze.OpenBank(vh, vb)` |
| Music | MIDI | [`smf2seq`](../tools/smf2seq.md) | `bank.Sequence(seq)`, `Play` |
| A scene of models, lights and sprites | scenes/NAME.json, from the playground's editor | [`mkscene`](../tools/mkscene.md) | `scene.Scene`, `Load`, `Draw` |
| Many files packed as one | the files under pack/NAME/ | [`mkarc`](../tools/mkarc.md) | `archive.Open`, `Lookup`, `Unpack` |

## A pipeline that remakes itself

The converters run from `go:generate` lines in the program's `main.go`, so that `go generate` makes the files under `assets/` from the sources beside them, and then declares them for the disc with [`discgen`](../tools/discgen.md), last, which leaves the sources off the disc: pictures, models, sounds and scores by their extension, and whatever a line reads:

```go
//go:generate mkdir -p assets
//go:generate go run go.chrisrx.dev/psx/tools/png2tim --at 640,0 --clut 0,481 -o assets/SHIP.TIM assets/ship.png
//go:generate go run go.chrisrx.dev/psx/tools/png2tim --at 704,0 --clut 0,483 -o assets/STONE.TIM assets/stone.png
//go:generate go run go.chrisrx.dev/psx/tools/obj2tmd --scale 64 -o assets/CUBE.TMD assets/cube.obj
//go:generate go run go.chrisrx.dev/psx/tools/wav2vag assets/tone.wav assets/blip.wav
//go:generate go run go.chrisrx.dev/psx/tools/mkvab -f assets/sounds.def -o assets/SOUNDS.VAB --split assets/tone.VAG assets/blip.VAG
//go:generate go run go.chrisrx.dev/psx/tools/smf2seq -o assets/TUNE.SEQ assets/tune.mid
//go:generate go run go.chrisrx.dev/psx/tools/discgen assets
```

The examples commit the generated files under `assets/` as well, beside their sources, so that a build needs no generating.

## Video memory

Every image goes to a place in video memory that its TIM records, given to `png2tim` with `--at` for the pixels and `--clut` for the palette, and planning those places is the one part of the work no tool does. Video memory is 1024 by 512 units:

```pikchr
# Video memory to scale, 1024 by 512 units, from its top left corner.
$sx = 6.0 / 1024
$sy = 3.6 / 512
B0: box "display" "buffer 0" with nw at (0, 0) wid 320*$sx ht 240*$sy
B1: box "display" "buffer 1" with nw at B0.sw wid 320*$sx ht 240*$sy
FREE: box "free" with nw at B0.ne wid 320*$sx ht 480*$sy
TEX: box with nw at FREE.ne wid 384*$sx ht 480*$sy
PAL: box "palettes, a row each, x a multiple of 16" with nw at B1.sw wid 1024*$sx ht 32*$sy

# Texture pages, 64 units wide and 256 lines tall, under their label.
text "textures, in pages" ljust at (650*$sx, -22*$sy)
text "64 units wide and 256 tall" italic small ljust at (650*$sx, -48*$sy)
line dashed from (704*$sx, -70*$sy) to (704*$sx, -480*$sy)
line dashed from (768*$sx, -70*$sy) to (768*$sx, -480*$sy)
line dashed from (832*$sx, -70*$sy) to (832*$sx, -480*$sy)
line dashed from (896*$sx, -70*$sy) to (896*$sx, -480*$sy)
line dashed from (960*$sx, -70*$sy) to (960*$sx, -480*$sy)
line dashed from (640*$sx, -256*$sy) to (1024*$sx, -256*$sy)

# The coordinates along the top and down the left.
text "0" above at (0, 0)
text "320" above at (320*$sx, 0)
text "640" above at (640*$sx, 0)
text "1023" above at (1024*$sx, 0)
text "0" rjust at (-0.06, 0)
text "240" rjust at (-0.06, -240*$sy)
text "480" rjust at (-0.06, -480*$sy)
text "511" rjust at (-0.06, -512*$sy)
```

That is the default `Screen`, 320 by 240 with its two buffers one above the other. `SideBySide` puts them both at the top, which frees the rows below for images. A texture lies inside one texture page, 64 units wide and 256 lines tall, which is 256 pixels of a 4-bit image, 128 of an 8-bit one and 64 of a 16-bit one, and several small images can share a page. `Screen.Font` puts the font at the corner it is given, with its palette 128 lines below. Two images must not overlap, and `png2tim` prints where each one goes, with `-r` for a TIM already made.

A model's TMD names its textures by where they are, so the TIM a model's material names is converted first, and the program loads that same TIM before it draws the model.

## Sound memory

A bank's samples go to the SPU's 512 KB. A second of sound at 22050 Hz is about 12.6 KB of samples, and a bank's definition says which sample each program plays at which notes. The boot disc's General MIDI bank, STD0, plays any General MIDI score but takes about 450 KB, so a game with sound effects of its own makes a bank of the instruments its music uses as well, with the programs numbered as the MIDI file numbers them. [`mkvab`](../tools/mkvab.md) gives a definition to start from, and `mkvab -r` prints the definition of any bank, a 1998 game's included.

## Older data

[`png2tim -r`](../tools/png2tim.md) turns any TIM into a PNG, `mkvab -r` reads any bank and [`rsdlink`](../tools/rsdlink.md) links any of Sony's RSD models, so the art and sound of the Net Yaroze samples, and of games whose files are on their discs, can be looked at, changed and put back.
