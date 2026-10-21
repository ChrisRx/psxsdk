---
title: psxplay
weight: 7
---

`psxplay` runs the emulator in a window, with the keyboard or a gamepad as the pad in port 1 and the SPU's sound on the speakers. It is the desktop counterpart of the player on the [examples](../examples/index.md) pages of this site, which runs the same emulator in a browser:

```bash
$ psxplay .build/check/check.cue
```

OpenBIOS is built in, so no BIOS image is needed; `--bios` boots another, such as Sony's. `--exe` loads a PSX-EXE when the BIOS reaches its shell instead of booting a disc, as `psxemu --exe` does, `--card` attaches a memory card image to slot 1 and writes it back on exit, `--scale` sets the window's initial size as a multiple of 320x240, and `--mute` leaves the sound off. The BIOS's start and a program's loading, a few seconds of black screen, run as fast as the host allows, with no picture or sound, until the display first shows something, and in real time from then on; `--fast-start=false` runs them in real time too. What the BIOS prints to the TTY goes to standard output. `--break 0x80012345` prints the frame and the argument, return address and stack registers each time the program reaches an address, which `go tool nm` on the program's ELF gives for a function; it can be given more than once.

The keyboard is laid out as the player has it:

| Keys | Pad |
| --- | --- |
| Arrows | D-pad |
| Z, X, A, S | Cross, circle, square, triangle |
| Q, W | L1, R1 |
| 1, 2 | L2, R2 |
| Enter, Backspace | Start, Select |

A gamepad with a standard layout works as itself, with the left stick doubling as the d-pad. F11 toggles full screen and Escape leaves it.

[`nyc run`](nyc.md) builds a program onto a disc and runs it in the same window, with the same flags: `nyc run --mute ./examples/check`. In the repository, `task play NAME=check` does that for an example, with flags after `--` passed on: `task play NAME=check -- --mute`.

The window is drawn with [Ebitengine](https://ebitengine.org), which loads X11, OpenGL and ALSA at run time rather than linking against them, so building needs nothing beyond Go. On Linux the libraries must be on the loader's path to run it; the Nix development shell puts them there.
