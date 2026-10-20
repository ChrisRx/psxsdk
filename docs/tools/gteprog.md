---
title: gteprog
weight: 22
---

`gteprog` writes the GTE exercise program as a PSX-EXE, to run in another emulator and compare its GTE with this one's. For each case the program writes a value to every writable GTE register, runs one command, and stores all 64 registers to RAM. The cases cover every command with each combination of its shift and saturation bits, and MVMVA with every combination of matrix, vector and translation, with values drawn at several scales so that both the saturating and the ordinary paths are taken. The values come from a fixed seed, so the program is the same every time it is built.

```bash
$ gteprog gte.exe
508 cases; done at 80014d50; results 80100000-8011fc00
```

It prints the number of cases, the address the program finishes at, and where its results are in RAM. Run in psxe with the RAM traced, those results become `emu/cpu/testdata/gte.golden`, which `TestGTE` in [`emu/cpu`](emu/cpu) compares every register of every case against:

```bash
PSXE_TRACE=/dev/null PSXE_TRACE_RAM=psxe.ram PSXE_TRACE_COUNT=4000000 psxe -a -b bios.bin -x gte.exe
dd if=psxe.ram of=emu/cpu/testdata/gte.golden bs=256 skip=4096 count=<cases>
```

The golden file has to be made again whenever the program changes. To find where two GTEs part, trace both runs and compare them with [psxtrace](psxtrace.md):

```bash
PSXE_TRACE=psxe.trace PSXE_TRACE_COUNT=4000000 psxe -a -b bios.bin -x gte.exe
psxemu --bios bios.bin --exe gte.exe --trace emu.trace --count 4000000
psxtrace diff psxe.trace emu.trace
```
