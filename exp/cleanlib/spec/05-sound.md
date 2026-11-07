# Milestone 5: sound

The examples that play sound, in rough order of what they add. Their
reference discs are built (`.build/NAME-ref`). For each:

    exp/cleanlib/build.sh NAME-clean examples/NAME
    PSX_CLEAN=NAME PSX_CLEAN_FRAMES=600 go test ./tests -run 'TestClean$' -v

TestClean compares the GPU stream swap by swap (cut before every GP1
05h word; PSX_CLEAN_FRAMES counts swaps, and PSX_CLEAN_PAD holds its
buttons from the 30th swap to the 60th). It also compares every SPU
register write (offset from 0x1F801C00 and the halfword written; a word
write counts as two) frame by frame, from the frame of the first swap,
and the SPU's 512KB of sound memory at the end.
The streams are in `.build/NAME-clean/ref-spu.txt` and `clean-spu.txt`.
Sample data uploaded by DMA does not show as writes; the sound memory
comparison catches it. Use input (PSX_CLEAN_PAD) where an example makes
sounds on a button.

| Example  | Adds                                                                 |
|----------|----------------------------------------------------------------------|
| check    | SsVabTransfer SsVabClose SsSetMVol; SEQ: SsSeqOpen SsSeqPlay SsSeqSetVol SsSeqStop SsSeqClose |
| breakout | as check, sound effects with the music                               |
| invaders | as breakout                                                          |
| dogtale  | SsSeqPause SsSeqReplay                                               |
| cdda     | CdPlay SsSetSerialAttr SsSetSerialVol (CD audio through the SPU)     |
| square   | sequences with 3D                                                    |
| tunnel   | SsUtKeyOn SsUtKeyOff (voices played directly), PushMatrix PopMatrix  |
| clone    | SsUtAllKeyOff SsUtSetVVol, GsScaleScreen StoreImage RotMatrixY, GsSortLine |
| dino     | reverb: SsUtReverbOn SsUtSetReverbType SsUtSetReverbDepth; gteMIMefunc |
| terra    | everything above, SsSeqGetVol, RotMatrixZ, ClearImage                |

Also SsSetTickMode, which hello's `Screen` calls.

## Sources

The Library Reference's sound chapter, the User Guide, and the File
Formats manual (VAB: the header, program and tone attributes, the VAG
sizes and bodies; SEQ: the header, resolution, tempo, and the MIDI
events) describe the interface. The SPU's registers (voices, key on and
off, ADSR, pitch, volumes, reverb, the transfer port, CD audio input)
are in public hardware documentation and in `emu/spu.go`. The reference
SPU stream is the observed behavior, and many details below can only be
read from it: derive formulas from what it writes for known inputs (a
note on a known tone at a known velocity), and check them across the
examples.

## Notes

**Initialization.** The sound system starts in the library's start-up,
the program's first ResetGraph (spec 07, "The library's start-up": what
the resident library did before main), after the drive's reset below:
SPU on, voices off, main volume, reverb off, the transfer mode, then the
tick at every vertical blank. Match the setup sequence the reference
writes before the first swap. A sound or CD call before any ResetGraph
starts it there instead.

**VAB.** SsVabTransfer(vh, vb, vabid, mode) reads the header and the
program and tone attributes from vh, copies the sample bodies from vb
into sound memory, and returns the VAB's id (-1 on failure). Where the
bodies go in sound memory, and in what order, must match the reference
(the sound memory comparison checks it). The program and tone
attributes are not to be copied: the library reads them from the header
at every key on (see "The header is read at every key on" below).
SsVabClose(id) frees the VAB's sound memory.

**SEQ.** SsSeqOpen(addr, vabid) parses a SEQ and returns its id;
SsSeqPlay(id, mode, loops) starts it (mode SSPLAY_PLAY or SSPLAY_PAUSE);
the sequencer advances on a tick driven from the vertical blank interrupt
at the rate SsSetTickMode sets, so its events land on the same frames as
the reference's. A note on allocates a voice (match the reference's
choice of voice), sets its pitch from the note against the tone's center
note and fine tune, its volumes from velocity, tone, program, sequence,
main volume and pan, its ADSR from the tone, and keys it on. Note off,
program change, control changes (volume, pan, expression, data entry
for loop points as the manual describes), pitch bend and tempo meta
events follow the manual. SsSeqSetVol, SsSeqGetVol, SsSeqPause,
SsSeqReplay, SsSeqStop and SsSeqClose as named.

**SsUt.** SsUtKeyOn(vabid, prog, tone, note, fine, voll, volr) plays a
tone directly and returns the voice; SsUtKeyOff releases it,
SsUtAllKeyOff every voice. SsUtSetVVol sets a voice's volumes.
SsUtReverbOn, SsUtSetReverbType and SsUtSetReverbDepth set up the reverb
work area and registers for the manual's reverb types (the preset
register values for the standard types are public; check them against
the stream).

**CD audio.** CdPlay plays CD-DA tracks (the CD-ROM controller's Play
with track and mode settings); SsSetSerialAttr and SsSetSerialVol switch
the CD input into the mix and set its volume.

**Matrix stack.** PushMatrix and PopMatrix save and restore the GTE's
rotation and translation. RotMatrixY and RotMatrixZ rotate a matrix about
one axis. GsScaleScreen scales the screen matrix. StoreImage reads VRAM
back (GP0 C0h); ClearImage fills a VRAM rectangle (GP0 02h).
gteMIMefunc is the GTE's vertex interpolation for morphing (the
manual).

## Findings (milestone 5, first pass)

Every sound example builds and their SPU write sequences match for 98
to 99.5% of the writes; none passes yet. check's GPU stream and sound
memory match. What remains is voice allocation (now specified below),
the unimplemented calls listed at the end, and tunnel and clone, which
use GsDIV (see spec 04's Subdivision, implemented since: tunnel's GPU
stream matches, clone differs before its 3D view, at a StoreImage).

**Start-up.** Sound starts at the first Ss call (SsVabTransfer,
SsSetMVol or SsSetSerialAttr), not at program start, after the CD
drive's own five words if they have not been written. The sequence:

1. 180=0, 182=0, 1aa=0, 180=0, 182=0, 1ac=4, 184=0, 186=0,
   18c/18e=ffff, 198/19a=0, 190/192=0, 194/196=0, 1b0/1b2/1b4/1b6=0.
2. 1a6=0200, eight writes of 0707 to 1a8, 1aa=0010, 1aa=0.
3. Each voice: 0=0, 2=0, 4=3fff, 6=0200, 8=0, a=0.
4. 188=ffff, 18a=00ff, 18c=ffff, 18e=00ff, 1aa=c000, 1a2=fffe.
5. Each voice: 0=0, 2=0, 4=1000, 6=3000, 8=00bf, a=0, c=0, e=0.
6. 180=3fff, 182=3fff, then 184 to 19e all 0.
7. Each voice: 6=0200, 4=1000, 8=80ff, 0=0, 2=0, a=4000 (the writes
   SsUtAllKeyOff makes).
8. 18c=0000, 18e=00ff, 188=0, 18a=0, 198=0, 19a=0.

**Tick.** Runs every vertical blank once sound has started, playing or
not. It writes the changed voice registers in voice order (0, 2, 4, 6,
8, a), then key off (18c/18e), key on (188/18a) and the reverb voice mask
(198/19a). SsUtKeyOn and SsUtSetVVol only stage writes for the next
tick. A key on clears the voice's pending key off and the reverse. A
voice's reverb bit is set or cleared at key on from tone mode bit 2.

