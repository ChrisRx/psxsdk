# The image's room above its last entry point

The resident image has to end below 80068000h, because a Net Yaroze's monitor lives from there up to 80090000h, between the library and the program (`serial/monitor/target.json`). It now ends at 8007A8A8h.
One thing does that: the Go global `exp/cleanlib.kanji`, 83,484 bytes of `.bss`, too big for any gap between the entry points, so it spills past the last.

Sony's library keeps nothing like it there. Probes (spec 26) showed its Krom2Tim2 takes a stack frame of 73,872 bytes, and the clean one 56: its working memory is on the stack for the length of the call.

## What to do

Bring `exp/cleanlib`'s `.bss` needs down so that the image ends below 80068000h, with room to spare if you can, and everything else as it is. Move working memory that only lives for a call onto the stack, as Sony's does, sized as a probe shows, which also brings the frame closer to Sony's: spec 26 lists the stack frame as a difference. What has to last between calls, such as a KanjiFntOpen stream's text, stays, at the size probes show it needs. The TIM a Krom2Tim builds is the caller's memory.

Behavior and time don't change: specs 13, 23 and 26 still hold.

## Acceptance

- `exp/cleanlib/image/make.sh`: the line it prints gives where the image ends, below 80068000h.
- `task build:serial`, then `go test ./tests -run 'Monitor|Siocons|Yaroze|Debugger'`, with `PSX_SONY_TESTDATA=$HOME/src/ChrisRx/psxsdk-sony/testdata`: pass. Don't read that directory: the test reads it. The monitor's tests were failing only because the image overlapped the monitor.
- Your Kanji probes: same results and cycles as before on the image. `exp/cleanlib/spec/probes/` holds those of spec 13.
- `exp/cleanlib/regress.sh` without `PSX_BIOS` and `exp/cleanlib/image/regress.sh`: FAIL stays 0, with 33 PASS and 8 TIME before.

## What the work on this spec found

The flush's image and placement table went into a frame on the stack of Sony's 65,536 bytes, starting 65,672 below the probe's sp as Sony's does. Cells that hold no character keep what the stack held. `kanji` is 42,164 bytes, in a gap, and the image ends at 80066288h. The frame is past a 64 KB stack on both libraries, so calls made from it write below the stack's reserve, which on the static target is the top of the heap.

Krom2Tim's frame is 73,872 bytes on Sony's, and every successful call clears 73,728 bytes of it, even for an empty string: 589,824 of its 610,500 cycles at 8 a byte. The clean one has no working memory to move and spends that time in a pad. See spec 30.

The monitor's tests depended on Sony's instructions as well as the overlap, and now load their own code. Sony's sample `main-ecoff` reads Time 36 on Sony's and 34 on the clean image, not explained.
