//go:build yaroze

// Command seqnoise is a probe for the spec: tones whose VAG byte is ffh,
// which play the SPU's noise generator. The bank gen.go makes
// (assets/NOISE.VH and NOISE.VB) has program 0 a sample tone, program 1 a
// noise tone, program 2 tones 0 (noise) and 1 (sample), program 3 tones 0
// (sample) and 1 (noise), program 4 two noise tones; the VAG bytes are
// patched in the header before SsVabTransfer.
//
// Each case prints a line "case N label", waits a blank, runs, prints the
// noise registers as read back and "end N", then the voices are quieted
// and the header put back before the next. The harness's dump prints the
// TTY offset of each frame, which matches SPU writes to cases.
package main

import (
	"runtime/volatile"
	"unsafe"

	"go.chrisrx.dev/psx/disc"
	"go.chrisrx.dev/psx/yaroze"
)

const vhSize = 5152

var (
	vhData   [(vhSize + 2047) / 2048 * 512]uint32
	vbData   [512]uint32
	note0    [512]uint32
	note1    [512]uint32
	note2    [512]uint32
	note3    [512]uint32
	note4    [512]uint32
	velsData [512]uint32
	vel0Data [512]uint32
	holdData [512]uint32
	longData [512]uint32
	walkData [512]uint32
	cc7Data  [512]uint32
	cc10Data [512]uint32
	cc11Data [512]uint32
	pbbData  [512]uint32
	ccdData  [512]uint32
	pbdData  [512]uint32
	twoAData [512]uint32
	twoBData [512]uint32
	mixAData [512]uint32
	mixBData [512]uint32
	mixCData [512]uint32
	endNData [512]uint32
	fastData [512]uint32
	pan0     [512]uint32
	pan32    [512]uint32
	pan63    [512]uint32
	pan64    [512]uint32
	pan65    [512]uint32
	pan96    [512]uint32
	pan127   [512]uint32

	vh = disc.Register("ASSETS/NOISE.VH", "assets/NOISE.VH").Into(vhData[:])
	vb = disc.Register("ASSETS/NOISE.VB", "assets/NOISE.VB").Into(vbData[:])
	_  = disc.Register("ASSETS/NOTE0.SEQ", "assets/NOTE0.SEQ").Into(note0[:])
	_  = disc.Register("ASSETS/NOTE1.SEQ", "assets/NOTE1.SEQ").Into(note1[:])
	_  = disc.Register("ASSETS/NOTE2.SEQ", "assets/NOTE2.SEQ").Into(note2[:])
	_  = disc.Register("ASSETS/NOTE3.SEQ", "assets/NOTE3.SEQ").Into(note3[:])
	_  = disc.Register("ASSETS/NOTE4.SEQ", "assets/NOTE4.SEQ").Into(note4[:])
	_  = disc.Register("ASSETS/VELS.SEQ", "assets/VELS.SEQ").Into(velsData[:])
	_  = disc.Register("ASSETS/VEL0.SEQ", "assets/VEL0.SEQ").Into(vel0Data[:])
	_  = disc.Register("ASSETS/HOLD.SEQ", "assets/HOLD.SEQ").Into(holdData[:])
	_  = disc.Register("ASSETS/LONG.SEQ", "assets/LONG.SEQ").Into(longData[:])
	_  = disc.Register("ASSETS/WALK.SEQ", "assets/WALK.SEQ").Into(walkData[:])
	_  = disc.Register("ASSETS/CC7.SEQ", "assets/CC7.SEQ").Into(cc7Data[:])
	_  = disc.Register("ASSETS/CC10.SEQ", "assets/CC10.SEQ").Into(cc10Data[:])
	_  = disc.Register("ASSETS/CC11.SEQ", "assets/CC11.SEQ").Into(cc11Data[:])
	_  = disc.Register("ASSETS/PBB.SEQ", "assets/PBB.SEQ").Into(pbbData[:])
	_  = disc.Register("ASSETS/CCD.SEQ", "assets/CCD.SEQ").Into(ccdData[:])
	_  = disc.Register("ASSETS/PBD.SEQ", "assets/PBD.SEQ").Into(pbdData[:])
	_  = disc.Register("ASSETS/TWOA.SEQ", "assets/TWOA.SEQ").Into(twoAData[:])
	_  = disc.Register("ASSETS/TWOB.SEQ", "assets/TWOB.SEQ").Into(twoBData[:])
	_  = disc.Register("ASSETS/MIXA.SEQ", "assets/MIXA.SEQ").Into(mixAData[:])
	_  = disc.Register("ASSETS/MIXB.SEQ", "assets/MIXB.SEQ").Into(mixBData[:])
	_  = disc.Register("ASSETS/MIXC.SEQ", "assets/MIXC.SEQ").Into(mixCData[:])
	_  = disc.Register("ASSETS/ENDN.SEQ", "assets/ENDN.SEQ").Into(endNData[:])
	_  = disc.Register("ASSETS/FAST.SEQ", "assets/FAST.SEQ").Into(fastData[:])
	_  = disc.Register("ASSETS/PAN0.SEQ", "assets/PAN0.SEQ").Into(pan0[:])
	_  = disc.Register("ASSETS/PAN32.SEQ", "assets/PAN32.SEQ").Into(pan32[:])
	_  = disc.Register("ASSETS/PAN63.SEQ", "assets/PAN63.SEQ").Into(pan63[:])
	_  = disc.Register("ASSETS/PAN64.SEQ", "assets/PAN64.SEQ").Into(pan64[:])
	_  = disc.Register("ASSETS/PAN65.SEQ", "assets/PAN65.SEQ").Into(pan65[:])
	_  = disc.Register("ASSETS/PAN96.SEQ", "assets/PAN96.SEQ").Into(pan96[:])
	_  = disc.Register("ASSETS/PAN127.SEQ", "assets/PAN127.SEQ").Into(pan127[:])

	// saved is the header as SsVabTransfer left it, put back after each
	// case.
	saved [vhSize]byte
	vab   int16

	// opened holds the case's sequences, closed after it.
	opened [4]int16
	nopen  int
	n      int
)

