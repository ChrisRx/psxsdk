---
title: Writing programs in Go
weight: 1
---

A program imports [`yaroze`](yaroze), the Net Yaroze library, [`input`](input) for the controllers, and, when it reads files, [`disc`](#files-from-the-disc). This one moves a box with the controller:

```go
package main

import (
	"go.chrisrx.dev/psx/input"
	"go.chrisrx.dev/psx/yaroze"
)

var (
	screen = yaroze.Screen{Background: yaroze.Color{R: 40, G: 60, B: 100}}
	pad    input.Controller
	box    = yaroze.Box{
		W:     40,
		H:     40,
		Color: yaroze.Color{R: 255, G: 160, B: 32},
	}
)

func main() {
	screen.Open()
	bounds := screen.Bounds()
	for frame := range screen.Frames() {
		pad.Update()
		dx, dy := pad.Dir()
		x, y := box.Position()
		w, h := box.Size()
		box.MoveTo(bounds.Clamp(x+dx*2, y+dy*2, w, h))
		box.Draw(frame, 0)
	}
}
```

## The library

The `yaroze` package is the Net Yaroze library for TinyGo, in two forms a program can mix: the library's own functions, types and constants, with the exact names from Sony's `libps.h`, so Sony's [manuals](../manuals/index.md) apply to the Go code unchanged, and [Go types](#what-the-package-adds) over them.

- **On the console**, under the `yaroze` build tag, each function is a bodyless declaration the linker resolves. By default that is the clean-room library, `internal/libps`, linked into the program. With `--libps resident` it is the library's fixed addresses, exactly as a C program's calls are on a Net Yaroze.
- **In every other build**, each is a stub that panics, naming itself and saying where the real one is. A program still compiles and type-checks on the host: an editor shows the library's documentation, `go vet` runs, and a call into the library fails at run time rather than at the build.

The bindings are generated from `libps.h` by [`hdr2go`](../tools/hdr2go.md), with comments condensed from Sony's library reference. Every struct's size is asserted against clang's layout when the console build compiles, so a struct a byte off from the C one fails the build rather than making a disc that draws nothing.

The library's types also carry Go methods, each the library function of the same effect, inlined to the same call: a sprite's `Sort` and `SortFast`, a matrix's `Rotate` and `Mul`, a coordinate system's `Init` and `Ls`, an image's `Read` and `Load`. The functions remain for anything the methods leave out.

## What the package adds

Beside the library's names is a Go API: types with methods that take ints, strings and slices and return errors. Nothing in it allocates after `Screen.Open`.

### The screen

A `Screen` is the double-buffered frame: two ordering tables and two packet areas. Its fields are its settings: the size, the tables' `Depth`, the `Background` each frame is cleared to, and the `Interval` that sets the frame rate.

- `Open` puts the console in its own video mode, `NativeVideoMode`, sets up the display, and allocates the storage, unless `Init` gave it some.
- `Font` opens a window of the on-screen font and returns it as a `Text`, whose `Print` and `Int` chain.
- `Begin` and `End` do what every frame does around its drawing. `Frames` puts them around the body of a loop, `for frame := range screen.Frames()`, so a `break` leaves between frames. The `Frame` is what the Go types draw on, and `frame.OT()` is its ordering table, for the library's own sorts.
- `Bounds` is the screen in the frame's coordinates, wherever the origin is, as a `geom.Rect`, whose `Clamp` keeps a box inside it, and `Frame` counts the frames.

### Sprites, boxes and backgrounds

- A `Sprite` is a piece of a texture, placed by its pivot, tinted, scaled and turned. A `Box` is a filled rectangle, and a `Line` and a `GradientLine` are lines. Each has `Draw(frame, depth)`, with depth 0 in front.
- `LoadTIM` sends a TIM file to video memory and returns an `Image`, whose `Sprite` shows the whole of it and whose `Cell` cuts a tile from it.
- A `TileMap` is a grid of cells, and a `Background` a scrolling window onto one ([backgrounds](../cookbook/backgrounds.md)).
- `Frame.Packet` takes room for one of the [`gpu`](gpu) package's packets and links it into the frame, for a program that builds its own primitives.

### Models

- `LoadTMD` maps a TMD file and returns a `Model`. An `Object` is a model drawn somewhere, with its `Position`, `Rotation` and `Scale`. `Link` ties it to a model and `Draw` draws it on a frame, lit.
- A `Camera` is an eye, a target and the projection's distance, and `Set` makes it the one the frames are drawn with.
- `SetLight` sets one of the three directional `Light`s, `SetAmbient` the light from everywhere, and `SetFog` a `Fog`.

The SDK's own calls are here too, for a program written as Sony's samples are: `GsMapModelingData` and `GsLinkObject4` tie a `GsDOBJ2` to a model in a TMD file, `GsGetTimInfo` describes a TIM file in a `GsIMAGE`, which `LoadImage` puts into video memory, and a `GsCOORDINATE2` places a model, with `SetMatrix` setting its matrix and clearing the flag that tells the library it moved.

### Sound

- `OpenBank` sends a sound bank to the SPU and returns a `Bank`, and its `Sequence` opens a piece of music on it. Each returns an error when it did not load, and the methods of one that did not load do nothing, so a program can carry on without its sound.
- `PlayTracks` and `LoopTracks` play the disc's CD audio tracks, which `disc.Track` declares, `PlayingTrack` says which is playing, and `StopTracks` stops them ([CD audio](../cookbook/cd-audio.md)).
- `SetVolume`, `SetDiscVolume` and `SetReverb` set the mix.

### Controllers and memory cards

- The [`input`](input) package reads the controllers. An `input.Controller` is one controller frame by frame: `Update` reads it, `Pressed`, `Held` and `Released` tell a press from a hold, `Dir` is the direction buttons as steps across and down, `Duration` is how long a button has been held, and its fields hold the sticks, a mouse's movement and a gun's aim ([controllers](../cookbook/controllers.md)). `input.Read` is a port's buttons with no state kept.
- `PadRead` reads both controllers as the SDK's samples do, with the `PAD` constants naming the buttons as the SDK's `pad.h` does.
- `CardWrite`, `CardRead`, `CardFiles` and `CardDelete` save to and load from a memory card, with a `SaveHeader` for the title and icon ([memory cards](../cookbook/memory-cards.md)).

### Time

A `Counter` is one of the CPU's three root counters, `DotClock`, `Lines` or `SystemClock`, with `Start`, `Reset` and `Read`. `FieldCount` counts vertical blanks ([timing](../cookbook/timing.md)).

### Memory and text

- `Pointer`, `Word` and `BytesAt` reach memory by address, for the addresses a mapped model holds and the hardware's registers, and `Scratch` is the CPU's scratchpad.
- `Print` sends a string to the on-screen font, and `Buffer` and `CString` make the C strings `FntPrint` takes, without allocating.

### Geometry and packets

- [`geom`](geom) has the types the rest share: `Point` and `Rect` on the screen, `Vector`, `Rotation` and `Matrix` in the world, and the GTE's fixed point numbers, `Fixed` with `One` as 4096, and `Angle`.
- [`gpu`](gpu) has the GPU's packets as it takes them, triangles, quads, lines, tiles and sprites, flat, shaded or textured, and `Color`, which `yaroze.Color` is.

## SDK names or Go types

Use the Go types for a new program. They are shorter, take ordinary ints and strings, and do the bookkeeping the SDK leaves to the program: the two ordering tables and packet areas, the attribute bits, the handles.

Use the SDK names when porting a C program, or when working from Sony's manuals and samples, where a call is written the same way: `GsInitGraph`, `GsSortSprite`, `SsVabOpenHead`.

The two mix in one program, because the Go types sit on the same library:

- A `Sprite`, a `Box` and a `Background` begin with the layout of the library's `GsSPRITE`, `GsBOXF` and `GsBG`, which the console build asserts, and `Draw` hands them to the library's sort as they are.
- `frame.OT()` is the frame's `GsOT`, so `GsSortSprite`, `GsSortObject4` and the rest sort into the same frame the Go types draw on.
- `Bank.ID`, `Sequence.ID` and `Text.ID` give the library's own ids for its calls, and `ToRECT` makes a `RECT` of a `geom.Rect`.

## Files from the disc

The [`disc`](disc) package declares a program's files. A registration names the file on the disc and the file on the host that goes there, and the build places it on the disc. The program then reads it and finds it in `Data`, with no address anywhere in the code:

- `Load` reads a file and `Wait` waits for it. `Load` allocates the file's memory the first time, as many sectors as the build recorded.
- `LoadAll` reads a list of files, or every file registered when given none, and `Registered` lists those.
- A `Loader` reads them a step a frame, while the frames go on.
- With `Into`, a registration names memory of the program's own instead, and the build checks the file fits it. Use it when a program needs to know a file's size at build time, or to share one buffer between files.

[`discgen`](../tools/discgen.md), also `yaroze generate`, writes the registrations for every file under a directory from `go:generate`, which the playground runs too.

## Memory

A program's data lives in its own memory, as everything else does. The program loads at 0x80010000 with its library inside it, and has the rest of the console's two megabytes, less the stack, to itself. With `--libps resident` it loads at 0x80090000, where Sony's memory map starts the application area, just above the resident library.

The [examples](../examples/index.md) are ports of Sony's Net Yaroze samples and demos, each using a different part of the machine, and [Getting started](index.md) starts from nothing.
