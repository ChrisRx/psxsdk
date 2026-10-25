---
title: smf2seq
weight: 18
---

`smf2seq` converts Standard MIDI Files into SEQ files, the scores the sound library plays with `SsSeqOpen` and `SsSeqPlay` on a VAB's programs, as Sony's smf2seq did.

[`yaroze`](yaroze.md) has it as `yaroze seq`, with the same flags.

```bash
$ smf2seq -V test.mid
created "test.SEQ": 75 events, 120 ticks a quarter note, 120.0 beats a minute
  dropped meta event 4 (0x04): 1
  dropped meta event 33 (0x21): 4
  dropped meta event 84 (0x54): 1
  dropped meta event 127 (0x7f): 1
```

A SEQ is the MIDI file's tracks merged into one stream of events with running status, behind a short header of the resolution, the tempo and the time signature, as [`format/seq`](format/seq) describes. It keeps notes, program changes, pitch bend and pressure, tempo changes, and the control changes the library acts on: bank select, data entry, volume, pan, expression and the NRPNs the library uses for loops and reverb. The rest is dropped, and `-V` lists it; `-B` drops bank select too.

The programs a sequence names are the VAB's, so a General MIDI file plays as it was written on the General MIDI bank on the Net Yaroze boot disc, `STD0.VH` and `STD0.VB`, which the [square](../examples/square.md) example plays its music on, or on a bank of the instruments it uses made with [`mkvab`](mkvab.md).

Sony released two versions of smf2seq, which ordered events that fall at the same time differently. The output here is SCEA's, byte for byte, and the tests check that every MIDI file in the Net Yaroze websites' archive converts to a sequence that plays the same as the one the member made.
