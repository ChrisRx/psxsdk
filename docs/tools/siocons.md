---
title: siocons
weight: 12
---

`siocons` is SIOCONS, the console program Sony's Net Yaroze came with for its serial cable, written again in Go from SIOCONS.EXE's source and the [User Guide](../manuals/index.md). What is typed goes to the monitor on the console, and what the monitor and the programs it runs send comes back, so a program's `printf`, or `println` in Go, shows on the host. The function keys and batch files run SIOCONS's own local commands. [`yaroze`](yaroze.md) has it as `yaroze sio`.

```bash
$ siocons -p /dev/ttyUSB0 -B 115200 boot.bat
```

The serial device and its speed come from `-p DEVICE[,BAUD]`, or from the `DTLH3000` environment variable in the same form, as SIOCONS took its port from it, with `-B` for the speed. Without a device, `siocons` uses the first serial device it finds, and without a speed, 9600 bps. A batch file named on the command line runs first.

## Keys

| Key | Local command | What it does |
| --- | --- | --- |
| F1 | `help` | Show the keys, and with `y`, the commands |
| F2 | `dos`, `sh` or `!` | Run a command on the host |
| F3 | `auto FILE` | Run a batch file, or stop the ones running |
| F4 | `load FILE...` | Load programs: PS-X EXE, ECOFF or ELF, and set the registers they start with |
| F9 F4 | `dload FILE ADDR...` | Load files into memory, each at the hexadecimal address after it |
| F10 F4 | `dsave FILE ADDR SIZE` | Save memory into a file |
| F5 | `log FILE` | Log what the console shows into a file, added to it if it is there |
| F9 F5 | `log` | Stop logging |
| F8 | `edittgl` | Turn local edit mode on or off |
| F10 F2 | `quit` | Quit |
| Escape | | Quit, wherever it is pressed |

Control-C goes to the console, where it stops the monitor's command, and during a transfer it stops `siocons` too.

In local edit mode, a line typed at the monitor's prompt is edited on the host and sent whole when Enter is pressed. That line editor, and the one at the local commands' prompts, take the arrow keys and these:

| Key | What it does |
| --- | --- |
| Control-S, Control-D | Move left, right |
| Control-A, Control-F | Move to the start, the end |
| Control-H, Backspace | Delete the character before the cursor |
| Control-G, Delete | Delete the character at the cursor |
| Control-K | Delete to the end of the line |
| Control-Y | Delete the line |
| Control-E, Control-X | Go back, forward through the history |
| Tab, Control-J | Go back, forward to the next line that starts as this one does |

Typing `his` lists the history. It keeps the last 50 lines, and is saved in `siocons.his`, or the file `SIOCONSHIS` names, between runs.

## Batch files

A batch file has a line for each command typed. A line that starts with `local` is a local command, which runs at once. Any other line is typed to the monitor once the monitor shows its prompt. Besides the commands above, a batch file can use these:

| Command | What it does |
| --- | --- |
| `echo TEXT` | Show the text |
| `beep` | Sound the bell |
| `pause [TEXT]` | Wait for a key, after the text or a question mark. F3 stops the batch files |
| `sleep SECONDS` | Wait |
| `wait-prompt` | Wait for the monitor's prompt before the next line, even a local one |
| `auto-again` | Run the batch file again from its start |

A batch file can run another with `local auto`, up to 16 deep. A command that fails stops them all. A log of what was typed through the keys has each local command as a `local` line, so a log can be run as a batch file.

```text
local dload wheel256.tim 80190000
local load main.exe
local echo Press a key to start
local pause
go
```

## Debugging

The monitor's `rdb` starts the remote debugger, after which the line carries HSS, the binary protocol of the gdb that came with the Net Yaroze, rather than text. The [gdb manual](../manuals/gdb.md) has SIOCONS quit at that point and gdb take over with `target hss`. No gdb here speaks HSS, but package [`serial`](https://pkg.go.dev/go.chrisrx.dev/psx/serial)'s `Debugger` is the host's end of it in Go, which reads and writes registers and memory, steps and continues the program, and collects what it prints.

## Differences from SIOCONS

- **Escape always quits.** The User Guide has Escape put the line into the history in the line editor, but SIOCONS.EXE quits on Escape before the line editor gets the key, and so does `siocons`.
- **S-records are not loaded.** SIOCONS also loaded S-record files, through a command of the monitor's, and `siocons` says it does not when the monitor asks for one.
- **ELF is loaded too.** `yaroze build -o prog.elf` writes a program alone, without the library the boot disc leaves in memory, which is what `load` sends.
