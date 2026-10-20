---
title: Split screens
weight: 7
description: Two players on one screen: a camera for each, the scene sorted twice into two ordering tables, and each drawn into its half with its own offset and clip.
---

A split screen is two pictures in one frame, each from its own camera. On the PlayStation it is two ordering tables: the scene is sorted into one with the GTE set to the first camera and into the other with it set to the second, and then each table is drawn into its half of the screen. The [split](../examples/split.md) example, a port of the SCEE tutorial's chapter 10 examples 20 and 21, gives each of two players a cube and a camera that follows it:

```go src=../../examples/split/main.go#main
```

`Screen.Begin` gives the first half's table, and the program keeps the second half's, one for each display buffer as the screen's are.

## Drawing each half

Each half is drawn with its own drawing offset, the point on the screen the camera's centre lands on, the middle of the half, and its own clip, the rectangle drawing is kept inside. `GsSetOffset` and `GsSetClip` set both for the buffer being drawn, and the next `GsSwapDispBuff` puts them back:

```go src=../../examples/split/main.go#draw
```

The two calls change the GPU's settings at once, not in turn with what it is drawing, and the library asks that the GPU not be busy when they are called. So the frame waits with `DrawSync` for the first half to be drawn before it sets up the second. The tutorial did not wait, which leaves the second half's settings to land in the middle of the first half's drawing, depending on how quickly the GPU gets through it.

The screen clear is a fill, which ignores the clip, so it is sorted into the first table and clears the whole screen. The text is drawn last, with the whole screen's offset and clip put back.

## What it costs

The scene is sorted twice, once for each camera, so a split screen takes twice the CPU and GTE work of one view, and each half has half the pixels to show it in. Two-player games drew less far, used simpler models, or ran at thirty frames a second instead of sixty. The [timing](timing.md) page shows how to measure what a frame costs.

Select switches the example between halves one above the other and side by side. Side by side, each half is 160 pixels wide and the full height, and with the same projection it shows as much of the scene up and down but half as much across.

<!-- player split -->
