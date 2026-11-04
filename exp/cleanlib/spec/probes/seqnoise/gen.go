//go:build ignore

// gen writes the probe's assets: two looping VAGs (wav2vag over raw
// samples), the bank NOISE.VH and NOISE.VB from them (mkvab over
// assets/NOISE.txt), and the SEQs (MIDI files converted with smf2seq). The
// probe patches the VAG byte of the noise tones in the header before
// SsVabTransfer, since mkvab only names VAGs it has. Run it from this
// directory: go run gen.go.
package main

import (
	"encoding/binary"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const res = 96

type event struct {
	tick int
	data []byte
}

// track is a format 0 MIDI file's events, in time order.
type track []event

func (t *track) add(tick int, data ...byte) { *t = append(*t, event{tick, data}) }

func (t *track) tempo(tick, us int) {
	t.add(tick, 0xff, 0x51, 3, byte(us>>16), byte(us>>8), byte(us))
}

func (t *track) on(tick, ch, n, v int) { t.add(tick, 0x90|byte(ch), byte(n), byte(v)) }

func (t *track) off(tick, ch, n int) { t.add(tick, 0x80|byte(ch), byte(n), 0) }

func (t *track) cc(tick, ch, c, v int) { t.add(tick, 0xb0|byte(ch), byte(c), byte(v)) }

func (t *track) pc(tick, ch, p int) { t.add(tick, 0xc0|byte(ch), byte(p)) }

func (t *track) bend(tick, ch, hi int) { t.add(tick, 0xe0|byte(ch), 0, byte(hi)) }

func vlq(n int) []byte {
	b := []byte{byte(n & 0x7f)}
	for n >>= 7; n > 0; n >>= 7 {
		b = append([]byte{byte(n&0x7f) | 0x80}, b...)
	}
	return b
}

func run(name string, args ...string) {
	cmd := exec.Command(name, args...)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		log.Fatal(name, " ", strings.Join(args, " "), ": ", err)
	}
}

func (t track) write(name string, end int) {
	var body []byte
	last := 0
	for _, e := range t {
		if e.tick < last {
			log.Fatalf("%s: event at %d after %d", name, e.tick, last)
		}
		body = append(body, vlq(e.tick-last)...)
		body = append(body, e.data...)
		last = e.tick
	}
	body = append(body, vlq(end-last)...)
	body = append(body, 0xff, 0x2f, 0)
	f := []byte("MThd\x00\x00\x00\x06\x00\x00\x00\x01")
	f = binary.BigEndian.AppendUint16(f, res)
	f = append(f, "MTrk"...)
	f = binary.BigEndian.AppendUint32(f, uint32(len(body)))
	f = append(f, body...)
	mid := "assets/" + name + ".mid"
	if err := os.WriteFile(mid, f, 0o644); err != nil {
		log.Fatal(err)
	}
	run("go", "run", "go.chrisrx.dev/psx/tools/smf2seq", "-o", "assets/"+name+".SEQ", mid)
}

// tone is a VagAtr section: every tone loops VAG 1 with a fast attack and
// no decay, so a note holds until its key off.
func tone(p, t, center int) string {
	return fmt.Sprintf(`VagAtr %d %d
	prior = 64
	vol = 127
	pan = 64
	center = %d
	min = 0
	max = 127
	ar = 127
	dr = 0
	sr = 0
	rr = 31
	sl = 15
	prog = %d
	vag = 1

`, p, t, center, p)
}

// vags writes two VAGs of 4 and 8 blocks of a square wave, looped whole.
func vags() []string {
	dir, err := os.MkdirTemp("", "seqnoise")
	if err != nil {
		log.Fatal(err)
	}
	defer os.RemoveAll(dir)
	var names []string
	for i := 1; i <= 2; i++ {
		var raw []byte
		for s := range 28 * 4 * i {
			v := int16(8000)
			if s/14%2 == 1 {
				v = -8000
			}
			raw = binary.LittleEndian.AppendUint16(raw, uint16(v))
		}
		src := filepath.Join(dir, fmt.Sprintf("V%d.raw", i))
		if err := os.WriteFile(src, raw, 0o644); err != nil {
			log.Fatal(err)
		}
		out := fmt.Sprintf("assets/V%d.VAG", i)
		run("go", "run", "go.chrisrx.dev/psx/tools/wav2vag", "-R", "22050", "-L", "-o", out, src)
		names = append(names, out)
	}
	return names
}

