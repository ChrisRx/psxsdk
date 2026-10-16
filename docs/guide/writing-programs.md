---
title: Writing programs in Go
weight: 2
---

The [`yaroze`](yaroze) package binds the Net Yaroze library for TinyGo. Nothing in it is implemented in Go: each function is a bodyless declaration resolved by the linker against the fixed addresses of the resident library, exactly as a C program's calls are. The bindings are generated from the SDK's `libps.h` by [`hdr2go`](../tools/hdr2go.md), and every struct's size is asserted against clang's layout at compile time, so a struct that disagrees with the C one by a byte fails to build rather than producing a disc that draws nothing. Names are kept as the header spells them, so Sony's documentation reads across directly.

```go
import "go.chrisrx.dev/psx/yaroze"

yaroze.ResetGraph(0)
yaroze.GsInitGraph(320, 240, 4, 0, 0)
yaroze.FntLoad(960, 256)
```

The [examples](../examples/index.md) are ports of Sony's Net Yaroze samples and demos, each putting a different part of the machine to work, and the [tutorial](../tutorial/index.md) starts from nothing.
