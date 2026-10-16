---
title: psxemu
weight: 6
---

`psxemu` boots a BIOS in the emulator headless, printing what it writes to the TTY, with a disc in the drive or a PSX-EXE loaded at the shell:

```bash
$ psxemu --bios openbios.bin --disc .build/check/check.cue --png screen.png
```

`--trace`, `--skip` and `--count` write an instruction trace in the format the patched psxe writes with `PSXE_TRACE`. `--vram` dumps VRAM at the end, `--png` saves what the display shows, `--card` attaches a memory card image to slot 1, and `--cpuprofile` profiles the run for `go tool pprof`.
