---
title: Packages
weight: 1
---

The module is `go.chrisrx.dev/psx`. This page is a map of its packages. Each one's API is on [pkg.go.dev](https://pkg.go.dev/go.chrisrx.dev/psx).

## In a program

These are what a PlayStation program imports. Each builds on the host too, with the library's functions as stubs, so a program type-checks there and its pure Go parts can be tested.

| Package | Purpose |
| --- | --- |
| [`yaroze`](https://pkg.go.dev/go.chrisrx.dev/psx/yaroze) | The Net Yaroze library, `libps`, with Sony's names |
| [`disc`](https://pkg.go.dev/go.chrisrx.dev/psx/disc) | Declaring the files a program reads off its disc, which the build puts there |
| [`archive`](https://pkg.go.dev/go.chrisrx.dev/psx/archive) | Reading archives: many files in one, unpacked piece by piece |
| [`scene`](https://pkg.go.dev/go.chrisrx.dev/psx/scene) | Drawing a scene made in the playground's scene editor |

## On the host

| Package | Purpose |
| --- | --- |
| [`link`](https://pkg.go.dev/go.chrisrx.dev/psx/link) | A linked program made a PSX-EXE, with `libps` combined in |
| [`emu`](https://pkg.go.dev/go.chrisrx.dev/psx/emu), [`emu/cpu`](https://pkg.go.dev/go.chrisrx.dev/psx/emu/cpu) | The emulator, and its R3000A and GTE |
| [`emu/emutest`](https://pkg.go.dev/go.chrisrx.dev/psx/emu/emutest) | Running programs in the emulator from Go tests |
| [`emu/openbios`](https://pkg.go.dev/go.chrisrx.dev/psx/emu/openbios) | The open source BIOS everything here boots |
| [`format/psx`](https://pkg.go.dev/go.chrisrx.dev/psx/format/psx) | PSX-EXE executables |
| [`format/ecoff`](https://pkg.go.dev/go.chrisrx.dev/psx/format/ecoff) | ECOFF objects, archives, symbols and relocations |
| [`format/cdrom`](https://pkg.go.dev/go.chrisrx.dev/psx/format/cdrom) | CD-ROM sectors, with EDC and ECC |
| [`format/iso9660`](https://pkg.go.dev/go.chrisrx.dev/psx/format/iso9660) | ISO 9660 with CD-XA extensions, reading and writing |
| [`format/psxdisc`](https://pkg.go.dev/go.chrisrx.dev/psx/format/psxdisc) | Whole PlayStation discs: tracks, cue sheets, audio |
| [`format/mdec`](https://pkg.go.dev/go.chrisrx.dev/psx/format/mdec) | MDEC bitstreams and movies |
| [`format/tim`](https://pkg.go.dev/go.chrisrx.dev/psx/format/tim) | TIM images, and pictures into them |
| [`format/tmd`](https://pkg.go.dev/go.chrisrx.dev/psx/format/tmd) | TMD models and their primitives |
| [`format/rsd`](https://pkg.go.dev/go.chrisrx.dev/psx/format/rsd) | Sony's RSD models, linked into TMDs |
| [`format/vag`](https://pkg.go.dev/go.chrisrx.dev/psx/format/vag) | SPU-ADPCM and VAG sounds |
| [`format/vab`](https://pkg.go.dev/go.chrisrx.dev/psx/format/vab) | VAB sound banks and mkvab's definition files |
| [`format/seq`](https://pkg.go.dev/go.chrisrx.dev/psx/format/seq) | SEQ scores from MIDI files |
| [`format/scene`](https://pkg.go.dev/go.chrisrx.dev/psx/format/scene) | Scene files, as the editor writes them and as a program reads them |
| [`format/archive`](https://pkg.go.dev/go.chrisrx.dev/psx/format/archive) | Writing the archives package `archive` reads |
| [`format/psxtrace`](https://pkg.go.dev/go.chrisrx.dev/psx/format/psxtrace) | Instruction traces shared with psxe |