**VAB.** Sound memory is allocated first fit from 0x1010 and a freed
bank's space is reused. Sizes are the VH table's, not rounded. The bank
goes over DMA channel 4: 1a6=addr>>3, SPUCNT |= 0x20, blocks of 64
bytes (up to 62 bytes past the bank's end are copied), then SPUCNT
restored. In this emulator the DMA upload shows as 1a8 writes. Open: in
the reference, SsVabTransfer with id 7 returned -1.

SsVabTransfer also writes into the header it is given (probe
`spec/probes/vabsize` on Sony's library). The 128 program entries,
16 bytes each from offset 32, change in their last eight bytes; nothing
else in the header changes:

- Bytes 8 to 11 of entry p, a 32-bit word: p for p below the header's
  program count (ps), and ps for every entry after.
- Bytes 12 to 15 of the entries, taken in order as 16-bit halves (entry
  0's first and second half, then entry 1's, and so on): the sound
  memory address, divided by 8, of each of the vs VAGs, then the end of
  the bank's body, which is vs + 1 values. Halves past those keep what
  was there. The probe's bank (ps 2, vs 2, VAG sizes 656 and 352 bytes,
  at 0x1010): entry 0 gets 0202h and 0254h, entry 1 gets 0280h and
  keeps FFFFh. The end is the unrounded sum, not the 64-byte blocks the
  upload sends.
- The table's sizes decide the upload, not the header's total size: a
  table entry 8 larger (64 bytes) sends 32 more halfwords, and changes
  to the total size send the same.
- A header with a VAG count of 0 or above 255, or whose table's sizes come to
  0, is refused with -1, with nothing written to the header and no sound memory
  taken (`spec/probes/vabrefuse`, spec 36). A count of 255 is accepted. The sizes
  were only tried all 0, so a first size of 0 with later ones isn't known.

terra sends a buffer that still holds a header written this way as the
start of a later bank's body, so the written values reach sound memory
there.

**Volumes.**
- SsSetMVol: register = (v·129) & 0x7fff.
- SsSetSerialVol: register = min(v, 127)·258, unmasked (-1 gives fefe).
- SsSetSerialAttr: read-modify-write of SPUCNT, bit 0 for MIX, bit 2
  for REV.
- SsUtSetVVol: v·129, linear. SsUtGetVVol returns the l and r SsUtKeyOn
  was given (0 and 0 for a noise voice, see "Noise voices").
- A voice's volume, per side: x = 0x3fff·(vol·bank mvol)/16129, then
  ·(tone vol·program vol)/16129, then ·pan(tone)/63, ·pan(program)/63,
  ·pan(key)/63, then ·sequence volume/127; register = x²/0x3fff. The
  left pan factor is min(127-pan, 63), the right min(pan, 63). The
  bank's master pan is not used.
- For SsUtKeyOn vol = max(l, r) and the key pan is r·64/l when l > r,
  127 - l·64/r when l < r. For a sequence vol is the velocity and the
  key pan 64.

**Pitch.** k = (note - center + 60)·16 + (fine + shift)>>3;
table[i] = floor(4096·2^(i/192)); pitch = table[k%192] shifted by
k/192 - 5, left shifts wrapping at 16 bits.

**Sequence timing.** bpm = round(60e6/tempo). rate = round(res·bpm·10/
3600) tenths of a tick per blank, 3000 in place of 3600 on PAL. The
first tick after SsSeqPlay or SsSeqReplay does not advance. An event at
tick t plays in blank m when 10t < rate·m; events exactly at rate·m play
only if they are the first processed in that blank.

**Note off** matches voices by (sequence, program, note), not channel,
and finds a released voice however long ago it was released, until the
voice is taken again. SsSeqStop keys the sequence's voices off on the
next two ticks; SsSeqPlay after a stop restarts from the beginning.
SsSeqClose stages volume 0 and a key off for its voices. At the end of a
sequence with plays left at 1, all its voices are keyed off that tick.
(Spec 10 corrects this: the tick after SsSeqStop still plays the
sequence before its key off; the end of the track comes at once after
the event before it; SsSeqClose of a closed id silences its voices
again.)

**Voice allocation.** Probed on Sony's library with
`spec/probes/voices` (SsUtKeyOn only, 69 cases). The library reads no
SPU register but the voices' envelopes (0x0c): each tick reads all 24,
before it writes that tick's voice registers, key offs and key ons, and
keeps the value read per voice. Everything below is in terms of those
stored reads, so a key on staged in a frame and written at the next tick
is read there as the envelope the voice had before it.

- *Free.* A voice is free when no key on is pending for it and either
  it was keyed off (by SsUtKeyOff, a note off or SsUtAllKeyOff) and a
  tick read its envelope as 0, or it is still keyed on and 16 ticks in
  a row read 0, counting from the first tick after the key on was
  staged (a sound that ends by itself, without a loop, is freed that
  way, about 15 frames after it goes quiet). A voice never used is free.
  SsUtAllKeyOff frees every voice at once, but a voice whose envelope
  the next tick still reads above 0 (still releasing) is busy again
  until a read finds 0.
- *Key on.* The lowest free voice is taken. If none is free, a voice is
  stolen from those whose tone priority (VagAtr prior) is at most the
  new tone's; program priority is not used. If there are none,
  SsUtKeyOn returns -1. Among them the lowest priority, then the lowest
  stored envelope, then the oldest key on, then the lowest voice
  number. A voice keyed or stolen counts as the newest, and its stored
  envelope becomes 0 at once, until the next tick reads it again.
  SsUtAllKeyOff makes every voice equally old.
- Consequences seen in the probe, all following from the above: right
  after a voice is keyed on from silence the tick reads 0 for it, so
  for one frame it is the first stolen; several steals in one frame
  take the same voice again and again (its stored envelope is 0) unless
  several share the lowest envelope, when they go round oldest first;
  a voice released by a key off is stolen before sustained ones once a
  tick reads its envelope lower; with all voices sustained at the same
  level, steals one a frame go round from the oldest.

Sequences were not probed; they are expected to use the same
allocation. Note off matching is above.

**Reverb.** The ten presets (1c0 to 1fe and the 1a2 base) are in
reverb.go. SsUtSetReverbType sets depth 0 and writes the preset; types above 9 return -1 and
keep the type, with nothing written. Type 0 clears SPUCNT bit 7 and leaves it clear.
Any other type with the bit set clears it first and sets it again after the 1A2 base,
the last write. A negative type, -1 to -9, also clears the work area in sound memory,
from the preset's base to the top, by DMA in kilobytes before the base is written
(`revctrl`, spec 37).
SsUtReverbOn and Off set and clear bit 7. Depth register = v·0x7fff/127.

**GPU, found on the way.**
- Sorted primitives always have the dither bit (E1 bit 9) set, even
  with dithering off in GsInitGraph (spec 02 said it came from
  GsDRAWENV).
- GsSwapDispBuff writes GP1 06h and 07h before 08h when GsDISPENV's
  screen has changed.
- On NTSC the vertical display range ends at line 258 at the latest.
- On PAL a display shorter than 256 lines is centered:
  y0 = 35 + (256-h)/2. (Wrong: see the second pass. It held for 240
  lines only.)
- A rotated and scaled sprite folds the scale into the rotation first
  (m00 = cos·Sx>>12 and so on).

**Not implemented** (at the first pass; see the second pass for what
has been done since). Pitch bend, CC 7, 10 and 11, tempo meta events,
SsSetTempo, SsSeqSetNext, SsSeqSetAccelerando, SsSeqSetRitardando,
SsIsEos, SsPlayBack, SsSetMute and SsGetMute, SsGetSerialAttr and
SsGetSerialVol, SsUtChangePitch, SsUtPitchBend, SsUtSetReverbDelay and
Feedback, and the voice allocation above. CdPlay mode 3 matches without
input; moving to the next track does not (cdda with 4000). Not checked against
a reference: RotMatrixY and Z, ApplyMatrix, GsScaleScreen, PushMatrix
and PopMatrix, gteMIMefunc, StoreImage, ClearImage.

## Findings (milestone 5, second pass)

Passing TestClean at 300 swaps: breakout, square, dogtale, cdda and
clone (also with pads: breakout 0800 and 4000, clone 0010, 0800 and
4000, dogtale 4000, square 0008). check's sound matches; its only
difference is its own "total time" readout. invaders and tunnel match
in everything but one tick at the end of the SPU stream: the frame the
sound starts in, relative to the first swap, depends on the sub-frame
phase the program starts in (see Timing), and both have passed and
failed with changes of a few thousand cycles elsewhere. invaders,
dogtale, breakout and square match over 2000 swaps too (invaders but
for that one tick). dino and terra are discussed under Timing.

**Voice allocation, as implemented** (`spu.go`), all from the voices
probe and the sound examples' reference streams:

- Each tick reads the 24 envelopes first, then runs the sequences, then
  writes the voice registers, key offs, key ons and reverb mask, and
  only then keeps what it read as each voice's stored envelope. So the
  sequences' note ons in a tick see the envelopes the previous tick
  read; SsUt calls between ticks see this tick's.
  (This order was corrected on 2026-11-03: the sequences run at the end
  of the tick before, which shows when the program calls SsUt between; see
  "Where the sequences run" under "Noise voices". What a sequence stage sees
  of the envelopes is the same.)
- A voice is free when it is keyed off and its stored envelope is 0, or
  keyed on with 16 stored zero reads in a row (counted at the end of
  each tick while the voice is keyed on, the tick that writes its key on
  included). A voice keyed on since the last tick ended ("fresh") is
  never free, even when keyed off again in the same tick.
- So a sequence's note off frees its voice at once when the voice's
  stored envelope is 0 (a sound that has died away while still keyed
  on, read as 0 at the last tick): a note on later in the same tick
  takes it (check). A voice keyed off whose envelope the last tick read
  above 0 is busy until a tick reads 0 (dogtale).
- SsUtKeyOn sets the stored envelope of the voice it takes to 0; a
  sequence's note on leaves it as last read.
- Steals: the lowest priority, then the lowest stored envelope, then the
  oldest, then the lowest number, among voices whose tone priority is
  at most the new tone's. SsUtKeyOn's steals may take a fresh voice
  (several steals in a frame take the same one); a sequence's never
  take a fresh voice (dogtale: a note on, its note off and another note
  on in one tick do not reuse the first voice).
- SsUtAllKeyOff parks each voice (the six writes 006, 004, 008, 000,
  002, 00a) and stages its key off one voice after another with
  interrupts on: a tick in between keys off the voices done so far
  (Sony's stream shows 18c=001f, then the rest at the next tick).
- The voices probe matches Sony's TTY in every case's results. Three
  "quiet after" counts (the frames the probe waits for silence before a
  case) differ by one or two frames: they depend on where in the frame
  the probe polls, not on the library's choices.

**Where the tick runs.** From the library's own interrupt handler, after
the kernel's handlers (the BIOS pad driver among them), at the vertical
blank: the reference reads the envelopes about 8,500 cycles into the
frame with the pad driver running (the clean library about 7,600). A
tick on the kernel's interrupt chain at priority 0 ran before the pad
driver and read about 6,000 cycles earlier, which changes which tick
first reads a dying envelope as 0.

**Pitch bend.** (The formula below is close but not exact: see spec 10,
"Pitch bend (spec 05 corrected)".) Only the high byte of the 14-bit
value is used (0 to 127, 64 the center). A bend retunes every voice of the sequence on that
channel, at once (one pitch write a tick, the last bend's, however many
come in it); a note started later starts unbent. In 128ths of a
semitone the bend is floor((b - 64)·range·128/63), range being the
tone's pbmax for b above 64 and pbmin below; it adds to the tone's
shift: k = (note - center + 60)·16 + (shift + bend)>>3. (Checked with
shifts 124 and 71: b 57 to 63 and 68 give -3, -3, -3, -2, -2(60), -1,
0 and +2 steps as observed.)

**CD.** Black-box, from the drive commands the reference sends:

- The library's start-up (spec 07, "The library's start-up"; before it
  was moved there, the first CD or sound call) runs the drive's reset:
  Getstat, Init (waiting for its second response), Demute, then the
  five SPU words (180, 182, 1b0, 1b2 = 3fff, 1aa = c001), then the
  drive's audio volume registers (Sony's initialization routine and the
  reference alike, 2026-11-02): index 2, 1F801802h = 80h, 1F801803h =
  00h; index 3, 1F801801h = 80h, 1F801802h = 00h, 1F801803h = 20h (the
  apply bit). The reference takes about 60,000 cycles before the
  Getstat: that is the "addr=" line below, printed first. A CD or sound
  call before any ResetGraph runs the reset there.
