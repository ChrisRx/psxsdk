---
title: wav2vag
weight: 16
---

`wav2vag` encodes sounds into VAG files, the SPU's ADPCM, which a program plays as a sound effect or puts in a VAB as an instrument with [`mkvab`](mkvab.md). It does what Sony's aiff2vag did, and reads WAV and AIFF files of 8 or 16 bits, mixing stereo down to mono.

[`yaroze`](yaroze.md) has it as `yaroze vag`, with the same flags.

```bash
$ wav2vag MYU.AIF
created "MYU.VAG": 4518 samples at 22254 Hz, 2624 bytes
```

The SPU plays blocks of 16 bytes, each 28 samples of 4 bits with a shift and one of five prediction filters, as [`format/vag`](format/vag) describes. For each block `wav2vag` tries every filter and shift against the SPU's own arithmetic and keeps the one that decodes closest to the sound. Over the 116 sounds in the Net Yaroze websites' archive that sit beside the VAG a member made of them with Sony's encoder, it comes out 5 decibels cleaner on average, and never worse.

A loop the file marks is kept: a WAV's `smpl` chunk or an AIFF's sustain loop. The sound plays to the loop's end and then repeats from its start, a whole block at a time, so a loop sounds right when its ends are multiples of 28 samples. `-1` leaves the loop out and `-L` loops the whole sound. The blocks are flagged as aiff2vag flagged them, which the tests check against Sony's for every looped sound in the archive.

The rate in the header is the file's. A tone in a VAB plays the sample at that rate at its center note, and higher or lower notes faster or slower; `SsUtKeyOn` plays a sound effect at any note. A lower rate takes less of the SPU's 512 KB.

`-R` reads headerless 16-bit samples at a given rate, little-endian unless `-E`.
