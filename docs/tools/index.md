---
title: Tools
weight: 7
group: Docs
section: Tools
description: The commands: yaroze to build a program onto a disc, the converters that make its pictures, sounds, music and models, and the tools for discs, executables and the emulator.
order: yaroze
---

| Tool | Purpose |
| --- | --- |
| [yaroze](yaroze.md) | Build a Go program into a PSX-EXE or a disc image in one step, with TinyGo compiled to WebAssembly, and play a disc in a window |

## Also in yaroze

Each of these is a tool of its own, which `go:generate` lines and the playground run by its name, and a [`yaroze`](yaroze.md) subcommand with the same flags.

| Tool | In yaroze | Purpose |
| --- | --- | --- |
| [discgen](discgen.md) | `yaroze generate` | Write the declarations that put a program's data files on its disc |
| [mkdisc](mkdisc.md) | `yaroze iso` | Build a CD-XA disc image from a directory |
| [png2tim](png2tim.md) | `yaroze tim` | Convert pictures into TIM images for sprites and textures, or with `-r` back into PNGs |
| [obj2tmd](obj2tmd.md) | `yaroze tmd` | Convert a Wavefront OBJ model into a TMD |
| [wav2vag](wav2vag.md) | `yaroze vag` | Encode WAV or AIFF sounds into the SPU's ADPCM, as VAG files |
| [mkvab](mkvab.md) | `yaroze vab` | Build a VAB sound bank from a definition file and VAGs, read one back, or split one into VH and VB |
| [smf2seq](smf2seq.md) | `yaroze seq` | Convert MIDI files into SEQ scores for the sound library |
| [mkscene](mkscene.md) | `yaroze scene` | Cook a scene, as the playground's scene editor writes it, into the file package scene draws |
| [mkarc](mkarc.md) | `yaroze arc` | Pack files into one archive for the disc, with deflate, which package archive unpacks piece by piece |
| [sioload](sioload.md) | `yaroze sio` | Upload an executable to a Net Yaroze over the serial cable |

## Standalone

| Tool | Purpose |
| --- | --- |
| [eco2exe](eco2exe.md) | Combine a Net Yaroze program with `libps` into a bootable PSX-EXE |
| [elf2exe](elf2exe.md) | Write a standalone ELF or ECOFF program as a PSX-EXE |
| [mips1fix](mips1fix.md) | Replace the MIPS II traps LLVM emits with nops so the code runs on an R3000A |
| [dumpsxiso](dumpsxiso.md) | List or extract a disc image |
| [mdecenc](mdecenc.md) | Encode pictures into an MDEC movie |
| [rsdlink](rsdlink.md) | Link Sony's RSD models into a TMD, byte for byte as Sony's rsdlink did |
| [hdr2go](hdr2go.md) | Generate layout-checked Go bindings from a C header |
| [psxemu](psxemu.md) | Run the emulator headless, with traces, VRAM dumps and screenshots, or drive a program with scripted presses, watched and poked memory, and a profile |
| [psxtrace](psxtrace.md) | Print and diff instruction traces, compare VRAM dumps |
| [ecoffdump](ecoffdump.md) | Display ECOFF object files, with disassembly |
| [ecoff2asm](ecoff2asm.md) | Convert ECOFF objects to GNU as source, for relinking the Net Yaroze library as ELF |
| [ecoffcmp](ecoff2asm.md) | Check ecoff2asm output against the original objects |

[dumpsxiso](dumpsxiso.md) is a translation of the dumpsxiso in Lameguy64's mkpsxiso, and is under the GPL-2.0 as that project is.
