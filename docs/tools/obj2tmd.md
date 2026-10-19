---
title: obj2tmd
weight: 20
---

`obj2tmd` converts a Wavefront OBJ model, which Blender and most 3D programs export, into a TMD, the model format `GsSortObject4` draws, laid out as [`format/tmd`](format/tmd) describes.

```bash
$ png2tim --at 640,0 --clut 0,480 stone.png
$ obj2tmd --scale 64 cube.obj
created "cube.TMD": 344 bytes
  object 0 "cube": 8 vertices, 6 normals, 6 polygons
```

Each `o` in the file becomes an object of the TMD, which a program links with `GsLinkObject4`, or a `TMD`'s `Link`, by its number from 0. The axes are turned from the OBJ's, Y up with the camera looking down -Z, to the console's, Y down and Z away from the camera, and the corners of each face reversed, since the console draws the side of a polygon whose corners go clockwise on the screen and an OBJ's go anticlockwise. Faces of more than four corners are split into triangles. Coordinates are multiplied by `--scale` and rounded to the 16-bit integers a TMD holds.

Faces are lit by the library, flat with a normal worked out from each face, or with `--smooth` gouraud with the file's vertex normals. `--unlit` draws colours and textures as they are. A material's `Kd` colours an untextured face, and one with `map_Kd` is textured from a TIM made with [`png2tim`](png2tim.md): the map's name with the extension `.TIM`, beside the MTL file. The TIM says where the texture sits in video memory, so the program loads the same TIM; texture coordinates are relative to the image, and the page offset is added.

Sony's FAQ lists what the Net Yaroze library does not draw from a TMD: lines, sprites, double-sided polygons and gouraud-shaded quads without a texture. `--both` makes each face two faces, back to back, instead of one double-sided one. The tests write the [square](../examples/square.md) example's cube as an OBJ and check that it converts back to the same faces.
