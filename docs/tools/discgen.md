---
title: discgen
weight: 14
---

`discgen` writes the Go declarations that put a program's data files on its disc: for each file under a directory, the `disc.File` that registers it with [`mkdisc`](mkdisc.md), and a slice of every file for `disc.LoadAll`, which reads each into memory it allocates.

It is meant for `go:generate`, in the program's package:

```go
//go:generate go run go.chrisrx.dev/psx/tools/discgen assets
```

The [playground](/play/) runs it too, before each build, for a program with that line, over the data files beside it, so a program there names its files without declaring them, as the examples a project there starts from, jumpcube and quest, do.

The sources the converters read lie in the same directory and are left off the disc: pictures, models, sounds and scores by their extension, and any file a `go:generate` line of the package reads.

For `assets/CUBE.TIM` and `assets/CUBE.TMD` that writes `zz_generated.disc.go`:

```go
var (
	cubeTIM = disc.Register("ASSETS/CUBE.TIM", "assets/CUBE.TIM")
	cubeTMD = disc.Register("ASSETS/CUBE.TMD", "assets/CUBE.TMD")
)

var files = []*disc.File{
	&cubeTIM,
	&cubeTMD,
}
```

and the program loads everything with one call, and takes each file's memory from it:

```go
if f := disc.LoadAll(300, files...); f != nil {
	// f.Name is missing or unreadable
}
yaroze.TIM(cubeTIM.Data()).Load()
```

With `--into` it declares the memory as well, for a program that needs to know its size at build time or keep it out of the heap: an array per file, sized to the whole 2048-byte sectors the drive delivers, given to the file with `Into`, and a `yaroze.TIM` or `yaroze.TMD` over the array for an image or a model.

```go
var (
	cubeTMDData [512]uint32 // 344 bytes
	cubeTMDFile = disc.Register("ASSETS/CUBE.TMD", "assets/CUBE.TMD").Into(cubeTMDData[:])
	cubeTMD     = yaroze.TMD(cubeTMDData[:])
)
```

A name is the file's path under the directory, its words run together in camel case, then its extension in capitals: `sound/STD0.VH` is `soundStd0VH`. Two files that would get the same name are an error. Files are placed under the directory's name in capitals unless `--root` says otherwise, `-o` names the output, and `--files` the slice. With `--into`, a file that grows needs the command run again, and `mkdisc` fails the build of a disc for one that has outgrown its array.

[`yaroze`](yaroze.md) has it as `yaroze generate`, with the same flags.
