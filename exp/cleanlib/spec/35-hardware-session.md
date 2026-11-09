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

Built, and rehearsed in the emulator on the boot disc's own monitor and Sony's BIOS:

- **`tools/hwrun`** (`internal/cmd/hwrun`), the runner. It uploads each program, runs it, keeps what it prints in `results/NAME-LIBRARY.txt`, skips any program with a file so that a session can stop and go on later, and asks for the reset button when the monitor won't come back. The emulated console can't be reset, so that path has run only against a fake console in `hwrun_test.go`. With `-archive .build/carchive/programs.tsv`, its arguments are program names, and each brings its data files as its `AUTO` batch does. `TestYarozeArchiveJob` rehearses that on both libraries, with `PSX_HW_ARCHIVE` and `PSX_HW_PROGRAM`.
- **`tests/testdata/serial/hwprobe`**, the probe, built by `task build:serial` onto `.build/serial/hwprobe.elf`. It prints `T` lines of five readings of the system clock (the first with a cold instruction cache), `S` lines at an eighth of the clock for long routines, `F frame`, and `H scene`, a hash of a drawn picture. Interrupts are off while it measures. Its readings include the `null` line's.
- **`serial.TakeOver`**, how the clean library gets onto the console, with `serial/monitor` built onto `.build/serial/monitor.elf`. See below.
- **`tools/sioloop`**, the adapter check, tested only against fakes.
- **`yaroze sio`** (SIOCONS in Go, `-p DEVICE,BAUD`, or `DTLH3000` in the environment) and `tools/sioload`, on `serial`.
- **The fallback, `.build/sioload-v1.0.0`:** the last `sioload` on v1.0.0, with all the old delays. It takes ECOFF files only. To rebuild it, extract `go.mod go.sum tools/sioload format/ecoff` from `751f0fd~1` with `git archive` into an empty directory, and run `GOFLAGS=-mod=mod go build ./tools/sioload` there.

To run the rehearsal, about 90 seconds:

```
PSX_HW_OUT=$PWD/.build/hardware \
PSX_SONY_BIOS=$HOME/.mednafen/firmware/scph1001.bin \
PSX_YAROZE_BOOT="$HOME/Downloads/nyboot/Net Yaroze Boot Disc (USA, Europe)_redump.zip" \
go test ./tests -run TestYarozeCleanImage -count=1 -v
```

The other serial tests use the same two variables: `go test ./tests -run 'TestYaroze|TestMonitor|TestDebugger|TestSiocons'`, about 6 minutes, passes on both monitors.

Not built:

- **Static programs.** They need relinking at 80090000h (`--defsym=__load=0x80090000`), because the clean target's default of 80010000h overwrites the monitor as it loads. Nothing has been tried.

### Loading the clean library, as rehearsed

`-library clean` replaces the boot disc's monitor and library with ours, once per boot, and programs then load and return with no reset, as they do on Sony's. The disc only has to boot.

What the rehearsal found on the way there:

1. **Uploading the image straight over Sony's fails.** The monitor goes silent once the upload reaches 80012000h, 8 KB in, as it still needs that library. `TestYarozeCleanImageInPlace` records this.
2. **Leaving Sony's monitor on the clean library works, but needs a reset per run,** because it can't restart when a program returns. That version is `serial.Relocated` in `eeb6a80`, the fallback if taking over fails on a console.
3. **Starting our monitor with the kernel as Sony's left it crashes.** Sony's monitor replaces the kernel's `tty` device with its own and adds `mem` and `cdx`, with code in 8006xxxx, and opens card events with handlers in 80079xxx. Those addresses land in our code. This was read from the kernel's public tables, not from Sony's code.

So `serial.TakeOver` uploads the image and our monitor to high RAM with a small loader, which:

1. Turns interrupts off and masks them.
2. Calls the kernel's `RemoveDevice` on `tty`, `mem` and `cdx`, while Sony's code is still there for the kernel to call, then `AddDummyTty`, and `CloseEvent` on events 1 to 15.
3. Copies the pieces into place and calls `FlushCache`.
4. Jumps to our monitor.

