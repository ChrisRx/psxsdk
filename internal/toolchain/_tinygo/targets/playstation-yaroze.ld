/*
 * playstation-yaroze.ld - link script for Net Yaroze programs built as
 * mipsel-none-elf
 *
 * The Net Yaroze boot disk (DTL-S3035) leaves libps.exe resident in RAM and
 * runs a serial loader. A Net Yaroze program is therefore not self-contained:
 * it is linked against fixed addresses inside that resident library.
 * playstation-yaroze-symusr.S supplies those addresses, converted directly
 * from the SDK's symusr.o so they cannot drift from the original.
 *
 * RAM map:
 *
 *   0x80000000  kernel
 *   0x80010000  resident libps.exe, 388KB
 *   0x80090000  user program (default; override with -Wl,--defsym,__load=...)
 *   0x801fff00  initial stack pointer, growing down
 *   0x80200000  end of 2MB RAM
 */

ENTRY(_start)

/* Load address of the program. 0x80090000 is where Sony's memory map starts
 * the application area, just above the system area that holds the kernel
 * and the resident library. The SDK's own default was 0x80140000, which
 * left 704KB below the program for data the serial loader's batch files
 * sent to fixed addresses; a Go program keeps its data in its own
 * variables, so the gap would only be 704KB of zeros in every executable
 * and 704KB less for those variables. A program that needs the old layout
 * can ask for it. */
__load = DEFINED(__load) ? __load : 0x80090000;

/* Where the stack starts and how much of it is reserved. These match what
 * eco2exe records in the PSX-EXE header and what stdef2.o gives startup.s as
 * _sys_init_stacksize, so the heap computed from them agrees with the one
 * startup.s hands to InitHeap. */
__stack_top = DEFINED(__stack_top) ? __stack_top : 0x801fff00;
__stack_size = DEFINED(__stack_size) ? __stack_size : 0x10000;

