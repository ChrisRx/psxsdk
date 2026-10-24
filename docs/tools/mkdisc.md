---
title: mkdisc
weight: 3
---

`mkdisc` builds a PlayStation disc image from a directory. A disc is a `.bin` of raw 2352-byte CD-ROM XA Mode 2 sectors plus a `.cue` naming its tracks, not a plain `.iso`, because the format also has to carry CD audio and streamed XA that will not fit in 2048-byte blocks.

The common case is an executable plus the data it loads, so the directory *is* the interface and no layout file is involved:

```bash
$ mkdisc -o game.bin --boot MAIN.EXE --volume-id MYGAME ./disc
wrote game.bin: 789 blocks, 150 of them run-out; game.cue beside it
```

The tree is placed on the disc as it appears on disk. Names are folded to the restricted uppercase set ISO 9660 allows and given the `;1` version suffix that PlayStation programs use when opening files, so `assets/sound/sample1.seq` becomes `\ASSETS\SOUND\SAMPLE1.SEQ;1`. `--boot` writes a `SYSTEM.CNF` naming the executable, unless the tree already has one. The stack it names, `--stack`, is the BIOS's rather than the program's: the BIOS runs on it while it starts the program and returns through a frame just below it, so it goes 4KB below the program's own stack at the top of RAM, where the program cannot overwrite that frame before `main` returns; at the same address Sony's BIOS faults when the program ends. `-v` lists what was placed where:

```
  \MAIN.EXE;1                                1251328 bytes  LBA 23
  \MUSIC.XA;1                                   9344 bytes  LBA 72  raw sectors
  \SYSTEM.CNF;1                                   65 bytes  LBA 634
  \ASSETS                                        <dir>        LBA 21
  \ASSETS\SOUND\SAMPLE1.SEQ;1                     5000 bytes  LBA 636
```

The image ends with two seconds of empty sectors. A drive reads ahead of the sector it was asked for, and the console reads a file by asking for whole sectors, so a file that ends at the end of the track is one the console cannot finish reading: the request runs into the lead-out and fails, which looks like a file that is present and unreadable. Pressed discs carry a run-out for the same reason. `--padding` sets how many sectors, and `0` leaves none, which is fine for an image only ever read by an emulator:

```bash
$ mkdisc --padding 0 -o game.bin --boot MAIN.EXE ./disc
wrote game.bin: 639 blocks; game.cue beside it
$ mednafen game.cue
[CDC] In leadout area: 639
```

`--file DISC=HOST` places one more host file at a path on the disc, making the directories on the way, and may be repeated. With it an image needs no directory at all.

## Registered files

A program can declare the files it reads itself, with the [`disc`](disc) package, instead of a directory being kept in step with it by hand:

```go
import "go.chrisrx.dev/psx/disc"

var stone = disc.Register("ASSETS/STONE.TIM", "assets/STONE.TIM")
```

`Register` is called with two constants in a package-level declaration: the path the file is to have on the disc, and the file on the host that goes there. TinyGo evaluates the call when it compiles the program, so each registration becomes a record of constant data in the executable. When `--boot` names an executable in the tree, `mkdisc` finds the records in it, places each file from the directory `--src` names, and once the image is laid out writes the file's first sector and its length back into the record before the executable goes onto the disc. `File.Read` then reads the file from that sector, through `CdControl` and `CdRead`, with no search of the directory, and a file that is missing or misnamed on the host fails the build rather than the program.

```bash
$ mkdisc -v -o game.bin --boot MAIN.EXE --file MAIN.EXE=prog.exe --src ./examples/dungeon
  \MAIN.EXE;1                                1251328 bytes  LBA 23
  \SYSTEM.CNF;1                                   65 bytes  LBA 634
  \ASSETS                                        <dir>        LBA 21
  \ASSETS\STONE.TIM;1                            33296 bytes  LBA 635  registered from assets/STONE.TIM
wrote game.bin: 787 blocks, 150 of them run-out; game.cue beside it
```

A registered path that the directory already holds is an error rather than a choice between the two, as is one path registered from two different host files. An executable not built onto a disc this way, or run from the serial loader, sees a zero sector and length, which `Placed` reports, and `Read` falls back to `CdReadFile` by name.

