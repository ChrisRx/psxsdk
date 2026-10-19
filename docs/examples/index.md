---
title: Examples
weight: 3
section: Examples
description: Eighteen PlayStation programs in Go, running in the browser.
order: dino, square
---

The examples are ports of Sony's Net Yaroze samples and demos and of games its members wrote, each chosen to put a different part of the machine to work, and a few programs of this repository's own. Every one runs in the browser, on its page, in the emulator this repository contains, with the keyboard, a gamepad or, on a phone, the controller drawn under the picture.

Each example is one directory under [examples](examples). Its `main.go` begins with a comment that is the example's page here: what it is, what it shows of the machine, where a port departs from the original and why, and its controls. Its `data/` directory holds the files it reads off the CD, each registered with the [`disc`](disc) package so that the build places it, and `go run ./cmd/nyc disc ./examples/dino` builds its disc, which `task disc NAME=dino` does too.
