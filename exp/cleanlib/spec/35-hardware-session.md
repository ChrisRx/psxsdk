# The real hardware session

The plan for running the clean library on a real Net Yaroze, and everything needed to pick it up cold. Nothing here has run on a console yet. The owner of the kit says when the session happens: it ties up their office and their laptop, so don't plan around a date.

Update this file as things get built or learned. Results go in the last section.

## Why it matters

**All cycle matching so far is against the emulator**, which charges only for instruction fetch, with no cache, RAM wait states or write buffer. A routine matching Sony's exactly there may not match on a console. The session's main question is how far off that model is, and whether the clean library and Sony's are close on real hardware. The answer may change how much more cycle tuning is worth doing, so hold further tuning until then.

In order of importance, it should tell us:

1. **Timing truth:** the same routines timed with the console's own counters, under both libraries.
2. **Whether the clean library works on a console:** graphics, sound, pads, CD and memory card, on a few programs including Terra.
3. **Known risks in the clean library:**
   - `lwc2 $1` right before NCCT or NCDT in `gso_lg4`, `lgt4`, `fg4` and `fgt4` in `playstation-yaroze-clean.S`. The emulator doesn't model that hazard. The order came from timing traces, so check before assuming it's safe.
   - The Kanji flush's DMA wait.
   - Anything relying on the emulator's DMA timing.
4. **Whether the serial tools work:** they have only ever run against the emulated port.

Don't run the whole list of 38 programs that aren't `same`. They're on that list for differences hardware can't check without capture, and each hang means a reset and another boot from the original disc. Run these instead:

- The 9 `noboot` programs, on Sony's library only, to learn whether the emulator or the archive copy is at fault: `demos-scea-ricochet`, `demos-scee-tunelfin`, `event-event001-tantank`, `event-event003-free-hijump-hijump`, `pssoft-tantank2-2`, `scee-demos-all3d2-follow`, `showcase-gasgar`, `showcase-t-tank2f`, `test-kbo3d06-3d`.
- A few programs that print their own timings: the TIME set in `image/regress.sh` (`check`, `dungeon`, `flame`, `morph`, `offscr`, `timers`).

## The kit

- Black Net Yaroze console, boot disc and access card. The owner handles the kit as little as possible: never plan on dumping the disc or the card. There is no USB CD drive to hand.
- A regular memory card. The boot disc reads the line speed from an ordinary save file, `BWDTLS-03030CIP`, in either slot, and defaults to 9600 without it. The boot screen prints `TERMINAL SPEED %d BPS`. The owner believes the kit was last packed with 115200 saved. The monitor's `baud` command changes it and offers to save, which has worked for them.
- The kit's serial cable, a USB serial adapter (in a closet, to be found, a major brand, quality unknown) and a TV.
- With no capture card, comparisons are what comes over the cable and what's on the TV. Programs can checksum the screen and print the result, so both libraries can be compared without one.

## Serial history

This is why the tools look as they do.

- The owner's working setup, around 2019 or 2020, was their own `sioload` on `go.bug.st/serial` v1.0.0 at 115200, and also the real `SIOCONS.EXE` in DOSBox. Both took about the same time. The Linux `siocons` port in the web archive could never set 115200, which is why `sioload` exists. Don't trust that port.
- `sioload`'s delays have **no known root cause**. They came from a lot of manual testing, and they worked. Its comments say a missing 100 ms after the `bwr` header "will fail with WriteByte timeout", and a missing 1 ms after each block's 02h "will fail and hang". It also slept 100 ms after every read, 500 ms before a load and 2 s after `go`.
- The owner once tried handling RTS, and the program stalled as soon as it was used, so they took it out. They don't know if the code or the adapter was at fault.
- `go.bug.st/serial` went to v1.8.0 in `751f0fd`. That commit gives the macOS cgo reason, which the owner also wanted, but their main reason was that its timing and timeout changes might fix what the delays handled. **v1.8.0 has never run on the console.** That is an experiment still to run. v1.0.0's `Read` blocks forever, so the old "hang" and the RTS stall may each have been a lost byte waiting with no timeout. That's a theory, nothing has tested it.
- `c74af62` wrote the `serial` package and dropped the delays, because the emulator passes without them, and began pulsing RTS on every read. `959813c` fixed both: `DevicePacing` applies on real devices only, and RTS pulses only inside `ReadMemory`. Neither can be tested without a console.
- Reading memory back (`brd`) is the only thing that needs RTS pulses, and it has never run on this adapter. Probes print results as text so that the session doesn't depend on it.

