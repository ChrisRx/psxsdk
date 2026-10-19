---
title: How the machine works
weight: 3
description: What a program for the PlayStation is working with, the memories, the frame, the GPU's ordering tables, the GTE, the SPU and the controllers, and how the yaroze package lays them out.
---

Most of what looks strange about writing for the PlayStation stops being strange once the machine is in view. This page goes through it in the order a program meets it, with the `yaroze` package's names for each part, and links to the answers in Sony's [Programming FAQ](../manuals/faq.md) that say more.

## Four memories

The console has four separate memories, and a program reaches each one differently.

**Main RAM** is 2 MB. The Net Yaroze library sits in the first 576 KB, up to 0x80090000, and a program loads from there and has the rest, less its stack and the library's small heap. The files a program reads off the disc are read into its own memory: [`disc`](disc) allocates them once, when they are loaded, or reads them into arrays the program gives it with `Into`. Nothing is ever freed. TinyGo's collector does not run here, so a program sets up what it needs at the start, in package-level variables, and its frame loop allocates nothing.

**Video RAM** is 1 MB, laid out as a picture of 1024 by 512 16-bit units, and only the GPU can reach it. The two pictures a program shows in turn, the display buffers, are two rectangles in it, and every image a program draws with, its sprites, its textures and their palettes, is another rectangle somewhere else. A TIM file says where its pixels and palette go, and `TIM.Load` puts them there. The CPU cannot read video memory as memory: `RECT.Store` asks the GPU to copy a rectangle back, which arrives some time later ([reading and writing pixels](../manuals/faq.md#how-can-i-readwrite-individual-pixels)).

**Sound RAM** is 512 KB, inside the SPU. A sound bank's samples, its VB file, are sent there once, and the program can then drop its own copy. The bank's header, the VH, stays in main memory, where the sound library reads it.

**The scratchpad** is 1 KB of fast RAM at 0x1f800000, which `yaroze.Scratch` returns. The library uses it while it sorts models and while it plays sound, so it is not somewhere to keep anything across a library call ([the DCache](../manuals/faq.md#what-is-the-dcache)).

## The frame

The GPU does not draw what it is told as it is told. A frame is built as a list first. Each sprite, rectangle or model a program sorts becomes one or more GPU commands, packets, written into a packet area, and is linked into an ordering table, an array of lists indexed by depth. When the frame is finished, the table goes to the GPU, which walks it and draws while the program goes on to build the next frame in a second table and a second packet area ([why two](../manuals/faq.md#why-do-i-need-two-packet-areas-and-ordering-tables)).

`Screen` is all of that. `Open` sets up the display and allocates both tables and both packet areas. `Frames` is the loop: each time round it clears the table for the frame being built and hands it to the body, and when the body is done it waits for the GPU to finish the frame before, waits for the vertical blank, swaps the display to show that frame, sorts a clear to `Background` at the back of the new table, and starts the GPU on it. Two things follow from this.

**The picture is behind the program.** What is on the screen was computed two frames ago ([double buffering](../manuals/faq.md#what-is-double-buffering)). A game decides everything from its own state and never from the picture.

**A frame has fixed room.** A packet area holds one frame's packets, 16 KB by default (`Screen.Packets`). A sprite takes about 24 bytes and a textured quad of a model 40 to 52, so a frame of a few hundred of each fits. A frame that sorts more than its area holds writes past it, and the GPU draws garbage or the library reports a GPU code it does not know ([sizing the area](../manuals/faq.md#how-big-should-i-make-my-gssetworkbase-area)).

The frame rate is the television's, 60 frames a second on an NTSC console and 50 on a PAL one, which `Screen.Open` sets by the console's own region. A frame that does not fit in a vertical blank's time slows the game down. `Screen.Interval` of 2 runs at half the rate instead, evenly.

## Depth

An ordering table is drawn from its highest depth down to 0, so depth 0 is drawn last and is in front of everything. Things sorted at the same depth are drawn in the reverse of the order they were sorted, so the last one sorted at a depth is behind the others there ([ordering tables](../manuals/faq.md#what-is-an-ordering-table-what-is-an-ordering-table-length)).

A 2D program gives each layer a depth of its own: the background at the back, the sprites in front of it, the score at 0. `Screen.Depth` sets the table's size, 1<<Depth entries.

A 3D model's polygons are sorted by their distance from the camera, which the library shifts down to fit the table: `Object.Draw` puts a polygon at its Z shifted right by 14 minus the table's depth, so that distances up to 16383 spread across it. With a small table, a model near the camera lands at depth 0, where sprites are too. Sprites that must be in front of a 3D scene go in the overlay table instead, which `Screen.OverlayDepth` adds and `End` draws after the frame's whole table, so it is in front whatever the scene's depths are.

## Images

The GPU draws textures from texture pages, rectangles of video memory 64 units wide and 256 lines tall, and a sprite or polygon names its texture by its page, its U and V inside the page, and its palette's position. A 4-bit image fits four pixels in a unit, so a page holds an image 256 pixels wide, an 8-bit one 128 and a 16-bit one 64. No sprite is bigger than 256 by 256 ([sizes](../manuals/faq.md#what-are-the-minimummaximum-sizes-for-sprites)).

A texel of 0x0000 is not drawn at all, which is how sprites have holes in them. Any other colour is drawn, black included as 0x8000. The top bit of a colour, STP, marks it as one that blends with what is behind it when the sprite or polygon turns semitransparency on, at one of four rates. The GPU keeps the last 2 KB of texture it read in a cache, so small 4-bit textures draw fastest ([the cache](../manuals/faq.md#what-is-the-gpu-cache)).

A sprite's R, G and B scale its image: 128 draws it as it is, 255 twice as bright, 0 black. `TIM.Load().Sprite()` makes a sprite of a whole image at 128 with its middle as its position, and `Sort` draws it turned and scaled about that middle, its rotation in 4096ths of a degree. `SortFast` draws it unturned from its top left corner, faster, with an even U and width ([stray lines](../manuals/faq.md#why-do-i-get-weird-extra-vertical-lines-in-some-of-my-sprites)).

## The GTE

The GTE is a coprocessor for 3D arithmetic: it transforms vertices by a matrix, projects them and lights them. Its numbers are fixed point, with 4096 as 1.0, and an angle is in 4096ths of a whole turn. The CPU has no floating point unit, and Go's floats are emulated in software, slowly, so a program works in fixed point throughout ([fixed point](../manuals/faq.md#what-is-fixed-point-arithmetic)).

A model is drawn in a coordinate system. `Object` holds one with a position, a rotation and a scale, and `Draw` makes the system's matrix from them each frame, sets the GTE's matrices from it and from the camera's, and sorts the model's polygons, lit by up to three directional lights and an ambient one. The camera is a `GsRVIEW2`, a point to look from and a point to look at, and `GsSetProjection` sets the distance to the screen, which is the lens. The axes are X right, Y down and Z away from the camera.

A polygon is drawn from the side its corners go round clockwise on the screen. The Net Yaroze library does not draw lines, sprites, double-sided polygons or untextured gouraud quads that are in a TMD, as Sony's FAQ says ([unsupported primitives](../manuals/faq.md#why-am-im-getting-errors-like-gpu-code-xxh-not-supported-or-gpu-code-xxh-not-assigned)), which [`obj2tmd`](../tools/obj2tmd.md) works round.

## Sound

The SPU plays 24 voices at once from samples in sound RAM, each a VAG of the SPU's ADPCM. A bank, a VAB, groups the samples into programs, instruments of up to 16 tones, each tone a sample over a range of notes with its own envelope. A sequence, a SEQ, is a MIDI score that plays the bank's programs. A sound effect is a note played by hand with `SsUtKeyOn`.

`OpenBank` sends a bank to the SPU and `Sequence` opens a score on it. `SsSetMVol` has to be called before anything sounds. A tone plays its sample at the sample's own rate at its centre note, and faster for higher notes. The boot disc's General MIDI bank plays any General MIDI score, but takes most of sound RAM, so a game with effects of its own usually makes a bank of only the instruments it needs ([MIDI on the PlayStation](../manuals/faq.md#whats-the-deal-with-midi-on-the-psx)). [Making data](data.md) says how.

## Controllers

The library reads both controllers every vertical blank. A `Pad` turns one controller's buttons into what a game asks: whether a button is held, whether it went down this frame, how long it has been held, and which way the directions point. The library decodes only the standard controller. Analog sticks, the mouse, the neGcon and the GunCon arrive as the raw bytes `GetPadBuf` finds, which the SCEE tutorial's [peripherals chapter](../manuals/tutorial-peripherals.md) decodes. There is no vibration and no multitap ([peripherals](../manuals/peripherals.md)).