- The library delivers kernel events for the drive's interrupts:
  B0h function 07h, DeliverEvent(F0000003h, x), with x seen as 20h and
  40h in Sony's library (invaders and cdda built for it) and in the
  reference. Traced next to the drive's responses (Sony's library,
  invaders and cdda with cross held), each about 470 cycles after the
  response is read, in the interrupt:
  - 20h at the INT3 of a command sent on its own: Setloc before a file
    read (CdReadFile), the Setmode CdPlay(1|2) sends after the table of
    contents, Play, and CdPlay(3)'s Getstat.
  - Nothing for the commands an operation sends on the way: the Setmode
    and ReadN of a read, the INT1s of its sectors, CdPlay's GetTN and
    GetTD, and the Setloc before Play. Nothing for the drive's reset
    (Getstat, Init's two responses, Demute; reference).
  - Pause, which answers twice (sent at the end of a read): 40h at its
    INT3 and 20h at its INT2.
  - INT4 (data end) gives none.
  The reference differs from Sony's library in two places: it also gives
  20h at GetTN and every GetTD, and none at the Play sent after a data
  end (its Getstat after gives 20h).
  The commands a program sends through CdControl and CdControlB
  (spec 09; probe `spec/probes/cdmore`, 2026-11-03, which counts the
  events through two events of class F0000003h opened with callbacks and
  was traced with iotrace), on Sony's library and the clean one alike:
  - A command that answers once (Getstat, GetTN, GetTD, GetlocP, Setloc,
    Play, ReadN, ReadS): 20h at its INT3.
  - A command that answers twice (SeekL, SeekP, Pause): nothing at its
    INT3 and 20h at its INT2, through CdControl (whose caller has
    returned by then) as through CdControlB. So the 40h at the INT3 of
    the Pause above is the read's, not a Pause's: a Pause the program
    sends gives none.
  - The Setloc the library sends before SeekL, SeekP, ReadN, Play and
    ReadS: nothing at its INT3, through CdControl and CdControlB alike.
    Refused (a byte that is not BCD), it gives 40h at each of its four
    tries and the command is never sent.
  - The program's own command refused: 40h then 20h at each of the four
    tries, about 560 cycles apart (CdControl(Setloc, AAh 00 00) gives
    four of each). A second response that is an error (SeekL to
    70:00:00, 06h 10h): 40h then 20h.
  - Each sector the drive delivers while no read of the library's is
    in progress, after a ReadN or ReadS the program sent: 40h, the
    library clearing the data request (index 0, 1F801803h = 00h) and
    taking no data; 75 of them in 30 frames at double speed.
- A command waits until the previous one has fully answered: a command
  that answers twice (Init, Pause, Stop, MotorOn, SeekL, SeekP, GetID,
  ReadTOC) holds the next until its second response. This is what
  paces file reads: the Pause sent at the end of a read holds the next
  Setloc for about 1,185,000 cycles.
- CdReadSync(1) during a read (probe `spec/probes/readsync` on Sony's
  library, with the drive's commands and responses traced): it gives
  the sector count as soon as CdRead returns, and CdRead returns once
  ReadN is sent, before the drive acknowledges it (the clean library
  waited for the acknowledge). Each sector's INT1 lowers the count by
  one, but the last sector leaves it at 1: it reaches 0 only when the
  drive acknowledges (INT3) the Pause sent after the last sector, some
  52,000 cycles later. The clean library gave 0 at the last sector,
  which near a vertical blank lets a program polling once a frame see
  the read done a frame early (check's and terra's later loads).
- CdRead reads 2340-byte sectors (Setmode with 20h added to the mode
  given) and DMAs each sector's 12-byte header (3 words) and then its
  2048 bytes of data (BCR 00010003, then 00010200). Setmode is sent only
  when the mode differs from the last one set.
- CdPlay(1|2): GetTN, then GetTD for every track from 0 to the last,
  Setmode 03h (CD-DA, autopause), Setloc to the track's start (minutes
  and seconds from GetTD, sector 0) and Play without a parameter. The
  drive pauses at the track's end and raises its data end interrupt
  (INT4). CdPlay(3), which a program calls every frame: after a data
  end, Setloc and Play the next track (round again for mode 2, stop for
  mode 1), then Getstat; while the last status the drive gave does not
  say playing (80h), Setloc and Play the track again; otherwise
  Getstat. At each data end, before the next Setloc, the library prints
  "cbdataready: CdlDataEnd (track=N,time=T)\r\n" through the kernel's
  write, one character per call as ResetGraph's line: N is the index in
  the list of the track that ended (0 for the first) and T is VSync(-1)
  then, both in decimal. That is the 190,000 cycles the reference
  seemed to wait (Sony's library, cdda with cross held, traced: track=0
  time=287, track=1 time=514, track=2 time=740, 44 characters each).

**Start-up from any call.** SsSeqPause on a sequence never opened, with
no sound call before (square with start held while it loads), starts
the sound system as SsVabTransfer would. The clean library starts it
from every sequence, SsUt and bank call. That SsSeqPause on sequence 0,
before any SsSeqOpen, is remembered: after SsSeqOpen and SsSeqPlay the
first tick that plays notes keys the sequence's voices off after them
(their key ons never reach the SPU), and the music goes on. The clean
library models a pause as a key off of the sequence's voices at its next
tick (the next advancing one if it plays), surviving SsSeqOpen. A pause
of a playing sequence was not observed. (Spec 10 has it, and when a
pause before SsSeqPlay acts this way.)

**Sound start-up timing.** Between the steps of the start-up sequence
the reference spends: 2,408 cycles after "1aa=0"; 2,412 after the 1a6
write; 7,242 after 1aa=0010; 9,630 after the key on (188/18a); 9,632
after the key off; about 2,300 after 1aa=c000 (it opens and enables an
event there); 2,474 before parking the voices; 2,698 after; about 9,800
after the start-up before the call's own writes (it calls ChangeClearPad
and ChangeClearRCnt twice there). The clean library waits as long.

