---
title: objdump
weight: 11
---

`objdump` displays information from ECOFF object files. It is similar in functionality to the objdump included in [GNU Binutils](https://www.gnu.org/software/binutils/) (although not intended to be exactly the same).

This was built while reverse engineering the Net Yaroze development static library, as the first step towards converting it from the aging ECOFF format to ELF (see [ecoff2asm](ecoff2asm.md)):

```bash
$ objdump puts.o
MIPSEL-BE ECOFF executable - start=0x00000000 size=96 sections=2

Sections:
 0 .text     len=80   offset=156  0x00000000 0x00000000
 1 .rdata    len=16   offset=236  0x00000050 0x00000050

Symbols:
[  0] e 0000000000000000 st 0 sc 0 index=FFFFF  gcc2_compiled.
[  1] e 0000000000000000 st 0 sc 0 index=FFFFF  __gnu_compiled_c
[  2] e 0000000000000000 st 0 sc 0 index=FFFFF  $LC0
[  3] e 0000000000000000 st 6 sc 1 index=0000   puts
[  4] e 0000000000000000 st 0 sc 0 index=FFFFF  $L13
[  5] e 0000000000000000 st 0 sc 0 index=FFFFF  $L11
[  6] e 0000000000000000 st 1 sc 6 index=FFFFF  putchar
[  7] l 0000000000000000 st b sc 1 index=0004   puts.c
[  8] l 0000000000000000 st 6 sc 1 index=0002   puts
[  9] l 000000000000004C st 8 sc 1 index=0001   puts
[ 10] l 0000000000000000 st 8 sc 1 index=0000   puts.c
```

It uses mewmew's [mips](https://github.com/mewmew/mips) library to decode and disassemble the provided file. Adding the `-d/-disassemble` flag will add the disassembly to the output:

```assembly
...

puts:
        addiu   $sp, $sp, 0xFFE8
        sw      $s0, 0x10($sp)
        addu    $s0, $a0, $zero
        bne     $s0, $zero, 24
        sw      $ra, 0x14($sp)
        lui     $s0, 0x0
        j       0x28
        addiu   $s0, $s0, 0x50
        jal     0x0
        sra     $a0, $a0, 24
        lbu     $a0, 0($s0)
        nop
        sll     $a0, $a0, 24
        bne     $a0, $zero, -24
        addiu   $s0, $s0, 0x1
        lw      $ra, 0x14($sp)
        lw      $s0, 0x10($sp)
        jr      $ra
        addiu   $sp, $sp, 0x18
        nop
        cfc3    $s5, $cp3_9
        syscall 0xF9
        nop
        nop
```
