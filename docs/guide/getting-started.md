---
title: Getting started
weight: 1
---

Everything is built with Go 1.27 or newer:

```bash
git clone https://github.com/ChrisRx/psxsdk
cd psxsdk
go build -o bin/ ./cmd/... ./tools/...
go test $(go list ./... | grep -v examples)   # or: task test
```

That is enough for the build command, [`nyc`](../tools/nyc.md), the other tools, the emulator and the host-side tests. Building a program also needs a compiler, which `nyc` finds for itself: the tinygo command when there is one, with the PlayStation targets added if it lacks them, or else TinyGo compiled to WebAssembly, which a `nyc` built by `task nyc` carries. The Nix flake provides the TinyGo fork, along with the cross binutils, [go-task](https://taskfile.dev), mednafen and a patched [psxe](https://github.com/allkern/psxe) for comparison:

```bash
nix develop            # or: direnv allow
task discs             # every example onto .build/NAME/NAME.cue
task test:emu          # boot each disc in the emulator and compare screenshots
task site site:serve   # build the browser demo and serve it on localhost:8000
```

`task test:emu` boots with the OpenBIOS image in `emu/openbios`, or whatever `PSX_BIOS` names. The memory card test drives Sony's shell, which OpenBIOS does not have, and only runs when `PSX_SONY_BIOS` points at a Sony BIOS. No Sony BIOS is distributed here.

`nyc` builds a single program, onto a disc or into a window:

```bash
go run ./cmd/nyc disc ./examples/check   # check.bin and check.cue
go run ./cmd/nyc run ./examples/check    # the same disc, played in a window
```

`task disc NAME=check` and `task play NAME=check` do the same through the Taskfile, and the tools of the build's single steps, such as `mkpsxiso` for a disc laid out by hand, are declared as Go tools, so `go tool mkpsxiso` runs one without installing anything.