Sony exported `CdRead` but not the `CdControl` that positions the drive for it, so the TinyGo target's link script provides `CdControl`, `CdControlB`, `CdIntToPos` and `CdPosToInt` at the addresses the library's own `CdReadFile` calls, and the `yaroze` package declares them.

The volume descriptor fields are flags: `--volume-id`, `--system-id`, `--publisher`, `--preparer` and `--application`. `--date` sets the volume date, and leaving it unset records no date at all, so the same tree always produces the same bytes. `--file-times` records each file's own modification time instead, which is honest about the tree but makes the image depend on the mtimes in it.

## Streamed media

Files named `.xa` or `.str` are streamed media: XA audio and video are produced as whole 2336-byte Mode 2 sectors with their subheaders already in place, saying which form each sector is and which interleaved channel it belongs to. Those are written through untouched rather than packed into logical blocks, and their directory records carry the CD-XA attributes that mark them Form 2. `--raw` adds patterns to the set and `--no-raw` empties it. A file taken for a stream that is not a whole number of sectors is an error rather than a file padded into nonsense.

Every image is a CD-XA disc: the `CD-XA001` signature in the volume descriptor and the 14-byte attribute block on every directory record, which is what a PlayStation disc carries and what says a file holds a stream. `--no-xa` leaves both out, for comparing against a plain ISO 9660 writer.

## Audio tracks

`--audio` appends a CD audio track, which a program plays by its number rather than opening as a file. Input is a WAV file or headerless samples, in both cases 44.1kHz 16-bit stereo, which is the only thing a CD holds. A two second pause goes before every audio track, as on most CDs: the standard requires one between the data track and the audio after it, and the library's `CdPlay` needs one before each of the rest, since it starts a track at the second `GetTD` gives for it, up to a second early, and without a gap that second is the end of the track before. The `.cue` describes the result:

```bash
$ mkdisc -o game.bin --boot MAIN.EXE --audio track1.wav --audio track2.wav ./disc
wrote game.bin: 789 blocks, 150 of them run-out, and 2 audio track(s), 15639 sectors in all; game.cue beside it
$ cat game.cue
FILE "game.bin" BINARY
  TRACK 01 MODE2/2352
    INDEX 01 00:00:00
  TRACK 02 AUDIO
    INDEX 00 00:10:39
    INDEX 01 00:12:39
  TRACK 03 AUDIO
    INDEX 00 01:44:00
    INDEX 01 01:46:00
```

A program can declare its own tracks instead, with [`disc.Track`](disc), which `yaroze disc` and `mkdisc --boot` record after any `--audio` tracks and whose numbers they write into the program, as they do the sectors of its files. The [cdda example](../examples/cdda.md) plays three.

## Plain ISO 9660

`--iso` writes plain 2048-byte blocks instead. That cannot represent audio tracks or Form 2 sectors, and says so rather than writing an image missing either, but ordinary ISO 9660 tools can read it, which is how the filesystem is checked against an independent implementation:

```bash
$ mkdisc --iso -o test.iso ./disc
$ xorriso -indev test.iso -find /
```

Every image carries the 64-byte string that identifies it as a PlayStation disc, written into the system area at sector 4. This is format identification: without it mednafen cannot tell the image from any other CD-XA disc and rejects it outright with *"Could not find a system that supports this CD"*. It is not a protection bypass, and could not be: an unmodified console decides whether to boot from the wobble encoded in a pressed disc's lead-in groove, a physical property no file can reproduce.

Booting on real hardware additionally wants Sony's license data, which holds the logo shown at startup. That cannot be redistributed, so supply your own copy with `--license`; it replaces the whole system area.

[`yaroze`](yaroze.md) builds a program onto a disc with it in one step, `yaroze disc`, and `yaroze iso` is this command, with every flag.

## Origins

The repository began with a translation into Go of Lameguy64's [mkpsxiso](https://github.com/Lameguy64/mkpsxiso), which is under the GPL-2.0 and so is the translation. It is kept as it was in `tools/mkpsxiso`, under that license and with its own notice, and still runs as `go tool mkpsxiso`. `mkdisc` was written in its place from the ISO 9660 and CD-ROM XA standards and the `format` packages, takes the same flags, and is under the MIT license like the rest; the two lay a disc out the same way, and differ only in how `SYSTEM.CNF` is written.
