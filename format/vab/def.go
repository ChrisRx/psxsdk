package vab

import (
	"bufio"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// An Envelope is a tone's ADSR as a definition file gives it, in Sony's
// rates, where a bigger number is faster:
//
//	AR  0-127   attack, from key on to full volume; 127 is instant
//	DR  0-15    decay, from full volume to the sustain level
//	SL  0-15    sustain level; 15 is full volume
//	SR  -127-127 sustain, the change while the key is held: negative
//	            falls, positive rises, 0 holds as nearly as the SPU can
//	RR  0-31    release, from key off to silence; 0 never ends
//
// Every curve is exponential, as mkvab made them, and the rates become the
// SPU's shifts by subtracting them from their maximum.
type Envelope struct {
	AR, DR, SL, SR, RR int
}

// Words returns the envelope as the SPU's two ADSR registers.
func (e Envelope) Words() (adsr1, adsr2 uint16) {
	clamp := func(v, hi int) uint16 { return uint16(max(min(v, hi), 0)) }
	adsr1 = 0x8000 | (127-clamp(e.AR, 127))<<8 | (15-clamp(e.DR, 15))<<4 | clamp(e.SL, 15)
	dir, rate := uint16(1), 127-clamp(-e.SR, 127)
	if e.SR > 0 {
		dir, rate = 0, 127-clamp(e.SR, 127)
	}
	adsr2 = 0x8000 | dir<<14 | rate<<6 | 0x20 | (31 - clamp(e.RR, 31))
	return adsr1, adsr2
}

// EnvelopeOf returns the envelope two ADSR registers hold, in a definition
// file's rates. The registers' linear modes, which mkvab never wrote, read
// as exponential.
func EnvelopeOf(adsr1, adsr2 uint16) Envelope {
	e := Envelope{
		AR: 127 - int(adsr1>>8&0x7f),
		DR: 15 - int(adsr1>>4&15),
		SL: int(adsr1 & 15),
		RR: 31 - int(adsr2&0x1f),
	}
	rate := 127 - int(adsr2>>6&0x7f)
	if adsr2&0x4000 != 0 {
		e.SR = -rate
	} else {
		e.SR = rate
	}
	return e
}

// ParseDef reads a definition file, as Sony's mkvab took it, into a bank
// with no VAGs yet. The file is sections of "label = value" lines:
//
//	VabHdr              the bank: ver, id, mvol, pan, attr1, attr2
//	ProgAtr P           program P: tones, mvol, mpan, prior, mode, attr
//	VagAtr P T          tone T of program P, also spelt ToneAtr: prior,
//	                    mode, vol, pan, center, shift, min, max, vibW,
//	                    vibT, porW, porT, pbmin, pbmax, the envelope as
//	                    ar, dr, sr, rr and sl, prog and vag
//	vsize               the VAGs' sizes, which are ignored
//
// form, fsize, ps, ts and vs, and the sizes, are worked out from what is
// there, as mkvab did. A negative value may be written with its sign after
// it, "64-", as Sony's manual had it. Anything after a '#' is a comment.
func ParseDef(text string) (*VAB, error) {
	v := &VAB{Version: 7, Volume: 127, Pan: 64}
	for i := range v.Programs {
		v.Programs[i].Mode = 0x14
	}
	type toneAt struct{ p, t int }
	tones := map[toneAt]*Tone{}
	envs := map[toneAt]*Envelope{}
	var section string
	var prog int
	var at toneAt
	sc := bufio.NewScanner(strings.NewReader(text))
	line := 0
	for sc.Scan() {
		line++
		s := sc.Text()
		if i := strings.IndexByte(s, '#'); i >= 0 {
			s = s[:i]
		}
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		fail := func(format string, args ...any) error {
			return fmt.Errorf("vab: definition line %d: %s", line, fmt.Sprintf(format, args...))
		}
		if !strings.Contains(s, "=") {
			f := strings.Fields(s)
			switch strings.ToLower(f[0]) {
			case "vabhdr":
				section = "hdr"
			case "progatr":
				if len(f) != 2 {
					return nil, fail("ProgAtr wants a program number")
				}
				p, err := strconv.Atoi(f[1])
				if err != nil || p < 0 || p >= MaxPrograms {
					return nil, fail("program %q is not 0-127", f[1])
				}
				section, prog = "prog", p
			case "vagatr", "toneatr":
				if len(f) != 3 {
					return nil, fail("%s wants a program and a tone number", f[0])
				}
				p, err1 := strconv.Atoi(f[1])
				t, err2 := strconv.Atoi(f[2])
				if err1 != nil || err2 != nil || p < 0 || p >= MaxPrograms || t < 0 || t >= MaxTones {
					return nil, fail("tone %s %s is not a program 0-127 and a tone 0-15", f[1], f[2])
				}
				section, at = "tone", toneAt{p, t}
				if tones[at] == nil {
					tones[at] = &Tone{Program: int16(p), Volume: 127, Pan: 64, Center: 60, Max: 127}
					envs[at] = &Envelope{AR: 127, SL: 15, RR: 20}
				}
			case "vsize":
				section = "vsize"
			default:
				if section != "vsize" {
					return nil, fail("%q is not a section", s)
				}
			}
			continue
		}
		key, val, _ := strings.Cut(s, "=")
		key = strings.ToLower(strings.TrimSpace(key))
		val = strings.TrimSpace(val)
		if key == "form" {
			continue
		}
		n, err := number(val)
		if err != nil {
			return nil, fail("%s: %v", key, err)
		}
		u8 := func(dst *uint8) { *dst = uint8(n) }
		switch section {
		case "hdr":
			switch key {
			case "ver":
				v.Version = uint32(n)
			case "id":
				v.ID = uint32(n)
			case "mvol":
				u8(&v.Volume)
			case "pan", "mpan":
				u8(&v.Pan)
			case "attr1":
				u8(&v.Attr1)
			case "attr2":
				u8(&v.Attr2)
			case "fsize", "ps", "ts", "vs":
			default:
				return nil, fail("VabHdr has no %q", key)
			}
		case "prog":
			p := &v.Programs[prog]
			switch key {
			case "tones":
				if n < 0 || n > MaxTones {
					return nil, fail("tones %d is not 0-16", n)
				}
				u8(&p.Tones)
			case "mvol", "vol":
				u8(&p.Volume)
			case "mpan", "pan":
				u8(&p.Pan)
			case "prior":
				u8(&p.Priority)
			case "mode":
				u8(&p.Mode)
			case "attr":
				p.Attr = uint16(n)
			default:
				return nil, fail("ProgAtr has no %q", key)
			}
		case "tone":
			t, e := tones[at], envs[at]
			switch key {
			case "prior":
				u8(&t.Priority)
			case "mode":
				u8(&t.Mode)
			case "vol":
				u8(&t.Volume)
			case "pan":
				u8(&t.Pan)
			case "center":
				u8(&t.Center)
			case "shift":
				u8(&t.Shift)
			case "min":
				u8(&t.Min)
			case "max":
				u8(&t.Max)
			case "vibw":
				u8(&t.VibW)
			case "vibt":
				u8(&t.VibT)
			case "porw":
				u8(&t.PorW)
			case "port":
				u8(&t.PorT)
			case "pbmin":
				u8(&t.PBMin)
			case "pbmax":
				u8(&t.PBMax)
			case "ar":
				e.AR = n
			case "dr":
				e.DR = n
			case "sr":
				e.SR = n
			case "rr":
				e.RR = n
			case "sl":
				e.SL = n
			case "prog":
				if n != at.p {
					return nil, fail("prog %d in the section for program %d", n, at.p)
				}
			case "vag":
				if n < 1 || n > MaxVAGs {
					return nil, fail("vag %d is not 1-254", n)
				}
				t.VAG = int16(n)
			default:
				return nil, fail("VagAtr has no %q", key)
			}
		case "vsize":
		default:
			return nil, fail("%q comes before any section", s)
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}

	// Lay the tones out by program, in the order the VH keeps them: a
	// block of 16 for each program with tones.
	for at := range tones {
		if v.Programs[at.p].Tones == 0 {
			return nil, fmt.Errorf("vab: tone %d %d is in program %d, which has no tones", at.p, at.t, at.p)
		}
	}
	rank := map[int]int{}
	for _, p := range v.programsUsed() {
		rank[p] = len(rank)
	}
	v.Tones = make([][MaxTones]Tone, len(rank))
	keys := make([]toneAt, 0, len(tones))
	for at := range tones {
		keys = append(keys, at)
	}
	sort.Slice(keys, func(i, j int) bool {
		return keys[i].p < keys[j].p || keys[i].p == keys[j].p && keys[i].t < keys[j].t
	})
	for _, at := range keys {
		t := tones[at]
		if t.VAG == 0 {
			return nil, fmt.Errorf("vab: tone %d %d names no vag", at.p, at.t)
		}
		t.ADSR1, t.ADSR2 = envs[at].Words()
		v.Tones[rank[at.p]][at.t] = *t
	}
	return v, nil
}

// number reads a definition's value: decimal, 0x hex, or a negative number
// with its sign before or after it.
func number(s string) (int, error) {
	neg := false
	if t, ok := strings.CutSuffix(s, "-"); ok {
		neg, s = true, t
	}
	n, err := strconv.ParseInt(s, 0, 32)
	if err != nil {
		return 0, fmt.Errorf("%q is not a number", s)
	}
	if neg {
		n = -n
	}
	return int(n), nil
}

// MaxVAG returns the highest VAG number a tone names, which is how many
// VAG files the bank needs.
func (v *VAB) MaxVAG() int {
	n := 0
	for _, ts := range v.Tones {
		for _, t := range ts {
			n = max(n, int(t.VAG))
		}
	}
	return n
}

// Def returns the bank as a definition file, which ParseDef reads back to
// the same bank: what mkvab -r printed. Its VAGs' sizes are listed under
// vsize for reference.
func (v *VAB) Def() string {
	var b strings.Builder
	fmt.Fprintf(&b, "VabHdr\n\tform = 'VABp'\n\tver = %#x\n\tid = %d\n\tfsize = %d\n\tps = %d\n\tts = %d\n\tvs = %d\n\tmvol = %d\n\tpan = %d\n",
		v.Version, v.ID, v.Size(), len(v.programsUsed()), v.tones(), len(v.VAGs), v.Volume, v.Pan)
	if v.Attr1 != 0 || v.Attr2 != 0 {
		fmt.Fprintf(&b, "\tattr1 = %d\n\tattr2 = %d\n", v.Attr1, v.Attr2)
	}
	for _, i := range v.programsUsed() {
		p := v.Programs[i]
		fmt.Fprintf(&b, "\nProgAtr %d\n\ttones = %d\n\tmvol = %d\n\tmpan = %d\n", i, p.Tones, p.Volume, p.Pan)
		if p.Priority != 0 {
			fmt.Fprintf(&b, "\tprior = %d\n", p.Priority)
		}
		if p.Mode != 0x14 {
			fmt.Fprintf(&b, "\tmode = %#x\n", p.Mode)
		}
		if p.Attr != 0 {
			fmt.Fprintf(&b, "\tattr = %d\n", p.Attr)
		}
	}
	used := v.programsUsed()
	for r, ts := range v.Tones {
		for j, t := range ts {
			if t.VAG == 0 {
				continue
			}
			p := used[r]
			e := EnvelopeOf(t.ADSR1, t.ADSR2)
			fmt.Fprintf(&b, "\nVagAtr %d %d\n", p, j)
			for _, kv := range []struct {
				k string
				v int
			}{
				{"prior", int(t.Priority)}, {"mode", int(t.Mode)}, {"vol", int(t.Volume)}, {"pan", int(t.Pan)},
				{"center", int(t.Center)}, {"shift", int(t.Shift)}, {"min", int(t.Min)}, {"max", int(t.Max)},
				{"vibW", int(t.VibW)}, {"vibT", int(t.VibT)}, {"porW", int(t.PorW)}, {"porT", int(t.PorT)},
				{"pbmin", int(t.PBMin)}, {"pbmax", int(t.PBMax)},
				{"ar", e.AR}, {"dr", e.DR}, {"sr", e.SR}, {"rr", e.RR}, {"sl", e.SL},
				{"prog", p}, {"vag", int(t.VAG)},
			} {
				fmt.Fprintf(&b, "\t%s = %d\n", kv.k, kv.v)
			}
		}
	}
	b.WriteString("\nvsize\n")
	for _, g := range v.VAGs {
		fmt.Fprintf(&b, "\t%d\n", len(g))
	}
	return b.String()
}
