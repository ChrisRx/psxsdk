// Package seq converts Standard MIDI Files into SEQ, the score format the
// PlayStation's sound library plays with SsSeqOpen and SsSeqPlay, as Sony's
// smf2seq did.
//
// A SEQ is a header and then one stream of MIDI events, the SMF's tracks
// merged by time:
//
//	000h  "pQES"
//	004h  version, 1 (32 bits, big-endian)
//	008h  ticks per quarter note (16 bits, big-endian)
//	00Ah  tempo, microseconds per quarter note (24 bits, big-endian)
//	00Dh  time signature: numerator, and denominator as a power of 2
//	00Fh  events, each a delta time as in an SMF and then the event, with
//	      running status; a tempo change is FF 51 and its three bytes,
//	      with no length, and the end is FF 2F 00
//
// smf2seq kept notes, program changes, pitch bend and pressure, the control
// changes the library acts on (bank select, data entry, volume, pan,
// expression, and the NRPNs the library uses for loops and reverb), and
// tempo changes, and dropped the rest. A note off became a note on of
// velocity 0, so that running status carries over more of them.
package seq

import (
	"encoding/binary"
	"errors"
	"fmt"
)

// Magic starts a SEQ file.
const Magic = "pQES"

// controls are the control changes smf2seq kept.
var controls = map[byte]bool{0: true, 6: true, 7: true, 10: true, 11: true, 98: true, 99: true}

// Options says what Convert keeps.
type Options struct {
	// NoBank drops bank select, as smf2seq -B did, for a file made for a
	// synthesizer whose banks the bank on the console does not have.
	NoBank bool
}

// Dropped counts what Convert left out: control changes by number and meta
// events by type, as smf2seq -V listed them.
type Dropped struct {
	Controls map[byte]int
	Metas    map[byte]int
	SysEx    int
}

type event struct {
	time  int
	track int
	data  []byte // status and data; a tempo change is FF 51 and its 3 bytes
	end   bool   // end of track
}

// Convert turns an SMF, format 0 or 1, into a SEQ.
func Convert(smf []byte, o Options) ([]byte, Dropped, error) {
	dropped := Dropped{Controls: map[byte]int{}, Metas: map[byte]int{}}
	if len(smf) < 14 || string(smf[:4]) != "MThd" {
		return nil, dropped, errors.New("seq: not a MIDI file")
	}
	hlen := int(binary.BigEndian.Uint32(smf[4:]))
	format := binary.BigEndian.Uint16(smf[8:])
	ntracks := int(binary.BigEndian.Uint16(smf[10:]))
	division := binary.BigEndian.Uint16(smf[12:])
	if format > 1 {
		return nil, dropped, fmt.Errorf("seq: MIDI format %d is not supported, only 0 and 1", format)
	}
	if division&0x8000 != 0 {
		return nil, dropped, errors.New("seq: SMPTE time division is not supported")
	}
	pos := 8 + hlen
	var tracks [][]event
	tempo := []byte{0x07, 0xa1, 0x20} // 120 beats a minute
	rhythm := []byte{0, 0}            // none given, as smf2seq wrote it
	tempoSet, rhythmSet := false, false
	for tr := 0; tr < ntracks && pos+8 <= len(smf); tr++ {
		if string(smf[pos:pos+4]) != "MTrk" {
			return nil, dropped, fmt.Errorf("seq: track %d does not start with MTrk", tr)
		}
		n := int(binary.BigEndian.Uint32(smf[pos+4:]))
		end := min(pos+8+n, len(smf))
		evs, err := readTrack(smf[pos+8:end], tr, o, &dropped)
		if err != nil {
			return nil, dropped, fmt.Errorf("seq: track %d: %w", tr, err)
		}
		// The first tempo and time signature at the start go in the header.
		kept := evs[:0]
		for _, e := range evs {
			switch {
			case e.time == 0 && !tempoSet && len(e.data) == 5 && e.data[1] == 0x51:
				copy(tempo, e.data[2:])
				tempoSet = true
			case len(e.data) == 6 && e.data[1] == 0x58:
				if e.time == 0 && !rhythmSet {
					copy(rhythm, e.data[2:4])
					rhythmSet = true
				}
			default:
				kept = append(kept, e)
			}
		}
		tracks = append(tracks, kept)
		pos += 8 + n
	}

	out := []byte(Magic)
	out = binary.BigEndian.AppendUint32(out, 1)
	out = binary.BigEndian.AppendUint16(out, division)
	out = append(out, tempo...)
	out = append(out, rhythm...)

	// Merge the tracks. Events at the same time are taken in passes over
	// the tracks in order, one from each track that has one then, so that
	// they alternate between tracks, as smf2seq's did.
	next := make([]int, len(tracks))
	var status byte
	now := 0
	for {
		t := -1
		for tr, evs := range tracks {
			if next[tr] < len(evs) && (t < 0 || evs[next[tr]].time < t) {
				t = evs[next[tr]].time
			}
		}
		if t < 0 {
			break
		}
		for more := true; more; {
			more = false
			for tr, evs := range tracks {
				if next[tr] >= len(evs) || evs[next[tr]].time != t {
					continue
				}
				e := evs[next[tr]]
				next[tr]++
				more = true
				if e.end {
					continue
				}
				out = appendVLQ(out, e.time-now)
				now = e.time
				if e.data[0] != status {
					out = append(out, e.data[0])
					status = e.data[0]
				}
				if status == 0xff {
					out = append(out, e.data[1])
					out = append(out, e.data[2:]...)
				} else {
					out = append(out, e.data[1:]...)
				}
			}
		}
	}
	// The end comes with the last event, wherever the tracks end.
	out = append(out, 0x00, 0xff, 0x2f, 0x00)
	return out, dropped, nil
}

