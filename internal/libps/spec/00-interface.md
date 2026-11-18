# The library's interface and environment

## What links against it

A program built for the `playstation-yaroze-go` target with the `cleanlib`
tag imports this package through `yaroze/libps_clean.go`. The `yaroze`
package declares each library function without a body and with
`//export Name`, so the linker resolves it to whatever defines the C
symbol `Name`. This package defines them:

```go
//export GsSortClear
func GsSortClear(r, g, b uint8, ot *GsOT) { ... }
```

The signature must match the binding's in `yaroze/` exactly, argument for
argument (the C calling convention: a0 to a3, then the stack). Struct
types are this package's own copies with the same layout as the `yaroze`
ones (this package must not import `yaroze`, which imports it). Pointers
programs pass are C pointers: never assume Go allocated them.

Library variables programs reach by name (`GsDISPENV`, `GsDRAWENV`,
`GsIDMATRIX` and the rest) have their storage in
`internal/toolchain/_tinygo/targets/playstation-yaroze-libps-data.c`. Reach
them with `//go:extern Name` on a package-level variable of the right
type. A `//go:extern` variable must be declared in the package that uses
it: TinyGo does not resolve one declared in another package.

Kernel calls (the A0, B0 and C0 tables) go through the trampolines in
`internal/toolchain/_tinygo/targets/playstation-yaroze-bios.S`; read that
file for their Go-side declarations and how to add one.

## What has happened before main

The disc boots `MAIN.EXE` straight from the BIOS. The crt0 sets up the
stack and clears .bss; the runtime then calls `InitHeap` with the area
between the end of the program and the Go heap, then runs `main`. Nothing
else of the library has run: no interrupt handling is installed, the GPU
is as the BIOS left it. Whatever state the functions need (a VSync
counter, say) the library sets up itself, on first use or in a Go `init`.

## TinyGo pitfalls on this target

- A nil Go pointer check panics where a C program would happily read
  address 0. Where a program may pass a null pointer that the hardware
  would read as RAM at address 0, read through 0x80000000 instead (the
  same memory, cached).
- Ranging over an array by value copies it: range over a pointer or index.
- Keep interrupt handlers short, allocation-free, and save every register
  the kernel does not.
- A panic shows as the PC spinning in `runtime.runtimePanicAt`; the TTY
  carries the message.
- Use `internal/volatile` (or `unsafe` pointer reads of hardware
  addresses) for registers; TinyGo may otherwise cache or drop the access.

## Hardware

The emulator in `emu/` is the reference for hardware behavior: `emu/gpu.go`
(GP0/GP1, the status register), `emu/dma.go`, `emu/bus.go` (interrupts), `emu/timers.go` and
so on. Public PlayStation documentation describes the same registers.
