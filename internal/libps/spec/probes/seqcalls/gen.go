//go:build ignore

// gen writes the probe's MIDI files into assets/ and converts each to a
// SEQ with the repository's smf2seq, and writes assets/PROBE2.VH, the
// voices probe's bank with program 1's tone given a pitch bend range
// (pbmin 2, pbmax 7) and a fine tune (shift 37). Run it from this
// directory: go run gen.go.
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

func (t *track) bend(tick, ch, b int) { t.add(tick, 0xe0|byte(ch), 0, byte(b)) }

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
	// BEND: one held note on program 1 and a pitch bend every tick, its
	// high byte 4 to 123 round again, so each blank's pitch write tells
	// the last tick played.
	bendSeq := func(tempos map[int]int) track {
		var t track
		t.tempo(0, 500000)
		t.pc(0, 0, 1)
		t.on(0, 0, 60, 100)
		for i := 1; i <= 1200; i++ {
			if us, ok := tempos[i]; ok {
				t.tempo(i, us)
			}
			t.bend(i, 0, 4+i%120)
		}
		t.off(1201, 0, 60)
		return t
	}
	bendSeq(nil).write("BEND", 1202)
	// TMETA: as BEND with tempo changes.
	bendSeq(map[int]int{
		200: 450000, 300: 470000, 400: 1000000, 500: 250000,
		600: 461538, 700: 500000, 800: 2000000, 900: 300000,
	}).write("TMETA", 1202)

	// CTL: controllers 7, 10 and 11 on channel 0, with a note on channel
	// 1 held throughout.
	{
		var t track
		t.tempo(0, 500000)
		t.pc(0, 0, 1)
		t.pc(0, 1, 1)
		t.on(0, 0, 60, 100)
		t.on(0, 1, 64, 100)
		t.cc(24, 0, 7, 64)
		t.on(48, 0, 62, 100)
		t.cc(72, 0, 7, 0)
		t.on(96, 0, 65, 100)
		t.cc(120, 0, 7, 127)
		t.cc(144, 0, 10, 0)
		t.on(168, 0, 67, 100)
		t.cc(192, 0, 10, 127)
		t.cc(216, 0, 10, 32)
		t.on(240, 0, 69, 100)
		t.cc(264, 0, 10, 64)
		t.cc(288, 0, 11, 64)
		t.on(312, 0, 71, 100)
		t.cc(336, 0, 11, 0)
		t.on(360, 0, 72, 100)
		t.cc(384, 0, 11, 127)
		t.cc(408, 0, 7, 100)
		t.cc(408, 0, 11, 50)
		t.cc(408, 0, 10, 90)
		t.on(432, 0, 74, 100)
		t.cc(456, 1, 7, 30)
		t.cc(468, 1, 10, 100)
		t.cc(476, 1, 11, 90)
		for _, n := range []int{60, 62, 65, 67, 69, 71, 72, 74} {
			t.off(480, 0, n)
		}
		t.off(480, 1, 64)
		t.write("CTL", 504)
	}

	// CTL2: which voices a controller reaches and whose settings it
	// uses: channels 0 and 1 on program 1, channel 2 on program 0, each
	// at its own velocity.
	{
		var t track
		t.tempo(0, 500000)
		t.pc(0, 0, 1)
		t.pc(0, 1, 1)
		t.pc(0, 2, 0)
		t.on(0, 0, 60, 100)
		t.on(0, 1, 64, 80)
		t.on(0, 2, 67, 60)
		t.cc(24, 1, 11, 64)
		t.cc(48, 0, 7, 100)
		t.cc(72, 2, 10, 20)
		t.cc(96, 2, 7, 90)
		t.on(120, 0, 62, 100)
		t.on(144, 2, 69, 60)
		t.cc(168, 1, 10, 100)
		t.on(192, 1, 66, 80)
		t.cc(216, 0, 11, 127)
		t.on(240, 1, 67, 80)
		t.pc(264, 1, 0)
		t.cc(264, 1, 7, 50)
		t.on(288, 1, 70, 80)
		t.cc(300, 2, 11, 100)
		for _, n := range []int{60, 62} {
			t.off(312, 0, n)
		}
		for _, n := range []int{64, 66, 67, 70} {
			t.off(312, 1, n)
		}
		for _, n := range []int{67, 69} {
			t.off(312, 2, n)
		}
		t.write("CTL2", 336)
	}

	// SHORT and SHORT2: two notes each, for the ends of sequences.
	for _, s := range []struct {
		name string
		n    int
	}{{"SHORT", 48}, {"SHORT2", 72}} {
		var t track
		t.tempo(0, 500000)
		t.pc(0, 0, 1)
		t.on(0, 0, s.n, 100)
		t.off(48, 0, s.n)
		t.on(48, 0, s.n+2, 100)
		t.off(96, 0, s.n+2)
		t.write(s.name, 120)
	}

	vh, err := os.ReadFile("../voices/assets/PROBE.VH")
	if err != nil {
		log.Fatal(err)
	}
	// Program 1's tone 0: 32 bytes a tone, 16 tones a program, after the
	// 32-byte header and 128 programs of 16 bytes.
	tone := vh[32+2048+16*32:]
	tone[5] = 37 // shift
	tone[12] = 2 // pbmin
	tone[13] = 7 // pbmax
	for _, dir := range []string{"assets", "../sndcalls/assets"} {
		if err := os.WriteFile(dir+"/PROBE2.VH", vh, 0o644); err != nil {
			log.Fatal(err)
		}
		vb, err := os.ReadFile("../voices/assets/PROBE.VB")
		if err != nil {
			log.Fatal(err)
		}
		if err := os.WriteFile(dir+"/PROBE.VB", vb, 0o644); err != nil {
			log.Fatal(err)
		}
	}
}