func main() {
	// The bank: program 0 a sample tone; program 1 a tone the probe makes
	// noise; program 2 tones 0 (noise) and 1 (sample); program 3 tones 0
	// (sample) and 1 (noise); program 4 two noise tones, centers 60 and 72.
	// Centers name a tone in the registers when it is a sample.
	def := "VabHdr\n\tver = 0x7\n\tmvol = 127\n\tpan = 64\n\n"
	for p, tones := range []int{1, 1, 2, 2, 2} {
		def += fmt.Sprintf("ProgAtr %d\n\ttones = %d\n\tmvol = 127\n\tmpan = 64\n\n", p, tones)
	}
	def += tone(0, 0, 60)
	def += tone(1, 0, 60)
	def += tone(2, 0, 60) + tone(2, 1, 72)
	def += tone(3, 0, 60) + tone(3, 1, 72)
	def += tone(4, 0, 60) + tone(4, 1, 72)
	if err := os.WriteFile("assets/NOISE.txt", []byte(def), 0o644); err != nil {
		log.Fatal(err)
	}
	args := append([]string{"run", "go.chrisrx.dev/psx/tools/mkvab", "-f", "assets/NOISE.txt", "--split", "-o", "assets/NOISE"}, vags()...)
	run("go", args...)

	// Timing: 120 beats a minute at 96 ticks a quarter, so tick 48 is
	// blank 16 and tick 96 blank 31.
	seq := func(name string, end int, f func(t *track)) {
		var t track
		t.tempo(0, 500000)
		f(&t)
		t.write(name, end)
	}

	// One note on each of programs 0 to 4, 60 at velocity 100, with a
	// program change long after it: the end of the track comes at once
	// after the event before it, and keys the sequence's voices off.
	for p := range 5 {
		seq(fmt.Sprintf("NOTE%d", p), 320, func(t *track) {
			t.pc(0, 0, p)
			t.on(48, 0, 60, 100)
			t.off(96, 0, 60)
			t.pc(300, 0, p)
		})
	}
	// The same on the noise program with the note off the last event.
	seq("ENDN", 120, func(t *track) {
		t.pc(0, 0, 1)
		t.on(48, 0, 60, 100)
		t.off(96, 0, 60)
	})
	// A note on at once, for the volume grid.
	seq("FAST", 320, func(t *track) {
		t.pc(0, 0, 1)
		t.on(3, 0, 60, 100)
		t.pc(300, 0, 1)
	})
	// Controller 10 before the note, for the pan.
	for _, pan := range []int{0, 32, 63, 64, 65, 96, 127} {
		seq(fmt.Sprintf("PAN%d", pan), 320, func(t *track) {
			t.pc(0, 0, 1)
			t.cc(0, 0, 10, pan)
			t.on(3, 0, 60, 100)
			t.pc(300, 0, 1)
		})
	}
	// Velocities on the noise program.
	seq("VELS", 320, func(t *track) {
		t.pc(0, 0, 1)
		for i, v := range []int{127, 64, 1, 100} {
			t.on(48+48*i, 0, 60, v)
			t.off(72+48*i, 0, 60)
		}
		t.pc(300, 0, 1)
	})
	// A note on of velocity 0 in place of the note off.
	seq("VEL0", 320, func(t *track) {
		t.pc(0, 0, 1)
		t.on(48, 0, 60, 100)
		t.on(96, 0, 60, 0)
		t.pc(300, 0, 1)
	})
	// A noise note held when the track ends, a sample note the last event.
	seq("HOLD", 120, func(t *track) {
		t.pc(0, 0, 1)
		t.pc(0, 1, 0)
		t.on(48, 0, 60, 100)
		t.on(96, 1, 60, 100)
	})
	// A noise note held, long enough to stop by hand.
	seq("LONG", 960, func(t *track) {
		t.pc(0, 0, 1)
		t.on(48, 0, 60, 100)
		t.pc(900, 0, 1)
	})
	// Every note from 0 to 127 on the noise program.
	seq("WALK", 16*129, func(t *track) {
		t.pc(0, 0, 1)
		for n := range 128 {
			t.on(16*(n+1), 0, n, 100)
			t.off(16*(n+1)+12, 0, n)
		}
	})
	// Controllers and a bend before the note.
	for _, c := range []struct {
		name string
		f    func(t *track)
	}{
		{"CC7", func(t *track) { t.cc(0, 0, 7, 64) }},
		{"CC10", func(t *track) { t.cc(0, 0, 10, 0) }},
		{"CC11", func(t *track) { t.cc(0, 0, 11, 64) }},
		{"PBB", func(t *track) { t.bend(0, 0, 127) }},
	} {
		seq(c.name, 120, func(t *track) {
			t.pc(0, 0, 1)
			c.f(t)
			t.on(48, 0, 60, 100)
			t.off(96, 0, 60)
		})
	}
	// Controllers while the note holds, one 12 ticks after the other.
	seq("CCD", 320, func(t *track) {
		t.pc(0, 0, 1)
		t.on(24, 0, 60, 100)
		t.cc(48, 0, 7, 64)
		t.cc(60, 0, 10, 0)
		t.cc(72, 0, 11, 64)
		t.off(120, 0, 60)
		t.pc(300, 0, 1)
	})
	// Bends while the note holds.
	seq("PBD", 320, func(t *track) {
		t.pc(0, 0, 1)
		t.on(24, 0, 72, 100)
		t.bend(48, 0, 127)
		t.bend(72, 0, 0)
		t.bend(96, 0, 64)
		t.off(120, 0, 72)
		t.pc(300, 0, 1)
	})
	// Two noise notes, the first ending first, then the last first.
	seq("TWOA", 320, func(t *track) {
		t.pc(0, 0, 1)
		t.pc(0, 1, 1)
		t.on(48, 0, 60, 100)
		t.on(72, 1, 72, 100)
		t.off(120, 0, 60)
		t.off(168, 1, 72)
		t.pc(300, 0, 1)
	})
	seq("TWOB", 320, func(t *track) {
		t.pc(0, 0, 1)
		t.pc(0, 1, 1)
		t.on(48, 0, 60, 100)
		t.on(72, 1, 72, 100)
		t.off(120, 1, 72)
		t.off(168, 0, 60)
		t.pc(300, 0, 1)
	})
	// A noise note on channel 0 and a sample note on channel 1, the sample
	// keyed off first in MIXA, the noise in MIXB; and the sample keyed on
	// first in MIXC.
	seq("MIXA", 320, func(t *track) {
		t.pc(0, 0, 1)
		t.pc(0, 1, 0)
		t.on(48, 0, 72, 100)
		t.on(72, 1, 60, 100)
		t.off(120, 1, 60)
		t.off(168, 0, 72)
		t.pc(300, 0, 1)
	})
	seq("MIXB", 320, func(t *track) {
		t.pc(0, 0, 1)
		t.pc(0, 1, 0)
		t.on(48, 0, 72, 100)
		t.on(72, 1, 60, 100)
		t.off(120, 0, 72)
		t.off(168, 1, 60)
		t.pc(300, 0, 1)
	})
	seq("MIXC", 320, func(t *track) {
		t.pc(0, 0, 1)
		t.pc(0, 1, 0)
		t.on(48, 1, 60, 100)
		t.on(72, 0, 72, 100)
		t.off(120, 1, 60)
		t.off(168, 0, 72)
		t.pc(300, 0, 1)
	})
}