// Field offsets in a tone row; rows are 32 bytes from 820h, row 16·slot + t,
// and each program with tones here has the slot of its number.
const (
	fPrior  = 0
	fMode   = 1
	fVol    = 2
	fPan    = 3
	fCenter = 4
	fShift  = 5
	fMin    = 6
	fMax    = 7
	fPbMin  = 12
	fAdsr2  = 18
	fVag    = 22

	hdrMVol = 0x18

	regCtrl  = 0x1aa
	regNoise = 0x194
)

func hdr() []byte { return unsafe.Slice((*byte)(unsafe.Pointer(&vhData[0])), vhSize) }

func set(off int, b ...byte) {
	h := hdr()
	for i, v := range b {
		volatile.StoreUint8(&h[off+i], v)
	}
}

func tone(p, t, field int, b ...byte) { set(0x820+32*(16*p+t)+field, b...) }

// prog sets a byte of program p's entry: 1 the volume, 4 the pan.
func prog(p, off int, b byte) { set(0x20+16*p+off, b) }

func setVag(p, t int, v uint16) { tone(p, t, fVag, byte(v), byte(v>>8)) }

func restore() {
	h := hdr()
	for i := range saved {
		volatile.StoreUint8(&h[i], saved[i])
	}
}

func spu(off uintptr) *uint16 { return (*uint16)(yaroze.Pointer(0x1f801c00 + off)) }

func peek(off uintptr) uint16 { return volatile.LoadUint16(spu(off)) }

func poke(off uintptr, v uint16) { volatile.StoreUint16(spu(off), v) }

func env(v int16) uint16 { return peek(uintptr(v)*16 + 0xc) }

const hex = "0123456789abcdef"

func word(w uint16) string {
	var b [4]byte
	for i := range b {
		b[i] = hex[w>>(12-4*i)&15]
	}
	return string(b[:])
}

// state prints the noise registers as read back.
func state() {
	println("  1aa=", word(peek(regCtrl)), " 194=", word(peek(regNoise)), " 196=", word(peek(regNoise+2)))
}

// voice prints a voice's registers: volumes, pitch, start address, ADSR,
// envelope.
func voice(v int16) {
	if v < 0 || v >= 24 {
		return
	}
	print("  v", v)
	for _, off := range [...]uintptr{0, 2, 4, 6, 8, 0xa, 0xc} {
		print(" ", word(peek(uintptr(v)*16+off)))
	}
	println()
}

func wait(frames int) {
	for range frames {
		yaroze.VSync(0)
	}
}

func sounding() bool {
	for v := range int16(24) {
		if env(v) != 0 {
			return true
		}
	}
	return false
}

func quiet() {
	yaroze.SsUtAllKeyOff(0)
	for n := 0; sounding() && n < 60; n++ {
		yaroze.VSync(0)
	}
	wait(4)
}

func kase(label string, f func()) {
	n++
	println("case ", n, " ", label)
	yaroze.VSync(0)
	f()
	state()
	println("end ", n)
	yaroze.VSync(0)
	for _, s := range opened[:nopen] {
		yaroze.SsSeqStop(s)
		yaroze.SsSeqClose(s)
	}
	nopen = 0
	restore()
	quiet()
	poke(regNoise, 0)
	poke(regNoise+2, 0)
}

func open(d *[512]uint32) int16 {
	s := yaroze.SsSeqOpen(&d[0], vab)
	opened[nopen] = s
	nopen++
	return s
}

// play opens d, plays it once and runs frames blanks, calling at(f) before
// blank f.
func play(d *[512]uint32, frames int, at func(f int)) int16 {
	s := open(d)
	yaroze.SsSeqPlay(s, 1, 1)
	for f := range frames {
		if at != nil {
			at(f)
		}
		yaroze.VSync(0)
	}
	return s
}

// on keys tone t of program p on at note with volumes 100, printing the
// result.
func on(p, t, note int16) int16 { return onv(p, t, note, 0, 100, 100) }

func onv(p, t, note, fine, l, r int16) int16 {
	v := yaroze.SsUtKeyOn(vab, p, t, note, fine, l, r)
	println("  on p", p, " t", t, " n", note, " f", fine, " l", l, " r", r, ": ", v)
	return v
}

func off(v, p, t, note int16) {
	r := yaroze.SsUtKeyOff(v, vab, p, t, note)
	println("  off v", v, ": ", r)
}

// fill keys count voices on the sample program, notes from 30, and returns
// the last voice.
func fill(count int) int16 {
	last := int16(-1)
	for i := range count {
		last = yaroze.SsUtKeyOn(vab, 0, 0, 30+int16(i), 0, 100, 100)
	}
	println("  filled ", count, ", last ", last)
	return last
}

func walks() {
	utWalk := func() {
		for note := range int16(128) {
			v := yaroze.SsUtKeyOn(vab, 1, 0, note, 0, 100, 100)
			wait(2)
			println("  n", note, " r", v, " 1aa=", word(peek(regCtrl)))
			if v >= 0 && v < 24 {
				yaroze.SsUtKeyOff(v, vab, 1, 0, note)
			}
			wait(2)
		}
	}
	kase("walk SsUtKeyOn notes 0 to 127, center 60 shift 0", utWalk)
	kase("walk SsUtKeyOn notes 0 to 127, center 60 shift 0, min 40 max 80", func() {
		tone(1, 0, fMin, 40, 80)
		utWalk()
	})
	kase("walk SsUtKeyOn notes 0 to 127, center 72 shift 64", func() {
		tone(1, 0, fCenter, 72, 64)
		utWalk()
	})
	kase("walk sequence notes 0 to 127, center 60 shift 0", func() {
		play(&walkData, 660, nil)
	})
	kase("walk sequence notes 0 to 127, center 60 shift 0, min 40 max 80", func() {
		tone(1, 0, fMin, 40, 80)
		play(&walkData, 660, nil)
	})
	kase("walk sequence notes 0 to 127, center 72 shift 64", func() {
		tone(1, 0, fCenter, 72, 64)
		play(&walkData, 660, nil)
	})
	kase("grid center shift note: c s n f r 1aa", func() {
		for _, c := range [...]byte{0, 12, 48, 60, 61, 72, 96, 127, 128, 255} {
			for _, s := range [...]byte{0, 1, 63, 64, 127, 128, 255} {
				tone(1, 0, fCenter, c, s)
				for _, note := range [...]int16{0, 24, 48, 60, 72, 84, 96, 127} {
					v := yaroze.SsUtKeyOn(vab, 1, 0, note, 0, 100, 100)
					wait(1)
					println("  g ", c, " ", s, " ", note, " 0 ", v, " ", word(peek(regCtrl)))
					if v >= 0 && v < 24 {
						yaroze.SsUtKeyOff(v, vab, 1, 0, note)
					}
					wait(1)
				}
			}
		}
	})
	kase("grid fine pitch and out of range notes: n f r 1aa", func() {
		for _, note := range [...]int16{60, 72, 128, 200, 255, 256, 316, -1, -12} {
			for _, f := range [...]int16{0, 1, 64, 127, 128, -1} {
				v := yaroze.SsUtKeyOn(vab, 1, 0, note, f, 100, 100)
				wait(1)
				println("  h ", note, " ", f, " ", v, " ", word(peek(regCtrl)))
				if v >= 0 && v < 24 {
					yaroze.SsUtKeyOff(v, vab, 1, 0, note)
				}
				wait(1)
			}
		}
	})
	kase("clock sequence center 72 shift 0 note 60", func() {
		tone(1, 0, fCenter, 72, 0)
		play(&note1, 40, nil)
	})
}

