---
title: Timing
weight: 4
description: Keeping time on the PlayStation: the frame count, the four root counters, and measuring how much of a frame a program's work takes.
---

A game keeps time by its frames. The television draws a picture sixty times a second on an American or Japanese console and fifty on a European one, and a program draws one frame for each, or for every second one, so the frame is the clock everything else runs by: a sprite moves so many pixels a frame, a timer counts down so many frames. `VSync(-1)` returns how many fields have passed since the program started, which is a clock without anything else:

```go src=../../examples/timers/main.go#rate
```

A program that wants seconds divides by the console's rate, which `GetVideoMode` says. A PAL game that took fifty frames to a second ran slow on an NTSC console's sixty, the other way round, and games converted for Europe often did run slow.

## The root counters

The CPU has four timers, the root counters, each a 16-bit count that runs up to 65,535 and round again:

```go src=../../yaroze/libps.go#RCntCNT0,RCntCNT1,RCntCNT2,RCntCNT3
```

`StartRCnt` starts one, `ResetRCnt` puts it back to zero and `GetRCnt` reads it. The library keeps the vertical blank's for itself. A program measures how long something takes by reading a counter before and after and taking the difference, which works across a wrap as long as the span is shorter than the counter's whole round: about four seconds of horizontal lines, or 15 milliseconds of the system clock.

## How much of a frame

`VSync(1)` returns the horizontal lines drawn since the last vertical blank, without waiting. Read when a frame's work is done, before the program waits for the next blank, it is how much of the frame the work took: the television draws 263 lines a field on NTSC and 313 on PAL, and work that takes more misses the next blank, so the program runs at half its rate until it takes less again. The [timers](../examples/timers.md) example draws it as a bar, and up adds sprites until the bar fills:

```go src=../../examples/timers/main.go#main
```

This is how Net Yaroze programmers found where their frames went, since the console had no profiler: time a part of the frame with a counter, or the whole of it with `VSync(1)`, and draw the number.

<!-- player timers -->

The SCEE tutorial's chapter 5 is the original, three programs that this example puts together.
