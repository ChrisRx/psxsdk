---
title: ecoff2asm and ecoffcmp
weight: 9
---

The Net Yaroze library and SDK objects are ECOFF, a format current binutils no longer links. `ecoff2asm` converts ECOFF objects and archives to GNU as source, along with an ld script of `PROVIDE` definitions for every absolute symbol, which is how the `libps` entry points are described. `ecoffcmp` checks the conversion: it assembles the output and compares its `.text` against the original word by word, masking only the immediate fields where ECOFF and ELF keep relocation addends in different places.
