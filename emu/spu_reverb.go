package emu

import "encoding/binary"

// Registers and internal state the mix and the reverb use.
const (
	spuVLOut = 0x184 // reverb output volume, left and right
	spuVROut = 0x186
	spuEON   = 0x198 // the voices sent to the reverb

	// The reverb's configuration: delays, as addresses in 8-byte units,
	// and volumes.
	spuDAPF1   = 0x1c0
	spuDAPF2   = 0x1c2
	spuVIIR    = 0x1c4
	spuVComb1  = 0x1c6
	spuVComb2  = 0x1c8
	spuVComb3  = 0x1ca
	spuVComb4  = 0x1cc
	spuVWall   = 0x1ce
	spuVAPF1   = 0x1d0
	spuVAPF2   = 0x1d2
	spuMLSame  = 0x1d4
	spuMRSame  = 0x1d6
	spuMLComb1 = 0x1d8
	spuMRComb1 = 0x1da
	spuMLComb2 = 0x1dc
	spuMRComb2 = 0x1de
	spuDLSame  = 0x1e0
	spuDRSame  = 0x1e2
	spuMLDiff  = 0x1e4
	spuMRDiff  = 0x1e6
	spuMLComb3 = 0x1e8
	spuMRComb3 = 0x1ea
	spuMLComb4 = 0x1ec
	spuMRComb4 = 0x1ee
	spuDLDiff  = 0x1f0
	spuDRDiff  = 0x1f2
	spuMLAPF1  = 0x1f4
	spuMRAPF1  = 0x1f6
	spuMLAPF2  = 0x1f8
	spuMRAPF2  = 0x1fa
	spuVLIn    = 0x1fc
	spuVRIn    = 0x1fe
)

// reverb is the reverb's part of a sample: it takes the sum of what is sent
// to it, left and right, and returns its output, before the output volume.
// It works as the hardware does, at half the sample rate: the input is
// filtered down to 22.05kHz, on every other sample the reverb runs once
// for each side, and its output is filtered back up, so each sample has
// one. The reverb reads its work area, the ring in SPU RAM from MBASE to
// the end, whether or not it is on; on, it writes to it too.
//
// The arithmetic is the hardware's as psx-spx documents it, in 16-bit
// fixed point, with products shifted down and sums saturated where the
// hardware does so, as Mednafen has it from tests on a console.
func (s *spu) reverb(inl, inr int16) (int16, int16) {
	r := &s.rev
	pos := r.pos
	r.in[0][pos], r.in[0][pos+64] = inl, inl
	r.in[1][pos], r.in[1][pos+64] = inr, inr
	r.pos = (pos + 1) & 63

	k := pos >> 1
	if pos&1 == 0 {
		// Between the reverb's samples, the output is the one ten back,
		// which is where the filter's window is centred.
		w := (k - 19) & 31
		return r.out[0][w+9], r.out[1][w+9]
	}

	w := reverbArea{
		ram:   &s.ram,
		start: uint32(s.get16(spuMBASE)) << 2,
		cur:   s.get32(spuIntRevbAddr) >> 1,
		on:    s.get16(spuSPUCNT)&0x80 != 0,
	}
	in := [2]int32{downsample(r.in[0][(pos-38)&63:]), downsample(r.in[1][(pos-38)&63:])}

	vIIR, vWall := s.revVol(spuVIIR), s.revVol(spuVWall)
	vAPF1, vAPF2 := s.revVol(spuVAPF1), s.revVol(spuVAPF2)
	dAPF1, dAPF2 := s.get16(spuDAPF1), s.get16(spuDAPF2)
	vComb := [4]int32{s.revVol(spuVComb1), s.revVol(spuVComb2), s.revVol(spuVComb3), s.revVol(spuVComb4)}

	for lr := range uint32(2) {
		// Each register comes in a pair, left then right.
		reg := func(left uint32) uint16 { return s.get16(left + 2*lr) }
		vIn := s.revVol(spuVLIn + 2*lr)
		x := in[lr] * vIn >> 14

		// Reflections: the input with the wall's echo of a delayed sample,
		// from the same side and from the other, each through a low-pass
		// filter whose last output is the halfword before it.
		same := revSat((w.read(reg(spuDLSame))*vWall>>14 + x) >> 1)
		diff := revSat((w.read(s.get16(spuDRDiff-2*lr))*vWall>>14 + x) >> 1)
		mSame, mDiff := reg(spuMLSame), reg(spuMLDiff)
		same = revSat((same*vIIR>>14 + revIIR(vIIR, w.readPrev(mSame))>>14) >> 1)
		diff = revSat((diff*vIIR>>14 + revIIR(vIIR, w.readPrev(mDiff))>>14) >> 1)
		w.write(mSame, same)
		w.write(mDiff, diff)

		// Early echo, the four combs summed.
		var acc int32
		for i, c := range [4]uint32{spuMLComb1, spuMLComb2, spuMLComb3, spuMLComb4} {
			acc += w.read(reg(c)) * vComb[i] >> 14
		}

		// Late reverb, through two all-pass filters, the first's output
		// going straight into the second.
		mAPF1, mAPF2 := reg(spuMLAPF1), reg(spuMLAPF2)
		fb1, fb2 := w.read(mAPF1-dAPF1), w.read(mAPF2-dAPF2)
		apf1 := revSat((acc + fb1*revNeg(vAPF1)>>14) >> 1)
		apf2 := revSat(fb1 + (apf1*vAPF1>>14+fb2*revNeg(vAPF2)>>14)>>1)
		out := revSat(fb2 + apf2*vAPF2>>15)
		w.write(mAPF1, apf1)
		w.write(mAPF2, apf2)

		r.out[lr][k], r.out[lr][k+32] = int16(out), int16(out)
	}

	// The ring steps on a halfword for every sample the reverb makes, back
	// to MBASE past the end of SPU RAM.
	cur := (w.cur + 1) & 0x3ffff
	if cur == 0 {
		cur = w.start
	}
	s.put32(spuIntRevbAddr, cur<<1)

	win := (k - 19) & 31
	return int16(upsample(r.out[0][win:])), int16(upsample(r.out[1][win:]))
}

