---
title: Files from the disc
weight: 3
description: The square example's model, texture and music put on the disc with discgen and yaroze, and read with a disc.Loader while the frames go on.
---

Everything bigger than the program's own code comes off the CD: models, pictures, sound. The square reads five files, and this page puts them on the disc and reads them.

## The files

Save these in a `data` directory beside `main.go`:

- [CUBE.TMD](../../examples/square/assets/CUBE.TMD), the cube. A TMD is the model format Sony's tools wrote: a list of vertices and normals, and a list of polygons that index them, each with its color or its texture coordinates. This one is 128 units along each edge, with six textured faces, and is written by [a short script](../../examples/square/mkcube.py), which is the whole of the format for a model this simple.
- [CUBE.TIM](../../examples/square/assets/CUBE.TIM), its texture, from Sony's own Net Yaroze tutorials. A TIM is a picture in one of the GPU's formats, with the place in video memory it is to be loaded to and its palette if it has one; this one is 64 by 64 in 16 colors.
- [STD0.VH](../../examples/square/assets/STD0.VH) and [STD0.VB](../../examples/square/assets/STD0.VB), a sound bank from the Net Yaroze SDK's samples: the header, which says what instruments it holds, and the waveforms themselves.
- [SAMPLE1.SEQ](../../examples/square/assets/SAMPLE1.SEQ), the SDK's sample tune, a sequence of notes to play on the bank, much like a MIDI file.

## Declaring them

A program names each file it reads with `disc.Register`, giving the path it has on the disc and the file on the host that goes there. Writing those by hand is easy to get wrong, so `discgen` writes them from what is in the directory. One line near the top of `main.go` runs it:

```go
//go:generate go run go.chrisrx.dev/psx/tools/discgen assets
```

and `go generate` then writes `zz_generated.disc.go`:

```go src=../../examples/square/zz_generated.disc.go#cubeTIM,files
```

Each name is the file's name in camel case with its extension in capitals, and `files` is all of them, in the order they will be read. Run `go generate` again whenever a file is added or renamed. `discgen` is also `yaroze generate`, so with `yaroze` as the module's tool the line can be `//go:generate go tool yaroze generate data`; the [playground](/play/) recognizes only the first form.

A registration does more than it looks like it does. TinyGo works the call out when it compiles the program, so each one ends up as a small record of constant data in the executable. When `yaroze` builds the disc it finds the records in the executable it is told to boot, places each file at its path, and, once it knows where on the disc each one landed, writes the file's first sector and its length back into the record. The program is built knowing where its files are, and reads each from its sector directly, without searching the disc's directory for it by name. A file that is missing or misnamed fails the build, not the program.

## Reading them while the frames go on

The drive is slow, and a read takes many frames. A program could stop and wait, and `disc.LoadAll` does that, but a black screen while the disc spins looks like a crash. The square uses a `disc.Loader` instead, which reads the files a step a frame while the frame loop goes on drawing:

```go src=../../examples/square/main.go#loader
```

Before the loop, `main` gives it the files and how many frames each may take before it is given up on, `loader.Start(300, files...)`: 300 frames, five seconds, which is far longer than a file that is there takes. Then, in each frame, the first case of the `switch` in [main](frame.md#the-frame) runs until the loader is done. `Step` starts the next file's read, or sees whether the one in progress has finished, and reports whether the loader is done, either with every file read or with one that failed, and whether one did. On the frame it finishes with every file read, the program sets up the cube and starts the music. Until then the bar's width is how many of the files have arrived, out of 200 pixels, and the bar is drawn at depth 0 like the box.

If a file could not be read, `Failed` says which, and the second case prints its name each frame from then on. The box still moves: a program that cannot find its disc should still respond, and say why it is not doing more.

Each file's memory is allocated the first time it is read, enough for the whole sectors the drive delivers, and kept for good; `Data` returns it, as words, which is how the next two pages hand the files to the library.

## Building the disc

The build is the one from [setting up](setting-up.md), after `go generate` has written the registrations. `yaroze disc` finds the records in the executable it builds and takes each file from its path relative to the package directory:

```bash
go generate
go tool yaroze disc -v
```

`-v` lists what was placed where, with the five files under `DATA` among it. The next page draws the cube: [a model in 3D](model.md).
