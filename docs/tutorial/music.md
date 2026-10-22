---
title: Music
weight: 5
description: The square example's music: a sound bank sent to the sound processor, a sequence played on it and paused with Start, and the whole program.
---

The PlayStation's sound processor, the SPU, has 24 voices and half a megabyte of memory of its own, which holds the sounds it plays, compressed. The library's sequencer plays music on it: a sound bank, a VAB, is the instruments, and a sequence, a SEQ, is the notes, played on the bank's instruments at the times and pitches it gives, much as a synthesizer plays a MIDI file.

## Starting it

On the frame the loader finishes, after the cube, `main` starts the music:

```go src=../../examples/square/main.go#startMusic
```

`SsSetMVol` is the master volume, which is zero until something sets it, so nothing plays without it. `OpenBank` sends the bank to the SPU: it reads the header, the VH file, to find what instruments the bank holds, and copies the waveforms, the VB file, into the SPU's memory. `Sequence` opens the SEQ file to play on that bank, and `Play` with `SSPLAY_INFINITY` plays it round and round for as long as the program runs. From then on the library plays it by itself, from an interrupt, with nothing more for the frame loop to do.

The bank and the sequence are kept in package-level variables, for pausing:

```go src=../../examples/square/main.go#bank,song,paused
```

Each of `OpenBank` and `Sequence` also reports whether it worked, which the square ignores. A bank that was not accepted, or a sequence that did not open, has a negative ID, and every method on it does nothing, so a program whose sound did not load carries on silently rather than stopping.

## Pausing it

Start pauses the music, through `Pressed`, on the frame the button goes down:

```go src=../../examples/square/main.go#pauseMusic
```

`Resume` carries on from where `Pause` stopped, rather than from the start.

## The whole program

That is all of the square:

```go src=../../examples/square/main.go
```

`go generate` and then `go tool yaroze run` build it and play it in a window, with Enter as Start. It runs here as it does on a console, built as [files from the disc](disc.md) builds it. The directions move the box, and Start pauses the music:

<!-- player square -->

## Where to go next

- The [playground](/play/) starts from this program and builds it in the browser, so it can be changed and run without installing anything.
- The [examples](../examples/index.md) do much more with each of these parts: [check](../examples/check.md) draws hundreds of sprites, [dino](../examples/dino.md) animates a model vertex by vertex with the GTE, [clone](../examples/clone.md) plays sound effects on voices of its own, and [tunnel](../examples/tunnel.md) adds fog and colored lights.
- [Writing programs in Go](../guide/writing-programs.md) in the guide covers the `yaroze` package as a whole: the library as Go, the methods on its types, and what it leaves to the program.