`serial.OpenMonitor` sets our monitor's speed in the upload, so it doesn't read the memory card: a failed read there would leave it at a different speed from the host. Its `ld` and other card and CD commands are as they were, and only its start-up skips the card.

**Things only a console can show:**

- Whether its BIOS matches. The rehearsal uses SCPH-1001, and the Yaroze's may differ. A difference in these calls, or a DMA still running during the copy, wouldn't show in the emulator. If taking over fails, use the fallback above, or stay on Sony's library for timing alone.
- Our monitor on real hardware at all, which has only run in the emulator. Its receive path has its own timing, and the delays in `DevicePacing` came from Sony's monitor.
- Stale pointers remain in the kernel's RAM that nothing has used so far, such as 8000E2C0h to 80046918h. If something odd happens late in a run, look there first.

### Getting back from a program that never returns

`go` on the clean monitor sets a handler on the kernel's exception vector. At each interrupt it looks for a Control-C on the serial port, and if it finds one it jumps to `bfc00000`, the BIOS's reset address. `hwrun` sends those bytes when a program has run its time, waits for the boot disc's monitor, and takes over again. It asks for the button only if that doesn't work, as with a console with its interrupts off. It needs the boot disc in the drive.

Rehearsed in the emulator with `TestYarozeArchiveJob`, on `demos-scee-tunelfin`. Not yet tried on a console, where a jump to the reset address isn't the same as the button: the BIOS has to set up whatever the program left running. Sony's library has no such handler, so comparing a game on Sony's still takes the button once per program.

### The emulator's readings

They're what to compare a console's with. In the emulator both are flat across the five readings. Sony's and clean's agree except:

| Reading | Sony's | Clean |
|---|---|---|
| GsSortLine, this long line | 514 | 548 |
| GsSortSprite, rotated | 1,364 | 1,356 |
| RotMatrix | 288 | 286 |
| `__muldf3`, 3.25 x 1.5 | 3,898 | 3,880 |
| Sin(1.25), eighths | 32,077 | 32,516 |
| Sqrt(2.5), eighths | 26,391 | 27,332 |

Those are gaps the emulator already shows, listed so they aren't mistaken for what the console adds. The picture's hash is 882646469 on both. `frame` is a wrapped count of eighths: add 65,536. The two sides read 5,240 and 5,233.

## Before the session

Without the console, whenever the adapter turns up:

1. Join pins 2 and 3 of the adapter's DB9, and 7 and 8, with jumper wires or solid wire, keeping the two apart and leaving pin 5 free. Do this before plugging it in.
2. Run `go run ./tools/sioloop -d /dev/ttyUSB0`.

It reports bytes returned, whether CTS follows RTS, CTS lag, the time to read CTS and pulse RTS, round trip, and an upload estimate. If CTS doesn't follow RTS, the adapter is at fault. CTS lag shows how old a CTS reading can be: at 115200 a byte takes 87 microseconds, so lag of 1 ms or more means CTS can't pace single bytes as `Port.Send` assumes.

## The session

Each step can be a stopping point.

1. **Cable check:** boot, reach the `>>` prompt, write a little memory and check it with `dw`. Measure the upload rate. If it fails, change one thing at a time:

   | Try | Tool | If it works | If it fails |
   |---|---|---|---|
   | 1 | `yaroze sio`, on `serial` and v1.8.0 | Carry on | Try 2 |
   | 2 | `.build/sioload-v1.0.0`, with an original ECOFF sample | The fault is in `serial` or v1.8.0 | Try 3 |
   | 3 | `SIOCONS.EXE` in DOSBox | See what the old `sioload` lacks. Today's OS and driver are also new since 2020 | Suspect the adapter, cable or console |

   To split `serial` from v1.8.0 once 2 works, build the same old code on v1.8.0. Its `Read` no longer returns the same way, so that takes a small change.
