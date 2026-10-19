/*
 * Sony PlayStation (R3000A), standalone.
 *
 * RAM on a retail console is 2MB, mapped three times: cached through KSEG0
 * at 0x80000000, uncached through KSEG1 at 0xA0000000, and at 0x00000000 in
 * user space. Programs run cached, so every address here is a KSEG0 one.
 *
 *   0x80000000  BIOS kernel: exception vectors, jump tables, its own scratch.
 *               64K, and not ours to touch.
 *   0x80010000  where the BIOS loader places a PSX-EXE, so where this loads.
 *   0x801ffff0  initial stack pointer, growing down
 *   0x80200000  end of RAM
 *
 * There is no ROM and nothing is memory mapped: the loader reads the whole
 * image into RAM before the program runs, so .data arrives initialised and
 * only .bss has to be cleared. That is why _sidata and _sdata are the same
 * address, unlike a target that executes out of flash.
 *
 * Override __load to run somewhere other than the usual address, and
 * __ram_end for a development unit with 8MB:
 *
 *   -Wl,--defsym,__ram_end=0x80800000
 */

OUTPUT_ARCH(mips)
ENTRY(_start)

__load = DEFINED(__load) ? __load : 0x80010000;
__ram_end = DEFINED(__ram_end) ? __ram_end : 0x80200000;

/* Reserved for the stack, below which the heap ends. The R3000A has no
 * memory protection, so an overflow runs into the heap silently; this is a
 * budget, not a guard. */
__stack_size = DEFINED(__stack_size) ? __stack_size : 0x10000;

SECTIONS
{
  . = __load;

  .text : ALIGN(4)
  {
    /* The crt0 is the entry point and nothing references it, so it has to be
     * kept explicitly and placed first: the loader is given one address and
     * jumps there. */
    KEEP(*(.init))
    *(.text)
    *(.text.*)
    . = ALIGN(4);
  }

  .rodata : ALIGN(4)
  {
    *(.rodata)
    *(.rodata.*)
    . = ALIGN(4);
  }

  _sidata = LOADADDR(.data);

  .data : ALIGN(4)
  {
    _sdata = .;
    *(.data)
    *(.data.*)
    /* -G0 should mean these are empty, but an object compiled without it
     * would otherwise have its small data silently dropped. */
    *(.sdata)
    *(.sdata.*)
    /* LLVM emits a small .got for this target even with -fno-pic and
     * -mno-abicalls. Placing it rather than leaving it to be an orphan keeps
     * it inside _globals_start to _globals_end, where a collector will scan
     * it. */
    *(.got)
    *(.got.*)
    . = ALIGN(4);
    _edata = .;
  }

  .bss (NOLOAD) : ALIGN(4)
  {
    _sbss = .;
    *(.bss)
    *(.bss.*)
    *(.sbss)
    *(.sbss.*)
    *(COMMON)
    . = ALIGN(4);
    _ebss = .;
  }

  _end = .;

  /* MIPS object attributes and the ABI flags section describe the producing
   * toolchain and have no place in a flat image. */
  /DISCARD/ :
  {
    *(.MIPS.abiflags)
    *(.gnu.attributes)
    *(.reginfo)
    *(.comment)
    *(.note)
    *(.note.*)
  }
}

/* What TinyGo's runtime reads: the heap is everything between the end of the
 * program and the stack, and the globals are what a collector has to scan as
 * roots. */
_heap_start = _end;
_heap_end = __ram_end - __stack_size;
_globals_start = _sdata;
_globals_end = _ebss;
_stack_top = __ram_end - 16;
