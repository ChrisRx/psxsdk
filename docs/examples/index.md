---
title: Examples
weight: 3
section: Examples
description: Sixteen Net Yaroze programs ported to Go, running in the browser.
---

The examples are ports of Sony's Net Yaroze samples and demos, and of one of the games members wrote, chosen to put a different part of the machine to work each:

| Example | Original | What it exercises |
| --- | --- | --- |
| [check](check.md) | Sony SDK sample | sprites, controller, CD reads, sequenced music |
| [chrome](chrome.md) | S. Ashley, 1997 | GTE, 3D library, lighting, environment mapping |
| [clone](clone.md) | S. Ashley, 1997 | first-person maze shooter: fog, subdivision, SPU voices |
| [dungeon](dungeon.md) | Mizuki, 1996 | first-person maze built per frame, fog, two lights |
| [offscr](offscr.md) | L. Evans, 1997 | drawing into a texture at run time |
| [tunnel](tunnel.md) | L. Evans, 1997 | endless tunnel shooter, three lights, VAB sound effects |
| [flame](flame.md) | S. Ashley, 1997 | a sheet of fire: a TMD built in memory, recolored every frame, at 640 by 480 |
| [blow](blow.md) | S. Ashley, 1997 | a model viewer that blows its model apart, with every light and fog parameter on a menu |
| [dino](dino.md) | Sony, 1996 | a walking dinosaur: GTE vertex interpolation, transfer functions, a full-screen background, reverb |
| [invaders](invaders.md) | L. Evans, 1996 | Space Invaders in sprites, with a high score table |
| [breakout](breakout.md) | L. Evans, 1996 | Breakout in sprites, with the ball from a texture table |
| [dogtale](dogtale.md) | Ira Rainey, 1997 | A Dog Tale, a platform game: parallax scrolling, a screen grab spun away, PAL |
| [car](car.md) | Sony, 1995 | a textured car driving in from the distance, at sixty frames a second or thirty |
| [oden](oden.md) | Sony, 1993 | the lighting calculation: three shapes under three lights you aim and color |
| [morph](morph.md) | S. Ashley, 1997 | a textured sheet morphing into a cylinder, a torus and a sphere, vertex by vertex |
| [mdec](mdec.md) | new | full-color video through the motion decoder, driven by its registers |

Every example runs in the browser, in the emulator this repository contains, on the pages that follow and together in [the player](/play/). Each example's `data/` directory holds the files it reads off the CD, each registered in its `main.go` with the [`disc`](disc) package so that `mkpsxiso` places it, and `task disc NAME=check` builds one. The header comment of each `main.go` notes where the port departs from the original and why.