**GPU and others, found on the way.**

- ResetGraph goes by its level's low three bits: 8, 16, 256, 65536 and -8 act as 0, 11
  and 259 as 3, and every other level as 1, GP1 02h and 01h with no line printed. That
  was read from the GPU's writes for 5 to 12 and up; 2, 4 and -1 only from the print and
  the return. Level 0 returns 3, whatever came before it, and every other returns 0
  (`resetret`, spec 37).
- ResetGraph(0) and (3) print a line before resetting the GPU:
  "ResetGraph:jtb=8004829c,env=800482e4\r\n", through the kernel's
  write (B0h function 35h) once per character, each call write(1, &c,
  1). The two numbers are addresses of the library's own tables, as
  %08x, so the line is always 38 characters. In this emulator each
  call takes about 4,320 cycles, some 164,000 for the line, and the
  earlier "wait of about 165,000 cycles" was this printing. Then the
  kernel's A0h function 49h is called (GP0 word 0004829Ch shows in the
  stream). ResetGraph(1) prints nothing. Every call prints: there is
  no first-call exception (probe `spec/probes/resetwait` on Sony's
  library, traced with the kernel calls). After mode 0's GP1 00h both
  wait 51,250 cycles before GP1 10000007h. Mode 3 then only writes GP1
  02h and 01h, as mode 1 does. The GPU's answer to GP1 10000007h is
  read from GPUREAD: left unread, it was the first word a later
  StoreImage read (clone).
- A program that brings its own library starts it in its first
  ResetGraph(0), after the line above and before A0h function 49h
  (Sony's library does the same work while the boot disc starts it).
  The kernel calls, in this order (reference, hello and invaders):
  B0h 19h HookEntryInt(the library's own handler record); B0h 5Bh
  ChangeClearPad(0); C0h 0Ah ChangeClearRCnt(3, 0); A0h 72h
  CdRemove(); B0h 09h CloseEvent(F1000000h), and the same for
  F1000001h to F1000004h; A0h A3h DequeueCdIntr(); C0h 03h
  SysDeqIntRP(0, 62E4h) and SysDeqIntRP(0, 62D4h). No other kernel
  call comes before main's first library call's GPU work. The clean
  library made B0h 19h, C0h 0Ah and C0h 02h before the line instead,
  and A0h 72h at the first CD use, which moves square's CD commands
  some 6,000 cycles later (its loading bar at swap 5).
- The CD system's start prints "addr=8004d724\r\n" the same way
  (15 write calls), the address of the library's own CD state as
  %08x. Sony's library does this in its initialization routine, before
  the program; the reference and the clean library do it in the
  library's start-up, the first ResetGraph (spec 07), after the "PS-X
  Control PAD Driver" line that comes from the kernel's own pad start
  (B0h function 12h) there. (Before the start-up was moved there, the
  reference printed it at the program's first CD call: invaders after
  FntOpen, at its first file read.)
- GsInitGraph in PAL mode sets GsDISPENV's screen y to 24, and the PAL
  vertical range starts at line 19: y0 = 19 + screen.y, 240 lines
  (Sony's library, probe `spec/probes/palscreen`: 43 to 283 for heights
  200, 240 and 256; NTSC 16 to 256). A program that calls GsInitGraph
  in NTSC and then SetVideoMode(PAL) gets 19 + its own screen.y (dino:
  46).
- GsSwapDispBuff writes GP1 08h when the display moved or the mode word
  changed (SetVideoMode since), not merely when the screen area changed
  (breakout changes the screen and gets 06h, 07h and no 08h).
- GsOFSGTE: the drawing environment's offset is left alone (0, 0) and
  each buffer change writes one environment, not two. The screen offset
  goes into the GTE (OFX, OFY, 16.16): the corner of the other buffer,
  the one drawn into after the next swap, plus the origin.
- GsClearOt clears the table by DMA channel 6 (OTC), as the reference
  does, then points the first entry at the shared end packet.
- The drawing DMA's completion raises no interrupt (the reference takes
  none): the queue moves on when the library next looks and from the
  interrupt chain.
- StoreImage writes GP1 04000000h before its C0h command.
- ClearImage (probe `spec/probes/clearimage` on Sony's library, 34
  rectangles) has two forms, chosen by the rectangle:
  - When x and w are both multiples of 64 and w is below 1024 (w = 0
    included): GP1 04000002h, then E6000000h and the 02h fill, with the
    color as 02BBGGRRh, then y<<16|x, then h<<16|w, h at most 511.
  - Otherwise a rectangle drawn with the drawing area opened: GP1
    10000003h, 10000004h and 10000005h (the drawing area's corners and
    offset are read back), GP1 04000002h, then E3000000h, E4FFFFFFh,
    E5000000h, E6000000h, the rectangle 60BBGGRRh, y<<16|x,
    h<<16|w (w at most 1023, h at most 511), and then E3h, E4h and E5h
    again, each with the value read back for it (after GsInitGraph at
    320 by 240: E3000000h, E403BD3Fh, E5000000h).
  - y and h play no part in the choice. Examples: 320 by 240 at (0, 0)
    and 512 by 256 at (512, 256) fill; 16 by 16 at (0, 0), 64 by 240
    at (16, 0), 1 by 1 at (836, 352) and 1024 by 512 draw a rectangle.
- GsScaleScreen scales the translation's elements as well as the rows.
- A sprite's texture page takes the color mode from attribute bits 24
  and 25 (clone's face: 030ch, not 020ch).
- FntPrint's ~c takes any three characters, each (c - '0')·16 kept to a
  byte: "~cF00" is red 60h.
- FntOpen takes about 19 cycles a character of n longer in the
  reference; the clean library waits as long.
- gteMIMefunc on the GTE (GPL) is fast enough for dino to keep its two
  frames a loop; the Go loop missed every sixteenth.

**Timing (what TestClean cannot see past).** Everything after the
program starts is aligned with the reference to within a few thousand
cycles over the CD loads and the sound start-up, but two things are not
the library's:

- The reference build's executable carries the decompiled library, so
  it is bigger (16 sectors for dino, 7 for tunnel): the BIOS loads it
  later, the program starts at another point of the frame, and the
  disc's audio tracks sit 6 sectors further on (cdda: the track change
  comes 2.7 million cycles later in the reference). build.sh pads
  the clean executable to the reference's size (PAD_TO_REF=0 turns it
  off): dino and
  cdda:4000 then match swap for swap. TestClean counts each run's
  frames from its first swap, so the boot time no longer changes the
  swap counts.
- The time from the program's start to main differs by about 130,000
  cycles (the start-up's clearing and the runtime's), so events after a
  load land in a frame chosen by that phase. tunnel, invaders and dino
  fail on it: one run writes one more idle tick (zero key on and key
  off words) before sound starts. Counting frames from the first swap
  does not fix this, since the phase is within a frame. It is left
  open until the start-up before main matches the reference's.

