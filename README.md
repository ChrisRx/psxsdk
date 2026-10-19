# psxsdk

[![PkgGoDev](https://pkg.go.dev/badge/go.chrisrx.dev/psx)](https://pkg.go.dev/go.chrisrx.dev/psx)
[![Build/Test](https://github.com/ChrisRx/psxsdk/actions/workflows/go.yml/badge.svg)](https://github.com/ChrisRx/psxsdk/actions/workflows/go.yml)
[![Pages](https://github.com/ChrisRx/psxsdk/actions/workflows/pages.yml/badge.svg)](https://github.com/ChrisRx/psxsdk/actions/workflows/pages.yml)

psxsdk is a Go toolchain for the original PlayStation, built around the Net Yaroze. It covers the whole path from source to screen:

- **Write** PlayStation programs in Go, compiled with TinyGo against generated bindings for the Net Yaroze library.
- **Build** them into bootable discs with tools that convert executables and write CD-XA images, including streamed XA/STR files and CD audio tracks.
- **Run** them in a PlayStation emulator written in Go, which drives the test suite headless, runs in a browser as WebAssembly, and opens a window on the desktop.

**[The documentation](https://chrisrx.github.io/psxsdk/)** has the guide, a tutorial that writes a program from nothing, and the reference for every tool. **[The examples](https://chrisrx.github.io/psxsdk/examples/)** are seventeen Net Yaroze programs ported to Go, running in the browser on the discs this repository builds, in the emulator it contains.

## Getting started

Everything is built with Go 1.27 or newer:

```bash
git clone https://github.com/ChrisRx/psxsdk
cd psxsdk
go build -o bin/ ./cmd/... ./tools/...
go test $(go list ./... | grep -v examples)   # or: task test
```

Building the examples also needs TinyGo with the `playstation-yaroze` target. The Nix flake provides it, along with the cross binutils, [go-task](https://taskfile.dev), mednafen and a patched [psxe](https://github.com/allkern/psxe) for comparison:

```bash
nix develop            # or: direnv allow
task discs             # every example onto .build/NAME/NAME.cue
task test:emu          # boot each disc in the emulator and compare screenshots
task site site:serve   # build the documentation site and serve it on localhost:8000
```

The tutorial's [first chapter](https://chrisrx.github.io/psxsdk/tutorial/setting-up/) starts a program of your own outside the repository, with the tools as `go tool` dependencies.

## What is here

| Directory | Purpose |
| --- | --- |
| [`yaroze`](yaroze) | TinyGo bindings for the resident Net Yaroze library, generated from the SDK's header, with stubs for every other build, and the host-side `libps` image |
| [`cmd/nyc`](cmd/nyc) | The build command: a Go program to a PSX-EXE or a disc image in one step, with the tinygo command or TinyGo compiled to WebAssembly |
| [`disc`](disc) | Registering the files a program reads off the disc, which the build reads out of the executable |
| [`examples`](examples) | The ported demos, one directory each, with the data they load |
| [`tools`](tools) | `eco2exe`, `mkpsxiso`, `dumpsxiso`, `psxemu`, `psxplay` and the rest; the [tools section](https://chrisrx.github.io/psxsdk/tools/) describes each |
| [`emu`](emu) | The emulator, with [`emu/emutest`](emu/emutest) for driving it from tests |
| [`format`](format) | PSX-EXE, ECOFF, CD-ROM sectors, ISO 9660 with CD-XA, whole discs, MDEC movies, instruction traces |
| [`web`](web) | The emulator's WebAssembly build, the player, and the generator for the documentation site |
| [`docs`](docs) | The documentation's pages; the examples' pages come from their package comments |

## Background

psxsdk started as software for using the Net Yaroze on Linux. When getting started with a Net Yaroze DTL-H3001, the development environment on modern computers, Linux in particular, was badly lacking. After getting a working compiler with [mipsel-ecoff-toolchain](https://github.com/ChrisRx/mipsel-ecoff-toolchain), the project grew to cover whatever else came out of making Net Yaroze development work on modern machines, and then to writing the programs themselves in Go and running them in an emulator that can check them. The Net Yaroze might not be the ideal PS1 development environment, but the ability to use it should not be lost to time.

The goals have not changed:

* Software that is cross-platform and easy to build
* Modern languages with features that help the software last
* Libraries and tools that are well-defined and well-documented
* Source code for everything, always available

## TODO

- [x] ECOFF to PSX-EXE converter (eco2exe)
- [x] Net Yaroze executable serial loader (sioload)
- [x] PSX ISO builder (mkpsxiso), with CD-XA attributes, streamed XA/STR files and CD audio tracks
- [x] PSX ISO reader and extractor (dumpsxiso)
- [x] Go bindings for the Net Yaroze library, and examples in Go
- [x] Emulator, headless test harness and browser demo
- [x] One build command (nyc), with TinyGo native or compiled to WebAssembly
- [ ] Document code
- [ ] Tests for all packages
- [ ] Include collected PDFs and other Net Yaroze documents in repo

## License

[MIT](LICENSE). Two parts carry their own notices:

- The emulator in `emu` is derived from [psxe](https://github.com/allkern/psxe), Copyright (c) 2024 Allkern (Lisandro Alarcón), under the MIT license in [`emu/PSXE-LICENSE.txt`](emu/PSXE-LICENSE.txt).
- The OpenBIOS image in `web/static` is under the license in [`web/static/OPENBIOS-LICENSE.txt`](web/static/OPENBIOS-LICENSE.txt).
