---
title: mkarc
weight: 7
description: Pack files into one archive for the disc, each with deflate or stored as it is, which a program reads with package archive.
---

`mkarc` packs files into one archive, a file the program registers with package `disc` as one and reads with package `archive`, unpacking the entries it wants into memory of its own. The files named, and every file in each directory named, go in, in order:

```
$ mkarc -o assets/LEVEL.ARC pack/level
created "assets/LEVEL.ARC": 48,236 bytes from 119,320 (40%), 3 files, the largest BG.TIM at 65,556 bytes
  BG.TIM            65556 bytes, deflate, 31%
  CUBE.TIM           2112 bytes, deflate, 60%
  CUBE.TMD            344 bytes, deflate, 55%
```

[`yaroze`](yaroze.md) has it as `yaroze arc`, with the same flags.

Each entry is packed with deflate, unless that saves less than a tenth of it, as it does a sound bank or a movie, when it is stored as it is and the program can use it where it lies, with no copy; `-s NAME` stores one whatever it would pack to. Entries are named as the disc names files, in capitals, eight and three, from each file's name, so two files of a name cannot share an archive whatever their directories. `-l` lists an archive.

In a program, the archive is one declaration and one load, and each piece is unpacked when wanted, into an array of the program's own or into new memory:

```go
var level = disc.Register("ASSETS/LEVEL.ARC", "assets/LEVEL.ARC")

a, err := archive.Open(level.Data())
e, ok := a.Lookup("CUBE.TMD")
words, err := e.Unpack(buf[:])   // or e.Load()
model := yaroze.TMD(words)
```

A scene draws from an archive through `scene.NewArchive`, as the [scene](../examples/scene.md) example does with its textures and model. The format is `format/archive`'s: a header, a table of entries with a method, the sizes and an offset each, then the entries' bytes, each starting on a word. Deflate is the one packing, chosen on measurements of the examples' data and of the decoders on the console; [Compression](../architecture/compression.md) has them. In the playground, "Pack into an archive" on a data file moves it under `pack/NAME/` and adds the `mkarc` line that makes `assets/NAME.ARC` from that directory, so the archive is remade as its files change.
