---
title: psxemu
weight: 6
---

`psxemu` boots a BIOS in the emulator headless, printing what it writes to the TTY, with a disc in the drive or a PSX-EXE loaded at the shell:

```bash
$ psxemu --bios openbios.bin --disc .build/check/check.cue --png screen.png
```

`--trace`, `--skip` and `--count` write an instruction trace in the format the patched psxe writes with `PSXE_TRACE`. `--vram` dumps VRAM at the end, `--png` saves what the display shows, `--card` attaches a memory card image to slot 1, and `--cpuprofile` profiles the run for `go tool pprof`.

## Driving a program

The rest of the options run the program frame by frame, which is how a port is compared with the program it was ported from: both run the same script, and the words of memory they are told to watch are compared frame by frame. Frames are vertical blanks counted from the BIOS starting the program.

```bash
$ psxemu --bios scph1001.bin --disc original.cue --until 8017fa54:e0 \
    --script "x:4;400;u+r:120" --every 100 --frames shots \
    --watch "*801742a4+64:2" --poke 6100:8017e250:100
```

- `--until addr:value` runs until the word at the address, in hexadecimal, equals the value, before the script starts; it can be given more than once, and `--until-budget` limits the frames each may take.
- `--script` holds buttons for frames: steps separated by semicolons, each `buttons:frames` or just `frames`, with buttons joined by `+` from `u d l r x o q t L1 R1 L2 R2 st se`.
- `--every N` saves the display every N frames of the script into the directory `--frames` names, as `s00100.png` and so on by the frame's offset into the script.
- `--watch addr:words` prints the words whenever one changes, with the frame; `*addr+off:words` follows the pointer held at the address, with the offset added, so an entity reached through a pointer can be watched while it moves. Several watches are printed with their numbers.
- `--poke frame:addr:value` writes the word after that frame of the script; a value starting with `|` is ORed into the word. With the player's position and an edge flag poked, a game can be taken through its areas without the walk.
- `--profile symbols.txt` samples where the cycles went through the script and prints the share of each symbol, then the hottest stretches of code; the file holds lines of a hexadecimal address and a name, as `go tool nm` prints. `--profile-window a:b` limits the sampling to those frames.
- `--polygons` prints how many polygons the GPU drew per vertical blank over the script, by kind.
- `--break 0x80012345` prints the frame and the argument, return address and stack registers each time the program reaches an address, with or without a script; it can be given more than once.

