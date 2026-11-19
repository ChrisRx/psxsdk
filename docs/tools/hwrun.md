---
title: hwrun
weight: 22
---

`hwrun` runs programs on a real Net Yaroze over the serial cable and keeps what each prints in a file. It is for the hardware sessions that compare the clean library with Sony's on the console. It runs from the repository, not as an installed tool.

```bash
$ go run ./tools/hwrun -d /dev/ttyUSB0 -library sony  -o results .build/serial/hwprobe.elf
$ go run ./tools/hwrun -d /dev/ttyUSB0 -library clean -o results .build/serial/hwprobe.elf
```

Each program's output goes to `NAME-LIBRARY.txt` in the directory `-o` names, `results` by default. A program that already has a file there is skipped, so a session can stop and continue later. A program runs until it prints its end mark, or for as long as `-watch` says, 60 seconds by default. When the console has to be reset, `hwrun` says so and waits for Enter.

`-library` chooses what the program runs on:

- `sony`, the default, is the library as the boot disc left it.
- `clean` first replaces the boot disc's monitor and library with the clean ones, once per boot: the monitor from `-monitor`, which `task build:serial` builds, and the image from `-image`, `link/libps.exe.gz` by default. Programs then return to that monitor.
- `sony-ourmon` is Sony's library under the clean monitor, for comparing the two libraries under one monitor.

The clean monitor can stop a running program and let it continue. `-peek 20s,30s` does that at those times into each run and saves the registers, with the memory `-read` names as `ADDR:SIZE,...`, the address in hexadecimal:

```bash
$ go run ./tools/hwrun -d /dev/ttyUSB0 -library clean -peek 20s,30s -read 80010000:256 prog.elf
```

With `-archive`, the arguments are names from a `programs.tsv`, and each program's data files are loaded as its `auto` batch file loads them:

```bash
$ go run ./tools/hwrun -d /dev/ttyUSB0 -archive .build/carchive/programs.tsv demos-scee-tunelfin
```

`-b` sets the baud rate, 115200 by default. [`sioloop`](sioloop.md) checks the serial adapter before a session.
