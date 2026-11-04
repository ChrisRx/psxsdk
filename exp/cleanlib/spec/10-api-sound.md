# Milestone 9: the rest of the sound calls

Status: implemented. Both probes below give the same TTY and SPU writes
on the clean library as on Sony's, frame for frame.

The sound calls the clean library still lacks, and the sequence events
it does not play yet:

- Calls: SsGetMute, SsSetMute, SsGetMVol, SsGetSerialAttr,
  SsGetSerialVol, SsIsEos, SsPlayBack, SsSetTempo, SsSeqSetNext,
  SsSeqSetAccelerando, SsSeqSetRitardando, SsUtChangePitch,
  SsUtPitchBend, SsUtSetReverbDelay, SsUtSetReverbFeedback.
- SEQ events: controller 7 (volume), 10 (pan) and 11 (expression), and
  the tempo meta event, which spec 05 lists as not played.
- What SsSeqPause does to a playing sequence's voices, which spec 05
  leaves open ("A pause of a playing sequence was not observed").

None of the examples makes these calls, so the work is probes, not
TestClean.

## Sources

- Spec 05, which holds everything known about the sound system: the
  start-up, the tick, voice allocation, sequences, pitch bend, the
  reverb types and depth, and the SPU registers each call writes.
- `docs/manuals/sound.md` and `docs/manuals/tutorial-sound.md`, the
  Library Reference (`docs/manuals/030-1997-libraryref.pdf`), and the
  `yaroze` package's comments for each call's intent and signature.
- Black-box probes on Sony's library, in `exp/cleanlib/spec/probes/`,
  built as spec 09 says. The probe for voice allocation,
  `spec/probes/voices`, is the model: it drives the calls step by step,
  logs results in memory and prints them after each case. Its bank
  (PROBE.VH and PROBE.VB) is reusable. For SEQ files with the events
  above, write a MIDI file and convert it with the repo's smf2seq (see
  the asset converters). The SPU writes come from `.build/voiceprobe`
  (TestDump: a line of writes per frame, and the TTY). Compare the two
  libraries' writes frame by frame and the values the getters return.

## What to find for each

- The getters (SsGetMute, SsGetMVol, SsGetSerialAttr, SsGetSerialVol):
  what each returns after the setters and after start-up, and the
  layout of the structures they fill.
- SsSetMute: which registers it writes, and what playing voices and
  later key ons do while muted.
- SsIsEos: its result for a sequence playing, paused, stopped, ended,
  and never opened.
- SsPlayBack, SsSeqSetNext: what plays when, at which tick.
- SsSetTempo, SsSeqSetAccelerando, SsSeqSetRitardando and the tempo meta
  event: how the tick's step size changes, to the tick (spec 05 has the
  tick's arithmetic).
- SsUtChangePitch and SsUtPitchBend: the pitch register values, against
  spec 05's pitch rules.
- SsUtSetReverbDelay and SsUtSetReverbFeedback: the reverb registers
  written, for each reverb type.
- Controllers 7, 10 and 11: the voice volume registers, for voices
  playing and keyed after.
- SsSeqPause on a playing sequence: its voices' writes at the next
  ticks, and what SsSeqReplay does after.

## Done when

- Each probe gives the same TTY and SPU writes on the clean library as
  on Sony's, frame for frame.
- Every rule found is written here, under a heading for its call.
- `exp/cleanlib/regress.sh` gives no FAIL lines, and the same TIME lines
  as before: cdda, cdda:4000, check, dino, dungeon, flame, morph,
  offscr, square, terra, timers.

## Findings (milestone 9)

