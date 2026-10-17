---
title: psxsdk
weight: 0
description: A Go toolchain for the original PlayStation, built around the Net Yaroze: write programs in Go, build them into discs, run them in an emulator.
---

psxsdk is a Go toolchain for the original PlayStation, built around the Net Yaroze. It covers the whole path from source to screen:

- **Write** PlayStation programs in Go, compiled with [TinyGo](https://github.com/ChrisRx/tinygo) against generated bindings for the Net Yaroze library.
- **Build** them into bootable discs with tools that convert executables and write CD-XA images, including streamed XA/STR files and CD audio tracks.
- **Run** them in a PlayStation emulator written in Go, which drives the test suite headless and runs in a browser as WebAssembly.

**[Try the examples in your browser](examples/index.md)**: sixteen Net Yaroze programs ported to Go, booting from the discs this repo builds, in the emulator this repo contains, with the open source [OpenBIOS](https://github.com/grumpycoders/pcsx-redux/tree/main/src/mips/openbios). Touch controls are included for phones. Or [write your own](/play/) in the playground, which builds it in the browser with TinyGo compiled to WebAssembly.

## Start here

- [How it fits together](guide/index.md), the path from a Go source file to a running disc.
- [Getting started](guide/getting-started.md), to build the tools and the examples.
- [The tutorial](tutorial/index.md), which writes a first program and puts it on a disc.
- [The examples](examples/index.md), sixteen ports running in the browser, with their sources.
- [The tools](tools/index.md), for converting executables, writing discs and inspecting the result.