Sound memory: dino's reverb work area (0x79020 up) and cdda:4000's CD
capture buffers hold audio, which depends on the key ons and the CD
play landing on the same SPU sample (768 cycles) as the reference's;
they do not.

**Still open.**

- terra: from swap 62 a gouraud-lit color is one lower in green and red
  (a6 against a7): the light matrix's first column comes out 1 or 2
  lower than the reference's (-11046 against -11048, from GsGetLws's
  local-to-world matrix and GsSetLightMatrix with scaled bones). Found:
  the root's matrix comes from RotMatrix, whose rounding rule in spec
  04 was wrong for general angles. See "RotMatrix rounding" there.
- check: its "total time" readout (the lines VSync(1) counts) is one
  more than the reference's.
- CC 7, 10 and 11, tempo meta events, SsSetTempo, SsSeqSetNext,
  accelerando and ritardando, SsIsEos, SsPlayBack, SsSetMute and
  SsGetMute, SsGetSerialAttr and SsGetSerialVol, SsUtChangePitch,
  SsUtPitchBend, SsUtSetReverbDelay and Feedback: none of the examples
  calls them or (for the SEQ events) has them. Specified and done in
  spec 10.

## The header is read at every key on (final review, 2026-11-02)

Status: open. Found by the final timing review (spec 08): terra's last
key ons differ from Sony's and the reference's alike. Probe
`spec/probes/vabhdr`, on the voices probe's bank: SsVabTransfer, then
each of 32 cases changes bytes of the header in memory, keys program 0
tone 0 on at note 60, and prints the voice's registers after the tick
(volumes, pitch, start address, ADSR, envelope up). Sony's output is
`.build/vabhdr-sony/out.tty`, the clean library's
`.build/vabhdr-clean/out.tty`; compare the "case" lines with carriage
returns dropped (`tr -d '\r'`). Six cases differ.

terra does this by accident: `soundInit` unpacks its effect bank's
header at the start of `arcBuf`, opens the bank, and then loads the
music bank's body into the same buffer (`soundStd0VB` is read into
`arcBuf[:]`). Its effects then play with what Sony's library finds in
the header at key on (pitch 1, start address 4, ADSR 0225h dddch), while
the clean library plays real tones (pitch 80h, start 3612h, ADSR 0224h
2124h): four key ons at the end of TestClean's 300 swaps.

Rules (Sony's library; the clean agrees where said):

- The tone's row and the program's entry are read from the header at
  every SsUtKeyOn: a changed ADSR, center, shift, tone volume or pan,
  program volume or pan shows in the voice's registers at the next tick
  (cases 1 to 4, 8 to 10). The clean library does this.
- Which rows are the program's is the word SsVabTransfer wrote at bytes
  8 to 11 of the program's entry (its index among the programs with
  tones, see "VAB" above), read at key on: with program 0's word set to
  1 and row 16's center lowered, Sony's pitch follows row 16 (case 20,
  2000h); the clean library keeps the index it took at the transfer
  (1000h). A program's tone t is row word·16 + t, 32 bytes each from
  820h.
- The sample's start address is a 16-bit half SsVabTransfer wrote into
  the entries' bytes 12 to 15, taken in sequence (entry 0's two halves,
  then entry 1's, and so on): VAG v starts at half v - 1, read at key
  on. The tone's VAG 1 with the first half set to 0300h starts at 0300h
  (case 13), VAG 2 with the second half 0400h at 0400h (case 23), VAG 3
  at the third half, the bank's end 0280h (case 22); halves set to 0
  give a start address of 0 (cases 24, 25). The clean library uses a
  table of its own (0202h, 0254h unchanged).
- A tone whose VAG index is 0 is refused: SsUtKeyOn returns -1 (case
  21), so a header zeroed entirely (case 19), or zeroed with only the
  entries' halves kept (case 26), is refused too. Zeroed with the tone's
  VAG put back to 1 it keys on with everything else 0: volumes 0, pitch
  0, ADSR 0, the start address from the half (case 27). The clean
  library keys on with zeros in all four cases (and a start address of
  0).
- Not read at key on: the header's magic, program count and VAG count,
  the program's tone count (0 still keys on, case 11), the tone's min
  and max notes (note 60 plays below min 7fh and above max 10h, cases 6
  and 7), and the VAG size table (case 17).
- The sequencer reads the same way, with the differences below ("The
  sequencer and the header", probe `spec/probes/seqhdr`). SsUtKeyOff is
  there too.

Done when the probe's "case" lines match Sony's in all 32 cases, and
terra's four key ons at the end of TestClean's run match the
reference's (pitch 0000h or 0001h, start 0004h, ADSR 0225h dddch).

Done 2026-11-02. Two details terra needed, read from its overwritten
header against the reference's key ons (not probed on Sony's library
separately):

- The slot is the entry's byte 8 alone, the low byte of the word:
  terra's program 2 entry held 020D10F2h and the tone row used was
  F2h times 16.
- The tone's VAG index is a signed 16-bit value: terra's garbage tone
  held FEFFh (-257), and half -258 lies 2,020 bytes before the header,
  where the start address 0004h both libraries write is found. Read
  unsigned or as a byte it gives other addresses.
- The slot word SsVabTransfer writes is the program's running index
  among the programs with tones, which is "p below ps, ps after" (the
  VAB note above) for every bank probed, whose programs with tones come
  first.