// ctrlBits is the SPU control's other bits around a noise key on.
func ctrlBits() {
	yaroze.SsUtSetReverbType(1)
	yaroze.SsUtSetReverbDepth(40, 40)
	wait(2)
	kase("ctrl SsUtReverbOn, then noise note 72, off, SsUtReverbOff", func() {
		yaroze.SsUtReverbOn()
		state()
		wait(2)
		v := on(1, 0, 72)
		wait(4)
		state()
		off(v, 1, 0, 72)
		wait(4)
		state()
		yaroze.SsUtReverbOff()
		state()
	})
	kase("ctrl noise note 72, SsUtReverbOn, SsUtReverbOff, off", func() {
		v := on(1, 0, 72)
		wait(2)
		yaroze.SsUtReverbOn()
		state()
		wait(2)
		yaroze.SsUtReverbOff()
		state()
		wait(2)
		off(v, 1, 0, 72)
		wait(2)
	})
	kase("ctrl noise note 72, SsSetMute on then off, off", func() {
		v := on(1, 0, 72)
		wait(2)
		yaroze.SsSetMute(1)
		state()
		wait(2)
		yaroze.SsSetMute(0)
		state()
		wait(2)
		off(v, 1, 0, 72)
		wait(2)
	})
	kase("ctrl SsSetMute on, noise note 72, off, mute off", func() {
		yaroze.SsSetMute(1)
		wait(2)
		v := on(1, 0, 72)
		wait(2)
		state()
		off(v, 1, 0, 72)
		wait(2)
		yaroze.SsSetMute(0)
		wait(2)
	})
	kase("ctrl SsSetSerialAttr CD mix and reverb on, noise note 72, attrs off, off", func() {
		yaroze.SsSetSerialAttr(0, 1, 1)
		yaroze.SsSetSerialAttr(0, 0, 1)
		state()
		wait(2)
		v := on(1, 0, 72)
		wait(2)
		state()
		yaroze.SsSetSerialAttr(0, 1, 0)
		yaroze.SsSetSerialAttr(0, 0, 0)
		state()
		wait(2)
		off(v, 1, 0, 72)
		wait(2)
	})
	kase("ctrl poke 1aa=c08f, noise note 72, off, poke 1aa=c000", func() {
		poke(regCtrl, 0xc08f)
		wait(2)
		v := on(1, 0, 72)
		wait(2)
		state()
		off(v, 1, 0, 72)
		wait(2)
		state()
		poke(regCtrl, 0xc000)
		wait(2)
	})
	kase("ctrl noise note 72 then sample key on, clock after both off", func() {
		v := on(1, 0, 72)
		wait(2)
		off(v, 1, 0, 72)
		wait(2)
		s := on(0, 0, 60)
		wait(2)
		state()
		off(s, 0, 0, 60)
		wait(2)
	})
	kase("ctrl sequence with reverb on, noise note 60 center 72", func() {
		tone(1, 0, fCenter, 72)
		yaroze.SsUtReverbOn()
		wait(2)
		play(&note1, 40, nil)
		state()
		yaroze.SsUtReverbOff()
		wait(2)
	})
	kase("ctrl clock after the cases above", func() { wait(2) })
}

