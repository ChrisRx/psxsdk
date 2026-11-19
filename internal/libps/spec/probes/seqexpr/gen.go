//go:build ignore

// gen writes the probe's assets: four looping VAGs of distinct sizes
// (wav2vag over raw samples), the bank PROBE3.VH and PROBE3.VB from them
// (mkvab over assets/PROBE3.txt), and the SEQs (MIDI files converted with
// smf2seq). Run it from this directory: go run gen.go.
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

// track is a format 0 MIDI file's events, kept in the order added within
// a tick.
type track []event

func (t *track) add(tick int, data ...byte) { *t = append(*t, event{tick, data}) }

func (t *track) tempo(tick, us int) {
	t.add(tick, 0xff, 0x51, 3, byte(us>>16), byte(us>>8), byte(us))
}

func (t *track) on(tick, ch, n, v int) { t.add(tick, 0x90|byte(ch), byte(n), byte(v)) }

func (t *track) off(tick, ch, n int) { t.add(tick, 0x80|byte(ch), byte(n), 0) }

func (t *track) cc(tick, ch, c, v int) { t.add(tick, 0xb0|byte(ch), byte(c), byte(v)) }

func (t *track) pc(tick, ch, p int) { t.add(tick, 0xc0|byte(ch), byte(p)) }

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

// write sorts nothing: events must be added in time order.
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

// tone is a VagAtr section: every tone loops VAG 1 with a fast attack
// and no decay, so a note holds until its key off.
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

// vags writes the four VAGs: 4, 8, 12 and 16 blocks of 28 samples of a
// square wave, looped whole, so that each VAG's start address in the bank
// is its own.
func vags() []string {
	dir, err := os.MkdirTemp("", "seqexpr")
	if err != nil {
		log.Fatal(err)
	}
	defer os.RemoveAll(dir)
	var names []string
	for i := 1; i <= 4; i++ {
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
	// The bank: programs 0 and 1 with one tone each (center 60), program
	// 2 with eight tones whose centers tell them apart by pitch (60, 72,
	// 84, 96, 108, 120, 48, 54: pitches 1000h, 0800h, 0400h, 0200h, 0100h,
	// 0080h, 2000h, 16a0h). All tones name VAG 1; the probe patches the
	// index in memory.
	def := "VabHdr\n\tver = 0x7\n\tmvol = 127\n\tpan = 64\n\n"
	def += "ProgAtr 0\n\ttones = 1\n\tmvol = 127\n\tmpan = 64\n\n"
	def += "ProgAtr 1\n\ttones = 1\n\tmvol = 127\n\tmpan = 64\n\n"
	def += "ProgAtr 2\n\ttones = 8\n\tmvol = 127\n\tmpan = 64\n\n"
	def += tone(0, 0, 60)
	def += tone(1, 0, 60)
	for t, c := range []int{60, 72, 84, 96, 108, 120, 48, 54} {
		def += tone(2, t, c)
	}
	if err := os.WriteFile("assets/PROBE3.txt", []byte(def), 0o644); err != nil {
		log.Fatal(err)
	}
	args := append([]string{"run", "go.chrisrx.dev/psx/tools/mkvab", "-f", "assets/PROBE3.txt", "--split", "-o", "assets/PROBE3"}, vags()...)
	run("go", args...)

	// Timing: 120 beats a minute at 96 ticks a quarter, so tick 48 is
	// blank 16 and tick 96 blank 31.

	// EXPR: expression 64 on channel 0 before a note at 48, then a second
	// note at 144 with no controller between.
	{
		var t track
		t.tempo(0, 500000)
		t.pc(0, 0, 0)
		t.cc(0, 0, 11, 64)
		t.on(48, 0, 60, 100)
		t.off(96, 0, 60)
		t.on(144, 0, 62, 100)
		t.off(192, 0, 62)
		t.write("EXPR", 240)
	}
	// AFTER: a note at full expression, then expression 64 at 120, then a
	// second note. Played twice and looped, its first note tells whether
	// the expression outlives the play.
	{
		var t track
		t.tempo(0, 500000)
		t.pc(0, 0, 0)
		t.on(48, 0, 60, 100)
		t.off(96, 0, 60)
		t.cc(120, 0, 11, 64)
		t.on(144, 0, 62, 100)
		t.off(192, 0, 62)
		t.write("AFTER", 240)
	}
	// PLAIN: one note on channel 0, program 0, no controllers.
	{
		var t track
		t.tempo(0, 500000)
		t.pc(0, 0, 0)
		t.on(48, 0, 60, 100)
		t.off(96, 0, 60)
		t.write("PLAIN", 120)
	}
	// CH: channels 0 and 1 both on program 0; expression 64 on channel 0
	// before notes on both at 48 (60 on channel 0, 62 on channel 1); then
	// expression 32 on channel 0 at 96 and 16 on channel 1 at 144 while
	// both notes hold.
	{
		var t track
		t.tempo(0, 500000)
		t.pc(0, 0, 0)
		t.pc(0, 1, 0)
		t.cc(0, 0, 11, 64)
		t.on(48, 0, 60, 100)
		t.on(48, 1, 62, 100)
		t.cc(96, 0, 11, 32)
		t.cc(144, 1, 11, 16)
		t.off(192, 0, 60)
		t.off(192, 1, 62)
		t.write("CH", 216)
	}
	// C7: volume 64 alone; C7A: expression 64 then volume 64; C7B: volume
	// 64 then expression 64; each before a note at 48.
	for _, c := range []struct {
		name string
		ccs  [][2]int
	}{
		{"C7", [][2]int{{7, 64}}},
		{"C7A", [][2]int{{11, 64}, {7, 64}}},
		{"C7B", [][2]int{{7, 64}, {11, 64}}},
	} {
		var t track
		t.tempo(0, 500000)
		t.pc(0, 0, 0)
		for _, cc := range c.ccs {
			t.cc(0, 0, cc[0], cc[1])
		}
		t.on(48, 0, 60, 100)
		t.off(96, 0, 60)
		t.write(c.name, 120)
	}
	// P2: one note on program 2, whose eight tones all play it.
	{
		var t track
		t.tempo(0, 500000)
		t.pc(0, 0, 2)
		t.on(48, 0, 60, 100)
		t.off(96, 0, 60)
		t.write("P2", 120)
	}
}
