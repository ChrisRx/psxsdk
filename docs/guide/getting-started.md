---
title: Getting started
weight: 1
---

Everything is built with Go 1.27 or newer:

```bash
git clone https://github.com/ChrisRx/psxsdk
cd psxsdk
go build -o bin/ ./tools/...
go test $(go list ./... | grep -v examples)   # or: task test
```

That is enough for the disc and executable tools, the emulator and the host-side tests. Building the examples additionally needs the TinyGo fork that carries the `playstation-yaroze` target. The Nix flake provides it, along with the cross binutils, [go-task](https://taskfile.dev), mednafen and a patched [psxe](https://github.com/allkern/psxe) for comparison:

```bash
nix develop            # or: direnv allow
task discs             # every example onto .build/NAME/NAME.cue
task test:emu          # boot each disc in the emulator and compare screenshots
task site site:serve   # build the browser demo and serve it on localhost:8000
```

`task test:emu` boots with the OpenBIOS image in `web/static`, or whatever `PSX_BIOS` names. The memory card test drives Sony's shell, which OpenBIOS does not have, and only runs when `PSX_SONY_BIOS` points at a Sony BIOS. No Sony BIOS is distributed here.

The tools a disc build uses are also declared as Go tools, so inside the module they run without installing anything:

```bash
go tool mkpsxiso -o game.bin --boot MAIN.EXE ./disc
```