func volumes() {
	seqVol := func(label string, patch func()) {
		kase("seqvol "+label, func() {
			if patch != nil {
				patch()
			}
			play(&note1, 40, func(f int) {
				if f == 20 {
					voice(0)
				}
			})
		})
	}
	seqVol("base", nil)
	seqVol("tone vol 64", func() { tone(1, 0, fVol, 64) })
	seqVol("tone vol 0", func() { tone(1, 0, fVol, 0) })
	seqVol("tone pan 0", func() { tone(1, 0, fPan, 0) })
	seqVol("tone pan 127", func() { tone(1, 0, fPan, 127) })
	seqVol("program vol 64", func() { prog(1, 1, 64) })
	seqVol("program vol 0", func() { prog(1, 1, 0) })
	seqVol("program pan 0", func() { prog(1, 4, 0) })
	seqVol("bank vol 64 pan 0", func() { set(hdrMVol, 64, 0) })
	seqVol("priority 1 ADSR 0225 ddc0", func() {
		tone(1, 0, fPrior, 1)
		tone(1, 0, 16, 0x25, 0x02, 0xc0, 0xdd)
	})
	seqVol("tone mode 4 (reverb)", func() { tone(1, 0, fMode, 4) })
	seqVol("SsSetMVol 64 32", func() { yaroze.SsSetMVol(64, 32) })
	yaroze.SsSetMVol(127, 127)
	kase("seqvol velocities 127 64 1 100", func() {
		play(&velsData, 90, nil)
	})
	kase("seqvol SsSeqSetVol 64 32", func() {
		s := open(&note1)
		yaroze.SsSeqSetVol(s, 64, 32)
		yaroze.SsSeqPlay(s, 1, 1)
		wait(40)
	})
	kase("seqvol CC7 64 before the note", func() { play(&cc7Data, 40, nil) })
	kase("seqvol CC10 0 before the note", func() { play(&cc10Data, 40, nil) })
	kase("seqvol CC11 64 before the note", func() { play(&cc11Data, 40, nil) })
	kase("seqvol CC7 at 48, CC10 at 60, CC11 at 72 while the note holds (24 to 120)", func() {
		play(&ccdData, 50, nil)
	})
	kase("seqvol velocity 0 as the note off", func() { play(&vel0Data, 40, nil) })

	utVol := func(label string, patch func(), l, r int16) {
		kase("utvol "+label, func() {
			if patch != nil {
				patch()
			}
			v := onv(1, 0, 60, 0, l, r)
			wait(3)
			voice(v)
			off(v, 1, 0, 60)
			wait(3)
		})
	}
	for _, a := range [...][2]int16{{0, 0}, {127, 127}, {127, 0}, {0, 127}, {64, 32}, {100, 100}, {200, 300}, {-1, -1}} {
		utVol("arguments", nil, a[0], a[1])
	}
	utVol("tone vol 64", func() { tone(1, 0, fVol, 64) }, 127, 127)
	utVol("tone pan 0", func() { tone(1, 0, fPan, 0) }, 127, 127)
	utVol("program vol 64", func() { prog(1, 1, 64) }, 127, 127)
	utVol("program pan 0", func() { prog(1, 4, 0) }, 127, 127)
	utVol("bank vol 64 pan 0", func() { set(hdrMVol, 64, 0) }, 127, 127)
	utVol("ADSR 0225 ddc0", func() { tone(1, 0, 16, 0x25, 0x02, 0xc0, 0xdd) }, 127, 127)
	utVol("tone mode 4", func() { tone(1, 0, fMode, 4) }, 127, 127)
	kase("utvol SsUtGetVVol and SsUtSetVVol on a noise voice", func() {
		v := on(1, 0, 60)
		wait(2)
		var l, r int16
		g := yaroze.SsUtGetVVol(v, &l, &r)
		println("  get ", g, " ", l, " ", r)
		s := yaroze.SsUtSetVVol(v, 50, 60)
		println("  set ", s)
		wait(2)
		voice(v)
		g = yaroze.SsUtGetVVol(v, &l, &r)
		println("  get ", g, " ", l, " ", r)
		off(v, 1, 0, 60)
		wait(2)
	})
}

// grid plays s's note after patch, prints voice 0's volumes and quiets.
func grid(s int16, label string, a, b int, patch func()) {
	patch()
	yaroze.SsSeqPlay(s, 1, 1)
	wait(4)
	println("  vg ", label, " ", a, " ", b, " ", word(peek(0)), " ", word(peek(2)))
	yaroze.SsSeqStop(s)
	wait(3)
	restore()
	yaroze.SsUtAllKeyOff(0)
	wait(6)
}

func volumeGrid() {
	levels := [...]byte{127, 100, 64, 32, 1}
	pans := [...]byte{0, 1, 32, 63, 64, 65, 96, 126, 127}
	kase("vgrid tone vol by program vol: tv pv l r", func() {
		s := open(&fastData)
		for _, tv := range levels {
			for _, pv := range levels {
				grid(s, "vol", int(tv), int(pv), func() {
					tone(1, 0, fVol, tv)
					prog(1, 1, pv)
				})
			}
		}
	})
	kase("vgrid tone pan: tp 0 l r", func() {
		s := open(&fastData)
		for _, p := range pans {
			grid(s, "tpan", int(p), 0, func() { tone(1, 0, fPan, p) })
		}
	})
	kase("vgrid program pan: 0 pp l r", func() {
		s := open(&fastData)
		for _, p := range pans {
			grid(s, "ppan", 0, int(p), func() { prog(1, 4, p) })
		}
	})
	kase("vgrid tone pan by program pan: tp pp l r", func() {
		s := open(&fastData)
		for _, tp := range [...]byte{32, 64, 96} {
			for _, pp := range [...]byte{32, 64, 96} {
				grid(s, "both", int(tp), int(pp), func() {
					tone(1, 0, fPan, tp)
					prog(1, 4, pp)
				})
			}
		}
	})
	kase("vgrid sequence volumes: sl sr l r", func() {
		s := open(&fastData)
		for _, v := range [...][2]int16{{127, 127}, {100, 50}, {64, 32}, {1, 1}, {0, 127}, {200, 300}, {-1, -1}} {
			grid(s, "seq", int(v[0]), int(v[1]), func() { yaroze.SsSeqSetVol(s, v[0], v[1]) })
		}
		yaroze.SsSeqSetVol(s, 127, 127)
	})
	kase("vgrid tone vol 100, program vol 100, pans 96, sequence: sl sr l r", func() {
		s := open(&fastData)
		for _, v := range [...][2]int16{{127, 127}, {100, 50}, {64, 32}} {
			grid(s, "mix", int(v[0]), int(v[1]), func() {
				tone(1, 0, fVol, 100, 96)
				prog(1, 1, 100)
				prog(1, 4, 96)
				yaroze.SsSeqSetVol(s, v[0], v[1])
			})
		}
		yaroze.SsSeqSetVol(s, 127, 127)
	})
	kase("vgrid bank vol and pan with tone vol 64: mvol pan l r", func() {
		s := open(&fastData)
		grid(s, "bank", 64, 0, func() {
			tone(1, 0, fVol, 64)
			set(hdrMVol, 64, 0)
		})
		grid(s, "bank", 127, 127, func() {
			tone(1, 0, fVol, 64)
			set(hdrMVol, 127, 127)
		})
	})
	for _, c := range [...]struct {
		pan int
		d   *[512]uint32
	}{{0, &pan0}, {32, &pan32}, {63, &pan63}, {64, &pan64}, {65, &pan65}, {96, &pan96}, {127, &pan127}} {
		kase("vgrid controller 10: pan 0 l r", func() {
			s := open(c.d)
			grid(s, "cc10", c.pan, 0, func() {})
			println("  with tone vol 64 program pan 96")
			grid(s, "cc10", c.pan, 1, func() {
				tone(1, 0, fVol, 64)
				prog(1, 4, 96)
			})
		})
	}
}

