---
title: The library in Go
weight: 4
description: The Net Yaroze library reimplemented in Go under exp/, how a program is built against it instead of the resident image, what it costs and saves, and the exp/ps API over it.
---

A Net Yaroze program calls `libps`, the library the boot disc leaves resident in RAM, and [`eco2exe`](../tools/eco2exe.md) combines Sony's image of it into every executable. `exp/libps` is that library written in Go, from the library's own code read with [`libpsdis`](../tools/libpsdis.md), and `exp/ps` is an ordinary Go API over it. Both are experimental, both live under `exp/`, and nothing on the default path uses them: the `yaroze` package, which [programs are written against](../guide/writing-programs.md), keeps the resident library.

## Two ways in

`yaroze build --libps go`, and the same flag on `disc` and `run`, build a program written against the `yaroze` package with the Go library linked in. The bindings' C names resolve to the library's exports, so the program is unchanged, and the twin program under `exp/libps/twin` is what says the two libraries agree: it calls each function of the original and its Go version on the same inputs and compares every packet, every register and everything printed.

A program that imports `exp/ps` is built against the Go library without a flag, since it cannot run on the resident one: the build command reads the main package's imports and picks the target.

## What changes in the build

The program is a plain PSX-EXE. It loads at `0x80010000`, where the resident image used to sit, and nothing is combined into it: `yaroze` writes it with `link.Standalone` rather than through `eco2exe`, and the emulator's skip of the resident library's wait for the vertical blank does not apply, since the wait is now the program's own loop.

Only what the program calls is linked, so the executable shrinks from the 532KB every program carried to 27KB for the tutorial's first program and 115KB for one that draws in 3D, and the disc and its boot shrink with it. The 512KB the resident build reserved under the program is the program's. And the compiler sees through the library's calls: it inlines them, drops what a program never reaches, and can fix what the original got wrong, where the resident library is Sony's bytes as they were.

Nothing of Sony's is in the output. The library's font is its own, drawn in `exp/libps/gen/font`; its tables of sines and square roots are generated from their formulas, and the rotation table the original carried folds out of the sine table; and the patch the original wrote over the kernel's exception handler is assembled from its instructions and applied only over a handler whose hash is Sony's. The `sonydata` build tag links the original's font in place of the library's own, for the twin and for the ports under `exp/examples`, whose pictures are compared with the originals' pixel for pixel; a program built without it, which is every program built with `yaroze`, has the library's own.

[Hot reload](../tools/yaroze.md) works against the Go library as it does against the resident one, with `--hot` or the playground's Hot box: the library's variables are the running program's, bound by name like the program's own. A hot build keeps every variable the size it was declared, since the optimizer would otherwise narrow one to what the program happens to store in it, and a patch built from a changed program could see it differently.

## What it costs

Measured in the emulator's cycles over the part of a frame outside the wait for the vertical blank, `PSX_MEASURE=1 go test ./exp/libps -run TestMeasure -v`, the 2D programs do the same work or less under the Go library, a third less for the small ones. The 3D programs do up to a tenth more: the library's model handlers are Go where the original's were hand-scheduled assembly, and the difference that is left is in how each model's drawing is entered, not in its loops.

The emulator runs a GTE command alongside the CPU, as the console does, charging what is left of it to the next GTE instruction, so work placed between a command and the read of its result is free in those figures too.

## The exp/ps API

`exp/ps` is the library under an ordinary Go surface: a `Screen` that gives out frames, sprites, boxes, lines and tile backgrounds drawn on a frame at a depth, text windows that take strings and integers, controllers with buttons and sticks, sound banks and sequences that return errors, models placed as objects under a camera and lights, GPU packets a program builds itself, typed, through a generic method on the frame, and the GTE's projections for a program that builds its own polygons. A sprite is the library's own layout under Go names, so drawing one copies nothing. `exp/ps/scene` and `exp/ps/stage` are the scene and stage packages on it ([mkscene](../tools/mkscene.md) cooks the files), so a scene file's camera, lights, objects, sprites and layers come out as the API's types and a room is drawn on a frame.

The package compiles on a host, where each call on the library panics with a message that says so, so a game's logic can be tested with `go test` and its drawing in the emulator. `exp/examples` holds the examples written against it, each checked against the original example's picture, pixel for pixel: `invaders`, `tiles`, `square`, `dino`, `timers`, `pads`, `card`, `cdda`, `jumpcube` and `quest` so far, which between them take in the root counters, every kind of controller, the memory card, the disc's audio tracks, the Kanji font, a scene's layers and a room drawn from its map. `gallery` is one of everything the API draws on a picture that stands still, checked against a picture of its own, since it is drawn with the library's own font.