2. **Timing on Sony's library,** which the boot disc has already loaded: `go run ./tools/hwrun -d /dev/ttyUSB0 -library sony -o results .build/serial/hwprobe.elf`.
3. **The clean library:** `go run ./tools/hwrun -d /dev/ttyUSB0 -library clean -o results .build/serial/hwprobe.elf`, with no reset needed. Look at the TV too. Then a few programs, including Terra. It takes over once per boot, and a return to Sony's library needs a reset.
4. **Experiments, if there's time:** take out `DevicePacing`'s delays one at a time, which answers the v1.8.0 question and may speed uploads. Try `Drain()` in place of the sleeps. Try `brd` once.

## Where things stood

The library, as of `994c8e6`, on branch `libps-go`, with spec 34 done. `.build/carchive/summary-verify47.txt` is the baseline since `Embed`'s fix: 86 `same`, 27 `differ`, 15 `still`, 9 `shift`, 1 `wild`, 1 `tty`, 1 `stall`. Baselines before it counted 42 programs as `same` that never loaded on either side. Regressions are 34, 0, 7 on the image and 33, 0, 8 linked.

### The 9 `noboot` programs don't need the console

They were on this plan to learn whether the emulator or the archive copy was at fault. Neither is. The BIOS searches only the root directory's first 2,048-byte sector, and on their discs the root runs over it, with `SYSTEM.CNF` and the executable in a later sector. So it never finds `SYSTEM.CNF` and looks for `PSX.EXE`. Across the archive the split is exact: all 131 discs that boot have a root of one sector, and all 9 `noboot` discs have two or three. Mednafen with SCPH-1001 stops at the logo on the same disc, and runs the game when `SYSTEM.CNF` and the executable come first in the directory.

Through the monitor, in the emulator, all 9 run on both libraries. `showcase-gasgar` differs: on the clean library it comes back to the monitor's `>>` prompt within 20 s, and on Sony's it's still running.

## Results

First session, 2026-11-07, on the Net Yaroze kit, booted from the original disc with its saved 115200 baud. The adapter is a Prolific PL2303 (`usb-Prolific_Technology_Inc._USB-Serial_Controller_D`). The console was reached from `serial` on go.bug.st/serial v1.8.0, with `DevicePacing`. The clean library is the committed `link/libps.exe.gz` of `f60e194`. The loopback test wasn't run, and `/dev/ttyUSB0` needed access granted first, since this account isn't in `dialout`.

- **The cable and the tools work** on the first try. `dw` answers in about 115 ms. Writes of 256, 4,096 and 65,536 bytes read back right at both ends, the largest at 6.6 KB/s. No RTS was used.
- **`serial.TakeOver` works on the console, with no reset.** Afterward `dw 80068000 4` answers 240bffff, our monitor's first word, and the prompt is back.
- **The clean library runs on a real console,** and draws the same picture: the scene hash is 882646469 on both libraries, and on the emulator.
- **Three runs repeat closely.** Clean twice gives readings within a few counts of each other, and `divdf3`'s are identical.

Medians of the five readings, in the system clock with interrupts off. Sin and Sqrt are in eighths of it. Each includes the `null` reading.

| Reading | Emulator, Sony | Emulator, clean | Console, Sony | Console, clean |
|---|---|---|---|---|
| null | 20 | 20 | 36 | 36 |
| loadRAM | 2074 | 2074 | 2363 | 2363 |
| storeRAM | 2076 | 2076 | 1052 | 1052 |
| rand | 56 | 56 | 69 | 69 |
| GetClut | 46 | 46 | 42 | 42 |
| GetTPage | 134 | 134 | 125 | 139 |
| GsGetActiveBuff | 40 | 40 | 46 | 47 |
| GsSetWorkBase | 42 | 42 | 44 | 45 |
| GsInitCoordinate2 | 86 | 86 | 164 | 119 |
| TransMatrix | 56 | 56 | 77 | 67 |
| RotMatrix | 288 | 286 | 271 | 346 |
| MulMatrix0 | 170 | 170 | 230 | 191 |
| ApplyMatrix | 88 | 88 | 120 | 111 |
| RotTransPers | 75 | 75 | 81 | 81 |
| memmove256 | 2106 | 2108 | 1085 | 1086 |
| addsf3 | 186 | 186 | 162 | 148 |
| muldf3 | 3898 | 3880 | 4498 | 5424 |
| divdf3 | 19616 | 19616 | 24731 | 9827 |
| GsSortLine | 514 | 548 | 898 | 904 |
| GsSortBoxFill | 514 | 514 | 961 | 554 |
| GsSortSprite | 1364 | 1356 | 2201 | 1518 |
| Sin | 32077 | 32516 | 39580 | 31645 |
| Sqrt | 26391 | 27332 | 32765 | 33236 |