func allocator() {
	kase("alloc sample on 0, noise, sample: voices", func() {
		a := on(0, 0, 60)
		b := on(1, 0, 60)
		c := on(0, 0, 62)
		wait(3)
		voice(a)
		voice(b)
		voice(c)
		off(b, 1, 0, 60)
		wait(3)
		off(a, 0, 0, 60)
		off(c, 0, 0, 62)
		wait(3)
	})
	kase("alloc 3 samples, middle off, noise takes it", func() {
		on(0, 0, 60)
		b := on(0, 0, 61)
		on(0, 0, 62)
		wait(2)
		off(b, 0, 0, 61)
		wait(40)
		v := on(1, 0, 60)
		wait(3)
		voice(v)
		off(v, 1, 0, 60)
		wait(3)
	})
	kase("alloc 16 samples, noise on voice 16", func() {
		fill(16)
		wait(2)
		v := on(1, 0, 72)
		wait(3)
		state()
		off(v, 1, 0, 72)
		wait(3)
	})
	kase("alloc 15 samples, noise on 15 and 16", func() {
		fill(15)
		wait(2)
		a := on(1, 0, 72)
		b := on(1, 0, 48)
		wait(3)
		state()
		off(a, 1, 0, 72)
		wait(3)
		state()
		off(b, 1, 0, 48)
		wait(3)
	})
	kase("alloc 24 samples, noise of equal priority steals", func() {
		fill(24)
		wait(4)
		v := on(1, 0, 72)
		wait(3)
		voice(v)
		off(v, 1, 0, 72)
		wait(3)
	})
	kase("alloc 24 samples, noise of priority 10 refused", func() {
		tone(1, 0, fPrior, 10)
		fill(24)
		wait(4)
		v := on(1, 0, 72)
		wait(3)
		println("  got ", v)
	})
	kase("alloc 23 samples, noise priority 10, sample steals it", func() {
		tone(1, 0, fPrior, 10)
		fill(23)
		wait(2)
		v := on(1, 0, 72)
		wait(4)
		state()
		s := on(0, 0, 70)
		wait(4)
		voice(s)
		state()
		off(v, 1, 0, 72)
		wait(4)
		state()
		off(s, 0, 0, 70)
		wait(4)
	})
	kase("alloc 24 samples, sequence noise of priority 10 refused", func() {
		tone(1, 0, fPrior, 10)
		fill(24)
		wait(4)
		play(&note1, 40, nil)
	})
	kase("alloc 23 samples, sequence noise priority 10, sample steals it, note off later", func() {
		tone(1, 0, fPrior, 10)
		fill(23)
		wait(2)
		play(&note1, 40, func(f int) {
			if f == 24 {
				on(0, 0, 70)
			}
		})
	})
	kase("alloc noise envelope falls to 0, voice freed, sample takes it", func() {
		tone(1, 0, fAdsr2, 0x1f, 0x40)
		v := on(1, 0, 72)
		for f := range 40 {
			if f%8 == 0 {
				println("  env ", word(env(v)))
			}
			yaroze.VSync(0)
		}
		s := on(0, 0, 60)
		wait(3)
		voice(s)
		state()
		off(s, 0, 0, 60)
		wait(3)
	})
}

// envs logs voice v's envelope for frames frames, then prints it.
func envs(v int16, frames int) {
	var log [64]uint16
	for f := range frames {
		yaroze.VSync(0)
		log[f] = env(v)
	}
	print("  env")
	for _, e := range log[:frames] {
		print(" ", word(e))
	}
	println()
}

func frees() {
	for _, a := range [...]struct {
		label string
		hi    byte
	}{{"fast", 0x40}, {"slow", 0x46}} {
		kase("free noise by SsUtKeyOn, envelope falls "+a.label+", no key off", func() {
			tone(1, 0, fAdsr2, 0x1f, a.hi)
			v := on(1, 0, 72)
			envs(v, 60)
			state()
		})
		kase("free sample by SsUtKeyOn, envelope falls "+a.label+", no key off", func() {
			tone(0, 0, fAdsr2, 0x1f, a.hi)
			v := on(0, 0, 60)
			poke(regNoise, 0x100)
			envs(v, 60)
			state()
		})
		kase("free noise on a sequence, envelope falls "+a.label+", note off late", func() {
			tone(1, 0, fAdsr2, 0x1f, a.hi)
			play(&longData, 60, nil)
			state()
		})
	}
	kase("free noise v0 falls fast, noise v1 holds: is v1's bit cleared", func() {
		tone(1, 0, fAdsr2, 0x1f, 0x40)
		a := on(1, 0, 72)
		wait(1)
		tone(1, 0, fAdsr2, 0xe0, 0xdf)
		b := on(1, 0, 48)
		envs(a, 40)
		state()
		voice(b)
	})
	kase("free noise SsUtKeyOff, then its envelope falls", func() {
		tone(1, 0, fAdsr2, 0x1f, 0x40)
		v := on(1, 0, 72)
		wait(2)
		off(v, 1, 0, 72)
		envs(v, 40)
		state()
	})
}

func steals() {
	low := func() { tone(1, 0, fPrior, 10) }
	kase("steal noise voice by a noise key on (both priority 10)", func() {
		low()
		fill(23)
		wait(2)
		on(1, 0, 72)
		wait(4)
		state()
		on(1, 0, 48)
		wait(4)
		state()
	})
	kase("steal noise voice keyed off, by a sample", func() {
		low()
		fill(23)
		wait(2)
		v := on(1, 0, 72)
		wait(4)
		off(v, 1, 0, 72)
		wait(4)
		state()
		on(0, 0, 70)
		wait(4)
		state()
	})
	kase("steal noise voice holding, another noise voice last on, by a sample", func() {
		low()
		fill(22)
		wait(2)
		on(1, 0, 72)
		wait(2)
		tone(1, 0, fPrior, 64)
		on(1, 0, 48)
		wait(4)
		state()
		on(0, 0, 70)
		wait(4)
		state()
	})
	kase("steal sample voice by a sample, with a noise bit poked on", func() {
		fill(24)
		wait(4)
		poke(regNoise, 0x100)
		println("  got ", yaroze.SsUtKeyOn(vab, 0, 0, 70, 0, 100, 100))
		wait(4)
		state()
	})
	kase("steal sequence's noise voice by a sequence's sample note", func() {
		low()
		fill(23)
		wait(2)
		play(&longData, 30, nil)
		state()
		play(&note0, 25, nil)
		state()
	})
	kase("steal a noise voice's sample thief, then noise again", func() {
		low()
		fill(23)
		wait(2)
		on(1, 0, 72)
		wait(4)
		tone(0, 0, fPrior, 9)
		s := on(0, 0, 70)
		wait(4)
		voice(s)
		tone(1, 0, fPrior, 64)
		println("  noise ", yaroze.SsUtKeyOn(vab, 1, 0, 36, 0, 100, 100))
		wait(4)
		state()
	})
}

