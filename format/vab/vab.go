// Package vab reads and writes VAB sound banks and the definition files
// Sony's mkvab built them from.
//
// A bank is a header, the VH, which a program keeps in main memory, and a
// body, the VB, which it sends to the SPU's memory and may then drop:
//
//	VH  000h  the bank's header, 32 bytes: "pBAV" (VABp read little-
//	          endian), version, ID, the size of VH and VB together, then
//	          the counts of programs, tones and VAGs, master volume and pan
//	    020h  128 programs, 16 bytes each: tones, volume, priority, mode,
//	          pan, attributes
//	    820h  16 tones, 32 bytes each, for each program that has tones, in
//	          program order: priority, mode, volume, pan, centre note and
//	          fine tune, the notes it plays, vibrato and portamento, pitch
//	          bend range, the two ADSR words, its program and its VAG
//	    ...   256 16-bit sizes of the VAGs, each divided by 8, the first 0
//	VB        the VAGs' blocks, one after another, without their headers
//
// A program is an instrument and its tones the samples it plays across
// ranges of notes. A sequence names programs, and a sound effect is
// usually a program of one tone played with SsUtKeyOn.
package vab

import (
	"encoding/binary"
	"errors"
	"fmt"
)

// Magic is the first word of a VH, "VABp" as it reads little-endian.
const Magic = 0x56414270

// Sizes of the VH's parts.
const (
	HeaderSize  = 32
	ProgramSize = 16
	ToneSize    = 32
	MaxPrograms = 128
	MaxTones    = 16  // tones a program holds
	MaxVAGs     = 254 // VAGs a bank holds
	offsetsSize = 2 * 256
)

// A Program is an instrument.
type Program struct {
	Tones    uint8 // how many of its tones are used
	Volume   uint8
	Priority uint8
	Mode     uint8
	Pan      uint8
	Attr     uint16
}

// A Tone is a sample an instrument plays over a range of notes.
type Tone struct {
	Priority     uint8
	Mode         uint8 // 0, or 4 for reverb
	Volume       uint8
	Pan          uint8
	Center       uint8 // the note the sample sounds at its own rate
	Shift        uint8 // fine tuning of the centre note, in 128ths of a semitone
	Min, Max     uint8 // the notes the tone plays
	VibW, VibT   uint8
	PorW, PorT   uint8
	PBMin, PBMax uint8 // pitch bend range down and up, in semitones
	ADSR1, ADSR2 uint16
	Program      int16
	VAG          int16 // 1 for the first VAG
}

// A VAB is a sound bank.
type VAB struct {
	Version      uint32
	ID           uint32
	Volume, Pan  uint8
	Attr1, Attr2 uint8

	// Programs are all 128; a program with no tones has none in Tones.
	Programs [MaxPrograms]Program

	// Tones holds the tones of each program with Tones > 0, 16 to a
	// program, in program order, as the VH lays them out.
	Tones [][MaxTones]Tone

	// VAGs are the samples' blocks, without the VAG files' headers.
	VAGs [][]byte
}

// programsUsed returns the numbers of the programs that have tones.
func (v *VAB) programsUsed() []int {
	var out []int
	for i, p := range v.Programs {
		if p.Tones > 0 {
			out = append(out, i)
		}
	}
	return out
}

// tones returns how many tones are defined, those that name a VAG: what
// mkvab counted, which a program's own count may overstate.
func (v *VAB) tones() int {
	n := 0
	for _, ts := range v.Tones {
		for _, t := range ts {
			if t.VAG != 0 {
				n++
			}
		}
	}
	return n
}

// VH returns the bank's header.
func (v *VAB) VH() []byte {
	used := v.programsUsed()
	b := make([]byte, 0, HeaderSize+MaxPrograms*ProgramSize+len(used)*MaxTones*ToneSize+offsetsSize)
	le := binary.LittleEndian
	b = le.AppendUint32(b, Magic)
	b = le.AppendUint32(b, v.Version)
	b = le.AppendUint32(b, v.ID)
	b = le.AppendUint32(b, uint32(v.Size()))
	b = le.AppendUint16(b, 0)
	b = le.AppendUint16(b, uint16(len(used)))
	b = le.AppendUint16(b, uint16(v.tones()))
	b = le.AppendUint16(b, uint16(len(v.VAGs)))
	b = append(b, v.Volume, v.Pan, v.Attr1, v.Attr2)
	b = le.AppendUint32(b, 0)
	for _, p := range v.Programs {
		b = append(b, p.Tones, p.Volume, p.Priority, p.Mode, p.Pan, 0)
		b = le.AppendUint16(b, p.Attr)
		// Reserved, all ones as Sony's tools wrote them.
		b = le.AppendUint32(b, 0xffffffff)
		b = le.AppendUint32(b, 0xffffffff)
	}
	for _, ts := range v.Tones {
		for _, t := range ts {
			b = append(b, t.Priority, t.Mode, t.Volume, t.Pan, t.Center, t.Shift, t.Min, t.Max,
				t.VibW, t.VibT, t.PorW, t.PorT, t.PBMin, t.PBMax, 0, 0)
			b = le.AppendUint16(b, t.ADSR1)
			b = le.AppendUint16(b, t.ADSR2)
			b = le.AppendUint16(b, uint16(t.Program))
			b = le.AppendUint16(b, uint16(t.VAG))
			b = append(b, make([]byte, 8)...)
		}
	}
	b = le.AppendUint16(b, 0)
	for i := range 255 {
		size := 0
		if i < len(v.VAGs) {
			size = len(v.VAGs[i]) / 8
		}
		b = le.AppendUint16(b, uint16(size))
	}
	return b
}

