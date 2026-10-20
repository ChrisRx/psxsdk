---
title: Packages
weight: 7
---

| Package | Purpose |
| --- | --- |
| [`yaroze`](yaroze) | TinyGo bindings for the resident Net Yaroze library, with stubs for every other build |
| [`emu`](emu), [`emu/cpu`](emu/cpu) | The emulator, and its R3000A and GTE |
| [`emu/emutest`](emu/emutest) | Running programs in the emulator from Go tests |
| [`link`](link) | Making a PSX-EXE of a linked program: its flat image, the MIPS II traps replaced, and `libps` combined in |
| [`format/psx`](format/psx) | PSX-EXE executables |
| [`format/ecoff`](format/ecoff) | ECOFF objects, archives, symbols and relocations |
| [`format/cdrom`](format/cdrom) | CD-ROM sectors, with EDC and ECC |
| [`format/iso9660`](format/iso9660) | ISO 9660 with CD-XA extensions, reading and writing |
| [`format/psxdisc`](format/psxdisc) | Whole PlayStation discs: tracks, cue sheets, audio |
| [`format/mdec`](format/mdec) | MDEC bitstreams and movies |
| [`format/tim`](format/tim) | TIM images, and pictures into them |
| [`format/tmd`](format/tmd) | TMD models and their primitives |
| [`format/rsd`](format/rsd) | Sony's RSD models, linked into TMDs |
| [`format/vag`](format/vag) | SPU-ADPCM and VAG sounds |
| [`format/vab`](format/vab) | VAB sound banks and mkvab's definition files |
| [`format/seq`](format/seq) | SEQ scores from MIDI files |
| [`format/psxtrace`](format/psxtrace) | Instruction traces shared with psxe |