**Programs.** `pssoft-terra-2`, a real Net Yaroze game, uploaded in 1 minute 25 seconds and ran on the clean library. The user saw it on the TV. It printed `go` and `ResetGraph`, and no more in 45 s. Since it never returns, the old monitor couldn't get control back, and `hwrun` stopped to ask for the button. The other four programs planned then didn't run: `demos-scee-tunelfin`, `showcase-gasgar`, `scee-topten-adven`, `pssoft-airbob-2-airbob`.

**After the serial reset, at 0104238's library.** With the boot disc's monitor back after a press of reset, `hwrun -library clean` ran four programs, 45 s each, with nobody watching the TV. The serial reset worked three times, with no button: each time the console rebooted from the disc, and the clean library went back in, in 1 minute 8 seconds.

| Program | Upload | Printed | After |
|---|---|---|---|
| `demos-scee-tunelfin` | 1 min 25 s | the pad driver, `ResetGraph`, `level updating` 1 and 2 | reset from the cable |
| `showcase-gasgar` | 1 min 31 s | `ResetGraph` twice, then the monitor's greeting and `>>` | back at the monitor by itself |
| `scee-topten-adven` | 2 min 21 s | `ResetGraph` | reset from the cable |
| `pssoft-airbob-2-airbob` | 1 min 42 s | `ResetGraph`, the pad driver | reset from the cable |

`showcase-gasgar` returns to the monitor on the clean library on a console, as in the emulator, where Sony's keeps it running. It's a difference in behavior, and isn't explained. Watched on the TV in a second run, it shows nothing: the screen stays black from upload to the prompt, so it leaves during its startup, before its first frame.

**Watched on the TV:** `pssoft-terra-2` looked normal, and the reboot from the cable went to the boot disc's screens as a press of reset does. The screen is black while the library goes in and while a program uploads, since our monitor draws nothing. `demos-scee-tunelfin` reached `level updating` 2 in 45 s, where the emulator reaches 4.

**Unexplained:** while Terra uploaded, the TV showed solid bright blue, where the probe's last picture should be dark blue (20, 20, 60) with a smaller bright box, a line and a sprite. Terra's picture then replaced it. It may be the TV's own no-signal color. That hasn't been checked.

What this shows, and what it doesn't yet:

- **The emulator's cost model is far from a console's.** Stores and copies cost about twice what they should, loads about 12% too little, and the routines that write to the GPU's packet area run about 1.7 times slower on a console than in the emulator.
- **Agreeing in the emulator doesn't mean agreeing on a console.** Reading by reading, the two libraries that the emulator has equal differ by up to 2.5 times here. `divdf3` is the extreme, and `GsSortBoxFill`, `GsSortSprite` and `Sin` are also much faster on clean, while `RotMatrix` and `muldf3` are slower. So cycle matching against the emulator doesn't carry over, and any more of it should wait for a better cost model.
- **Why isn't known.** The instruction cache, with each library's own layout, and the RAM's and bus's costs are the suspects, and nothing has separated them. A difference in these readings may belong to the code or only to where it sits.

### Evening, 2026-11-07: the monitor's screen and gasgar

**The monitor's screen.** `227484f` added a purple CLEAN MONITOR screen, drawn straight to the GPU, so that a black TV can't be mistaken for a fault. It showed in the emulator. On the console it didn't:

- The monitor never sent its greeting, and `hwrun` couldn't take over. The TV kept the boot disc monitor's last screen, shifted a little to the right. With the greeting moved before the drawing, the monitor answered, and the TV did the same.
- GPUSTAT, read through the monitor, was 10022400h. Bit 10 shows GP0(E1h) took effect and the display settings did too, which is the shift. Bit 26, ready for a command, stayed 0, so the drawing loop gave up and turned the display on over the old picture.
- By hand, `sw 1f801814 01000000` (GP1(01h), reset the command buffer) set bit 26, and a GP0(02h) fill then turned the TV purple, which the user saw as lavender on the 10-inch Sony monitor. So a command the GPU was partway through taking, from the boot disc's monitor, blocks it, and GP1(00h) doesn't drop it on a console.
- With GP1(01h) put straight after GP1(00h) in the code, the console was stuck the same way: GPUSTAT 90022400h. Replayed by hand afterward, one command at a time, GP1(01h), E1h, E3h, E4h and E5h each left bit 26 set. So it's likely the timing: GP1(01h) sent while GP1(00h) is still under way. That isn't proved.
- The working tree's version waits after each reset, resets the command buffer again if the GPU won't take a word, and if it still won't, leaves the GPU with its buffer reset and the display off, so that a program's `ResetGraph` isn't blocked. Not yet tried on a console.

**gasgar.** The fix in `227484f`, zeroing the 16 KB under a program's stack, isn't tested on a console yet. In both runs with the screen code, gasgar printed the pad driver line and no `ResetGraph` in 45 s, where before the screen code it printed `ResetGraph` twice and left. The GPU the screen code left blocked would stop `ResetGraph`, so those runs say nothing about the stack. The TV showed the shifted old picture throughout.

**Rebooting from the monitor.** `sr epc bfc00000` and then `go`, typed to the clean monitor, reboots the console from the disc, as the serial reset does, with no button. That's how the console was brought back to Sony's monitor to load a new clean monitor. `hwrun` only takes over when the word at 80068000h isn't the clean monitor's, which is the same in every build, so a console with an older clean monitor keeps it until it's rebooted.

**Sony's monitor draws on the TV:** the boot disc's monitor shows each command and transfer on screen (`ADRS`, `SIZE`, `INTO BURST TRANSFER`, a sum, `DONE.`), on a semitransparent panel.

**The adapter** is USB 067b:2303. `/dev/ttyUSB0` is root's and group `dialout`'s, and this account isn't in that group. The user plans a udev rule (`TAG+="uaccess"` on that ID) so access doesn't need granting at each plug-in.

**Next session, in order:**

1. Boot the disc and connect. Check the clean monitor's screen: the purple CLEAN MONITOR, or at worst a black screen with the monitor answering.
2. `showcase-gasgar` on the clean library, watched, for the stack fix.
3. Try one small `ReadMemory`, which is the only use of RTS from the host and the only thing the loopback test would still answer. If it stalls, nothing else needs it.
4. The same programs on Sony's library, one reset press each, for the comparison.

### Morning, 2026-11-08: the screen works, gasgar runs, RTS works

**gasgar on clean, with the stack fix.** It printed the pad driver line and one `ResetGraph`, then ran its tunnel with sound for the whole 45 s watch, and the serial reset brought the console back. Before the fix it quit after `ResetGraph`. The user saw a lot of flashing in the picture. Whether Sony's library shows the same is step 4's question: in the emulator the two libraries draw gasgar identically for 1,200 frames.

**The monitor's screen.** Last night's fix didn't help: the TV kept the boot disc's shifted screen. A log of GPUSTAT kept in the monitor and read back with `dw` found the real cause, and it wasn't a stuck GPU or a half-taken command from the boot disc:

- On a console, GPUSTAT bit 26 (ready for a command word) is clear while a command's parameters are still owed. After the fill's first word it read `90822400` and stayed so until the parameters came. The monitor waited for bit 26 before every word, so it waited for the fill's first parameter, gave up, and reset the command buffer, which dropped the fill. Each letter's rectangle went the same way.
- The emulator keeps bit 26 set while parameters are owed, which is why the screen showed there. That's an emulator inaccuracy worth fixing separately.
- Ruled out along the way: a GPU DMA from the boot disc's monitor still running (channel 2 was idle at the monitor's start), and polling GPUSTAT too fast (pausing between reads changed nothing).
- The fix waits for bit 26 only before a command's first word and writes parameters straight after. The purple CLEAN MONITOR screen then showed on the console.
- Last night's GPUSTAT `10022400`, with bit 10 set and bit 26 clear, fits the same cause: E1h was taken, then the fill's first word, with the monitor waiting for its parameter.

**RTS works.** `ReadMemory` (`brd`, the only use of RTS from the host) read 16 bytes and 1 KB at 80068000h correctly, and the monitor answered after. Both took 8.5 s. That wasn't a fixed cost: `brd` always sends whole 2,048-byte blocks, and the boot disc's monitor waits for an RTS pulse after every byte. On this adapter each change of RTS is a USB control transfer of 2 ms, so a pulse is 4 ms and a block 8.5 s. The clean monitor now holds a byte only while the host's RTS is off, as it does for any byte, and the host pulses once per read it makes. A block now takes about 0.33 s: 16 bytes in 0.67 s, 8 KB in 1.68 s, with words on both sides of each block boundary matching `dw`. Reads through the boot disc's monitor are unchanged.