Two probes, built for Sony's library (`go run ./cmd/yaroze disc --opt=2
--elf DIR/prog.elf -o DIR/NAME.bin SRC`) and the clean one
(`exp/cleanlib/build.sh NAME-clean SRC`), dumped with `.build/voiceprobe`
(TestDump) and compared frame by frame from the frame the TTY holds the
probe's "start" line:

- `spec/probes/sndcalls`: the getters after start-up and after their
  setters, SsSetMute with voices playing, SsUtSetReverbDelay and
  SsUtSetReverbFeedback for every reverb type, and SsUtChangePitch and
  SsUtPitchBend on voices SsUtKeyOn started. A call a frame; results
  printed after each part (1,780 frames, all equal).
- `spec/probes/seqcalls`: 37 cases on sequences (tempo meta events,
  SsSetTempo, accelerando and ritardando over many spans, SsIsEos every
  frame, SsSeqSetNext, SsPlayBack, SsSeqPause and SsSeqReplay at every
  point of a sequence's life, controllers 7, 10 and 11, SsSetMute while
  a sequence plays). Everything is printed at the end: printing takes
  the two libraries different times, which moved the cases' frames
  (12,262 frames, all equal).

The SEQs are made by `spec/probes/seqcalls/gen.go` from MIDI it writes,
with smf2seq. The bank, PROBE2.VH, is the voices probe's with program 1's
tone given pbmin 2, pbmax 7 and shift 37 (VagAtr bytes 12, 13 and 5).
BEND, a held note with a pitch bend every tick (high byte 4 to 123 round
again), shows in each blank's pitch write the last tick played; that is
how the tempo rules below were read.

### Requests and the tick (SsSeqPause, SsSeqReplay, SsSetTempo, SsSeqStop)

Spec 05 first put a sequence's events in a tick before that tick's register
writes. The order is the other way (spec 05, "Where the sequences run"):
a tick writes its registers and then runs the sequences, whose writes wait
for the next tick. Below, T_c is the tick after frame c's code as that first
model had it, its events being those written at the blank after frame c,
which the sequences in fact ran at the end of the tick before. Only what a
program can read shows the difference, and the timing of requests, which
these rules state in those terms. The clean library has the real order since
2026-11-03: a request made in frame c is taken in by the sequence stage of the
tick after frame c, before its events, and what it stages comes out at the
tick after that; SsSeqStop's first key off goes in before the tick's mask
words.

- A request made in frame c (SsSeqPause, SsSeqReplay, SsSetTempo, an
  accelerando, a ritardando, SsSeqStop) does not change T_c: T_c plays
  the sequence on as before, and takes the request in at its end. Its
  effect starts at T_c+1.
- SsIsEos is worked out at the end of each tick for the tick after: a
  sequence that the next tick will end already reports 0 (and a
  sequence set to follow it, 1).
- SsSeqPlay and SsPlayBack are as spec 05: T_c only starts the
  sequence, T_c+1 plays its first events.

### SsIsEos

1 while the sequence plays, 0 otherwise; 0 for an id never opened, and
for a closed one. In detail:

- SsSeqPlay(SSPLAY_PLAY) sets it to 1 at once; SsSeqPlay(SSPLAY_PAUSE)
  to 0 at once. SsSeqStop sets 0 at once.
- SsSeqPause and SsSeqReplay change it at the end of the tick that takes
  them in (it reads as before right after the call).
- The end: 0 from the end of the tick before the one that plays the
  last events.
- SsPlayBack leaves it as it was (0 for a sequence not played before):
  a sequence it plays reads 0, until a replay after a pause, which
  sets 1.
- A sequence started by SsSeqSetNext reads 1 from the same point its
  predecessor reads 0.

### The end of a sequence

The end-of-track event comes at once after the event before it,
whatever its own delta (SHORT ends with a delta of 24 after its last
note off; the end, a loop or the next sequence come in the note off's
tick). After a loop the first events play in the same tick, and the
loop's time counts from the last note's tick. Spec 05 waited for the
delta.

### SsSeqPause and SsSeqReplay

Spec 05 left a pause of a playing sequence open; its model (keys off at
the next tick, without playing it) was one tick early.

- On a playing sequence: T_c still plays it; T_c+1 keys its voices off
  and does not advance; it stays paused.
- On a paused sequence: T_c+1 keys its voices off again.
- Right after SsSeqPlay, before any tick: T_c only starts it, as
  usual, and from T_c+1 it is paused (nothing plays). SsIsEos reads 1
  until the end of T_c.
- Before SsSeqPlay (the sequence open and idle, or not open at all,
  when SsSeqPlay comes before any tick): SsSeqPlay turns the pause into
  spec 05's: the first tick that plays keys the sequence's voices off
  after its events (their key ons never reach the SPU), and the
  sequence plays on. SsIsEos reads 0 from the end of the starting tick,
  although it plays.
- On a sequence open but idle, with a tick before SsSeqPlay: the tick
  drops the pause; the play is normal. On a sequence not open, ticks do
  not see it (only open sequences tick), so it waits for the open
  (square).
- SsSeqReplay on a paused sequence: it plays from T_c+1, advancing at
  once (no starting tick).
- SsSeqPause and SsSeqReplay in the same frame on a playing sequence:
  nothing happens (no key off, no tick lost).
- SsSeqPlay(SSPLAY_PAUSE) is SsSeqPlay and SsSeqPause together: T_c+1
  keys the voices off (after SsSeqStop, Sony's library keyed voice 0
  off again there).

### SsSeqStop

T_c still plays the sequence and keys its voices off after its events
(a note on in that tick sets the voice up but its key on is dropped);
T_c+1 keys them off again. Spec 05 had both key offs without T_c
playing.

### SsSeqClose

Volumes 0 and a key off, at the next tick, for the voices the sequence
played, as spec 05. The voices stay the sequence's: SsSeqClose on the
same id again, open or not, does it again (Sony's library wrote the
volumes again for a sequence closed twice). SsUtAllKeyOff in the same
frame does not drop those volume writes: the tick still makes them.

### SsPlayBack

Plays the sequence as SsSeqPlay(id, SSPLAY_PLAY, l_count) does, with the
same timing, but leaves SsIsEos as it was. The sequences already
playing are not stopped (the manual says the playing song is
interrupted; with Sony's library it plays on).

### SsSeqSetNext

When sequence a (set with SsSeqSetNext(a, b)) ends (its plays left at
1), in that tick: a's voices are keyed off as at any end, and b starts
from its beginning, to play once (whatever SsSeqPlay gave it before),
advancing at once, with no starting tick. If b's slot comes after a's,
b's first events play in the same tick; if before, in the next. A
looping a (SSPLAY_INFINITY) never ends, and SsSeqStop does not start b.

### Tempo, the rate

The rate (tenths of a tick a blank) is round(res·bpm·10/3600) (3000 on
PAL), as spec 05. Below a tenth of a tick a blank (res·bpm·10 < 3600)
the library takes the inverse, 3600·100/(res·bpm·10) rounded down: a
tempo that slow plays fast. With res 96, SsSetTempo(1) plays 375 tenths
a blank, (2) 187, (3) 125, and (4) the normal 1.

### Tempo meta events

bpm = round(60,000,000/µs). The new rate counts from the start of the
blank the event comes in: the blank's total moves by new rate − old
rate at once, so events after it in that blank are measured against the
new total. Events at the same tick as the tempo event (delta 0) play
with it even when the new total is below them. (A tempo change at tick
600, its total at 6017 with rate 64, to 35: the total becomes 5988, and
the bend at tick 600 after the tempo event still plays.)

### SsSetTempo

SsSetTempo(id, 0, tempo): the tempo in beats a minute, rate as above,
rounded. From T_c+1; the total is not moved. It is kept by SsSeqPlay (a
tempo set before playing holds).

A tempo of 0 stops Sony's library (probes `spec/probes/seqhdr`, its
last case, and `spec/probes/tempo0`, `tempo0b` and `tempo0c`, one each
since the first stop ends the run, 2026-11-03), in every form tried.
Each time the kernel's unresolved exception dump ("Unimplemented a0:40
syscall", cause 24h: a break) is printed at once and nothing runs after,
no tick included:

- SsSetTempo(id, 0, 0) on a playing sequence (seqhdr, called in frame 2
  of the play) and before SsSeqPlay (tempo0c): the frame of the call has
  no tick writes and the dump follows in it. The seqexpr probe prints a
  line before the call and one after it: the first comes, the dump
  follows it at once, the second never comes. The division by the tempo
  is in the call itself, not in the tick (the dump's registers hold 3600,
  the rate formula's divisor, and the same program counter each time).
- A tempo meta event of 0 microseconds in the SEQ (tempo0: TZ, a note at
  tick 0, the event at tick 48, a note at tick 96): the first note plays
  and is keyed off at its time; the tick that reaches the event writes
  nothing and the dump follows (its registers hold 60,000,000). The
  second note never comes.
- A SEQ header whose tempo bytes are 0 (tempo0b, the bytes set before
  SsSeqOpen): SsSeqOpen itself stops the library (the line the probe
  prints after it never comes; 60,000,000 in the registers again).

The reference library does not stop: the sequence never advances (the
note at tick 48 never comes in 150 frames; after the meta event the
second note never comes), its ticks go on, a tempo of 0 given before
SsSeqPlay holds across later plays as any tempo does (tempo0c's control
does not advance either), and SsSetTempo(id, 0, 120) makes it play
normally again (tempo0's last cases, which the reference reaches). A clean library cannot
reproduce a crash; not advancing is the reference's choice, not an
observation of Sony's library.

### SsSeqSetAccelerando and SsSeqSetRitardando

Both move the tempo from the tempo at the time (B0) to the one asked
for (B1) over time t, in integer beats a minute, the same way; the name
does not set the direction (an accelerando to a slower tempo slows
down). T_c+1 plays at the old tempo; from T_c+2, every tick that
advances the sequence:

- d = |B1 − B0|. If d ≥ t: the tempo moves d/t+1 (rounded down) towards
  B1 every tick. If d < t: it moves 1 every t/d (rounded down) ticks
  (at the (t/d)th tick, the 2(t/d)th and so on).
- It stops at B1 exactly.
- The rate is res·bpm·10/3600 rounded down (not rounded, as for
  SsSetTempo).
- d = 0 changes nothing. t = 0 stops Sony's library at a break (a
  division by zero); the clean library jumps to B1.

Checked: 120 to 240 over 120 and 60, to 180 over 120, to 150 over 30,
to 500 over 100, to 60 over 120 and 60, to 90 over 120, to 100 over 30,
to 30 over 100, to 120 (none), to 133 over 7, the wrong ways, over 1.

Not specified: an accelerando together with SsSetTempo or tempo meta
events. After SsSetTempo(100) during an accelerando to 240, Sony's
library dropped to 100, held it about eight frames and rose again, more
slowly than before; across tempo meta events it did not follow the rule
above either. The probe leaves them out.

### Controllers 7, 10 and 11

Spec 05 listed them as not played.

- Controller 7 is the channel's volume and 10 its pan, kept for each
  channel of the sequence (127 and 64 at SsSeqOpen).
- Controller 11 (expression) writes its value into the volume byte of the
  channel's program in the bank's header in memory (entry byte 1; the
  value as it is, not scaled by the byte's old value), and the header is
  read at every key on: it stays for every note on the program
  afterwards, on any channel, from any sequence, and from SsUtKeyOn, until
  something writes the byte again (another controller 11, the program's
  own code, SsVabTransfer). No call of the library resets it: SsSeqStop,
  SsSeqClose, SsSeqOpen, SsSeqPause, SsSeqReplay, SsUtAllKeyOff and the
  sequence's end all leave it (spec 05, "Expression and the VAG index",
  probe seqexpr, 2026-11-03; the seqhdr probe's finding that it was not
  in the header read the header after putting it back). Controller 7
  multiplies with it, in either order.
- A note on: the volume formula of spec 05 with vol = velocity·(channel
  volume)/127 (rounded down) and the key pan the channel's pan.
- Any of the three on channel ch works out again, for the next tick,
  the volumes of every voice of the sequence that plays ch's program
  (by program, as note offs match, whichever channel started it), with
  ch's volume and pan and the voice's own velocity. Other programs'
  voices are left alone; a program change on ch changes which voices
  ch reaches.
- SsSeqSetVol does not work out playing voices again; later note ons
  and controllers use it.
- Sequence volumes outside 0 to 127 act as 127 on either side: (200, 300),
  (-1, -1) and (128, 64) play as (127, 127), (127, 127) and (127, 64), for
  samples and noise voices alike. A left volume of 0 means a note on keys
  nothing: (0, 127) and (0, 0) write no registers and no key on, while
  (127, 0) plays with the right side 0 (probe seqnoise, cases 64, 107, 108).
- A note on of a noise tone (VAG byte ffh) has its own volume formula and
  registers: spec 05, "Noise voices".

### SsSetMute and SsGetMute

SsSetMute(SS_MUTE_ON) clears SPUCNT bit 14, (SS_MUTE_OFF) sets it, at
once, read-modify-write, written even when already so. Other modes do
nothing. The voices go on being written while muted, key ons included.
SsGetMute is 1 while bit 14 is clear.

### SsGetMVol

Each side is the main volume register read back: its 15 bits taken as
signed, over 129, rounded toward zero. So it returns what SsSetMVol was
given for 0 to 127, and the wrap for others: 200 gives −54, 128 −126,
255 0, −128 126, 1000 −16. After start-up 127.

### SsGetSerialVol and SsGetSerialAttr

SsGetSerialVol: each register, signed, over 258 (200 gives 127, −1
gives −1). 0 after start-up. SsGetSerialAttr: SPUCNT bit 0 for SS_MIX,
bit 2 for SS_REV, as 1 or 0. SsSetSerialAttr with a mode other than 0
is on. Attributes other than SS_MIX and SS_REV run into other code in
Sony's library (registers 182, 1B0 and 1AA written) and are not
specified.

### SsUtSetReverbDelay and SsUtSetReverbFeedback

Only for the echo and delay types (7 and 8); for the others nothing is
written. Written at once.

- Delay d: u = d·4096/127 rounded toward zero; 1D4 = 2u−1, 1D6 = u−1,
  1D8 = u+5, 1E0 = u+5, 1F4 = u+4, 1F6 = u+2, in that order (127 gives
  the presets' values). Values outside 0 to 127 follow the formula.
- Feedback f: 1CE = f·8100h/127 rounded toward zero.
- SsUtSetReverbType afterwards writes the preset as it is.

### SsUtChangePitch

Matches the voice by bank, program and the note SsUtKeyOn gave it (the
old fine pitch is not checked); −1 otherwise, or for a voice number out
of range. Then the pitch of new_note and new_fine on the voice's tone,
by spec 05's formula, no bend, at the next tick; 0. The voice keeps its
key-on note for later matches (a second change must give the key-on
note as old_note). It works on a voice keyed off. Negative fine pitches
gave values off the pitch table in Sony's library: not specified.

On a noise voice it writes the pitch register the same way and leaves the
noise clock (spec 05, "Noise voices").

### SsUtPitchBend

−1 without writing for a voice number out of range or another bank or
program. Otherwise the pitch at the next tick is the key-on note bent by
pbend as a sequence's bend (below), without the key-on fine pitch and
without any SsUtChangePitch: the bend starts from the key-on note.
The result is −1 if note is not the key-on note, but the pitch is
written anyway; after a successful SsUtChangePitch on the voice the
result is 0 whatever the note (in a run where the voice had been given
negative fine pitches before, Sony's library gave −1 again: not
specified).

On a noise voice it writes the pitch register the same way and leaves the
noise clock (spec 05, "Noise voices").

### Pitch bend (spec 05 corrected)

Spec 05's floor((b−64)·range·128/63) is close but not it. With d = b −
64, in 128ths of a semitone:

- d > 0: v = d·pbmax; bend = (v/63)·128 + (v mod 63)·2.
- d < 0: bend = 2·d·pbmin − 1 (pbmin 0 still gives −1).
- d = 0: 0.

The same for SEQ pitch bends and SsUtPitchBend (whose pbend may be
outside 0 to 127 and follows the formula). The octave shift of the
pitch wraps as the CPU's shift does: only its low five bits count
(SsUtPitchBend 8192 gave 8800h).
