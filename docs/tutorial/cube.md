---
title: A cube from the disc
weight: 2
description: A textured cube, loaded as a model and a texture off the CD, drawn in 3D and turned with the directional buttons.
---

The second program draws a cube and turns it with the directional buttons. The cube is a model, and its texture is a picture, and both are files on the disc that the program reads at startup, which is how a program gets anything bigger than its own code into memory.

It starts from the module [setting up](setting-up.md) made; a new directory with the same `go.mod` does as well.

## The files

A model for the PlayStation is a TMD, the format Sony's tools wrote: a list of vertices and normals, and a list of polygons that index them, each with its color or its texture coordinates. A texture is a TIM, a picture in one of the GPU's formats with the place in video memory it is to be loaded to, and its palette if it has one. Save these two beside the program, in a `data` directory:

- [CUBE.TMD](cube/data/CUBE.TMD), a cube 128 units along each edge: eight vertices, six normals, and six textured polygons, one for each face.
- [CUBE.TIM](cube/data/CUBE.TIM), a 64 by 64 texture in 16 colors, with its palette.

The texture comes from Sony's own Net Yaroze tutorials, by way of the [flying](../examples/flying.md) example, which tiles the plane it flies over with it. The cube is written by [a short script](cube/mkcube.py), which is the whole of the TMD format for a model this simple, and the program, its files and the script are in the repository at [docs/tutorial/cube](docs/tutorial/cube).

## The program

```go src=cube/main.go
```

## Getting the files onto the disc

Two lines near the top do more than they look like they do:

```go
model   = disc.Register("DATA/CUBE.TMD", "data/CUBE.TMD")
texture = disc.Register("DATA/CUBE.TIM", "data/CUBE.TIM")
```

Each names a path on the disc and the file on the host that goes there. TinyGo works the call out when it compiles the program, so each registration ends up as a small record of constant data in the executable. When `mkpsxiso` builds the disc it finds the records in the executable it is told to boot, places each file at its path, and once it knows where on the disc each one landed writes the file's first sector and its length back into the record. The program is then built with the knowledge of where its files are, and `Read` reads a file from that sector directly, without searching the disc's directory for it by name. A file that is missing or misnamed fails the build rather than the program.

`load` wraps `Read` with the wait every disc read needs. The drive works in the background: `Read` starts it and returns, and `CdReadSync` reports whether it has finished, so the program polls it once a frame. Giving up after a few seconds turns a missing disc into a message instead of a hang.

The two buffers the files are read into are whole numbers of 2048-byte sectors, because that is what the drive delivers, whatever the file's length.

## Drawing in three dimensions

The library draws models through objects. A `GsDOBJ2` ties a model in a TMD to a coordinate system, a `GsCOORDINATE2`, which holds a matrix saying where the object is and which way it faces. The program links its one object to the one model in the file, after `GsMapModelingData` has turned the file's offsets into addresses, since the file could have been loaded anywhere.

The cube's orientation is a matrix the program keeps, `rotation`, and `turn` changes it: `RotMatrix` makes a matrix for a small turn, with 4096 standing for a whole turn as it does everywhere in the library, and `MulMatrix0` multiplies it onto the front of what is there. Multiplying on the front is what makes the controls behave: the new turn is applied after every turn so far, so it is always about the screen's axes, and up always tips the top towards the camera however the cube already sits. Multiplying on the back instead would turn the cube about its own axes, which feels right until it has been tipped over. Each frame the matrix is copied into the coordinate system and `Flg` set to zero, which tells the library the coordinates changed, so that it works out the object's screen matrix again. `GsGetLs` fetches that, `GsSetLsMatrix` hands it to the GTE, the geometry coprocessor that does the arithmetic in fixed point with twelve fractional bits, and `GsSortObject4` turns the model into drawing commands and sorts them into the ordering table by depth.

Only the faces turned towards the camera are drawn. The GPU decides that by which way round a polygon's corners come out on the screen, so a TMD's polygons have to be written with their corners in a consistent order as seen from outside, which the script that makes the cube is careful about.

The camera is a `GsRVIEW2`: a viewpoint, a point it looks at, and a twist about that line. Here it sits back from the origin and a little above it, looking at the origin, and `GsSetProjection` sets how strong the perspective is, as the distance from the eye to the screen in the same units. With `GsOFSGPU` the screen's origin is its middle, which three dimensions want, so the text window's position is counted from there.

The light is the simplest the library offers, one flat light shining along a direction, with some ambient light so that the faces turned away from it are not black. Each face has a normal, so each shades by its angle to the light as the cube turns, which is what makes it read as a solid.

## Trying it in the browser

This is the program, built onto a disc as the page describes and booted in the emulator that runs the [examples](../examples/index.md). The arrow keys turn it, or the controller, or the touch pad on a phone.

<!-- player cube -->

## Building and running it

The same steps as before, with the two files joining the disc through `--src`, the directory their registered paths are relative to:

```bash
tinygo build -target=playstation-yaroze -o prog.elf .
go tool mips1fix prog.elf
go tool eco2exe prog.elf MAIN.EXE
go tool mkpsxiso -v -o cube.bin --boot MAIN.EXE --file MAIN.EXE=MAIN.EXE --src .
go tool psxemu --bios openbios.bin --disc cube.cue --png cube.png
```

`-v` lists what was placed where, with the two registered files among it, and a run in mednafen or the console shows the cube, which the directions turn.
