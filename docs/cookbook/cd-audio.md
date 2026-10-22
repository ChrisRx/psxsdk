---
title: CD audio
weight: 3
description: Music recorded on the program's own disc as CD audio tracks, declared in Go with disc.Track, played by the drive while the program goes on.
---

A PlayStation disc is a CD, and after its data track it can carry tracks of ordinary CD audio, the same as a music CD's. The drive plays them by itself and sends the sound straight to the SPU, which mixes it with whatever the program is playing, so music from the disc costs the program nothing: no memory, no CPU, and none of the SPU's 512 KB. It costs disc space instead, 176 KB a second of CD-quality 44.1 kHz stereo, which a game had plenty of. Games kept their music on the disc and their sound effects in the SPU.

The Net Yaroze could only boot its own disc, so the SCEE tutorial's [CD chapter](../manuals/tutorial-cd.md) played whatever music CD was in the drive. A disc built here can carry its own tracks.

## Declaring the tracks

A program declares each track with `disc.Track`, as it declares a file with `disc.Register`, with the WAV file the track is made from:

```go src=../../examples/cdda/main.go#track1,track2,track3
```

The build records each one after the data track, in the order the program declares them, and writes each track's number into the program, which `TrackNumber` returns. The WAV is 44.1 kHz, 16-bit stereo, as a CD's audio is. [mkdisc](../tools/mkdisc.md) puts the standard two-second pause before each track, which is where the drive's play starts when the library asks for a track by number.

## Playing

`CdPlay` plays from a list of track numbers ending with a zero: mode 1 plays from one of them to the end of the list, mode 2 round and round, mode 0 stops, and mode 3 only asks. Each returns which of the list is playing, or -1:

```go src=../../examples/cdda/main.go#main
```

The SPU mixes in the disc's sound only once it is told to, with `SsSetSerialAttr` for the CD's input and `SsSetSerialVol` for its volume, beside `SsSetMVol` for the whole of the SPU's output.

A track plays while the program reads nothing else from the disc. A game that loaded a level stopped its music, or loaded everything first, or kept a short piece in the SPU to play over the loading. The [cdda](../examples/cdda.md) example's three tracks are made by its `gen` program, a rising arpeggio, a falling note and a beat, a few seconds each:

<!-- player cdda -->

The emulator plays CD audio in the browser and in tests, which is how the example's test checks the track it asked for is the one that sounds.
