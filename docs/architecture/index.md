---
title: How it fits together
weight: 2
group: Docs
section: Architecture
description: How psxsdk fits together and works inside: the path from a Go package to a disc, the emulator, the compiler in the browser, and the choice of compression.
---

```pikchr
SRC: file "examples/check/main.go" fit
SP: arrow down 2.1 from SRC.s
IMG: cylinder "check.bin + check.cue" fit with n at SP.end

# The build, beside the spine, with the tool for each step on the right.
$x1 = SP.start.x + 0.2
$x2 = $x1 + 0.2
$x3 = $x2 + 0.2
$x4 = $x3 + 0.2
$xt = $x1 + 2.9
$y0 = SP.start.y - 0.3
text "yaroze disc ./examples/check" bold ljust at ($x1, $y0)
text "TinyGo, -target=playstation-yaroze" ljust at ($x2, $y0 - 0.32)
text "→" ljust at ($x3, $y0 - 0.64)
text "prog.elf, traps replaced" ljust at ($x4, $y0 - 0.64)
text "→" ljust at ($x3, $y0 - 0.96)
text "MAIN.EXE, libps combined in" ljust at ($x4, $y0 - 0.96)
text "→" ljust at ($x3, $y0 - 1.28)
text "the disc, with the assets" ljust at ($x4, $y0 - 1.28)
text "the program registers" ljust at ($x4, $y0 - 1.52)
text "tinygo, or TinyGo as WebAssembly" italic ljust at ($xt, $y0 - 0.32)
text "mips1fix" italic ljust at ($xt, $y0 - 0.64)
text "eco2exe" italic ljust at ($xt, $y0 - 0.96)
text "mkdisc" italic ljust at ($xt, $y0 - 1.28)

# Where the disc runs: a bar under the disc, and a drop to each.
boxwid = 1.4
boxht = 0.5
$gap = 1.55
$xb = SP.start.x
$yb = IMG.s.y - 0.22
line from IMG.s to ($xb, $yb)
line from ($xb, $yb) to ($xb + 3*$gap, $yb)
arrow from ($xb, $yb) down 0.25
E1: box "emu" "go test, emutest" italic with n at last arrow.end
arrow from ($xb + $gap, $yb) down 0.25
E2: box "web" "WebAssembly, Pages" italic with n at last arrow.end
arrow from ($xb + 2*$gap, $yb) down 0.25
E3: box "yaroze run" "a window" italic with n at last arrow.end
arrow from ($xb + 3*$gap, $yb) down 0.25
E4: box "real hardware" "or psxe" italic with n at last arrow.end
```

[`yaroze`](../tools/yaroze.md) runs all of it, from the package to the disc, in one step, and `yaroze run` plays the disc in a window as well. Each step is also a tool of its own, named on its right, for a build that wants only one of them.

A Net Yaroze program is only half a program: the rest is `libps`, the library the Net Yaroze boot disc leaves resident in RAM. `eco2exe` combines the two into a standalone PSX-EXE, so the result boots from an ordinary disc in an emulator or on a console that plays burned discs, with no Net Yaroze hardware involved.
