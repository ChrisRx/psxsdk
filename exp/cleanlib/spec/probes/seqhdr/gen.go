//go:build ignore

// gen writes the probe's MIDI files into assets/ and converts each to a
// SEQ with the repository's smf2seq, and copies the voices probe's bank
// (PROBE.VH and PROBE.VB) beside them. Run it from this directory:
// go run gen.go.
package main

import (
	"encoding/binary"
	"log"
	"os"
	"os/exec"
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
	cmd := exec.Command("go", "run", "go.chrisrx.dev/psx/tools/smf2seq", "-o", "assets/"+name+".SEQ", mid)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		log.Fatal(err)
	}
}

func main() {
	// ONE: one note on program 1 at tick 48 (120 beats a minute, 96 ticks
	// a quarter: blank 16), off at 96.
	{
		var t track
		t.tempo(0, 500000)
		t.pc(0, 0, 1)
		t.on(48, 0, 60, 100)
		t.off(96, 0, 60)
		t.write("ONE", 120)
	}
	// P0: the same on program 0, whose five tones all cover note 60.
	{
		var t track
		t.tempo(0, 500000)
		t.pc(0, 0, 0)
		t.on(48, 0, 60, 100)
		t.off(96, 0, 60)
		t.write("P0", 120)
	}
	// CC11: ONE with expression 64 on the channel at tick 0.
	{
		var t track
		t.tempo(0, 500000)
		t.pc(0, 0, 1)
		t.cc(0, 0, 11, 64)
		t.on(48, 0, 60, 100)
		t.off(96, 0, 60)
		t.write("CC11", 120)
	}
	// NOPC: ONE without the program change, on the channel's default
	// program.
	{
		var t track
		t.tempo(0, 500000)
		t.on(48, 0, 60, 100)
		t.off(96, 0, 60)
		t.write("NOPC", 120)
	}
	// TZ: a note, then a tempo meta event of 0 microseconds, then another
	// note.
	{
		var t track
		t.tempo(0, 500000)
		t.pc(0, 0, 1)
		t.on(0, 0, 60, 100)
		t.off(24, 0, 60)
		t.tempo(48, 0)
		t.on(96, 0, 62, 100)
		t.off(144, 0, 62)
		t.write("TZ", 168)
	}
	for _, name := range []string{"PROBE.VH", "PROBE.VB"} {
		b, err := os.ReadFile("../voices/assets/" + name)
		if err != nil {
			log.Fatal(err)
		}
		if err := os.WriteFile("assets/"+name, b, 0o644); err != nil {
			log.Fatal(err)
		}
	}
}
