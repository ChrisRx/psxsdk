---
title: Writing programs in Go
weight: 2
---

The [`yaroze`](yaroze) package binds the Net Yaroze library for TinyGo. On the console, which is to say under the `yaroze` build tag the `playstation-yaroze` target sets, each function is a bodyless declaration resolved by the linker against the fixed addresses of the resident library, exactly as a C program's calls are. In every other build each is a stub that panics, naming itself and saying where the real one is, so that a program compiles and type-checks on a host: an editor shows the library's documentation, `go vet` runs, and a call into the library fails at run time rather than at the build. The bindings are generated from the SDK's `libps.h` by [`hdr2go`](../tools/hdr2go.md), with their comments condensed from Sony's library reference, and every struct's size is asserted against clang's layout when the console build compiles, so a struct that disagrees with the C one by a byte fails to build rather than producing a disc that draws nothing. Names are kept as the header spells them, so Sony's documentation reads across directly.

The library's types carry Go methods for what a program does with them, each the library function of the same effect and inlined to the same call: a sprite's `Sort` and `SortFast`, a matrix's `Rotate` and `Mul`, a coordinate system's `Init` and `Ls`, an image's `Read` and `Load`. The functions remain for anything the methods leave out.

What the library leaves to the program is in the package too. A `Screen` is the double-buffered frame, two ordering tables and two packet areas, with `Begin` and `End` doing what every frame does around its drawing. A `TMD` is a model file in memory, with `Map` and `Link`, a `TIM` an image file, with `Load`, and an `Object` a model with the coordinate system it is drawn in. A `Bank` is a sound bank sent to the SPU and a `Sequence` a piece of music open on it, each carrying whether it loaded so that a program whose sound did not can carry on without it. `PadInit` and `PadRead` read the controllers, with the `PAD` constants naming the buttons as the SDK's `pad.h` does, and a `Pad` tells a press from a hold. `Pointer`, `At` and `BytesAt` reach memory by address, for the addresses a mapped model holds and the hardware's registers, and `Scratch` is the CPU's scratchpad. `Buffer` and `CString` make the C strings `FntPrint` takes, without allocating.

Files come off the disc through the [`disc`](disc) package. A registration names the file on the disc, the file on the host that goes there, and with `Into` the memory it is read into, and the build places the file on the disc and checks that it fits; the program then reads it with `Load` and waits with `Wait`, with no address anywhere in it. A program's data lives in its own variables, as everything else does: the program loads at 0x80090000, where Sony's memory map starts the application area, just above the resident library, and has the rest of the console's two megabytes, less the stack, to itself.

```go
import "go.chrisrx.dev/psx/yaroze"

yaroze.ResetGraph(0)
yaroze.GsInitGraph(320, 240, 4, 0, 0)
yaroze.FntLoad(960, 256)
```

The [examples](../examples/index.md) are ports of Sony's Net Yaroze samples and demos, each putting a different part of the machine to work, and the [tutorial](../tutorial/index.md) starts from nothing.
