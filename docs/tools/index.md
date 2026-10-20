---
title: Tools
weight: 5
group: Docs
section: Tools
description: The commands: nyc to build a program onto a disc, the converters that make its pictures, sounds, music and models, and the tools for discs, executables and the emulator.
order: nyc
---

| Tool | Purpose |
| --- | --- |
| [nyc](nyc.md) | Build a Go program into a PSX-EXE or a disc image in one step, with the tinygo command or TinyGo compiled to WebAssembly |
| [eco2exe](eco2exe.md) | Combine a Net Yaroze program with `libps` into a bootable PSX-EXE |
| [elf2exe](elf2exe.md) | Write a standalone ELF or ECOFF program as a PSX-EXE |
| [mips1fix](mips1fix.md) | Replace the MIPS II traps LLVM emits with nops so the code runs on an R3000A |
| [discgen](discgen.md) | Write the declarations that put a program's data files on its disc |
| [mkpsxiso](mkpsxiso.md) | Build a CD-XA disc image from a directory |
| [dumpsxiso](dumpsxiso.md) | List or extract a disc image |
| [mdecenc](mdecenc.md) | Encode pictures into an MDEC movie |
| [png2tim](png2tim.md) | Convert pictures into TIM images for sprites and textures, and `tim2png` back |
| [wav2vag](wav2vag.md) | Encode WAV or AIFF sounds into the SPU's ADPCM, as VAG files |
| [mkvab](mkvab.md) | Build a VAB sound bank from a definition file and VAGs, or read one back, and `vabsplit` it into VH and VB |
| [smf2seq](smf2seq.md) | Convert MIDI files into SEQ scores for the sound library |
| [rsdlink](rsdlink.md) | Link Sony's RSD models into a TMD, byte for byte as Sony's rsdlink did |
| [obj2tmd](obj2tmd.md) | Convert a Wavefront OBJ model into a TMD |
| [mkscene](mkscene.md) | Cook a scene, as the playground's scene editor writes it, into the file package scene draws |
| [mkarc](mkarc.md) | Pack files into one archive for the disc, with deflate, which package archive unpacks piece by piece |
| [hdr2go](hdr2go.md) | Generate layout-checked Go bindings from a C header |
| [psxemu](psxemu.md) | Run the emulator headless, with traces, VRAM dumps and screenshots, or drive a program with scripted presses, watched and poked memory, and a profile |
| [psxplay](psxplay.md) | Run the emulator in a window, booting the built-in OpenBIOS, with the keyboard or a gamepad as the pad |
| [psxtrace](psxtrace.md) | Print and diff instruction traces, compare VRAM dumps |
| [sioload](sioload.md) | Upload an executable to a Net Yaroze over the serial cable |
| [ecoffdump](ecoffdump.md) | Display ECOFF object files, with disassembly |
| [ecoff2asm](ecoff2asm.md) | Convert ECOFF objects to GNU as source, for relinking the Net Yaroze library as ELF |
| [ecoffcmp](ecoff2asm.md) | Check ecoff2asm output against the original objects |
| [gteprog](gteprog.md) | Write a GTE exercise program as a PSX-EXE, for comparing emulators |