// tracked finds what the library remembers as the noise voice: a steal or a
// freed voice clears the registers only for the voice it remembers.
func tracked() {
	slow := func() { tone(1, 0, fAdsr2, 0xdc, 0xdf) }
	kase("track noise v22 and v23 (last, priority 10), SsUtKeyOff of v22, sample steals v23", func() {
		fill(22)
		wait(2)
		a := on(1, 0, 72)
		wait(2)
		tone(1, 0, fPrior, 10)
		b := on(1, 0, 48)
		wait(4)
		off(a, 1, 0, 72)
		wait(4)
		state()
		poke(regNoise+2, 0x80)
		println("  thief ", yaroze.SsUtKeyOn(vab, 0, 0, 70, 0, 100, 100), " of ", b)
		wait(4)
		state()
	})
	kase("track noise held with a slow release, SsUtAllKeyOff, other voices filled, sample steals it", func() {
		slow()
		v := on(1, 0, 72)
		wait(4)
		yaroze.SsUtAllKeyOff(0)
		wait(4)
		state()
		tone(1, 0, fAdsr2, 0xe0, 0xdf)
		fill(23)
		wait(2)
		voice(v)
		println("  thief ", yaroze.SsUtKeyOn(vab, 0, 0, 70, 0, 100, 100), " of ", v)
		wait(4)
		state()
	})
	kase("track sequence's noise held with a slow release, SsSeqStop, other voices filled, sample steals it", func() {
		slow()
		s := play(&longData, 25, nil)
		yaroze.SsSeqStop(s)
		wait(4)
		state()
		tone(1, 0, fAdsr2, 0xe0, 0xdf)
		fill(23)
		wait(2)
		voice(0)
		println("  thief ", yaroze.SsUtKeyOn(vab, 0, 0, 70, 0, 100, 100))
		wait(4)
		state()
	})
	kase("track noise keyed off by the sequence's end, then a key on takes its freed voice", func() {
		play(&holdData, 60, nil)
		state()
		println("  next ", yaroze.SsUtKeyOn(vab, 0, 0, 70, 0, 100, 100))
		wait(4)
		state()
	})
	kase("track SsUtKeyOff of a noise voice after SsUtAllKeyOff", func() {
		v := on(1, 0, 72)
		wait(2)
		yaroze.SsUtAllKeyOff(0)
		wait(2)
		off(v, 1, 0, 72)
		wait(2)
		state()
	})
	kase("track note off of a noise voice after SsUtAllKeyOff", func() {
		play(&note1, 40, func(f int) {
			if f == 20 {
				yaroze.SsUtAllKeyOff(0)
			}
			if f == 25 {
				state()
			}
		})
	})
	kase("track noise bit left by SsUtAllKeyOff, sample on the voice, its key off", func() {
		v := on(1, 0, 72)
		wait(2)
		yaroze.SsUtAllKeyOff(0)
		for range 20 {
			yaroze.VSync(0)
		}
		s := on(0, 0, 60)
		println("  same voice ", s == v)
		wait(3)
		state()
		off(s, 0, 0, 60)
		wait(3)
		state()
	})
	kase("track noise v23 priority 10, a sample steals it, another sample steals the sample", func() {
		low := func() {
			tone(1, 0, fPrior, 10)
			tone(0, 0, fPrior, 64)
		}
		low()
		fill(23)
		wait(2)
		on(1, 0, 72)
		wait(4)
		tone(0, 0, fPrior, 10)
		on(0, 0, 70)
		wait(4)
		poke(regNoise+2, 0x80)
		state()
		println("  thief ", yaroze.SsUtKeyOn(vab, 0, 0, 71, 0, 100, 100))
		wait(4)
		state()
	})
	kase("track noise held, SsUtAllKeyOff, sample on the voice falls to envelope 0", func() {
		v := on(1, 0, 72)
		wait(2)
		yaroze.SsUtAllKeyOff(0)
		for range 20 {
			yaroze.VSync(0)
		}
		tone(0, 0, fAdsr2, 0x1f, 0x40)
		s := on(0, 0, 60)
		println("  same voice ", s == v)
		wait(30)
		state()
	})
	seqSides := func(label string, patch func()) {
		kase("track sequence volume sides, "+label+": sl sr l r", func() {
			s := open(&fastData)
			for _, a := range [...][2]int16{{0, 127}, {127, 0}, {1, 127}, {127, 1}, {0, 0}, {200, 300}, {-1, -1}, {128, 64}} {
				grid(s, "sides", int(a[0]), int(a[1]), func() {
					patch()
					yaroze.SsSeqSetVol(s, a[0], a[1])
				})
			}
			yaroze.SsSeqSetVol(s, 127, 127)
		})
	}
	seqSides("noise", func() {})
	seqSides("sample", func() { setVag(1, 0, 1) })
}

// order finds where a sequence's noise writes fall against the program's:
// a noise key on by SsUtKeyOn in the frames around the note's.
func order() {
	for _, at := range [...]int{14, 15, 16} {
		kase("order SsUtKeyOn noise note 72 before blank "+word(uint16(at))+", sequence noise note 60 at tick 48", func() {
			play(&note1, 40, func(f int) {
				if f == at {
					on(1, 0, 72)
				}
			})
		})
		kase("order SsUtKeyOn sample note 72 before blank "+word(uint16(at))+", sequence sample note 60 at tick 48", func() {
			play(&note0, 40, func(f int) {
				if f == at {
					on(0, 0, 72)
				}
			})
		})
	}
}

