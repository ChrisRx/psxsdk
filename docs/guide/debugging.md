---
title: Debugging
weight: 6
description: Stopping a program at a line, stepping through it and reading its variables, with Delve for Go and GDB for C, in the emulator or on a console over the serial cable.
---

`yaroze debug` runs a program stopped at its first instruction, with a debugger at a prompt in the same terminal: breakpoints on lines and functions, steps, backtraces, and the program's variables and memory. A Go program is debugged with [Delve](https://github.com/go-delve/delve), which is built in, and a C program with GDB, which is downloaded the first time it is needed. The program runs in the emulator, or on a console at the other end of the serial cable.

## A Go program

Say `main.go` is this, a program that counts frames:

```go
// mygame counts frames, with functions that are not inlined to break in
// and step through, package variables to print, and a line printed each
// second.
package main

import "go.chrisrx.dev/psx/yaroze"

var screen = yaroze.Screen{Background: yaroze.Color{R: 60, G: 60, B: 120}}

var (
	frames int
	total  int
	odd    int
)

//go:noinline
func add(a, b int) int {
	return a + b
}

//go:noinline
func tick(n int) int {
	total = add(total, n)
	if n&1 != 0 {
		odd++
	}
	return total
}

func main() {
	println("debug: start")
	screen.Open()
	for range screen.Frames() {
		frames++
		tick(frames)
		if frames%60 == 0 {
			println("debug: second", frames/60)
		}
	}
}
```

In its directory, start it:

```console
$ go tool yaroze debug
yaroze: the program is stopped at 80010000, in the emulator, and Delve debugs it; GDB can connect on 127.0.0.1:1234 too
Type 'help' for list of commands.
(dlv)
```

The program is built with its debug information, which goes into `mygame.elf` beside it, and the window opens with the program stopped before its first instruction. Set a breakpoint on a function and let the program run to it:

```console
(dlv) break main.tick
Breakpoint 1 set at 0x80018604 for main.tick() /home/me/mygame/main.go:23
(dlv) continue
debug: start
> [Breakpoint 1] main.tick() /home/me/mygame/main.go:23 (hits total:1) (PC: 0x80018604)
    18:		return a + b
    19:	}
    20:
    21:	//go:noinline
    22:	func tick(n int) int {
=>  23:		total = add(total, n)
    24:		if n&1 != 0 {
    25:			odd++
    26:		}
    27:		return total
    28:	}
(dlv) args
n = 1
(dlv) print main.frames
1
(dlv) stack
0  0x0000000080018604 in main.tick
   at /home/me/mygame/main.go:23
1  0x000000008001805c in main.main$1
   at /home/me/mygame/main.go:35
2  0x000000008001805c in (*go.chrisrx.dev/psx/yaroze.Screen).Frames$1
   at .../yaroze/screen.go:356
3  0x000000008001805c in main.main
   at /home/me/mygame/main.go:0
4  0x000000008001805c in runtime.run
   at /sysroot/goroot/src/runtime/scheduler_none.go:27
5  0x000000008001805c in runtime.main
   at /sysroot/goroot/src/runtime/runtime_playstation.go:123
(dlv) next
> main.tick() /home/me/mygame/main.go:24 (PC: 0x80018614)
=>  24:		if n&1 != 0 {
(dlv) print main.total
1
(dlv) stepout
> runtime.main() /home/me/mygame/main.go:36 (PC: 0x8001805c)
=>  36:			if frames%60 == 0 {
(dlv) print main.screen.Background
go.chrisrx.dev/psx/yaroze.Color {R: 60, G: 60, B: 120}
```

Delve shows the lines around each stop, cut short here after the first. From there its usual commands work: `next` and `step` by lines, `step-instruction` by instructions, `stepout` to the end of a function, `args`, `locals` and `print` for variables, `examinemem` for memory, `regs` for the CPU's registers, `set` to change a variable, `clear` and `clearall` for breakpoints, and `help` for the rest. What the program prints with `println` shows while the program runs. Control-C stops a running program wherever it is, which is usually the library waiting for the next frame, and `stack` then says how it got there. Pressing it twice within half a second ends `yaroze debug`. `quit` ends the session.

### What to know for a Go program

