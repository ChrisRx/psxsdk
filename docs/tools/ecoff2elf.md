---
title: ecoff2elf
weight: 1
---

`ecoff2elf` writes an ELF with DWARF debugging information for a Net Yaroze program built as ECOFF, such as by GCC 2.95.3 for `mipsel-ecoff`, the compiler the SDK was made for. A debugger loads the ELF, and the console or the emulator runs the original program: the two have the same sections at the same addresses, so either can be the one uploaded.

```bash
$ ecoff2elf main main.elf
$ gdb main.elf -ex 'target remote 127.0.0.1:1234'
```

[`yaroze debug`](yaroze.md#debugging) does the same itself when it is given an ECOFF, and debugs the program with GDB on the ELF it wrote. `ecoff2elf` is for a debugger of another kind, or for keeping the ELF.

It exists because current debuggers cannot read the ECOFF's own symbols. GCC 2.95.3 writes its debugging information as stabs inside ECOFF's symbol table, which GDB from 16 on crashes on, GDB has deprecated, and LLDB never read. Even GDB 15.2, which reads them, crashes on a program that has one file built without `-g`, or that names a function of Sony's `libps.a`.

The ELF has:

- each section of the program, at its address, with the same contents;
- a symbol table, with each function's size, and the resident library's entry points as functions in a section of their own, so that `break VSync` works;
- a DWARF compile unit for each source file, with a function for each procedure and the source lines GCC recorded;
- for a file compiled with `-g`, its types, its global and static variables, and each function's result, parameters and locals, in the blocks they belong to, so that `print`, `ptype`, `info locals` and `info args` work;
- frame information from each function's prologue and epilogue, so that a debugger unwinds through functions with no lines, such as those of a file built without `-g`, and stops at the program's entry point.

Some limits are those of the stabs GCC 2.95.3 writes:

- A variable in a register is said to be there for the whole function, so in optimized code it reads wrong where the register holds something else, and a parameter can read wrong at a function's first line, before it is moved to the register it is said to be in.
- `const` and `volatile` are not recorded.
- For C++, classes, their members and base classes, references and `this` are converted, and function names are demangled (`Ball::move`). A class's methods are not described as its members, so GDB cannot call them.

A stab the converter does not understand leaves its symbol without a type; the rest of the program is converted all the same.

[`yaroze debug`](yaroze.md#debugging) does this itself when it is given an ECOFF.