## Built and not built

Built:

- `tools/sioloop`, the adapter check, tested only against fakes.
- `yaroze sio` (SIOCONS in Go, `-p DEVICE,BAUD`, or `DTLH3000` in the environment) and `tools/sioload`, on `serial`. The console side is rehearsed by `go test ./tests -run 'TestYaroze|TestMonitor|TestDebugger|TestSiocons'`, which needs `PSX_SONY_BIOS` (`$HOME/.mednafen/firmware/scph1001.bin`) and `PSX_YAROZE_BOOT` (`$HOME/Downloads/nyboot/Net Yaroze Boot Disc (USA, Europe)_redump.zip`). It takes about 6 minutes and passes on both monitors.
- The fallback, `.build/sioload-v1.0.0`: the last `sioload` on v1.0.0, with all the old delays. It takes ECOFF files only. To rebuild it, extract `go.mod go.sum tools/sioload format/ecoff` from `751f0fd~1` with `git archive` into an empty directory, and run `GOFLAGS=-mod=mod go build ./tools/sioload` there.

Not built, and needed first:

- **Loading the clean image.** The boot disc leaves Sony's library at 80010000h and its monitor at 80068000h. The clean image ends at 80066288h, so it fits below the monitor, but it isn't known whether the monitor needs Sony's library while it's overwritten, nor what its interrupts do. Rehearse with the real boot disc image in the emulator. If overwriting in place fails, upload the image to high RAM with a small loader that copies it once the monitor hands over.
- **Static programs** need relinking at 80090000h (`--defsym=__load=0x80090000`), because the clean target's default of 80010000h overwrites the monitor as it loads.
- **Timing probes** that print over the cable: the same routines under both libraries, by the console's root counters.
- **Screen checksum** printed as text, so both libraries can be compared without capture.
- **A runner:** loads each program, runs it, keeps what it prints, and pauses for a reset when the monitor goes silent. A crash leaves the console parked in the BIOS. It should resume where it stopped, since a session may be split.

Test all of it against the emulated port before the session.

## Before the session

Without the console, whenever the adapter turns up:

1. Join pins 2 and 3 of the adapter's DB9, and 7 and 8, with jumper wires or solid wire, keeping the two apart and leaving pin 5 free. Do this before plugging it in.
2. Run `go run ./tools/sioloop -d /dev/ttyUSB0`.

It reports bytes returned, whether CTS follows RTS, CTS lag, the time to read CTS and pulse RTS, round trip, and an upload estimate. If CTS doesn't follow RTS, the adapter is at fault. CTS lag shows how old a CTS reading can be: at 115200 a byte takes 87 microseconds, so lag of 1 ms or more means CTS can't pace single bytes as `Port.Send` assumes.

## The session

Each step can be a stopping point.

1. **Cable check:** boot, reach the `>>` prompt, write and read a little memory. Measure the upload rate. If it fails, change one thing at a time:

   | Try | Tool | If it works | If it fails |
   |---|---|---|---|
   | 1 | `yaroze sio`, on `serial` and v1.8.0 | Carry on | Try 2 |
   | 2 | `.build/sioload-v1.0.0`, with an original ECOFF sample | The fault is in `serial` or v1.8.0 | Try 3 |
   | 3 | `SIOCONS.EXE` in DOSBox | See what the old `sioload` lacks. Today's OS and driver are also new since 2020 | Suspect the adapter, cable or console |

   To split `serial` from v1.8.0 once 2 works, build the same old code on v1.8.0. Its `Read` no longer returns the same way, so that takes a small change.
2. **Timing on Sony's library,** which the boot disc has already loaded.
3. **The clean library:** load the image, repeat the same probes, then the program checks.
4. **The 9 `noboot` programs,** on Sony's library.
5. **Experiments, if there's time:** take out `DevicePacing`'s delays one at a time, which answers the v1.8.0 question and may speed uploads. Try `Drain()` in place of the sleeps. Try `brd` once.

## Where things stood

As of `994c8e6`, on branch `libps-go`, with spec 34 done. `.build/carchive/summary-verify44.txt` is the baseline: 102 `same`, 22 `differ`, 9 `noboot`, 4 `shift`, 1 `wild`, 1 `tty`, 1 `stall`. Regressions are 34, 0, 7 on the image and 33, 0, 8 linked.

## Results

None yet.
