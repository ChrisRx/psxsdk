---
title: psxplay
weight: 7
---

`psxplay` runs the emulator in a window, with the keyboard or a gamepad as the pad in port 1 and the SPU's sound on the speakers. It is the desktop counterpart of the player on the [examples](../examples/index.md) pages of this site, which runs the same emulator in a browser:

```bash
$ psxplay .build/check/check.cue
```

OpenBIOS is built in, so no BIOS image is needed; `--bios` boots another, such as Sony's. `--exe` loads a PSX-EXE when the BIOS reaches its shell instead of booting a disc, as `psxemu --exe` does, `--card` attaches a memory card image to slot 1 and writes it back on exit, `--scale` sets the window's initial size as a multiple of 320x240, and `--mute` leaves the sound off. What the BIOS prints to the TTY goes to standard output.

The keyboard is laid out as the player has it:

| Keys | Pad |
| --- | --- |
| Arrows | D-pad |
| Z, X, A, S | Cross, circle, square, triangle |
| Q, W | L1, R1 |
| 1, 2 | L2, R2 |
| Enter, Backspace | Start, Select |

A gamepad with a standard layout works as itself, with the left stick doubling as the d-pad. F11 toggles full screen and Escape leaves it.

In the repository, `task play NAME=check` builds an example's disc and runs it, with flags after `--` passed on: `task play NAME=check -- --mute`.

The window is drawn with [Ebitengine](https://ebitengine.org), which loads X11, OpenGL and ALSA at run time rather than linking against them, so building needs nothing beyond Go. On Linux the libraries must be on the loader's path to run it; the Nix development shell puts them there.