func keyOffs() {
	kase("off SsUtKeyOff of a noise voice, twice", func() {
		v := on(1, 0, 72)
		wait(3)
		off(v, 1, 0, 72)
		wait(1)
		state()
		wait(3)
		off(v, 1, 0, 72)
		wait(3)
		voice(v)
	})
	kase("off SsUtKeyOff with another note, refused", func() {
		v := on(1, 0, 72)
		wait(3)
		off(v, 1, 0, 73)
		wait(3)
		state()
		off(v, 1, 0, 72)
		wait(3)
	})
	kase("off SsUtKeyOff in the frame of the key on", func() {
		v := on(1, 0, 72)
		off(v, 1, 0, 72)
		wait(3)
	})
	kase("off a sample's SsUtKeyOff while noise holds", func() {
		v := on(1, 0, 72)
		s := on(0, 0, 60)
		wait(3)
		off(s, 0, 0, 60)
		wait(3)
		state()
		off(v, 1, 0, 72)
		wait(3)
	})
	kase("off a sample's refused SsUtKeyOff while noise holds", func() {
		v := on(1, 0, 72)
		s := on(0, 0, 60)
		wait(3)
		off(s, 0, 0, 61)
		wait(3)
		state()
		off(s, 0, 0, 60)
		off(v, 1, 0, 72)
		wait(3)
	})
	kase("off SsUtAllKeyOff", func() {
		on(1, 0, 72)
		wait(3)
		yaroze.SsUtAllKeyOff(0)
		wait(3)
		state()
	})
	kase("off SsUtAllKeyOff(1)", func() {
		on(1, 0, 72)
		wait(3)
		yaroze.SsUtAllKeyOff(1)
		wait(3)
		state()
	})
	kase("off sequence's note off (note 60 from 48 to 96)", func() { play(&note1, 40, nil) })
	kase("off sequence's note off the last event (note 60 from 48 to 96)", func() { play(&endNData, 40, nil) })
	kase("off sequence's end with the noise note held, sample note on the last event", func() {
		play(&holdData, 90, func(f int) {
			if f == 40 || f == 80 {
				state()
				voice(0)
			}
		})
	})
	kase("off SsUtKeyOff of a sequence's noise voice", func() {
		play(&longData, 30, func(f int) {
			if f == 25 {
				off(0, 1, 0, 60)
			}
		})
		voice(0)
	})
	kase("off SsUtAllKeyOff on a sequence's noise voice", func() {
		play(&longData, 30, func(f int) {
			if f == 25 {
				yaroze.SsUtAllKeyOff(0)
			}
		})
		voice(0)
	})
	kase("off noise voice busy after SsUtKeyOff: a key on 60 frames later", func() {
		v := on(1, 0, 72)
		wait(2)
		off(v, 1, 0, 72)
		wait(60)
		voice(v)
		println("  next ", yaroze.SsUtKeyOn(vab, 0, 0, 60, 0, 100, 100))
		wait(2)
	})
	kase("off noise voice, SsUtKeyOff, then SsUtAllKeyOff frees it", func() {
		v := on(1, 0, 72)
		wait(2)
		off(v, 1, 0, 72)
		wait(4)
		yaroze.SsUtAllKeyOff(0)
		for range 30 {
			yaroze.VSync(0)
		}
		voice(v)
		println("  next ", yaroze.SsUtKeyOn(vab, 0, 0, 60, 0, 100, 100))
		wait(2)
	})
	stopAt := func(label string, what func(s int16)) {
		kase("off "+label, func() {
			var s int16
			s = play(&longData, 40, func(f int) {
				if f == 25 {
					what(s)
				}
			})
		})
	}
	stopAt("SsSeqStop on a held note", func(s int16) { yaroze.SsSeqStop(s) })
	kase("off SsSeqPause on a held note, SsSeqReplay later", func() {
		var s int16
		s = play(&longData, 50, func(f int) {
			switch f {
			case 25:
				yaroze.SsSeqPause(s)
			case 40:
				state()
				yaroze.SsSeqReplay(s)
			}
		})
	})
	kase("off SsSeqClose on a held note", func() {
		s := yaroze.SsSeqOpen(&longData[0], vab)
		yaroze.SsSeqPlay(s, 1, 1)
		wait(25)
		yaroze.SsSeqClose(s)
		wait(10)
		state()
		voice(0)
	})
	kase("off SsSeqStop on a sequence with no note yet, noise from SsUtKeyOn holds", func() {
		v := on(1, 0, 72)
		s := play(&note0, 5, nil)
		yaroze.SsSeqStop(s)
		wait(5)
		state()
		off(v, 1, 0, 72)
		wait(3)
	})
}

func pairs() {
	kase("two noise by SsUtKeyOn in one frame, 60 then 72, first off first", func() {
		a := on(1, 0, 60)
		b := on(1, 0, 72)
		wait(3)
		state()
		off(a, 1, 0, 60)
		wait(3)
		state()
		voice(b)
		off(b, 1, 0, 72)
		wait(3)
	})
	kase("two noise by SsUtKeyOn frames apart, 60 then 72, second off first", func() {
		a := on(1, 0, 60)
		wait(3)
		b := on(1, 0, 72)
		wait(3)
		state()
		off(b, 1, 0, 72)
		wait(3)
		state()
		off(a, 1, 0, 60)
		wait(3)
	})
	kase("two noise, first's key off then a third", func() {
		a := on(1, 0, 60)
		wait(1)
		b := on(1, 0, 72)
		wait(1)
		off(a, 1, 0, 60)
		wait(1)
		c := on(1, 0, 48)
		wait(3)
		state()
		off(b, 1, 0, 72)
		off(c, 1, 0, 48)
		wait(3)
	})
	kase("two noise on a sequence, first off first", func() { play(&twoAData, 65, nil) })
	kase("two noise on a sequence, second off first", func() { play(&twoBData, 65, nil) })
	kase("noise and sample on a sequence, sample off first", func() { play(&mixAData, 65, nil) })
	kase("noise and sample on a sequence, noise off first", func() { play(&mixBData, 65, nil) })
	kase("sample then noise on a sequence, sample off first", func() { play(&mixCData, 65, nil) })
	kase("noise by SsUtKeyOn, sample note off on a sequence", func() {
		v := on(1, 0, 72)
		wait(2)
		play(&note0, 40, nil)
		state()
		off(v, 1, 0, 72)
		wait(3)
	})
	kase("noise on a sequence, SsUtKeyOff of a sample during it", func() {
		var s int16
		play(&note1, 40, func(f int) {
			switch f {
			case 20:
				s = on(0, 0, 70)
			case 24:
				off(s, 0, 0, 70)
			}
		})
	})
	kase("poke 194=0100 196=0001, noise key on and off", func() {
		poke(regNoise, 0x100)
		poke(regNoise+2, 1)
		wait(2)
		v := on(1, 0, 72)
		wait(3)
		state()
		off(v, 1, 0, 72)
		wait(3)
		state()
		poke(regNoise, 0)
		poke(regNoise+2, 0)
	})
	kase("poke 194=0100, sample key on and off", func() {
		poke(regNoise, 0x100)
		wait(2)
		v := on(0, 0, 60)
		wait(3)
		state()
		off(v, 0, 0, 60)
		wait(3)
		state()
		poke(regNoise, 0)
	})
}