Chosen, not probed: the sequencer refuses a VAG index of 0 as SsUtKeyOn
does (probed since: it does not, see "The sequencer and the header"
below); a pan above 127 (terra's tone pan 237, program pan 252) gives the
far side a factor of 0, not a negative one, which makes the reference's
zero volumes; a negative pitch step (a note far below the center) takes
the table's value at the step wrapped into it with the formula's own
shift, which is 0 at terra's depth but may not match Sony's for shallow
negative steps.

## The sequencer and the header (2026-11-03)

Probe `spec/probes/seqhdr`, on the voices probe's bank and the SEQs its
gen.go makes. Each "seq" case plays ONE (a program change to program 1
at tick 0 and a note on at 60 at tick 48, 120 beats a minute at 96 ticks
a quarter, so blank 16 after the start) or P0 (the same on program 0,
whose five tones all cover note 60), changes bytes of the header eight
frames after SsSeqPlay (after the program change has played, before the
note) and prints at frame 26 every voice whose registers changed since
the play, with its volumes, pitch, start address, ADSR and whether its
envelope is up. Program 1's entry is at 30h and its tone 0 is row 16
(a20h); the VAG address halves are entry 0's bytes 12 to 15 (0202h,
0254h) and entry 1's first (0280h, the end). Sony's output is in
`.build/seqhdr-sony/out` and `out.tty` and the reference's in
`.build/seqhdr-ref` (the harness's dump and its TTY, 8000 frames; the
tempo0 probes' outputs lie beside them the same way); the two agree in
every case
(Sony's run ends at the last case, see spec 10 "SsSetTempo").

Rules (Sony's library; the reference agrees in all):

- The header is read at the note on, not at SsSeqOpen, SsSeqPlay or the
  program change: a patch made eight frames into the play shows in the
  note's voice (cases 1 to 5: ADSR 0f0f 1f1f, center 48 gives pitch
  2000h, shift 64 gives 1078h, tone volume 20h and pan 0 give 0284h and
  0, VAG 2 gives start 0254h), and the same patch made at frame 22, after
  the note, does not (case 42, pitch 1000h). The tone volume and pan,
  the program volume and pan (cases 14, 15: 0284h both sides, 0 and
  27adh) and the ADSR are read from the rows and entries as SsUtKeyOn
  reads them, with the same results as the vabhdr probe's cases.
- Which rows are the program's is the entry's byte 8, read at the note
  on, as for SsUtKeyOn: program 1 with its slot set to 0 plays row 0
  (case 20: row 0's center 48 shows as pitch 2000h; case 19 with the
  slot alone changed is the base, rows 0 and 16 being alike); set to 2
  it plays nothing (case 21: rows 32 up are zero).
- The sample's start address comes from the halves as for SsUtKeyOn
  (VAG v at half v - 1): entry 0's halves set to 0300h 0400h give 0300h
  for VAG 1 (case 22), 0400h for VAG 2 (case 38), VAG 3 gives the end
  half 0280h (case 36), halves 0 give 0 (case 39), entry 1's halves
  changed change nothing (case 23). A VAG index past the end half (4)
  gives what the next half holds (case 37: ffffh, entry 1's untouched
  second half).
- Unlike SsUtKeyOn, the sequencer uses the tone's note range and the
  program's tone count: a note on plays every tone of the program, in
  row order, whose min <= note <= max (both inclusive: min 60 and max
  60 play note 60, min 61 or max 59 do not, cases 6 to 10), each in a
  voice of its own with its own row's values (P0 keys five voices, case
  44; with row 0's max 10h four, 45; with the program's tone count 2
  two, 46; with 0 none, 47; program 1 with tone count 2 and row 17
  copied from row 16 with center 48 plays two voices, pitches 1000h and
  2000h, case 18; with row 17 as it is, zero, one voice, case 17: row
  17's range 0 to 0 excludes the note). A tone count of 0 plays nothing
  (case 16).
- Unlike SsUtKeyOn, the header's program count is checked: the note on
  plays nothing when the program is not below it (counts 0 and 1 give
  nothing for program 1, cases 24 and 25; 3 plays, case 26). What is
  compared is not the program's number: Sony's STD0 bank (breakout,
  dogtale, invaders, square) has a program count of 113 for programs 0
  to 111 and 127, and program 127, the drums, plays (the examples'
  streams). The program's slot byte (entry byte 8, its index among the
  programs with tones) below the count fits the probe (slot 1 refused
  with counts 0 and 1, played with 2 and 3; slot 2 with count 2
  nothing, case 21) and the examples; the clean library does that
  (2026-11-03). Whether the byte or the whole slot word is compared is
  not probed. The VAG
  count (case 27), the VAG size table (28) and the magic (29) are not
  read. A header zeroed entirely plays nothing (30); zeroed but for the
  program count 2, entry 1's tone count 1 and slot 1, entry 0's halves
  and row 16's max 7fh and VAG 1, it plays with everything else 0:
  volumes 0, pitch 0, ADSR 0, start 0202h (case 41; without the program
  count put back, nothing, case 40).
- Unlike SsUtKeyOn, a tone whose VAG index is 0 is not refused. It plays
  with the start address of half 1 (0254h here, VAG 2's, case 31),
  whatever the halves hold: entry 0's halves set to 0300h 0400h give
  0400h (case 32), the slot word's high half or entry 1's halves changed
  change nothing (cases 33, 34), and the rest of the row is used (center
  48 gives 2000h, case 35). The reference does the same. The seqexpr
  probe (below) read the index on a bank of four VAGs: the sequencer
  takes the low byte of the half, and byte 0 reads half 1, VAG 2's.
- Tone priority and mode do not change the registers written at a note
  on from silence (cases 12, 13: priority 7fh, mode 4); mode bit 2 sets
  the voice's reverb bit at key on (the tick, above), not probed here.

The sequencer's expression (controller 11) and the header: this probe's
"expr" cases 50 to 57 read the header's program 1 volume byte and keyed
program 1 on with SsUtKeyOn after a sequence with expression 64, and found
127 and full volume (27adh); but each read came after seqCase had put the
saved header back, so they showed nothing. The seqexpr probe (below) reads
the header before putting it back: the expression is written into the
header's program volume byte, and SsUtKeyOn reads it there.

## Expression and the VAG index (probe seqexpr, 2026-11-03)

Probe `spec/probes/seqexpr`, on a bank its gen.go makes with mkvab
(PROBE3.VH and PROBE3.VB: programs 0 and 1 with one tone each on VAG 1,
program 2 with eight tones on VAG 1 whose centers 60, 72, 84, 96, 108,
120, 48 and 54 tell them apart by pitch, and four looping VAGs of 80, 144,
208 and 272 bytes, so the VAGs start at 0202h, 020ch, 021eh and 0238h and
end at 025ah) and SEQs at 120 beats a minute, 96 ticks a quarter (tick 48
is blank 16). Each case plays a sequence and logs, frame by frame, every
voice whose registers changed, printed after it; SsUtKeyOn cases print
the result and the voice's registers two frames later. Sony's output is
`.build/seqexpr-sony/out` and `out.tty`, the clean library's beside it in
`.build/seqexpr-clean`.

**Controller 11 (expression).** Sony's library writes the controller's
value into the header's volume byte of the channel's program (entry byte
1), and nothing else there: after EXPR (program change 0, expression 64 on
channel 0 at tick 0, notes at 48 and 144) the only header byte changed is
offset 33 (21h, program 0's volume), now 64 (case 17); after CH (64, 32,
16 in turn) it is 16 (case 32); after C7 (volume 64) nothing (case 31).
The byte is replaced, not scaled: with the byte set to 100 before the
play, expression 64 leaves 64 and the note's volumes are 0a12h (cases 29,
30), the value for a program volume of 64 (the volume formula above with
program volume 64, velocity 100: 27adh at 127). Everything else follows
from the header being read at every key on:

- The expression holds for the channel's next notes (EXPR's second note,
  0a12h, case 1), across the loop of a play with loops 2 (AFTER: a note
  at 48, expression 64 at 120, a note at 144; the second loop's first
  note is 0a12h, case 7), and for the other channels on the program (CH:
  channel 1's note at 48, with no controller of its own, is 0a12h; case
  10).
- SsUtKeyOn on the program sees it (0a12h while EXPR plays, 27adh on
  program 1, cases 2 and 16), and so does a later sequence on the program
  (PLAIN started while EXPR played, case 26: its note's volumes are
  0a12h, no change logged since the voice already held them).
- Nothing in the library undoes it: SsSeqPause, SsSeqReplay, SsSeqStop,
  SsUtAllKeyOff, the sequence's end, SsSeqClose and SsSeqOpen all leave
  SsUtKeyOn at 0a12h (cases 18 to 24); only writing the byte back (the
  probe's restore, SsVabTransfer) does. The probe's cases 4 to 9 (AFTER
  played again, after a reopen, and PLAIN after it, all starting at
  27adh) start at full volume because the header was put back after each
  case.
- A controller event re-works the volumes of the program's voices (spec
  10), a released voice included: AFTER's first voice, keyed off at tick
  96, is written 0a12h at tick 120 (frame 38, envelope 0; cases 4, 5).
- Controller 7 is the channel's own (velocity scaling, spec 10), not in
  the header, and multiplies with the expression in either order: C7
  alone gives 09ebh, C7A (expression 64 then volume 64) and C7B (the
  reverse) 0284h (cases 11 to 13).