// VB returns the bank's body.
func (v *VAB) VB() []byte {
	var b []byte
	for _, g := range v.VAGs {
		b = append(b, g...)
	}
	return b
}

// Size returns the size of the VH and the VB together, which the header
// records.
func (v *VAB) Size() int {
	n := HeaderSize + MaxPrograms*ProgramSize + len(v.programsUsed())*MaxTones*ToneSize + offsetsSize
	for _, g := range v.VAGs {
		n += len(g)
	}
	return n
}

// Bytes returns the bank as one file, the VH and then the VB, as mkvab
// wrote it before vabsplit.
func (v *VAB) Bytes() []byte { return append(v.VH(), v.VB()...) }

// Decode reads a bank from its VH and VB. vb may be nil, for a VH alone, or
// may be the rest of a whole VAB file.
func Decode(vh, vb []byte) (*VAB, error) {
	le := binary.LittleEndian
	if len(vh) < HeaderSize+MaxPrograms*ProgramSize || le.Uint32(vh) != Magic {
		return nil, errors.New("vab: not a VAB header")
	}
	v := &VAB{
		Version: le.Uint32(vh[4:]),
		ID:      le.Uint32(vh[8:]),
		Volume:  vh[24], Pan: vh[25], Attr1: vh[26], Attr2: vh[27],
	}
	ps := int(le.Uint16(vh[18:]))
	vs := int(le.Uint16(vh[22:]))
	for i := range v.Programs {
		p := vh[HeaderSize+i*ProgramSize:]
		v.Programs[i] = Program{Tones: p[0], Volume: p[1], Priority: p[2], Mode: p[3], Pan: p[4], Attr: le.Uint16(p[6:])}
	}
	off := HeaderSize + MaxPrograms*ProgramSize
	if len(vh) < off+ps*MaxTones*ToneSize+offsetsSize {
		return nil, fmt.Errorf("vab: header too short for %d programs' tones", ps)
	}
	v.Tones = make([][MaxTones]Tone, ps)
	for i := range ps {
		for j := range MaxTones {
			t := vh[off+(i*MaxTones+j)*ToneSize:]
			v.Tones[i][j] = Tone{
				Priority: t[0], Mode: t[1], Volume: t[2], Pan: t[3], Center: t[4], Shift: t[5], Min: t[6], Max: t[7],
				VibW: t[8], VibT: t[9], PorW: t[10], PorT: t[11], PBMin: t[12], PBMax: t[13],
				ADSR1: le.Uint16(t[16:]), ADSR2: le.Uint16(t[18:]),
				Program: int16(le.Uint16(t[20:])), VAG: int16(le.Uint16(t[22:])),
			}
		}
	}
	off += ps * MaxTones * ToneSize
	if vb != nil {
		pos := 0
		for i := 1; i <= vs; i++ {
			n := int(le.Uint16(vh[off+2*i:])) * 8
			if pos+n > len(vb) {
				return nil, fmt.Errorf("vab: VAG %d runs past the end of the body", i)
			}
			v.VAGs = append(v.VAGs, vb[pos:pos+n])
			pos += n
		}
	}
	return v, nil
}

// HeaderLen returns the size of the VH at the start of b, a whole VAB file
// or a VH alone, from the program count its header gives.
func HeaderLen(b []byte) (int, error) {
	if len(b) < HeaderSize || binary.LittleEndian.Uint32(b) != Magic {
		return 0, errors.New("vab: not a VAB header")
	}
	ps := int(binary.LittleEndian.Uint16(b[18:]))
	return HeaderSize + MaxPrograms*ProgramSize + ps*MaxTones*ToneSize + offsetsSize, nil
}
