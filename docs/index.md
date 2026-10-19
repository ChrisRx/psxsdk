---
title: psxsdk
weight: 0
description: A Go toolchain for the original PlayStation, built around the Net Yaroze: write programs in Go, build them into discs, run them in an emulator.
---

psxsdk is a Go toolchain for the original PlayStation, built around the Net Yaroze. It covers the whole path from source to screen:

- **Write** PlayStation programs in Go, compiled with [TinyGo](https://github.com/ChrisRx/tinygo) against generated bindings for the Net Yaroze library.
- **Build** them into bootable discs with tools that convert executables and write CD-XA images, including streamed XA/STR files and CD audio tracks.
- **Run** them in a PlayStation emulator written in Go, which drives the test suite headless and runs in a browser as WebAssembly.

**[Try the examples in your browser](examples/index.md)**: twenty-five programs in Go, most of them Net Yaroze demos ported, booting from the discs this repo builds, in the emulator this repo contains, with the open source [OpenBIOS](https://github.com/grumpycoders/pcsx-redux/tree/main/src/mips/openbios). Touch controls are included for phones. Or [write your own](/play/) in the playground, which builds it in the browser with TinyGo compiled to WebAssembly, keeps your projects in the browser, and downloads one as a zip that runs with `go run`.

## Start here

- [How it fits together](guide/index.md), the path from a Go source file to a running disc.
- [Getting started](guide/getting-started.md), to build the tools and the examples.
- [The tutorial](tutorial/index.md), which writes a first program and puts it on a disc.
- [The examples](examples/index.md), twenty-five programs running in the browser, with their sources.
- [Topics](topics/index.md), how games did what most games do: controllers, memory cards, CD audio, timing, scrolling, enemies and split screens.
- [The tools](tools/index.md), for converting executables, writing discs and inspecting the result.
- [Sony's manuals](manuals/index.md), with the FAQs, supplements and tutorial the Net Yaroze members' websites carried.
