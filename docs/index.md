---
title: psxsdk
tagline: A Go toolchain for the Net Yaroze
weight: 0
description: Write, build and play Net Yaroze programs: a Go toolchain for the original PlayStation, with Sony's manuals and its members' demos, running in the browser.
---

[![PkgGoDev](https://pkg.go.dev/badge/go.chrisrx.dev/psx)](https://pkg.go.dev/go.chrisrx.dev/psx)

The Net Yaroze was Sony's 1990s kit for making PlayStation games at home. psxsdk is a Go toolchain for it: write programs in Go, build them onto discs that boot anywhere, and run them in an emulator, here in the browser.

- **Play** [the examples](examples/index.md): Sony's Net Yaroze demos and members' games ported to Go, and a few new ones, each running on its own page, with touch controls on phones.
- **Write** your own in [the playground](/play/), which compiles Go into a PlayStation disc in the browser, or with nothing but Go installed: `go run go.chrisrx.dev/psx/cmd/nycw@latest disc ./mygame`.
- **Read** [Sony's manuals](manuals/index.md), with the FAQs, supplements and tutorial the members' websites carried.

## Start here

- [Getting started](guide/index.md): a program of your own, built onto a disc and played, with nothing but Go.
- [Writing programs in Go](guide/writing-programs.md): what the packages give a program.
- [The tutorial](tutorial/index.md) writes a program from nothing and puts it on a disc.
- [The cookbook](cookbook/index.md): how games did what most games do, with controllers, memory cards, CD audio, timing, scrolling, enemies and split screens.
- [History](about/index.md): the Net Yaroze, why psxsdk exists, and [how it was made](about/index.md#how-it-was-made).
