---
title: Topics
weight: 4
section: Topics
description: How games did the things most games do, a page each, with an example to run and read: controllers, memory cards, CD audio, timing, scrolling backgrounds, enemies and split screens.
---

Each page here takes one thing nearly every game needs and explains how the PlayStation does it and how a Go program asks for it, with an example that does nothing else, running on the page. The examples are ports of the SCEE Net Yaroze tutorial's chapters and of Sony's samples, kept close to the originals so that the original and the port can be read side by side, with what changed and why in each one's comment.

| Topic | What it covers | Example |
| --- | --- | --- |
| [Controllers](controllers.md) | Every kind of controller: analog sticks, the mouse, the neGcon, the GunCon | [pads](../examples/pads.md) |
| [Memory cards](memory-cards.md) | Saving and loading, the save header and icon, a card's files | [card](../examples/card.md) |
| [CD audio](cd-audio.md) | Music recorded on the disc as audio tracks, played by the drive | [cdda](../examples/cdda.md) |
| [Timing](timing.md) | The root counters, a clock, and how much of a frame the work takes | [timers](../examples/timers.md) |
| [Scrolling backgrounds](backgrounds.md) | A tile map bigger than the screen, drawn by the library | [tiles](../examples/tiles.md) |
| [Enemies](enemies.md) | Chasing, fleeing, patterns and states | [ai](../examples/ai.md) |
| [Split screens](split-screen.md) | Two players, two cameras, two ordering tables | [split](../examples/split.md) |

The [guide](../guide/index.md) covers what every program has, the frame, the disc and the data, and the [tutorial](../tutorial/index.md) writes one from nothing. These pages assume the tutorial's program, or one like it.
