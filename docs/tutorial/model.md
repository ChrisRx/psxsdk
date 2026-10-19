---
title: A model in 3D
weight: 4
description: The square example's cube: its texture loaded into video memory, its model linked to an object, a camera and a light set up, and the cube turned by the frame count.
---

Once the files are read, `setUpCube` turns two of them into a cube the frame can draw:

```go src=../../examples/square/main.go#setUpCube
```

## The texture

The GPU draws textures from its own video memory, a megabyte arranged as a picture 1024 pixels wide and 512 high, which also holds the two display buffers the frames are drawn into. A TIM file says where in it the picture goes, and `TIM.Load` copies the picture there, and its palette, and waits for the copy to finish. The model's polygons name their texture by where it is in video memory, so the TIM and the TMD were made to agree, and nothing in the program has to tell one about the other.

## The model and the object

`yaroze.TMD` is a view of the file's words as a model file. The file holds offsets, from where it starts to its vertices, normals and polygons, since it could have been loaded anywhere; `Map`, the library's `GsMapModelingData`, turns them into addresses once it is in memory, before anything uses it.

A model is drawn through an object, which ties it to a coordinate system that says where it is and which way it faces. `yaroze.Object` is the two together, since every drawn model needs both, declared with the rest of the program's state:

```go src=../../examples/square/main.go#cube
```

`Init` makes the object's coordinate system the identity with `WORLD` as its parent, and `Link`, the library's `GsLinkObject4`, ties it to the first model in the file, the only one. The object holds pointers into itself after `Init`, which is why it is a package-level variable and is never copied.

## The camera and the light

```go src=../../examples/square/main.go#view,light
```

The camera is a `GsRVIEW2`: a viewpoint, a point it looks at, and a twist about the line between them. With everything else zero, it sits 1000 units in front of the origin, looking at it. `GsSetProjection` sets how strong the perspective is, as the distance from the eye to the screen in the same units: 320 against a viewpoint 1000 away makes the 128-unit cube about 40 pixels across.

The light is the simplest the library offers, one flat light shining along a direction, here from over the camera's shoulder, with some ambient light so that the faces turned away from it are not black. Each face has a normal, so each shades by its angle to the light as the cube turns, which is what makes it read as a solid. Colors and the ambient level are fixed point, with 4096, `ONE`, as 1.0.

## Drawing it

The last case of the `switch` in [main](frame.md#the-frame) draws the cube once the files are in: it sets the cube's `Rotation` from `screen.Frame()` and calls `cube.Draw(ot)`. An object's `Rotation` is how far it is turned about each axis, with 4096 a whole turn. The square turns the cube by the frame count, `screen.Frame`, rather than by adding a little each frame, so the cube is where it is at a given frame however long the disc took; that is also what lets the [emulator test](tests/examples_test.go) compare a screenshot of it pixel for pixel.

`Draw` makes the coordinate system's matrix from the object's position, rotation and scale, hands the GTE, the geometry coprocessor that does the arithmetic in fixed point, the matrices from the object to the world and to the screen, and sorts the model's polygons into the frame's ordering table by their depth. The table has the 16 entries the screen's `Depth` asked for, so the faces nearer the camera are drawn after the ones further away, and the box, at depth 0, after them all.

Only the faces turned towards the camera are drawn. The GPU decides that by which way round a polygon's corners come out on the screen, so a TMD's polygons have to be written with their corners in a consistent order as seen from outside, which the script that makes the cube is careful about.

The last part is the music: [music](music.md).