- **Name a package's variables with the package.** `print main.frames` is the variable. Inside the body of a `range` loop over a function, as `screen.Frames()` is, a bare `frames` can name something else and read as unreadable.
- **A function may have no breakpoint of its own.** The compiler inlines small functions, and `main.main` nearly always, so a stop in `main` is reported as `runtime.main()`, the function it was inlined into. A line always works: `break main.go:34`. A function marked `//go:noinline` keeps its name.
- **Variables come and go.** A variable lives in a register for part of a function, and Delve cannot read it outside that part. Package-level variables are always there.
- **`stepout` does not show the returned value**, since the compiler passes values between a program's own functions its own way.
- **A stop in the library's assembly has no name.** After a Control-C the first frame is often `???`, the library's code that waits for the vertical blank, and the frames below it have their names and lines.
- **A fault stops at its instruction.** A misaligned access or an instruction the CPU does not have stops the program at the instruction that made it, and `stack` shows the line.
- **A Go panic does not stop the program, it parks it.** A nil pointer or an index out of range prints its message, such as `panic: runtime error: index out of range at 0x80014740`, and the program then waits in the runtime for good. Control-C stops it there.

The optimization level changes little of this. `--opt=2`, the default, inlines more than `--opt=z`, so fewer functions keep their names.

### In an editor

`--dap` serves Delve's debug adapter protocol on an address in place of the prompt, for an editor that connects to a debug adapter that is already running:

```bash
go tool yaroze debug --dap 127.0.0.1:2345
```

`yaroze debug` gives Delve the program and the way to it, so the editor's configuration only needs to connect. With VS Code's Go extension that is a launch configuration with the address:

```json
{
  "name": "PlayStation",
  "type": "go",
  "request": "launch",
  "mode": "exec",
  "program": "${workspaceFolder}/mygame.elf",
  "host": "127.0.0.1",
  "port": 2345
}
```

The session ends when the editor disconnects.

## A C program

