---
title: Scrolling backgrounds
weight: 5
description: A tile map bigger than the screen, made of 16 by 16 cells cut from one texture, scrolled and drawn by the library's fixed background.
---

A scrolling game's world is too big for video memory as a picture, so it is built of tiles: a few dozen small squares in one texture, and a map of which square goes where. The library draws such a map for a program, as its fixed background, in three parts:

- the cells, `GsCELL`, each a square of a texture page with its palette,
- the map, `GsMAP`, a grid of indexes into the cells,
- and the background, `GsBG`, the window on the screen it is drawn in, how far it is scrolled, and its map.

The [tiles](../examples/tiles.md) example cuts sixteen cells from the SCEE tutorial's own tile image, and its map is the tutorial's, forty cells by thirty-two, two screens each way:

```go src=../../examples/tiles/main.go#main
```

`GsInitFixBg16` prepares a work area once, sized for the window, and `GsSortFixBg16` then makes the packets for the cells that show each frame into that area rather than the frame's own packet space. It draws only what the window shows, 21 by 16 cells here of the map's 1,280, so the map can be as big as memory allows. Scrolling is changing `Scrollx` and `Scrolly`, kept within the map so the window never shows past its edge.

## Tiles in video memory

A cell is 16 by 16 pixels within one texture page, 256 by 256, so a page holds 256 of them. Cells of 8 bits each use a palette of 256 colors, and of 4 bits each one of 16, half the memory again. The tutorial's tiles are 8-bit, at 320, 0 in video memory beside the display buffers, with the palette at 320, 509 below them. [Making data](../guide/data.md) plans where images go.

A 0xffff in the map leaves a cell empty, so a background over another shows through it. Two backgrounds scrolled at different speeds, a near one over a far one, are the parallax of many 2D games.

<!-- player tiles -->