**The pacing delays.** With all of them at zero, the clean monitor took 10 commands at 8 ms each, three 64 KB writes at about the line's speed (5.8 s) with every byte read back right, and a program upload and run. The boot disc's monitor didn't: without `Header` (100 ms after a write's header) or `BlockStart` (1 ms after a block's 02h) it ended the binary write at its first block. With both, and `Reply`, `Load` and `Run` at zero, a full `hwrun` cycle passed: takeover, upload, run, reset from the cable, takeover again. The clean library now goes in in 48 s, not 1 min 8 s, and tunelfin uploads in 55 s, not 1 min 25 s. `Reply` had been 100 ms after every command and every block.

**Why the boot disc's monitor needs them, measured black-box:**

- Sampling the console's RTS (the host's CTS, read every 270 ns from the driver's copy): after a write's header it drops 0.59 ms later and stays low for 169.6 ms, which fits the monitor drawing its `ADRS` and `SIZE` panel. After a block's 02h it drops 0.53 ms later for about 2 ms. Idle in binary mode it stays high.
- Bytes sent while it's low aren't dropped as such: a single byte sent early, at any offset from 0 to 3 ms after the 02h, was kept (the block then failed its sum by the one extra byte), and a 02h sent straight after the header was kept through the 170 ms.
- Bursts sent into either stretch, ignoring CTS: 4 or 8 bytes always went through and the block got Y. 9, 10, 12, 16 and 24 bytes always ended the write. So the console's 8-byte receive buffer overruns while the monitor isn't reading, and the monitor ends the session.
- With no pause, the host sends a byte every 87 µs, and only sees CTS drop a little under a millisecond after the console lowered it, with more already queued in the adapter. That's more than 8 bytes, so the write ends. A pause of 2 ms after the header was enough in a sweep from 0 to 60 ms. The old loader's two notes fit: "fails with WriteByte timeout" is a CTS wait running out during the 170 ms, and "hangs" is the session ending under a host still sending.
- So the two pauses are now waits for CTS to drop, up to 3 ms each, which end as soon as the console lowers it. On the console: five 64 KB writes to the boot disc's monitor at 6.8 s each with the data read back right, and a full `hwrun` cycle on tunelfin, the clean library in 48 s and the upload in 54 s. The clean monitor never lowers CTS, so each wait there costs its 3 ms. The clean library's takeover takes 48 s. One of four took 54 s, while a GCC torture test in another session was spiking every core; steady load (about 50 on 24 cores) changed neither the takeover nor a 64 KB write (6.82 s both ways). Not looked into further.
- Left unexplained: once, with the early 02h sent straight after the header and the block 5 ms after CTS rose, the block got Y, and then the console held its RTS low and stopped answering until reset.

**The four programs on Sony's library**, one reset press each, then gasgar and tunelfin on clean again for comparison. Raw output in `.build/hardware/sony` and `.build/hardware/day2-clean`.

| Program | Sony's | Clean |
|---|---|---|
| `showcase-gasgar` | smooth, no flashing, clear audio | heavy flashing, painful to watch; audio fine |
| `demos-scee-tunelfin` | looks and sounds fine; froze with its frame counter at 2998 | ran the whole 75 s watch, counter about 4328, `level updating` 1 to 4 |
| `scee-topten-adven` | looks and sounds good | looked normal last night |
| `pssoft-airbob-2-airbob` | looks and sounds fine | looked normal last night |

- **gasgar's flashing is the clean library's**, and the emulator doesn't show it: there the two libraries draw gasgar identically for 1,200 frames. It's the first console-only picture difference. A first step offline: make the emulator clear GPUSTAT bit 26 while parameters are owed, as the console does, and see whether clean gasgar then flashes there.
- **tunelfin froze on Sony's library and not on clean**, at frame 2998 by its own counter. Clean went past it. Not yet known whether Sony's freeze happens every run. Deferred to a later session. The user expects Sony's freezes in the emulator too, which a run past frame 3000 on both libraries would confirm: the archive comparisons stop at 1,200 frames.
- **The pad driver line.** Clean prints `PS-X Control PAD Driver  Ver 3.0` and `addr=8004d724` whenever a program starts the pads. Sony's printed neither in any of the four. A guess, unconfirmed: Sony's prints it only on the first start after boot, which the boot disc itself uses up.
- Upload times on Sony's monitor were close to clean's: gasgar 1m37s against 1m31s, tunelfin 1m27s against 1m25s, adven 2m31s against 2m21s, airbob 1m45s against 1m42s.


### Evening, 2026-11-08: gasgar's flashing, what it is not

gasgar on the clean library still flashes on a console after specs 39 and 40. Measured since, each the same on both libraries unless said:

- **The GPU words** gasgar sends, in the emulator, for 200 frames: the same words in the same order at nearly the same times. With `Machine.SetDrawTime` and `Machine.SetFieldSkip` on as well, its screens come out byte for byte the same.
- **`VSync`** (`probes/vsynclate`), on the console: the same blanks and lines, interlaced too.
- **The cost of gasgar's calls** (`probes/callcost`), on the console: within an eighth of Sony's, `GsSortObject4` included.
- **gasgar's loop rate on the clean library**, from two peeks 10 s apart (`hwrun -peek`, `.build/hardware/peek1`): 574 buffer swaps in 575 blanks, a full 60 a second, waiting in `VSync` with the GPU idle. At each peek the frame's list, the font's and gasgar's 8 `LoadImage` calls were still queued, the DMA interrupt pending: the frame's DMA had just ended. The emulator with the drawing time on shows Sony's queue backed up the same way at each blank.
- **Images queued behind the drawing** (`probes/imgdrop`, gasgar's loop with 1 to 10 full-screen boxes), on the console: both land all of them up to 4 boxes, half at 5 and none from 6, identically.
- **`GsSortObject4`'s packets** (`probes/accmulti`, 1,056 cases), on the console: the same but for the lit Gouraud kinds (`lg3`, `lg4`) on the "on" shape, where 7 hashes differ between the libraries, and Sony's on the console differs from Sony's in the emulator too. Only the hashes' top byte differs, which this hash gives only for words differing in their top byte: most likely the code byte the GTE copies into the second and third vertex colors, which the GPU ignores. A dump of those packets on the console (`accmulti` with dumping on for those cases) would confirm it.

Peeking under Sony's library failed: with Sony's image put in place by the clean takeover, gasgar printed nothing and the stop timed out, and whether it ran at all is not known.

Left to try: a description of the flashing from someone watching (whole frames, stripes, the background, the sprites or the text); the packet dump above; FntPrint and FntFlush, and the sound library's interrupts, which no probe has covered on the console.

A probe that never returns, run on Sony's library, leaves the console for the reset button: Sony's monitor can't be reached while it runs. `accmulti` ends in a `VSync` loop and did that.
