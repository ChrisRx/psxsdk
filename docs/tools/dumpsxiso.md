---
title: dumpsxiso
weight: 4
---

`dumpsxiso` reads a disc image back. With no `--output` it lists what the image holds, which is the quickest way to see whether a file landed under the name a program will ask for:

```bash
$ dumpsxiso game.bin
volume MYGAME, 789 blocks
  system      PLAYSTATION
  created     2026-10-14T00:00:00Z
  extension   CD-XA

  \MAIN.EXE;1                              1251328 bytes  LBA 23
  \MUSIC.XA;1                                 9344 bytes  LBA 72  raw sectors
  \SYSTEM.CNF;1                                 53 bytes  LBA 634
  \DATA                                      <dir>        LBA 21
  \DATA\SOUND\SAMPLE1.SEQ;1                   5000 bytes  LBA 636
```

With `--output` the tree is extracted to a directory, and what comes out can be handed straight back to `mkdisc`. A `.bin` of raw sectors and a `.iso` of 2048-byte blocks are both accepted; which one it is follows from the image itself. Streamed files come out as the 2336-byte sectors they went in as, recognised by their CD-XA attributes rather than their names.

Reading a raw image checks every sector's error detection, so a damaged image is reported rather than quietly extracted. A sector of a filesystem is the kind that is meant to be read correctly or not at all, and a file recovered from a failing one is a file the console itself would refuse.

## Origins

`dumpsxiso` is a translation into Go of the dumpsxiso in Lameguy64's [mkpsxiso](https://github.com/Lameguy64/mkpsxiso), which is under the GPL-2.0, and so is the translation: it is in `tools/dumpsxiso` under that license, with its own notice, and nothing else in the repository uses it. It reads images through the `format` packages, which are independent and under the MIT license.
