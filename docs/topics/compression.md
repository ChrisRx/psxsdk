---
title: Compression
weight: 8
description: Why the archives pack with deflate and nothing else: what the games of the time did and why, how the examples' data compresses under each scheme, and how fast each decoder runs on the console.
---

The PlayStation's games packed their data, and most packed it with their own schemes, often several in one game. This page is the reasoning and the measurements behind the toolchain's choice of one: the archives [`mkarc`](../tools/mkarc.md) writes and package `arc` reads pack with deflate, the format of zip and gzip, and store what deflate cannot shrink. The benchmark that produced the numbers is `internal/compressbench`, run by `task bench:compress`, so that the question can be asked again when something changes.

## What the games did, and why

[Terra Incognita](../examples/terra.md), the Net Yaroze game ported here, is a clean specimen. Its three archives are tables of entries, each packed with Okumura's LZSS, a window of a few kilobytes and matches of up to eighteen bytes, flagged a bit at a time. The game kept the archives in memory and unpacked each piece into place as it needed it. That is the pattern nearly every game of the time used, for reasons that are more particular than "the CPU could not do better":

- **The drive was the bottleneck.** A double speed drive delivers about 300 KB a second, after a seek of a few hundred milliseconds. Packing mostly bought load time, and that only pays when unpacking keeps up with the drive. A bytewise LZ unpacks at a few megabytes a second on a 33 MHz R3000, so it was free. An entropy coder, with its tables and its bit by bit reads, was not: the console has no data cache, so every table lookup is a trip to memory.
- **Unpacking during play had to cost nothing.** Pieces streamed in while the game ran at thirty frames a second got the cheapest scheme there was, LZ or run lengths, while a loading screen could afford more. Two needs, two decoders.
- **Memory.** Two megabytes, so a decoder could not want tables or a window of its own. LZ into the destination, with the packed bytes parked at its end, was the trick.
- **Some data does not pack.** Sound is already ADPCM and movies are already MDEC; both were stored.
- **Habit.** Studios carried their LZ and RLE tools over from the 16-bit era, and zlib was not a thing on consoles.

## How the examples' data packs

Every file under `examples/*/data`, as a share of its size, under the LZSS of the time (a greedy estimate with Terra's parameters), the LZ4 block format (a greedy estimate; lz4hc finds a few percent more), and the real deflate, zstd and xz encoders:

| type | raw | LZSS 4K | LZ4 | deflate | zstd -19 | xz -9 |
| --- | --- | --- | --- | --- | --- | --- |
| TIM, textures | 1.4 MB | 28-96% | 2-91% | 13-77% | 12-76% | 11-74% |
| TMD, models | 286 KB | 52-73% | 58-71% | 30-53% | 40% | 33% |
| SEQ, music | 250 KB | 66% | 33% | 23% | 21% | 20% |
| DAT, scenes and game data | 295 KB | 29-47% | 22-45% | 15-32% | 29% | 28% |
| VH, bank headers | 437 KB | 17% | 7% | 4% | 3% | 3% |
| VB, samples | 3.4 MB | 102% | 97% | 80% | 79% | 72% |
| everything | 7.2 MB | | | 56% | 55% | 50% |

LZ4 is a modest step past the LZSS of the time: the wider window helps on some files, not on textures or models. Deflate is a real step, a third smaller than LZ4 on textures and models, because sixteen bit pixels and small vertex integers reward an entropy stage. Past deflate the returns vanish: zstd and xz buy a few more percent at a price the R3000 cannot pay, since zstd wants FSE tables and tens of kilobytes of workspace and LZMA's range coder would unpack well below the drive's speed.

## How fast each unpacks on the console

Two decoders written in Go and compiled by the toolchain at `-opt=2`, each checked byte for byte against the host, timed by the emulator's cycle count over three of the examples' files, decoding alone, the mean of two rounds after a checked one. The console runs at 33.87 MHz.

| file | raw | deflate size | deflate speed | LZ4 size | LZ4 speed |
| --- | --- | --- | --- | --- | --- |
| CASTLE.TIM, a texture | 307 KB | 13% | 960 KB/s, 34 cycles a byte | 27% | 1111 KB/s, 30 |
| DOG.DAT, game data | 292 KB | 33% | 524 KB/s, 63 | 46% | 1165 KB/s, 28 |
| MD0.TMD, a model | 115 KB | 54% | 347 KB/s, 95 | 72% | 909 KB/s, 36 |

Two things moved these numbers a great deal and are worth knowing for any decoder written for the console. Bounds checks on every byte of the copy loops cost almost half: taking them out, checking a run's bounds once, took LZ4 from 710 to 1110 KB/s and deflate from 250 to 930 on the texture. And the width of inflate's lookup table matters on data with long codes: nine to eleven bits took the model from 283 to 340 KB/s. The LZ4 decoder is forty lines; the inflate is about 250 with 8 KB of tables.

Against the drive, reading then unpacking with no overlap:

| file | raw read | deflate | LZ4 |
| --- | --- | --- | --- |
| CASTLE.TIM | 1.02 s | 0.13 + 0.32 = 0.45 s | 0.28 + 0.28 = 0.56 s |
| DOG.DAT | 0.97 s | 0.32 + 0.56 = 0.88 s | 0.45 + 0.25 = 0.70 s |
| MD0.TMD | 0.38 s | 0.21 + 0.33 = 0.54 s | 0.27 + 0.13 = 0.40 s |

On textures, and anything that packs well, deflate wins outright. On models it unpacks a little slower than the drive reads the raw file, and LZ4 only breaks even. Neither is a great load time win on its own at a Yaroze game's scale; the archive is, since one seek and one read replace one of each per file. Where packing clearly pays is memory and disc: an archive kept resident and unpacked piece by piece holds two to three times the assets in the same space.

## The choice

One packing, deflate, and stored for what it cannot shrink by a tenth. It is the best ratio the R3000 can afford, the encoder comes from Go's standard library, and at a third to a whole megabyte a second it is at or above the drive's speed, so nothing loads slower packed than raw. The archive carries a method per entry, so a second packing can be added without a change to readers. The case for one would be unpacking during play: at thirty cycles a byte LZ4 costs about five percent of a sixty frame a second frame per kilobyte, and deflate three times that. A game streaming pieces mid-play would want LZ4, and its decoder is cheap to add then.

These are the emulator's cycle costs; real memory timing may differ somewhat, not enough to reorder the conclusions.
