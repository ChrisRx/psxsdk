---
title: mkscene
weight: 21
---

`mkscene` cooks a scene, written as JSON by the playground's scene editor or by hand, into a DAT, the file a program reads off its disc and draws with [`scene`](scene), laid out as [`format/scene`](format/scene) describes.

```bash
$ mkscene -o assets/CUBES.DAT scenes/cubes.json
created "assets/CUBES.DAT": 232 bytes, 1 object, 0 sprites, 1 light, camera at 0,0,-1000 looking at 0,0,0
  cube: CUBE.TMD object 0 at 0,0,0
```

The JSON names a camera (where it is and what it looks at, with a twist), the projection distance, the ambient light, up to three lights, the TIMs and TMDs the scene needs by their paths on the disc, the objects placed from those models, each with a name, a position, a rotation (4096 a whole turn), a scale (4096 as 1.0) and two fields, numbers of the program's own that `ObjectFields` returns (a kind, a score, a door's key), the sprites cut from those textures with the fields of `GsSPRITE`, and a tile map, if there is one: a texture cut into 16 by 16 cells in reading order, a grid of cell indexes (0xffff for an empty cell), a scroll and a brightness, drawn behind everything by `GsSortFixBg16` as the [tiles](../examples/tiles.md) example draws its own. The program registers the same TIMs and TMDs with package `disc`, or packs them into an archive with [`mkarc`](mkarc.md), loads them with the scene, and `scene.Load` binds each to its file by that path through a `Source`: `scene.Files` for the disc's own files, `scene.NewArchive` for an archive's entries (`ASSETS/CUBE.TIM` finding the entry `CUBE.TIM`, a packed one unpacked once and kept), `scene.Sources` for both. It sends the textures to video memory, maps the models and places everything, so that `Draw` draws it each frame and `Object(name)` finds what the program wants to move.

`mkscene -r CUBES.DAT` prints a cooked scene as JSON. A project holds as many scenes as it likes, each cooked by a line of its own, and the program loads whichever it is on, as the example does on Start. In the playground, while the program runs, each change to the scene is also written into the program's memory over the file it loaded, and `scene` reads it again at the next frame, so the console follows the editor without a rebuild. The [scene](../examples/scene.md) example is the square example's cube placed by two scene files, and is the program the playground starts from.
