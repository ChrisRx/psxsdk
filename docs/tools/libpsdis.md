---
title: libpsdis
weight: 13
---

`libpsdis` writes an annotated disassembly of the Net Yaroze resident library, the image [`eco2exe`](eco2exe.md) combines into every program. It exists for one job: reading the library's code when reimplementing a function of it in Go, which is what `exp/libps` does. Its output is Sony's code and is never committed; it goes to a directory outside the repository.

```bash
$ libpsdis -o /tmp/dis          # the whole image, as /tmp/dis/libps.txt
$ libpsdis -f putchar -f VSync  # named functions, to standard output
```

The only names the image has are the entry points the SDK's `symusr.o` gives, as [`ecoff2asm`](ecoff2asm.md) converted them to `playstation-yaroze-symusr.S`. Every other function is named by its address, `func_8002048c`. A function starts at a named entry point or at the target of a `jal` and ends where the next begins; the few named symbols past the last call target are the library's variables, not code, unless a `jr ra` follows them.

Each function's listing begins with its name, address and size, what it calls and what calls it, and annotates each instruction with what it reaches for:

```
putchar 0x80043790 size 180  calls: putchar write  called by: putc gets ...
80043790: 27BDFFE0  addiu    sp, sp, -32
...
8004382C: 0C010BE2  jal      0x80042f88                  ; write
```

- A `jal` names the function it calls, and a `j` into another function names it with the offset.
- A `lui` and the `addiu` or `ori` that finishes it name the address they form: a function, a variable with its current value, a string in quotes, or the word there.
- A load or store through that pair says the same of what it touches, and one through `gp` says the offset.
- The library calls the BIOS by loading a table's address, `0xA0`, `0xB0` or `0xC0`, jumping through the register and putting the function number in `t1` in the delay slot; such a `jr` is annotated `BIOS B(0x35) write`, with the function's name where the table of the BIOS calls the library makes knows it.

`--image` reads another PSX-EXE of the library in place of the one built in, and `--symbols` another symbol file.
