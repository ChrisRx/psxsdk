/*
 * playstation-hot.ld - link script for a hot reload patch: the main
 * package's functions compiled afresh, bound by address to the program
 * that is running (see transform/hot.go), placed at __hot_at, an address
 * inside the region that program reserved with -hot. The loader copies
 * the patch's sections there, zeroes its .bss, and writes the slots
 * __hot_patch lists.
 */

ENTRY(__hot_entry)

__hot_entry = __hot_at;

SECTIONS
{
  . = __hot_at;

  .text : ALIGN(16)
  {
    *(.text)
    *(.text.*)
  }

  .rodata : ALIGN(4)
  {
    *(.rodata)
    *(.rodata.*)
    *(.rdata)
    *(.rdata.*)
  }

  .data : ALIGN(4)
  {
    *(.data)
    *(.data.*)
    *(.sdata)
    *(.sdata.*)
  }

  . = ALIGN(4);
  .bss : ALIGN(4)
  {
    *(.sbss)
    *(.sbss.*)
    *(.scommon)
    *(.bss)
    *(.bss.*)
    *(COMMON)
  }

  . = ALIGN(4);
  __hot_end_of_patch = .;

  /DISCARD/ :
  {
    *(.reginfo)
    *(.MIPS.abiflags)
    *(.MIPS.options)
    *(.gnu.attributes)
    *(.pdr)
    *(.comment)
    *(.note)
    *(.note.*)
    *(.eh_frame)
    *(.eh_frame_hdr)
    *(.gcc_except_table)
    *(.gcc_except_table.*)
    *(.mdebug.*)
    *(.gnu.lto_*)
  }
}
