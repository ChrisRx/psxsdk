---
title: Controllers
weight: 1
description: Reading every kind of controller, not only the standard pad: analog sticks and their dead zones, the mouse, the neGcon's twist and the GunCon's aim.
---

The library reads both controller ports at every vertical blank, by itself, into two buffers of 34 bytes. `yaroze.Pad` reads the standard controller's buttons out of the first one, which is all most programs need. Everything else that plugs into a port, analog controllers, joysticks, the mouse, the neGcon and the GunCon, sends more, and says what it is so a program can tell how to read the rest. `yaroze.ReadController` reads any of them:

```go src=../../yaroze/controller.go#Controller
```

A buffer starts with a byte that is 0 while something is connected and 0xff while nothing is, then a byte whose high four bits say what kind of controller it is and whose low four the length of what follows, in 16-bit halves. The buttons come first, as on the standard controller, held buttons as 0 bits, and then whatever else the kind sends: an analog controller's four axes, a mouse's movement since the last read, a neGcon's twist and its analog buttons, a GunCon's position.

## Sticks and dead zones

An analog stick reads 0 at one end of its travel and 255 at the other, but at rest it is near 128 rather than on it, and a different near on every controller. A program that moved by the raw value would drift. `Stick` centers an axis on zero and calls anything within a dead zone of the middle no movement:

```go src=../../yaroze/controller.go#Controller.Stick
```

The tutorial's analog example had the player calibrate the stick first. Sony's checklist for the DUAL SHOCK ([Sony's requirements](../manuals/requirements.md)) forbids a calibration screen, since its sticks calibrate themselves, so a dead zone is all a program does now.

An analog controller starts in digital mode, sending what a standard controller sends, until its player presses the analog button. A program that wants the sticks says so on its screen, and goes on working with the directions.

## The mouse, the neGcon and the GunCon

A mouse sends how far it moved since the last read, -128 to 127 each way, rather than where it is: a program adds it to a cursor of its own. A neGcon is a controller twisted in the middle, sent as an axis like a stick's, with its I and II buttons as axes too, pressed further for more. A GunCon sends where on the television it is pointing, measured as the television draws: dots of the GPU's clock from the start of a line, and lines from the top. Where a 320-wide picture starts and ends in those units depends on the television, so a game had the player shoot a few targets to calibrate it.

The [pads](../examples/pads.md) example names what is in each port, lists what it sends, and moves a cursor with any of them:

```go src=../../examples/pads/main.go#move
```

<!-- player pads -->

## In the emulator

The emulator plugs in a standard controller by default. Its tests plug in the others with `emu.NewPad` and `Machine.ConnectPad`, which is how the pads example's test drives an analog stick, a mouse, a GunCon and a neGcon.

The SCEE tutorial's [peripherals chapter](../manuals/tutorial-peripherals.md) and Sony's [peripherals notes](../manuals/peripherals.md) have the original programs and the buffers' layouts in full.
