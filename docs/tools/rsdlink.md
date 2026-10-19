---
title: rsdlink
weight: 19
---

`rsdlink` links RSD models, the text format Sony's 3D tools wrote and the format of most models in the Net Yaroze samples, into a TMD file, as Sony's rsdlink did, with the same arguments.

```bash
$ rsdlink -v -s 64 -o wall0.tmd wall0
WALL0.RSD: 4 vertices, 1 normals, 1 polygons, scale 2^6, range (-65,-128,-64)-(65,128,-64)
created "wall0.tmd": 1 object(s), 112 bytes
```

An RSD names a PLY of vertices, normals and polygons, a MAT of each polygon's colour, shading and texture, and the TIM files of the textures, read for where they sit in video memory, as [`format/rsd`](format/rsd) describes. Each RSD given becomes an object of the TMD, in order. `-s` scales the models that follow by the power of two nearest the factor and `-t` moves them; an argument ending `.arg` is a file of more arguments.

The output is byte for byte what Sony's rsdlink wrote: the tests link every model in the Net Yaroze websites' archive whose TMD sits beside it, with the command line its batch file or makefile gives, and compare them. For a model made in a current 3D program, [`obj2tmd`](obj2tmd.md) reads Wavefront OBJ.
