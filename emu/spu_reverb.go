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

	// The reverb's last output, left and right, kept by psxe as ints
	// among its internal state.
	spuIntRevL = 0x24a
	spuIntRevR = 0x24e
)

// reverb runs psxe's reverb for a sample, from the voices sent to it, and
// keeps its output for the mix. It works in the area of SPU RAM from
// MBASE to the end, as a ring whose current position steps on a halfword
// a sample: same-side and opposite-side reflections, four combs, and two
// all-pass filters, each reading and writing the ring.
//
// The arithmetic is psxe's, in floats, rounded where psxe's rounds: each
// value is clamped to 16 bits as a float, and only truncated to an integer
// where psxe assigns it to one. That is psxe's source as written; psxe is
// built with fast-math, which lets the compiler round differently, so its
// binary can store values a unit or so apart in the work area.
func (s *spu) reverb(inl, inr int16) {
	mbase := uint32(s.get16(spuMBASE)) << 3
	base := s.get32(spuIntRevbAddr)
	at := func(addr uint32) uint32 {
		rel := (addr + base - mbase) % (0x80000 - mbase)
		return (mbase + rel) & 0x7fffe
	}
	read := func(addr uint32) float32 {
		return float32(int16(binary.LittleEndian.Uint16(s.ram[at(addr):])))
	}
	write := func(addr uint32, v int16) {
		binary.LittleEndian.PutUint16(s.ram[at(addr):], uint16(v))
	}
	addr := func(reg uint32) uint32 { return uint32(s.get16(reg)) << 3 }
	vol := func(reg uint32) float32 { return float32(int16(s.get16(reg))) }
	// x / 32768, as a float, after x was rounded to one.
	scale := func(x float32) float32 { return x / 32768 }

	dapf1, dapf2 := addr(spuDAPF1), addr(spuDAPF2)
	viir, vwall := vol(spuVIIR), vol(spuVWall)
	vapf1, vapf2 := vol(spuVAPF1), vol(spuVAPF2)
	// The output volumes are unsigned in psxe, unlike the rest.
	vlout, vrout := float32(s.get16(spuVLOut)), float32(s.get16(spuVROut))

	lin := float32(int32(scale(float32(vol(spuVLIn) * float32(inl)))))
	rin := float32(int32(scale(float32(vol(spuVRIn) * float32(inr)))))

	// A reflection: the input, the wall's echo of the delayed sample, and
	// the last value, less its share by the IIR volume. As in psxe, each
	// stage works out both sides before writing either, which matters
	// where their addresses overlap.
	reflect := func(in float32, dst, src uint32) int16 {
		last := read(dst - 2)
		v := float32(in + scale(float32(read(src)*vwall)))
		v = float32(v - scale(float32(last*viir)))
		v = float32(v + last)
		return int16(saturate(v))
	}
	// Same side, left to left and right to right.
	mlsame, mrsame := addr(spuMLSame), addr(spuMRSame)
	sl := reflect(lin, mlsame, addr(spuDLSame))
	sr := reflect(rin, mrsame, addr(spuDRSame))
	write(mlsame, sl)
	write(mrsame, sr)
	// Opposite sides, right to left and left to right.
	mldiff, mrdiff := addr(spuMLDiff), addr(spuMRDiff)
	dl := reflect(lin, mldiff, addr(spuDRDiff))
	dr := reflect(rin, mrdiff, addr(spuDLDiff))
	write(mldiff, dl)
	write(mrdiff, dr)

	// Early echo, the four combs summed.
	comb := func(c1, c2, c3, c4 uint32) int16 {
		v := scale(float32(vol(spuVComb1) * read(addr(c1))))
		v = float32(v + scale(float32(vol(spuVComb2)*read(addr(c2)))))
		v = float32(v + scale(float32(vol(spuVComb3)*read(addr(c3)))))
		v = float32(v + scale(float32(vol(spuVComb4)*read(addr(c4)))))
		return int16(saturate(v))
	}
	l := comb(spuMLComb1, spuMLComb2, spuMLComb3, spuMLComb4)
	r := comb(spuMRComb1, spuMRComb2, spuMRComb3, spuMRComb4)

	// Late reverb, through two all-pass filters, each taking away its
	// share of the delayed value, storing the result, and then adding its
	// share of the result to the delayed value as it now reads.
	apf := func(l, r int16, ml, mr, d uint32, v float32) (int16, int16) {
		in := func(x int16, m uint32) int16 {
			return int16(saturate(float32(float32(x) - saturate(scale(float32(v*read(m-d)))))))
		}
		out := func(x int16, m uint32) int16 {
			return int16(saturate(float32(scale(float32(float32(x)*v)) + read(m-d))))
		}
		l, r = in(l, ml), in(r, mr)
		write(ml, l)
		write(mr, r)
		return out(l, ml), out(r, mr)
	}
	l, r = apf(l, r, addr(spuMLAPF1), addr(spuMRAPF1), dapf1, vapf1)
	l, r = apf(l, r, addr(spuMLAPF2), addr(spuMRAPF2), dapf2, vapf2)

	s.put32(spuIntRevL, uint32(int32(saturate(scale(float32(float32(l)*vlout))))))
	s.put32(spuIntRevR, uint32(int32(saturate(scale(float32(float32(r)*vrout))))))
	s.put32(spuIntRevbAddr, max(mbase, (base+2)&0x7fffe))
}

// saturate clamps a float to 16 bits, as psxe's SAT does, leaving it a
// float.
func saturate(v float32) float32 {
	switch {
	case v <= -32768:
		return -32768
	case v >= 32767:
		return 32767
	}
	return v
}