**The VAG index.** The start address at a key on, for both the sequencer
and SsUtKeyOn, is half h of the header: bytes 12 to 15 of entry h/2, for
all 256 halves of the 128 entries, read at the key on. No count bounds it,
neither the program count nor the VAG count, at SsVabTransfer or now
(spec 36, probe `vagtable`). half 4 (entry 2's first, the end) and half 5
marked 0444h and 0555h are read (VAG 5 and 6, cases 53, 54, 61, 62).
SsUtKeyOn's VAG 257 and up, half 256 and up, give 0000h.

(Corrected 2026-11-07. This said that halves from 2p, p the program count at
SsVabTransfer, held ffffh whatever the header held. There is no such table.
The ffffh is what mkvab leaves in those bytes, and this probe's "far" marks
never reached halves 6 and 7: eight bytes written from entry 2's byte 12 run on
into entry 3's bytes 0 to 3. So VAG 7 and 8, 250, 252, 254 and 256 in cases 55
to 59 and 63 to 68 read an unmarked ffffh from the file, and the program count
in cases 69 to 72 had nothing to do with it. A bank from another tool holds
something else there: `pssoft-airbob-2-airbob`'s SE.VH has zeros, and one
program with 14 VAGs.)

- The sequencer takes the low byte of the tone's VAG half: 0100h, 0200h,
  8000h read half 1 like 0, 0101h, 7f01h, 8001h and 1001h half 0 like 1,
  0102h and 8002h half 1 like 2, 0103h half 2 (cases 41 to 51). Byte 0
  reads half 1 (VAG 2's address 020ch, cases 34, 35; entry 0's halves set
  to 0300h 0400h give 0400h, case 36); byte b from 1 reads half b - 1.
  Bytes fah to feh read their halves too, ffffh in this bank.
- SsUtKeyOn takes the half whole and signed: 0 is refused (-1, case 74);
  v from 1 reads half v - 1 (1, 2 and 5 give 0202h, 020ch, 025ah; 6, 7
  and 256 ffffh, what this bank's file holds there, cases 73 to 78, 82);
  -1, -2, 258, 7fffh, 8000h and
  8002h give 0000h (cases 79, 80, 83 to 86), past the table either way,
  whatever entry 0's bytes 4 to 11 hold (marked a4h to abh in these
  cases).
- A VAG byte of ffh (255, half 254) in the sequencer, in any high byte
  (ffffh, 00ffh and 7fffh alike, cases 38, 40, 47), and a half of exactly
  00ffh in SsUtKeyOn (case 81) make a noise voice: the voice plays the
  SPU's noise generator, with the volumes 3fffh and no pitch seen here in
  the sequencer and volumes 0 in SsUtKeyOn. See "Noise voices" below.

The clean library follows both rules since 2026-11-03: its TTY on this
probe differs from Sony's only in the tempo 0 case at the end, where
Sony's stops at a break and the clean library holds the sequence (spec 10,
"SsSetTempo"). Halves read before the bank (SsUtKeyOn's negative indices)
depend on what lies there, and are not chased. Its SPU writes, taken line by
line of the TTY, differ in the frames of ten more lines (the first and "vag"
51 to 59), each the same writes split differently: the probe's SsUtAllKeyOff
falls across a tick on Sony's, 18ch=0001h and the rest a tick later, and
inside a frame on the clean library, whose call is not paced to Sony's.

**SsUtKeyOff** (the same probe, "koff" cases 59 to 75; each prints a
marker line before it, so the dump's "tty" counts find its frames; the
result, and the voice's envelope one and three frames after). Sony's
library and the reference agree in every result and in every SPU write;
the envelope a frame after a key off reads 4000h in one and 8000h in the
other in three cases (twice, header zeroed, sequence voice): where in the
frame the probe reads it, not a library difference, as the voices probe's
"quiet after" counts. The clean library, on seqhdr's same koff cases
(2026-11-03), reads as Sony's in all but case 72's second reading (4000h
against 8000h), the same thing.

- SsUtKeyOff(voice, vab, program, tone, note) stages a key off of voice
  for the next tick (18c bit written there, nothing else: no register of
  the voice changes) and returns 0 when voice is the voice SsUtKeyOn
  gave for that vab, program, tone and note (case 59: keyed on in frame
  F, the call in F+2, 18c=0002h written by the tick at the end of F+2).
  Each argument is checked against what the voice was keyed on with:
  another note (60), program (61), tone (62) or bank id (63) gives -1
  and no key off; the right four with another voice number gives -1 and
  keys off nothing, neither the given voice nor the matching one (64,
  voice+1, a free voice); a voice number out of range (24, -1) gives -1
  (65, 66). A free voice never keyed on, with the right arguments, gives -1
  (69, voice 3).
- Matching is by the voice's own record: two voices playing the same
  tone and note, SsUtKeyOff of the second keys off the second alone (72:
  18c=0004h, the first still at 8000h).
- A second SsUtKeyOff of the same voice, in the same frame or a frame
  later, gives 0 again (67, 68); so does one of a voice keyed off and
  silent for 40 frames (74): the record outlives the key off (until the
  voice is taken again, by inference from the note off matching above).
  After SsUtAllKeyOff it gives -1 (73): SsUtAllKeyOff drops the
  records.
- SsUtKeyOff in the same frame as the voice's SsUtKeyOn, before the tick
  (70): the result is 0, the tick writes the voice's registers and the
  key off bit (18c=0002h) and no key on (188=0000h): the voice never
  sounds. The reverse (a key on after a pending key off) is in "Tick"
  above.
- The header is not read: a zeroed header gives 0 and the key off (71).
- A sequence's voice can be keyed off by SsUtKeyOff with the sequence's
  bank, program, tone 0 and note (75: ONE's note on program 1 at 60; the
  result is 0 and the key off bit is written at the next tick). The
  sequencer's voices carry the same record.

## Noise voices (probe seqnoise, 2026-11-03)

Probe `spec/probes/seqnoise`, on a bank its gen.go makes (NOISE.VH and
NOISE.VB: program 0 a sample tone, program 1 a noise tone, program 2 tones
0 noise and 1 sample, program 3 tones 0 sample and 1 noise, program 4 two
noise tones, centers 60 and 72; the VAG bytes are written into the header
before SsVabTransfer) and SEQs at 120 beats a minute, 96 ticks a quarter.
Each case prints a line "case N label" before it and "end N" after, with
the noise registers read back; the dump's "tty" counts match the SPU
writes to cases. Sony's output is `.build/seqnoise-sony/out` and
`out.tty`, the clean library's beside it in `.build/seqnoise-clean`.
Below, "at once" means inside the call, or for a sequence's event at the
point in the tick given under "Where the sequences run".

**Which tones.** A tone whose VAG index selects the noise generator plays
no sample.

- The sequencer: the low byte of the tone's VAG half is ffh, whatever the
  high byte (00ffh, 01ffh, 7fffh, 80ffh, feffh, ffffh; cases 149 to 159).
- SsUtKeyOn: the half is exactly 00ffh (case 150). 01ffh, 7fffh, 80ffh,
  feffh and ffffh key on an ordinary sample voice with the start address
  the half gives ("The VAG index" above; cases 152 to 160).
- Whether the VAG byte is in the header at SsVabTransfer or written later
  makes no difference (case 165). The header is read at the key on, as
  for any tone, and the voice remembers its kind: writing the byte back to
  a sample index while the noise holds, or to ffh while a sample holds,
  does not change what its key off does (cases 166, 167).
- The sequencer applies the tone's min and max to a noise tone as to any
  (notes 40 to 80 of a walk over 0 to 127, case 5); SsUtKeyOn ignores them
  (case 2).

**Allocation.** The voice comes from the normal allocator, with the tone's
priority, before anything is written: lowest free voice (cases 74, 75),
steals by the same rules (case 78). A refused key on writes nothing, and
SsUtKeyOn returns -1 (cases 79, 81). SsUtKeyOn otherwise returns the voice.

**The noise clock.** There is one for all voices, in SPUCNT (1AAh) bits 8
to 13. A noise key on sets it:

    clock = (note - center) & 3fh
    1AAh  = (1AAh as read from the SPU & c0ffh) | clock << 8

note is the key on's note as given (SsUtKeyOn's argument whole: with center
60, notes 128 and 256 give 04h, -12 gives 38h) and center the tone's center
byte. The tone's
shift and SsUtKeyOn's fine pitch do not enter. Checked on 128 notes for
each of center 60 shift 0 and center 72 shift 64, by SsUtKeyOn and by the
sequencer, and on 560 combinations of center, shift and note (cases 1 to
9). So note 60 center 60 writes c000h, note 72 cc00h, note 48 f400h, note
59 ff00h.

The write keeps every other bit as the register reads: reverb bit 7 (case
10), mute bit 14 (case 13), CD bits 0 and 2 (case 14), and a value the
probe wrote itself, c08fh giving cc8fh (case 15). Nothing puts the clock
back. Each later noise key on replaces it, and the library's other writes
to 1AAh keep it: SsUtReverbOn, SsUtReverbOff, SsSetMute, SsSetSerialAttr
and SsVabTransfer's DMA (cc20h, then cc00h; cases 10 to 14, 17, 168). A key
off leaves it (case 16).

**What a noise key on writes.** At once, in this order:

1. 1AAh, as above.
2. 194h: the voice's bit if the voice is below 16, else 0.
3. 196h: the voice's bit (voice - 16) if the voice is 16 or above, else 0.

Both are written whole, with the one bit, not added to what they hold and
not read: a second noise voice takes the first's bit away (cases 77, 127,
128), and bits the probe wrote are overwritten (case 137). So only the
newest noise voice is noise in the SPU. The tick after writes the voice's
registers as for any key on but for the pitch, in this order: 0, 2 (the
volumes below), 6 (the start address, half 254 by the rule above, ffffh in
this bank), 8, a (the tone's ADSR, cases 34, 57), then the key on bit in
188h or 18Ah. Register 4 is not written, so the pitch stays what it was
(10f3h from the voice's last note, case 75). The reverb bit in 198h or 19Ah
follows tone mode bit 2 as for any key on (cases 35, 58, 148).

**Volumes on a sequence's note on.** Per side, truncating at each step and
with no squaring:

    x = 3fffh·(tone vol·program vol)/16129
    x = x·(sequence volume)/127
    x = x·pan(tone)/63
    x = x·pan(program)/63
    x = x·pan(channel)/63
    register = x

The pan factors are the normal ones (left min(127 - pan, 63), right
min(pan, 63)), the channel's pan being controller 10's value, 64 at
SsSeqOpen. A program volume written by controller 11 enters, being the
header's byte (case 41). The formula fits all 78 values of the probe's grid
(cases 60 to 73 and 107); for example tone and program vol 100 gives
27adh, with both pans 96 and sequence volume 100, 50 it gives 0790h, 0f9eh
(case 65).

The velocity (case 37), controller 7 (case 39), the bank's master volume
and pan (cases 33, 66) and SsSetMVol (case 36, which writes 180h and 182h
alone) do not enter. Hence the 3fffh of a full tone at any velocity.

A controller 7, 10 or 11 while the note holds works the volumes out again
by the normal formula, the sample one, with the note's velocity and the
squaring: 3fffh becomes 09ebh for controller 7 at 64 with velocity 100,
then 09ebh, 0 for controller 10 at 0, then 0284h, 0 for controller 11 at 64
(case 42).

**Volumes on SsUtKeyOn.** 0 and 0, whatever the arguments, the tone or the
program (cases 44 to 56). SsUtGetVVol then gives 0 and 0. SsUtSetVVol works
as for any voice (50, 60 writes 1932h, 1e3ch and SsUtGetVVol gives 50, 60;
case 59). So a noise voice keyed by SsUtKeyOn is silent until SsUtSetVVol.

**Noise off.** A noise off is the two writes 194h=0, 196h=0, whole and at
once: it stops whichever voice's bit is there, not only the voice's own.
It comes in three ways.

- *SsUtKeyOff* of a noise voice, the arguments checked as usual (-1 and
  nothing written when they do not match, case 110): the two writes and 0.
  No key off bit is written, at this tick or later, for a voice keyed by
  SsUtKeyOn or a sequence (cases 109, 119). A second call writes them
  again and gives 0 (case 109). In the frame of the key on, the writes come
  before the tick and the tick still writes the registers and the key on
  bit (case 111), unlike a sample, whose key off cancels its key on.
- *A sequence's note off* (or a note on of velocity 0, case 43): for each
  voice the note off matches, by the rule in "Note off" above, a noise
  voice gets the two writes and no key off bit, and a sample voice its key
  off bit at the next tick (cases 116, 139). A noise voice stays matched,
  so three earlier notes on the same note and program give four pairs of
  writes at the fourth note off (case 37), and a program with two noise
  tones gives two pairs (case 141).
- *A steal or a freeing of the last noise voice.* The library remembers the
  last voice that had a noise key on, until that voice is keyed off or
  taken. When that voice is stolen by any key on, the two writes come first, before the new key on's
  own (sample thief, cases 80, 82, 96; noise thief, case 92). When it is
  freed by 16 zero reads of its envelope while keyed on (the allocation
  rule above), the two writes come in the tick of the 16th read, after its
  register writes (cases 84, 86, 87, 89), and a sample voice never writes
  them (cases 85, 88). The other noise voices do not count: stealing a held
  noise voice that is not the last, or its freeing, writes nothing (cases
  90, 94). Nor do voices already keyed off (cases 91, 93, 99, 100), nor a
  voice taken after SsUtAllKeyOff (cases 104, 106). A key off of another
  noise voice does write the two (it stops the last voice's noise) but the
  last voice stays remembered, so a later steal writes them again (case
  98). A steal forgets it: a second steal writes nothing (case 105).

After SsUtKeyOff or a note off, the voice is still keyed on in the SPU, its
envelope at sustain, playing what lies at its start address with its
volumes. The library counts it as keyed off: it is busy while a tick reads
its envelope above 0, and that does not fall by itself under a sustaining
ADSR, so the voice stays busy until it is stolen, SsUtAllKeyOff or the
sequence's end, 60 frames later included (cases 1, 4, 121). A walk of noise
notes takes a new voice for every note and wraps by stealing. SsUtAllKeyOff
frees it as any voice, and the next key on takes it (case 122).

What does not clear the noise registers: SsUtAllKeyOff (either argument;
the parking writes and 18ch=ffffh, 18eh=00ffh as for any voice; cases 114,
115, 120), SsSeqStop (the key off bit on two ticks, case 123), SsSeqPause
and SsSeqReplay (case 124), SsSeqClose (volumes 0 and the key off bit, case
125), the sequence's end (the key off bit for its voices, noise ones
included, cases 117, 118; the end comes at once after the last event, so a
note on that is the last event never sounds), SsVabClose (case 168), and a sample's key on or
key off on another voice (cases 112, 113, 132 to 136, 138). After these the
bit stays set with the voice released or free, read back as 0001h in those
cases. SsUtKeyOff and a note off then find no record after SsUtAllKeyOff (-1
and no writes, cases 102, 103), so the bit stays until the next noise key
on or noise off. A sample keyed on that voice meanwhile leaves 194h alone
(case 104), so the bit applies to the sample too; this emulator has no
noise generator to show the sound.

**Two voices, several tones.** Each noise tone of a note takes its own voice
and makes its own three writes, tones in order, so the last tone's clock
and bit stay (case 141, program 4: 1aah=c000h 194h=0001h, then f400h
0002h). A sample tone of the same note is untouched: its registers and key
on come at the next tick with the noise tone's (cases 139, 140, 142). At a
note off the noise tones write the pairs and the sample tones get key off
bits (case 139: 194h=0, then 18ch=0002h at the next tick). Two noise notes
at once have one clock, the newest's (cases 127, 128, 130); whichever is
keyed off first stops the noise (cases 127, 128, 130, 131). A noise voice
alongside a sample voice shows no other link (cases 132 to 135).

**Pitch.** A noise key on does not write the pitch, but later calls do, by
the normal rules: SsUtPitchBend (127, 0 and 64 on note 72 center 60 with
range 12: 4000h, 0ff1h, 2000h, each with result 0, case 144), SsUtChangePitch
(4000h, 1000h, case 145) and a sequence's bends (case 147). The clock does
not follow them. A bend before the note writes nothing at the note (case
146).

**Reverb.** SsUtReverbOn and SsUtReverbOff change bit 7 of 1AAh, keeping the
clock (cases 10, 11, 17). The voice's reverb bit is as above (case 148).

**Where the sequences run.** The sequences' noise writes show where the
sequence stage falls in Sony's tick. A note on at tick 48 has its voice's
registers written in dump frame f16 of the play, and its 1AAh, 194h and
196h at the end of frame f15's tick, right after that tick's register
writes and before the program runs again. A note off is the same (194h and
196h in f30, the key off bit of a sample in f31). So a tick is: read the
envelopes (as above), write the staged registers, then run the sequences,
whose registers wait for the next tick. A program's call made after the blank
then comes after those events:

- SsUtKeyOn right after blank 15, with a sequence's note on in the tick at
  that blank: the sequence's note takes voice 0, the call voice 1 (cases
  21 and 22, noise and sample alike).
- SsUtKeyOn after blank 14 and after blank 16 agree in both (cases 19, 20,
  23, 24).

The 16th zero read's noise off above is written at the same point, after
the tick's register writes.

The clean library does all of this since 2026-11-03: on this probe its
TTY and its SPU writes, frame by frame, differ from Sony's only in case 158,
SsUtKeyOn with VAG FEFFh, whose start address comes from memory before the
bank and depends on layout (case 157's 80FFh happens to agree). So does the
order in spu.go's tick: the envelope reads, the stops asked for, the voice
registers, the 16th zero read's noise off, the mask words, then the sequence
stage. The 16th read's two writes come before the mask words in the dump
("194=0000 196=0000 18c=0000 ..."), which places them more exactly than
"after the tick's register writes" above.

The reference library has noise voices too since 2026-11-03: its SPU writes
on this probe match Sony's but for the start addresses of those two layout
dependent SsUtKeyOn cases, 80FFh and FEFFh.