`yaroze debug` debugs a C program built with GCC the same way, given the program in place of a package, with GDB at the prompt. The GDB is [GDB for WebAssembly](https://github.com/ChrisRx/gdb-wasi), which `yaroze debug` downloads the first time, 3 MB, keeps with its compilers, and runs itself. It is GNU GDB 17, under GDB's own license, which is why it is a download of its own rather than part of `yaroze`.

Both of [yaroze-toolchain](https://github.com/ChrisRx/yaroze-toolchain)'s compilers work: the current GCC, which builds ELF, and the Net Yaroze's own GCC 2.95.3, which builds ECOFF. To try either from nothing, start in an empty directory, and for the current GCC, with yaroze-toolchain's template:

```bash
nix flake init -t github:ChrisRx/yaroze-toolchain
```

Then save this as `main.c`, in place of the template's:

```c
#include <libps.h>

struct ball {
	int x, y;
	int dx, dy;
};

static struct ball ball = {10, 20, 1, 2};
int frames;

static void move(struct ball *b)
{
	int nx = b->x + b->dx;
	int ny = b->y + b->dy;
	if (nx < 0 || nx > 300)
		b->dx = -b->dx;
	if (ny < 0 || ny > 200)
		b->dy = -b->dy;
	b->x = nx;
	b->y = ny;
}

int main(void)
{
	SetVideoMode(MODE_NTSC);
	ResetGraph(0);
	for (;;) {
		frames++;
		move(&ball);
		VSync(0);
	}
	return 0;
}
```

### With the current GCC

The template's Makefile builds it, and `-g` adds the debugging information:

```bash
nix develop github:ChrisRx/yaroze-toolchain#sdk -c make CFLAGS="-g -O0"
go tool yaroze debug build/hello.elf
```

### With GCC 2.95.3

The `legacy-sdk` package is the old compiler with the SDK. It needs no template:

```bash
nix shell github:ChrisRx/yaroze-toolchain#legacy-sdk -c \
  mipsel-ecoff-gcc -O0 -g -msoft-float -Wl,-Ttext,0x80140000,-e,_start -o ball main.c
go tool yaroze debug ball
```

An ECOFF's debugging information is stabs, which current debuggers cannot read, so `yaroze debug` writes an ELF with DWARF made from it beside it, `ball.elf` for `ball`, and GDB reads that. C++ programs built with its `g++` work too. [`ecoff2elf`](../tools/ecoff2elf.md) has what is converted, and the limits of what GCC 2.95.3 recorded.

### The session

Either way, GDB starts at the program's first instruction:

```console
yaroze: the program is stopped at 80140000, in the emulator, and GDB debugs it; another GDB can connect on 127.0.0.1:1234
Reading symbols from /home/me/ball/build/hello.elf...
Remote debugging using 127.0.0.1:1234
0x80140000 in _start ()
(gdb) break move
Breakpoint 1 at 0x801401c8: file main.c, line 13.
(gdb) continue
Continuing.
Breakpoint 1, move (b=0x801404b4 <ball>) at main.c:13
13		int nx = b->x + b->dx;
(gdb) bt
#0  move (b=0x801404b4 <ball>) at main.c:13
#1  0x80140310 in main () at main.c:29
(gdb) print *b
$1 = {x = 10, y = 20, dx = 1, dy = 2}
(gdb) next
14		int ny = b->y + b->dy;
(gdb) next
15		if (nx < 0 || nx > 300)
(gdb) info locals
nx = 11
ny = 22
(gdb) finish
Run till exit from #0  move (b=0x801404b4 <ball>) at main.c:15
main () at main.c:30
30			VSync(0);
(gdb) print frames
$2 = 1
```

GDB's usual commands work: `next` and `step` by lines, `stepi` by instructions, `finish` to the end of a function, `info locals` and `print` for variables, `x` for memory, `info registers` for the CPU's registers, and `set var` to change a variable. A C program reads as C: `print ball` for a static, `ptype struct ball` for a type. Control-C stops a running program as it does for Delve, and GDB says it received `SIGINT`. `kill` ends the program and `quit` the session. The addresses differ between the two compilers.

`-O0` keeps every variable where the debugging information says it is. At `-O2`, the default of both Makefiles, GCC moves code around: the current GCC inlines `move` into `main`, so `break move` stops at its lines inside `main`, and GCC 2.95.3 says a variable is in its register for the whole function, so in places a variable reads what the register holds at the time.

`break VSync` stops in the library in a program from GCC 2.95.3. A program from the current GCC has the library's names with no type, which GDB does not break on by name: `break *&VSync` does, at its address.

## Another debugger

`yaroze debug` serves GDB's remote protocol whichever debugger it runs, on the address it prints, so another debugger connects too. `--serve` runs none, and says what to type in another terminal:

```console
$ go tool yaroze debug --serve
yaroze: the program is stopped at 80010000, in the emulator
yaroze: waiting for a debugger on 127.0.0.1:1234; in another terminal:

    gdb mygame.elf -ex 'target remote 127.0.0.1:1234'
```

The GDB has to be one built for MIPS targets. A distribution's `gdb-multiarch` is one, and so is the `gdb` in nixpkgs (`nix shell nixpkgs#gdb`). GDB reads a Go program as C, so a name with a dot in it needs quotes: `print 'main.frames'`, and `print 'main.screen'.Background` for a field of one.

## On a console

With a serial cable to a console whose monitor is waiting at its prompt, `--port` debugs the program there:

```bash
go tool yaroze debug --port /dev/ttyUSB0,115200
```

The program is built for the library the console has in memory, uploaded, and started with a breakpoint at its first instruction. Everything above works the same way, with these differences:

- Control-C stops the program only under the clean monitor. `yaroze debug` asks the monitor which it is, and under the Net Yaroze boot disc's monitor it says so, and the program runs until a breakpoint or a fault. There:
  - Set the breakpoints before `continue`. The debugger's prompt comes back only when the program reaches one or faults.
  - Control-C cannot stop the program. Pressing it twice within half a second ends `yaroze debug`.
  - The console stays in the monitor's remote debugger once the session ends, with no way back to the monitor's prompt. Reset it before the next upload.
- The library is the console's, and the program's ELF has no symbols for it. A program stopped inside the library has no backtrace past it. Set a breakpoint in the program and continue.
- Each stop reads the registers over the cable, a packet and its answer for each, about 2000 bytes on the cable in all. At 115200 bps that is a few tenths of a second a stop, and at 9600 bps a few seconds.

[`yaroze`](../tools/yaroze.md#debugging) has every flag, and the differences between the emulator and a console in full.