func multi() {
	kase("multi sequence program 2: tone 0 noise, tone 1 sample", func() {
		play(&note2, 40, func(f int) {
			if f == 20 {
				voice(0)
				voice(1)
			}
		})
	})
	kase("multi sequence program 3: tone 0 sample, tone 1 noise", func() {
		play(&note3, 40, func(f int) {
			if f == 20 {
				voice(0)
				voice(1)
			}
		})
	})
	kase("multi sequence program 4: both noise, centers 60 and 72", func() {
		play(&note4, 40, func(f int) {
			if f == 20 {
				state()
				voice(0)
				voice(1)
			}
		})
	})
	kase("multi sequence program 4 tone 0 made a sample", func() {
		setVag(4, 0, 1)
		play(&note4, 40, nil)
	})
	kase("multi program 2 tones by SsUtKeyOn", func() {
		a := on(2, 0, 60)
		b := on(2, 1, 60)
		wait(3)
		voice(a)
		voice(b)
		state()
		off(b, 2, 1, 60)
		wait(3)
		state()
		off(a, 2, 0, 60)
		wait(3)
	})
}

func interactions() {
	bendTone := func() { tone(1, 0, fPbMin, 12, 12) }
	kase("bend SsUtPitchBend on a noise voice, center 60 note 72", func() {
		bendTone()
		v := on(1, 0, 72)
		wait(3)
		for _, b := range [...]int16{127, 0, 64} {
			r := yaroze.SsUtPitchBend(v, vab, 1, 72, b)
			println("  bend ", b, ": ", r)
			wait(2)
			voice(v)
			state()
		}
		off(v, 1, 0, 72)
		wait(3)
	})
	kase("bend SsUtChangePitch on a noise voice", func() {
		v := on(1, 0, 72)
		wait(3)
		r := yaroze.SsUtChangePitch(v, vab, 1, 72, 0, 84, 0)
		println("  change: ", r)
		wait(2)
		voice(v)
		state()
		r = yaroze.SsUtChangePitch(v, vab, 1, 72, 0, 60, 0)
		println("  change: ", r)
		wait(2)
		voice(v)
		state()
		off(v, 1, 0, 72)
		wait(3)
	})
	kase("bend sequence bend 127 before the note", func() {
		bendTone()
		play(&pbbData, 40, nil)
	})
	kase("bend sequence bends 127 at 48, 0 at 72, 64 at 96, while note 72 holds (24 to 120)", func() {
		bendTone()
		play(&pbdData, 50, nil)
	})
	kase("reverb noise tone mode 4 with reverb on", func() {
		tone(1, 0, fMode, 4)
		yaroze.SsUtReverbOn()
		wait(2)
		v := on(1, 0, 72)
		wait(3)
		voice(v)
		off(v, 1, 0, 72)
		wait(3)
		yaroze.SsUtReverbOff()
	})
	for _, vag := range [...]uint16{0x00ff, 0x01ff, 0x7fff, 0x80ff, 0xfeff, 0xffff, 0x00fe, 0x0100} {
		kase("vag sequence "+word(vag), func() {
			setVag(1, 0, vag)
			play(&note1, 40, func(f int) {
				if f == 20 {
					voice(0)
				}
			})
		})
		kase("vag SsUtKeyOn "+word(vag), func() {
			setVag(1, 0, vag)
			v := on(1, 0, 72)
			wait(3)
			voice(v)
			state()
			if v >= 0 {
				off(v, 1, 0, 72)
			}
			wait(3)
		})
	}
	kase("vag patched after the transfer: program 0 made noise, sequence", func() {
		setVag(0, 0, 0xff)
		play(&note0, 40, nil)
	})
	kase("vag 1 back on program 1 while its noise holds, key off", func() {
		v := on(1, 0, 72)
		wait(3)
		setVag(1, 0, 1)
		off(v, 1, 0, 72)
		wait(3)
	})
	kase("vag program 0 made noise while its sample holds, key off", func() {
		v := on(0, 0, 60)
		wait(3)
		setVag(0, 0, 0xff)
		off(v, 0, 0, 60)
		wait(3)
	})
	kase("clock across SsVabClose and SsVabTransfer, noise held at the close", func() {
		on(1, 0, 72)
		wait(2)
		state()
		yaroze.SsVabClose(vab)
		wait(2)
		state()
		vab = yaroze.SsVabTransfer(vh.Bytes(), vb.Bytes(), -1, 1)
		println("  vab ", vab)
		wait(2)
		state()
		v := on(1, 0, 60)
		wait(2)
		off(v, 1, 0, 60)
		wait(2)
	})
}

func main() {
	yaroze.ResetGraph(0)
	yaroze.GsInitGraph(320, 240, yaroze.GsOFSGPU, 0, 0)
	yaroze.GsDefDispBuff(0, 0, 0, 240)
	yaroze.DrawSync(0)
	if f := disc.LoadAll(600); f != nil {
		println("load failed")
	}
	// The VAG bytes in the file, before the transfer, as a bank with noise
	// tones would have them.
	setVag(1, 0, 0xff)
	setVag(2, 0, 0xff)
	setVag(3, 1, 0xff)
	setVag(4, 0, 0xff)
	setVag(4, 1, 0xff)
	vab = yaroze.SsVabTransfer(vh.Bytes(), vb.Bytes(), -1, 1)
	yaroze.SsSetMVol(127, 127)
	println("vab ", vab)
	copy(saved[:], hdr())
	wait(2)
	state()
	quiet()

	walks()
	ctrlBits()
	order()
	volumes()
	volumeGrid()
	allocator()
	frees()
	steals()
	tracked()
	keyOffs()
	pairs()
	multi()
	interactions()

	println("done")
	for {
		yaroze.VSync(0)
	}
}
