---
title: mips1fix
weight: 11
---

`mips1fix` rewrites MIPS II trap instructions as nops so that an object built by LLVM for `mips1` will run on a PlayStation.

LLVM emits a `teq` after every integer division to catch division by zero. That instruction arrived with MIPS II; a MIPS I core takes a reserved instruction exception instead, and the program stops with no indication of why. GCC does not have the problem, branching around a `break` instead.

```bash
$ mips1fix prog.o
  .text+0x1c50: teq (0x00a001f4) -> nop
  patched 1 trap instruction(s)
```

A trap and a nop are the same width, so nothing moves and no relocation or symbol needs adjusting. Clang suppresses these with `-mno-check-zero-division`, but that is an LLVM backend option rather than a target feature and so cannot be reached through TinyGo. Either way, integer division by zero becomes undefined rather than trapping.

`-n` reports without writing.