// A Header is a SEQ's header.
type Header struct {
	Resolution int    // ticks per quarter note
	Tempo      int    // microseconds per quarter note
	Rhythm     [2]int // time signature: numerator, and denominator as a power of 2
}

// An Event is one event of a SEQ, at its time in ticks: a status byte and
// its data, a tempo change as FF 51 and its three bytes.
type Event struct {
	Time int
	Data []byte
}

// Decode reads a SEQ's header and its events, up to the end.
func Decode(b []byte) (Header, []Event, error) {
	if len(b) < 15 || string(b[:4]) != Magic {
		return Header{}, nil, errors.New("seq: not a SEQ file")
	}
	h := Header{
		Resolution: int(binary.BigEndian.Uint16(b[8:])),
		Tempo:      int(b[10])<<16 | int(b[11])<<8 | int(b[12]),
		Rhythm:     [2]int{int(b[13]), int(b[14])},
	}
	var evs []Event
	t := 0
	var status byte
	for i := 15; i < len(b); {
		d, n := readVLQ(b[i:])
		if n == 0 {
			return h, evs, fmt.Errorf("seq: delta time at %#x runs past the end", i)
		}
		i += n
		t += d
		if i >= len(b) {
			break
		}
		if b[i]&0x80 != 0 {
			status = b[i]
			i++
		}
		if status == 0 {
			return h, evs, fmt.Errorf("seq: data at %#x with no status", i)
		}
		size := 2
		switch {
		case status == 0xff:
			if i >= len(b) {
				return h, evs, errors.New("seq: meta event runs past the end")
			}
			switch b[i] {
			case 0x51:
				size = 4
			case 0x2f:
				return h, evs, nil
			default:
				size = 1 // what smf2seq wrote of any other meta event
			}
		case status>>4 == 0xc || status>>4 == 0xd:
			size = 1
		}
		if i+size > len(b) {
			return h, evs, errors.New("seq: event runs past the end")
		}
		evs = append(evs, Event{Time: t, Data: append([]byte{status}, b[i:i+size]...)})
		i += size
	}
	return h, evs, errors.New("seq: no end")
}

// readTrack reads a track's events, keeping what a SEQ keeps, with absolute
// times.
func readTrack(b []byte, tr int, o Options, dropped *Dropped) ([]event, error) {
	var out []event
	t := 0
	var status byte
	i := 0
	for i < len(b) {
		d, n := readVLQ(b[i:])
		if n == 0 {
			return nil, errors.New("delta time runs past the end")
		}
		i += n
		t += d
		if i >= len(b) {
			break
		}
		var st byte
		if b[i]&0x80 != 0 {
			st = b[i]
			i++
			if st < 0xf0 {
				status = st
			}
		} else if status == 0 {
			return nil, fmt.Errorf("data byte %#x with no status", b[i])
		} else {
			st = status // running status: the byte is data
		}
		switch {
		case st == 0xff:
			if i >= len(b) {
				return nil, errors.New("meta event runs past the end")
			}
			typ := b[i]
			l, n := readVLQ(b[i+1:])
			start := i + 1 + n
			if n == 0 || start+l > len(b) {
				return nil, errors.New("meta event runs past the end")
			}
			data := b[start : start+l]
			i = start + l
			switch typ {
			case 0x51:
				if l == 3 {
					out = append(out, event{time: t, track: tr, data: []byte{0xff, 0x51, data[0], data[1], data[2]}})
				}
			case 0x58:
				if l == 4 {
					out = append(out, event{time: t, track: tr, data: []byte{0xff, 0x58, data[0], data[1], data[2], data[3]}})
				}
			case 0x2f:
				out = append(out, event{time: t, track: tr, end: true})
				return out, nil
			default:
				dropped.Metas[typ]++
			}
		case st == 0xf0 || st == 0xf7:
			l, n := readVLQ(b[i:])
			if n == 0 || i+n+l > len(b) {
				return nil, errors.New("system exclusive message runs past the end")
			}
			i += n + l
			dropped.SysEx++
		default:
			size := 2
			if st>>4 == 0xc || st>>4 == 0xd {
				size = 1
			}
			if i+size > len(b) {
				return nil, errors.New("event runs past the end")
			}
			data := b[i : i+size]
			i += size
			e := []byte{st}
			e = append(e, data...)
			switch st >> 4 {
			case 0x8:
				e = []byte{0x90 | st&15, data[0], 0}
			case 0xb:
				if !controls[data[0]] || o.NoBank && data[0] == 0 {
					dropped.Controls[data[0]]++
					continue
				}
			}
			out = append(out, event{time: t, track: tr, data: e})
		}
	}
	out = append(out, event{time: t, track: tr, end: true})
	return out, nil
}

func readVLQ(b []byte) (int, int) {
	v := 0
	for i := 0; i < len(b) && i < 4; i++ {
		v = v<<7 | int(b[i]&0x7f)
		if b[i]&0x80 == 0 {
			return v, i + 1
		}
	}
	return 0, 0
}

func appendVLQ(b []byte, v int) []byte {
	var tmp [5]byte
	n := len(tmp) - 1
	tmp[n] = byte(v & 0x7f)
	for v >>= 7; v > 0; v >>= 7 {
		n--
		tmp[n] = byte(v&0x7f) | 0x80
	}
	return append(b, tmp[n:]...)
}
