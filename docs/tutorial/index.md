---
title: Tutorial
weight: 4
group: Docs
section: Tutorial
description: Writing a PlayStation program in Go from nothing: a project of your own, then the square example, a box moved with the controller in front of a cube from the disc, to music.
---

The tutorial writes a PlayStation program in Go from nothing, outside this repository, the way a program of your own would be written. It assumes Go and nothing about the PlayStation. Where the machine's way of doing things differs from what a modern one would do, the page says why, because most of what looks strange about writing for it stops being strange once the reason is known.

The program it writes is the [square](../examples/square.md) example, the one the [playground](/play/) starts from: a box moved around the screen with the controller, in front of a textured cube turning in 3D, to music, with the cube and the music read off the disc while the frames go on. It is small, but it has the parts nearly every program needs, and each page adds one of them.

<!-- player square -->

1. [Setting up](setting-up.md): a module of your own with the toolchain as a dependency, the smallest program that puts something on the screen, and `nyc`, which turns it into a disc and plays it.
2. [The frame and the controller](frame.md): the frame loop, drawing a box, and moving it with the directional buttons.
3. [Files from the disc](disc.md): putting a model, a texture and music on the disc, and reading them while the frames go on.
4. [A model in 3D](model.md): the texture into video memory, the model linked to an object, a camera and a light, and the cube turning.
5. [Music](music.md): the sound bank sent to the sound processor, a sequence played on it, and the whole program.

Every listing on these pages is taken from [examples/square](examples/square) when the site is built, and the example is built and tested with every change, so what the pages show is a program that works.
