---
title: mkvab
weight: 17
---

`mkvab` builds a VAB, a bank of sounds the SPU plays, from a definition file and VAG files made with [`wav2vag`](wav2vag.md), as Sony's mkvab did, and `vabsplit` splits a bank into the two halves a program loads: the VH, the bank's header, which stays in main memory, and the VB, the samples, which go to the SPU's memory. `--split` writes the two directly.

```bash
$ mkvab -f waves.def -o WAVES.VAB --split sine.VAG square.VAG triangle.VAG
created "WAVES.VH" and "WAVES.VB": 3 programs, 3 VAGs, 42048 bytes
$ mkvab -r WAVES.VH
VabHdr
	form = 'VABp'
	ver = 0x20
...
```

A bank is up to 128 programs, instruments, each of up to 16 tones, the samples it plays across ranges of notes. A sequence made with [`smf2seq`](smf2seq.md) names programs, and a sound effect is usually a program of one tone played with `SsUtKeyOn`. The definition file is Sony's, as the User's Guide gives it and [`format/vab`](format/vab) describes:

```
VabHdr
        ver = 0x7

ProgAtr 0
        tones = 1
        mvol = 127
        mpan = 64

VagAtr 0 0
        vol = 127
        pan = 64
        center = 60
        min = 0
        max = 127
        ar = 127
        dr = 0
        sr = 0
        rr = 20
        sl = 15
        prog = 0
        vag = 1
```

`center` is the note the sample sounds at its own rate, and `min` and `max` the notes the tone plays. The envelope is in Sony's rates, a bigger number faster: attack `ar` 0-127, decay `dr` 0-15, sustain level `sl` 0-15, sustain rate `sr` from -127 (falling fastest) through 0 (holding) to 127 (rising), and release `rr` 0-31. `vag` numbers the VAG files in the order they are given, from 1. The counts and sizes in the header are worked out, as mkvab worked them out.

`-r` reads a bank, a `.VAB` or a `.VH` with its `.VB` beside it, and prints its definition file, which `mkvab` reads back to the same bank. The tests build the banks in the Net Yaroze websites' archive whose definition, VAGs and command line survive, and check them against what Sony's mkvab made, the body byte for byte.
