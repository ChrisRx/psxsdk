---
title: Troubleshooting
weight: 4
description: What it usually is when the screen is empty, a sprite or a model is missing, the picture is garbled, the program freezes, or nothing sounds.
---

Most of what goes wrong in a PlayStation program goes wrong in one of a few ways, and the same few went wrong for Net Yaroze members in 1998, which is why many of the answers here are Sony's [Programming FAQ](../manuals/faq.md)'s, said again for this toolchain. The [emulator](../architecture/emulator.md) helps with all of them: `psxemu` prints what a program writes, saves what the screen shows and dumps video memory, and a test can read any of the program's variables by name.

## Nothing on the screen

- **The program stopped before its frame loop.** Run it in `psxemu`, which prints what it writes: a Go panic says what it was there, and a file that could not be read is whatever `disc.LoadAll` returned.
- **A file is not on the disc.** [`discgen`](../tools/discgen.md) declares the files that were under `assets/` when it last ran. Run `go generate` again after adding one, and `dumpsxiso` lists what the disc holds.
- **The frame loop never draws.** Everything has to be sorted into the table `Frames` gives, inside the loop.

## A sprite is missing

Sony's answer to [why a sprite cannot be seen](../manuals/faq.md#why-cant-i-see-my-sprite-on-screen) is the checklist, and with this toolchain:

- **Its image is not in video memory, or something drew over it.** `TIM.Load` has to run before the sprite is drawn. Two TIMs at overlapping places overwrite each other, as does an image under the font or inside the display buffers. `png2tim` prints where each one goes, with `-r` for a TIM already made, and `psxemu --vram` dumps video memory to look at.
- **Its palette was wiped.** A 256-line screen on a PAL console reaches row 511, and its clear each frame wipes palettes kept at row 480. `Screen` uses 240 lines unless asked for more for this reason.
- **It is somewhere else.** With `GsInit3D` the origin is the middle of the screen for 2D as well, so a sprite at 0, 0 is in the middle and one at 320, 240 is off the screen. Place things from `Screen.Bounds`.
- **It is black or invisible.** R, G and B of 0 draw nothing. 128 is the image as it is.
- **It is drawn, behind something.** At the same depth the last thing sorted is drawn first. A 3D model near the camera sorts at depth 0 too, and the overlay table, `Screen.OverlayDepth`, is for what must be in front of a 3D scene.
- **Its holes are wrong.** A texel of 0x0000 is not drawn. `png2tim` makes transparent pixels that, and opaque black 0x8000, unless `--black transparent` makes black transparent too.

Stray vertical lines in a sprite drawn with `SortFast` mean an odd U or width ([the FAQ](../manuals/faq.md#why-do-i-get-weird-extra-vertical-lines-in-some-of-my-sprites)).

## A model is missing or looks wrong

Sony's [checklist for models](../manuals/faq.md#why-cant-i-see-my-3d-model):

- **The TMD was not mapped,** with `TMD.Map`, once, before `Link`.
- **Its texture is not in video memory,** or not where the TMD says. Load the same TIM the model was converted with.
- **It is unlit or lit black.** Set a light with `GsF_LIGHT.Set` and some ambient light, or convert it with `obj2tmd --unlit`.
- **It is too big, too small or behind the camera.** A TMD's coordinates are 16-bit. A model a few hundred units across, a camera a few hundred to a thousand back and `GsSetProjection(320)` is a good start, and `rsdlink -v` prints a model's extent.
- **It is inside out.** Polygons are drawn from the side their corners go clockwise on the screen, and `obj2tmd` turns an OBJ's faces round. A model made another way may need its faces reversed in the modelling program.
- **Faces are missing.** The library does not draw double-sided polygons, lines, sprites or untextured gouraud quads from a TMD. `obj2tmd --both` makes two faces back to back.
- **Faces flicker or show through each other.** A deeper table, `Screen.Depth`, sorts the polygons more finely.

## The picture is garbled

- **The packet area is full.** A frame that sorts more than `Screen.Packets` holds writes past it, and the GPU draws garbage, or the library prints a GPU code it does not know ([the FAQ](../manuals/faq.md#why-am-im-getting-errors-like-gpu-code-xxh-not-supported-or-gpu-code-xxh-not-assigned)). Raise `Packets`.
- **A model was overwritten in memory.** A TMD is read and mapped in place, and an array overrun into it scrambles the model.
- **A packet area of the program's own is not aligned.** The library stores words into it, so it must be an array of `uint32` passed through `yaroze.Packets`, not of bytes. The emulator reports the misaligned store.

## The program freezes

A Go panic on the console prints its message and stops, and the picture freezes. The message is in what `psxemu` prints, and in `TTY()` from a test. The usual one is an index out of range, often from code written after C, which let a program read one past the end of an array. The emulator reports an illegal instruction or a misaligned access where it happens, with the registers and the instructions before it.

"out of memory" is the heap: the files a program loads and anything allocated, which is never freed. Allocating in the frame loop runs out in time. Passing the address of a local variable to a library function allocates it on every call, so a program keeps such things in package-level variables.

## It is slow

`psxemu --profile` shows where a frame's time goes. Ranging over a large array by value copies it on every iteration, so range over a pointer to it. Floating point is emulated, so the arithmetic is fixed point. `--polygons` counts what the GPU drew each frame. `Screen.Interval` of 2 halves the frame rate evenly when a frame cannot fit.

## No sound

- **`SsSetMVol` was never called.** Nothing sounds without it.
- **The bank did not open.** `OpenBank` reports it. A bank too big for the SPU's 512 KB with another one open, or a VH and VB swapped, will not open.
- **The music plays silence or the wrong instruments.** A sequence plays the bank's programs by the numbers its MIDI file used, so the bank needs programs with those numbers.
- **A sound is off key.** A tone plays its sample as recorded at its center note. Give the note the sample was recorded at, middle C being 60. A very short sample sounds off key whatever the note ([the FAQ](../manuals/faq.md#my-sample-seems-off-key)).

## The controller

`Pad.Pressed` is true on the frame a button goes down and `Held` while it is down, so a menu that moves on `Held` moves every frame. A second controller is a `Pad` with `Port` 1. The library decodes only the standard controller.
