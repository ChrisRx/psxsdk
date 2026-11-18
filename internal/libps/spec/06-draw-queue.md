# The drawing queue

The library's drawing calls do not wait for the GPU. The Library
Reference's DrawSync says so: mode 1 returns "the number of positions
in the current queue", and GsDrawOt, LoadImage, MoveImage, StoreImage,
ClearImage and the font's flush return at once while the GPU works.
Requests made while the GPU is busy are queued, in order, and each
starts when the one before it finishes (the GPU's DMA channel 2
completing, which raises its interrupt). DrawSync(0) waits until the
queue is empty and the GPU idle; DrawSync(1) returns how many requests
are still queued or running.

On hardware programs depend on this for speed: drawing overlaps the
next frame's CPU work.

What to keep in mind:

- The GPU stream must not change: the same words in the same order.
  Only when they are sent changes.
- A request's data must stay valid until it is sent: a queued FntFlush
  builds its packets in the library's own memory, so it must not reuse
  that memory for the next flush before the previous one has gone out.
- The queue runs from the DMA completion interrupt; keep the handler
  short and allocation-free, and make DrawSync's wait robust against
  the interrupt arriving between its checks.
- Drawing environment changes (the E1h to E6h words GsSwapDispBuff and
  the others send) go into the queue in order with the drawing they
  apply to.

## Measuring

    TGOFLAGS=-debug exp/cleanlib/build.sh flame-prof examples/flame
    PSX_PROFILE=flame-prof go test ./tests -run TestCleanProfile -v

prints cycles a frame by function, file and source line (the DWARF line
table; TinyGo inlines most of the library into main). Time in VSync's
wait loop is the frame's spare time. Do not profile the reference
builds: their symbols name the decompiled library's functions. For
flame the reference spends about 150,000 of a frame's 566,000 cycles
waiting in VSync; the clean build, before this change, about 123,000
there and 82,000 more waiting for the GPU inside its drawing calls.

Acceptance: every TestClean case still passes, and flame's readout comes
down to about the reference's (190); its digits matching exactly would
be a bonus, not a requirement.

## Findings

The queue is in and the GPU stream is unchanged, but the diagnosis above
was wrong for this emulator: its linked-list DMA to the GPU runs the
whole list when CHCR is written, and the GPU takes no time, so there is
no drawing to overlap. The "82,000 cycles waiting for the GPU" were
VSync's own wait loop, whose inlined volatile loads the profile charges
to `runtime/volatile/volatile.go`.

- The kernel's DMA handler acknowledges the DMA interrupt, so the queue
  runs from a verifier on the kernel's interrupt chain (SysEnqIntRP,
  C0 02h), through `libps_draw_irq` in `playstation-yaroze-bios.S`.
- FntFlush waits only until the previous flush's packets have gone out.
  LoadImage empties the queue first and stays synchronous.
- The time was in GsSortObject4 after all: loops specialized per
  primitive kind took flame from 240 lines to 208. The GTE FLAG check
  car needs (about 27 cycles a polygon, since reading FLAG waits for
  RTPT) brought it back to 232.
- About 13,900 cycles a frame (24,000 in dungeon) went to a 64-bit
  division routine (`r3000-builtins.c`), called from vector
  normalization and GsSetRefView2. A 32-bit divide when both operands
  fit gives the same results; flame now reads 220 against the
  reference's 190.
