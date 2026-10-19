---
title: Making data
weight: 4
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

## A pipeline that remakes itself

The converters run from `go:generate` lines in the program's `main.go`, so that `go generate` makes every file under `data/` from the sources under `art/`, and then declares them for the disc with [`discgen`](../tools/discgen.md), last:

```go
//go:generate mkdir -p data
//go:generate go run go.chrisrx.dev/psx/tools/png2tim --at 640,0 --clut 0,481 -o data/SHIP.TIM art/ship.png
//go:generate go run go.chrisrx.dev/psx/tools/png2tim --at 704,0 --clut 0,483 -o data/STONE.TIM art/stone.png
//go:generate go run go.chrisrx.dev/psx/tools/obj2tmd --scale 64 -o data/CUBE.TMD art/cube.obj
//go:generate go run go.chrisrx.dev/psx/tools/wav2vag art/tone.wav art/blip.wav
//go:generate go run go.chrisrx.dev/psx/tools/mkvab -f art/sounds.def -o data/SOUNDS.VAB --split art/tone.VAG art/blip.VAG
//go:generate go run go.chrisrx.dev/psx/tools/smf2seq -o data/TUNE.SEQ art/tune.mid
//go:generate go run go.chrisrx.dev/psx/tools/discgen data
```

The examples commit what is under `data/` as well, so that a build needs no generating.

## Video memory

Every image goes to a place in video memory that its TIM records, given to `png2tim` with `--at` for the pixels and `--clut` for the palette, and planning those places is the one part of the work no tool does. Video memory is 1024 by 512 units:

```
x:   0          320        640                     1023
y:0  +----------+----------+------------------------+
     | display  | free     | textures, in pages     |
     | buffer 0 |          | 64 units wide: 640,    |
240  +----------+          | 704, 768 ...           |
     | display  |          |                        |
     | buffer 1 |          |                        |
480  +----------+----------+------------------------+
     | palettes, a row each, x a multiple of 16      |
511  +-----------------------------------------------+
```

That is the default `Screen`, 320 by 240 with its two buffers one above the other. `SideBySide` puts them both at the top, which frees the rows below for images. A texture lies inside one texture page, 64 units wide and 256 lines tall, which is 256 pixels of a 4-bit image, 128 of an 8-bit one and 64 of a 16-bit one, and several small images can share a page. `Screen.Font` puts the font at the corner it is given, with its palette 128 lines below. Two images must not overlap, and `png2tim` and `tim2png` print where each one goes.

A model's TMD names its textures by where they are, so the TIM a model's material names is converted first, and the program loads that same TIM before it draws the model.

## Sound memory

A bank's samples go to the SPU's 512 KB. A second of sound at 22050 Hz is about 12.6 KB of samples, and a bank's definition says which sample each program plays at which notes. The boot disc's General MIDI bank, STD0, plays any General MIDI score but takes about 450 KB, so a game with sound effects of its own makes a bank of the instruments its music uses as well, with the programs numbered as the MIDI file numbers them. [`mkvab`](../tools/mkvab.md) gives a definition to start from, and `mkvab -r` prints the definition of any bank, a 1998 game's included.

## Older data

[`tim2png`](../tools/png2tim.md) turns any TIM into a PNG, `mkvab -r` reads any bank and [`rsdlink`](../tools/rsdlink.md) links any of Sony's RSD models, so the art and sound of the Net Yaroze samples, and of games whose files are on their discs, can be looked at, changed and put back.
