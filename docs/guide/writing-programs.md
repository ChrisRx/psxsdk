---
title: Writing programs in Go
weight: 1
---

A program imports [`yaroze`](yaroze), the Net Yaroze library, and [`disc`](disc) for its files:

```go
import "go.chrisrx.dev/psx/yaroze"

yaroze.ResetGraph(0)
yaroze.GsInitGraph(320, 240, 4, 0, 0)
yaroze.FntLoad(960, 256)
```

## The library

The `yaroze` package binds the Net Yaroze library for TinyGo, with the names as Sony's `libps.h` spells them, so Sony's [manuals](../manuals/index.md) read across directly.

- **On the console**, under the `yaroze` build tag the `playstation-yaroze` target sets, each function is a bodyless declaration the linker resolves against the resident library's fixed addresses, exactly as a C program's calls are.
- **In every other build**, each is a stub that panics, naming itself and saying where the real one is. A program still compiles and type-checks on the host: an editor shows the library's documentation, `go vet` runs, and a call into the library fails at run time rather than at the build.

The bindings are generated from `libps.h` by [`hdr2go`](../tools/hdr2go.md), with comments condensed from Sony's library reference. Every struct's size is asserted against clang's layout when the console build compiles, so a struct a byte off from the C one fails the build rather than making a disc that draws nothing.

The library's types also carry Go methods, each the library function of the same effect, inlined to the same call: a sprite's `Sort` and `SortFast`, a matrix's `Rotate` and `Mul`, a coordinate system's `Init` and `Ls`, an image's `Read` and `Load`. The functions remain for anything the methods leave out.

## What the package adds

What the library leaves to the program is in the package too.

### The screen

A `Screen` is the double-buffered frame: two ordering tables and two packet areas. Its fields are its settings: the size, the tables' `Depth`, the `Background` each frame is cleared to, and the `Interval` that sets the frame rate.

- `Open` puts the console in its own video mode, `NativeVideoMode`, sets up the display, and allocates the storage, unless `Init` gave it some.
- `Font` opens the on-screen font for its frames.
- `Begin` and `End` do what every frame does around its drawing. `Frames` puts them around the body of a loop, `for ot := range screen.Frames()`, so a `break` leaves between frames.
- `Bounds` is the screen in the frame's coordinates, wherever the origin is, `Clamp` keeps a box inside it, and `Frame` counts the frames.

### Models and images

- A `TMD` is a model file in memory, with `Map` and `Link`.
- A `TIM` is an image file, with `Load`, whose image makes a ready `Sprite`.
- An `Object` is a model with the coordinate system it is drawn in, and its `Position`, `Rotation` and `Scale`. Its `Draw` makes those the system's matrix and sorts the model lit, with the light and screen matrices set from where it is. `SetMatrix` places a coordinate system directly, clearing the flag that tells the library it moved.

### Sound

A `Bank` is a sound bank sent to the SPU, and a `Sequence` a piece of music open on it. Each records whether it loaded, so a program whose sound did not can carry on without it.

### Controllers and memory cards

- `PadRead` reads the controllers as `Buttons`, with the `PAD` constants naming the buttons as the SDK's `pad.h` does.
- A `Pad` is one controller frame by frame. It tells a press from a hold, and has `Dir` for the direction buttons as steps across and down, `Duration` for how long a button has been held, and `Connected`.
- `ReadController` reads any kind of controller in a port, analog sticks, a mouse, a neGcon or a GunCon, as a `Controller` ([controllers](../cookbook/controllers.md)).
- `SaveHeader`, `CardWrite`, `CardRead` and `CardFiles` save to and load from a memory card ([memory cards](../cookbook/memory-cards.md)).

### Memory and text

- `Pointer`, `At` and `BytesAt` reach memory by address, for the addresses a mapped model holds and the hardware's registers, and `Scratch` is the CPU's scratchpad.
- `Print` sends a string to the on-screen font, and `Buffer` and `CString` make the C strings `FntPrint` takes, without allocating.

## Files from the disc

The [`disc`](disc) package declares a program's files. A registration names the file on the disc and the file on the host that goes there, and the build places it on the disc. The program then reads it and finds it in `Data`, with no address anywhere in the code:

- `Load` reads a file and `Wait` waits for it. `Load` allocates the file's memory the first time, as many sectors as the build recorded.
- `LoadAll` reads a list of files.
- A `Loader` reads them a step a frame, while the frames go on.
- With `Into`, a registration names memory of the program's own instead, and the build checks the file fits it. That suits a program that wants to know a file's size, or to share one buffer between files.

[`discgen`](../tools/discgen.md), also `nyc generate`, writes the registrations for every file under a directory from `go:generate`, which the playground runs too.

## Memory

A program's data lives in its own memory, as everything else does. The program loads at 0x80090000, where Sony's memory map starts the application area, just above the resident library, and has the rest of the console's two megabytes, less the stack, to itself.

The [examples](../examples/index.md) are ports of Sony's Net Yaroze samples and demos, each putting a different part of the machine to work, and the [tutorial](../tutorial/index.md) starts from nothing.
