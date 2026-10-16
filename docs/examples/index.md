---
title: Examples
weight: 3
section: Examples
description: Eight Net Yaroze demos ported to Go, running in the browser.
---

The examples are ports of Sony's Net Yaroze samples and demos, chosen to put a different part of the machine to work each:

| Example | Original | What it exercises |
| --- | --- | --- |
| [check](check.md) | Sony SDK sample | sprites, controller, CD reads, sequenced music |
| [chrome](chrome.md) | S. Ashley, 1997 | GTE, 3D library, lighting, environment mapping |
| [clone](clone.md) | S. Ashley, 1997 | first-person maze shooter: fog, subdivision, SPU voices |
| [dungeon](dungeon.md) | Mizuki, 1996 | first-person maze built per frame, fog, two lights |
| [flying](flying.md) | L. Evans, 1996 | ship over a tiled plane, enemies, scanner, nine cameras |
| [offscr](offscr.md) | L. Evans, 1997 | drawing into a texture at run time |
| [tunnel](tunnel.md) | L. Evans, 1997 | endless tunnel shooter, three lights, VAB sound effects |
| [mdec](mdec.md) | new | full-colour video through the motion decoder, driven by its registers |

Every example runs in the browser, in the emulator this repository contains, on the pages that follow and together in [the player](/play/). Each example's `data/` directory holds the files it reads off the CD, each registered in its `main.go` with the [`disc`](disc) package so that `mkpsxiso` places it, and `task disc NAME=check` builds one. The header comment of each `main.go` notes where the port departs from the original and why.