// reverbState is the reverb's resampling: its last 64 inputs, at 44.1kHz,
// and 32 outputs, at 22.05kHz, each ring held twice over so that a filter's
// window can be read without wrapping, and where the next input goes.
type reverbState struct {
	in  [2][128]int16
	out [2][64]int16
	pos uint32
}

// revVol is one of the reverb's volumes, signed.
func (s *spu) revVol(reg uint32) int32 { return int32(int16(s.get16(reg))) }

// reverbArea is the reverb's work area for one of its samples: the ring of
// halfwords from start to the end of SPU RAM, with cur the reverb's place
// in it, from which the addresses in its registers are taken.
type reverbArea struct {
	ram        *[spuRAMSize]byte
	start, cur uint32 // in halfwords
	on         bool
}

// at is where a halfword offset from the reverb's place lands: past the
// end of SPU RAM, it wraps round to the start of the work area.
func (w *reverbArea) at(off uint32) uint32 {
	a := w.cur + off&0x3ffff
	if a&0x40000 != 0 {
		a += w.start
	}
	return (a & 0x3ffff) << 1
}

// read and write take an address register's value, in 8-byte units.
func (w *reverbArea) read(reg uint16) int32 { return w.readAt(uint32(reg) << 2) }

// readPrev reads the halfword before the address.
func (w *reverbArea) readPrev(reg uint16) int32 { return w.readAt(uint32(reg)<<2 - 1) }

func (w *reverbArea) readAt(off uint32) int32 {
	return int32(int16(binary.LittleEndian.Uint16(w.ram[w.at(off):])))
}

func (w *reverbArea) write(reg uint16, v int32) {
	if w.on {
		binary.LittleEndian.PutUint16(w.ram[w.at(uint32(reg)<<2):], uint16(v))
	}
}

// revTaps are the resampling filter's taps that are neither zero nor the
// middle one, which is 4000h: every other one of 39.
var revTaps = [20]int32{
	-1, 2, -10, 35, -103, 266, -616, 1332, -2960, 10246,
	10246, -2960, 1332, -616, 266, -103, 35, -10, 2, -1,
}

// downsample filters the 39 inputs from src to one at half the rate.
func downsample(src []int16) int32 {
	src = src[:39]
	sum := 0x4000 * int32(src[19])
	for i, t := range revTaps {
		sum += t * int32(src[2*i])
	}
	return int32(clamp16(sum >> 15))
}

// upsample filters the 20 outputs from src to the one between the middle
// two, at twice the rate; the zeros between them are left out, and with
// them half the gain, which the shift makes up.
func upsample(src []int16) int32 {
	src = src[:20]
	var sum int32
	for i, t := range revTaps {
		sum += t * int32(src[i])
	}
	return int32(clamp16(sum >> 14))
}

// revIIR is the low-pass filter's share of its last output: the output
// times one less the IIR volume, in a 17-bit product the hardware takes
// care of where the volume is -1.
func revIIR(vIIR, last int32) int32 {
	if vIIR == -0x8000 {
		if last == -0x8000 {
			return 0
		}
		return last * -0x10000
	}
	return last * (0x8000 - vIIR)
}

// revNeg negates a volume, as 16 bits, so -8000h becomes 7FFFh.
func revNeg(v int32) int32 {
	if v == -0x8000 {
		return 0x7fff
	}
	return -v
}

func revSat(v int32) int32 { return min(max(v, -0x8000), 0x7fff) }