SECTIONS
{
  /* playstation-yaroze-symusr.S defines every libps entry point as an offset
   * into .libps, so placing the section here is what gives those symbols
   * their real addresses. The code is already in RAM, so nothing is loaded.
   *
   * NOLOAD rather than INFO, because lld ignores the address on an INFO
   * section and resolves every one of these symbols without the 0x80000000
   * base: FntPrint comes out at 0x000258fc instead of 0x800258fc. It links
   * silently, since R_MIPS_26 still reaches, and the program dies before it
   * draws. GNU ld honours either spelling.
   *
   * The cost of NOLOAD is that the window counts as allocated memory, so
   * size(1) reports the 420KB span of the resident library as if it were
   * part of the program, and a second PT_LOAD covers it. That segment has a
   * file size of zero, which is what eco2exe and sioload select on, so
   * neither writes anything there. An ELF is also 420KB smaller now:
   * an INFO section is PROGBITS and occupies file space even unallocated,
   * whereas NOBITS does not.
   *
   * It has to be an object defining these rather than assignments in this
   * script. A script assignment is applied only after the linker has scanned
   * the input archives, so a reference to a routine libps shares with libgcc,
   * __extendsfdf2 for instance, would pull libgcc's copy into the image before
   * the resident address was ever considered. Defining them in an object that
   * is linked first means no such member is extracted at all, which is how the
   * SDK's own symusr.o behaved as a member of libps.a.
   */
  .libps 0x80000000 (NOLOAD) : { *(.libps) }

  . = __load;

  .text : ALIGN(4)
  {
    /* playstation-yaroze.S is first on the link line, so its _start lands at
     * the base of the image. Note that *(.text.*) has to stay below *(.text):
     * at -O2 GCC puts main in .text.startup, and matching that first would
     * place main ahead of the real entry point. */
    *(.text)
    *(.text.*)
    *(.init)
    *(.fini)
  }

  .rodata : ALIGN(4)
  {
    *(.rodata)
    *(.rodata.*)
    *(.rdata)
    *(.rdata.*)
  }

  . = ALIGN(4);
  _fdata = .;

  .data : ALIGN(4)
  {
    *(.data)
    *(.data.*)
  }

  /* $gp is only meaningful when building with a non-zero -G, which puts
   * small objects in .sdata and addresses them gp-relative. The library is
   * built with -G0 so .sdata stays empty, but startup.s still loads $gp from
   * _sys_init_gp and the symbol has to resolve. */
  . = ALIGN(16);
  _gp = . + 0x7ff0;
  PROVIDE(__gp = _gp);

  .sdata : ALIGN(4)
  {
    *(.sdata)
    *(.sdata.*)
  }

  . = ALIGN(4);

  /* startup.s zeroes memory from _fbss up to _end. Both must be word
   * aligned, and the loop is a do-while, so it always writes at least the
   * word at _fbss. */
  _fbss = .;
  PROVIDE(__bss_start = _fbss);

  .sbss : ALIGN(4)
  {
    *(.sbss)
    *(.sbss.*)
    *(.scommon)
  }

  .bss : ALIGN(4)
  {
    *(.bss)
    *(.bss.*)
    *(COMMON)
  }

  . = ALIGN(4);
  _end = .;
  PROVIDE(end = _end);
  PROVIDE(__bss_end = _end);

  /* TinyGo's generic bare-metal runtime locates the heap and the range of
   * global variables through these names, rather than being told at compile
   * time. Nothing here affects a C build: these are PROVIDEd, so they cost
   * nothing if unreferenced.
   *
   * Two allocators share this machine and they must not share memory.
   * startup.s hands libps's InitHeap everything from _end up to the stack,
   * less the stack's reserve, so a Go heap starting at _end too would sit
   * exactly on top of libps's bookkeeping and each would corrupt the other
   * on its first allocation. libps allocates upward from _end, so the low
   * part is reserved for it and Go starts above.
   *
   * The reserve is small because nothing in the library allocates from it.
   * Run under an emulator, every example in psxsdk left exactly one word
   * written above _end, the header InitHeap puts there, after hundreds of
   * frames of drawing, reading data off the disc and playing sound. The
   * library's malloc is there for C programs; in a Go program even C code
   * calling malloc gets TinyGo's, since symusr.S defines the library's
   * weak.
   * The reserve used to be 256K, which for a program whose globals filled
   * most of the space below the stack put _heap_start past _heap_end, so
   * that its first allocation was fatal. runtime_yaroze.go hands InitHeap
   * the reserve again at startup, so that libps's malloc stops at it rather
   * than growing into Go's heap should anything call it. */
  __libps_heap_size = DEFINED(__libps_heap_size) ? __libps_heap_size : 16K;
  PROVIDE(_heap_start = _end + __libps_heap_size);
  PROVIDE(_heap_end = __stack_top - __stack_size);
  PROVIDE(_globals_start = _fdata);
  PROVIDE(_globals_end = _end);
  PROVIDE(_stack_top = __stack_top);

  /* A program with no heap at all fails on its first allocation, and does
   * so silently on a console, since the runtime's message goes to the
   * serial port. Say so at link time instead. */
  ASSERT(_heap_start <= _heap_end,
         "the program leaves no room for a heap between its end and the stack: it is too large for the console")

  /* Entry points the resident library has that symusr.o does not list.
   * Sony exported CdRead but not the CdControl that positions the drive
   * for it, so a program could read a file only by name, through
   * CdReadFile, which searches the directory for it each time. The
   * addresses come from the library itself: CdReadFile calls CdControl
   * with CdlSetloc and the file's position before it calls CdRead, and
   * afterwards moves that position on through CdPosToInt and CdIntToPos.
   * All read as libcd's do: the two control routines with their retries,
   * told apart by the blocking flag only CdControlB passes to the routine
   * that talks to the drive, and the conversions as the arithmetic between
   * a sector number and binary coded minutes, seconds and sectors. Script
   * assignments are fine here, unlike for the entry points symusr.S
   * defines, because no archive member shares these names, so there is
   * nothing for the linker to pull in before it sees them. psxsdk's yaroze
   * package declares them. */
  PROVIDE(CdControl = 0x800394E0);
  PROVIDE(CdControlB = 0x80039618);
  PROVIDE(CdIntToPos = 0x80039914);
  PROVIDE(CdPosToInt = 0x80039A18);

  /* Sections a modern toolchain emits that have no meaning here, and which
   * would otherwise be copied into the flat image handed to the console. */
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
