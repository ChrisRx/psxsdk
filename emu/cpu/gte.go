package cpu

import "math/bits"

// GTE is the geometry transformation engine's registers. Each field has the
// width and signedness psxe gives it, since that decides how a value written
// through mtc2 or ctc2 reads back.
type GTE struct {
	// Data registers, 0 to 31.
	V [3]struct {
		XY uint32
		Z  int16
	}
	RGBC uint32
	OTZ  uint16
	IR   [4]int16
	SXY  [4]uint32 // SXY[3] is the FIFO's input
	SZ   [4]uint16
	RGB  [3]uint32
	Res1 uint32
	MAC  [4]int32
	IRGB uint16
	LZCS int32
	LZCR int32

	// Control registers, 32 to 63.
	RT, L, LR  matrix
	TR, BK, FC [3]int32
	OFX, OFY   uint32
	H          uint32
	DQA        int16
	DQB        int32
	ZSF3, ZSF4 int16
	Flag       uint32
}

// matrix is a 3x3 matrix of 16-bit values, stored as the hardware lays it out
// in registers: four words holding two elements each, then the ninth alone.
type matrix struct {
	M   [4]uint32
	M33 int16
}

// Read returns register r, 0 to 31 for the data registers and 32 to 63 for
// the control registers.
func (g *GTE) Read(r uint32) uint32 {
	switch r {
	case 0, 2, 4:
		return g.V[r/2].XY
	case 1, 3, 5:
		return uint32(int32(g.V[r/2].Z))
	case 6:
		return g.RGBC
	case 7:
		return uint32(g.OTZ)
	case 8, 9, 10, 11:
		return uint32(int32(g.IR[r-8]))
	case 12, 13, 14:
		return g.SXY[r-12]
	case 15:
		return g.SXY[2] // SXYP reads as SXY2
	case 16, 17, 18, 19:
		return uint32(g.SZ[r-16])
	case 20, 21, 22:
		return g.RGB[r-20]
	case 23:
		return g.Res1
	case 24, 25, 26, 27:
		return uint32(g.MAC[r-24])
	case 28:
		g.IRGB = uint16(clampIR5(g.IR[1]) | clampIR5(g.IR[2])<<5 | clampIR5(g.IR[3])<<10)
		return uint32(g.IRGB)
	case 29:
		// ORGB. psxe returns IRGB as it was last computed.
		return uint32(g.IRGB)
	case 30:
		return uint32(g.LZCS)
	case 31:
		return uint32(g.LZCR)
	case 32, 33, 34, 35:
		return g.RT.M[r-32]
	case 36:
		return uint32(int32(g.RT.M33))
	case 37, 38, 39:
		return uint32(g.TR[r-37])
	case 40, 41, 42, 43:
		return g.L.M[r-40]
	case 44:
		return uint32(int32(g.L.M33))
	case 45, 46, 47:
		return uint32(g.BK[r-45])
	case 48, 49, 50, 51:
		return g.LR.M[r-48]
	case 52:
		return uint32(int32(g.LR.M33))
	case 53, 54, 55:
		return uint32(g.FC[r-53])
	case 56:
		return g.OFX
	case 57:
		return g.OFY
	case 58:
		return uint32(int32(int16(g.H)))
	case 59:
		return uint32(int32(g.DQA))
	case 60:
		return uint32(g.DQB)
	case 61:
		return uint32(int32(g.ZSF3))
	case 62:
		return uint32(int32(g.ZSF4))
	case 63:
		v := g.Flag & 0x7ffff000
		if g.Flag&0x7f87e000 != 0 {
			v |= 0x80000000
		}
		return v
	}
	return 0
}

// clampIR5 converts an IR value to the 5-bit color component IRGB holds.
func clampIR5(ir int16) uint32 {
	return uint32(min(max(int32(ir)>>7, 0), 0x1f))
}

// Write sets register r, numbered as for Read.
func (g *GTE) Write(r, v uint32) {
	switch r {
	case 0, 2, 4:
		g.V[r/2].XY = v
	case 1, 3, 5:
		g.V[r/2].Z = int16(v)
	case 6:
		g.RGBC = v
	case 7:
		g.OTZ = uint16(v)
	case 8, 9, 10, 11:
		g.IR[r-8] = int16(v)
	case 12, 13, 14:
		g.SXY[r-12] = v
	case 15:
		// Writing SXYP pushes onto the screen coordinate FIFO.
		g.SXY[3] = v
		g.SXY[0], g.SXY[1], g.SXY[2] = g.SXY[1], g.SXY[2], g.SXY[3]
	case 16, 17, 18, 19:
		g.SZ[r-16] = uint16(v)
	case 20, 21, 22:
		g.RGB[r-20] = v
	case 23:
		g.Res1 = v
	case 24, 25, 26, 27:
		g.MAC[r-24] = int32(v)
	case 28:
		g.IRGB = uint16(v & 0x7fff)
		g.IR[1] = int16((v >> 0 & 0x1f) * 0x80)
		g.IR[2] = int16((v >> 5 & 0x1f) * 0x80)
		g.IR[3] = int16((v >> 10 & 0x1f) * 0x80)
	case 29, 31:
		// ORGB and LZCR are read-only.
	case 30:
		g.LZCS = int32(v)
		g.LZCR = leadingSignBits(g.LZCS)
	case 32, 33, 34, 35:
		g.RT.M[r-32] = v
	case 36:
		g.RT.M33 = int16(v)
	case 37, 38, 39:
		g.TR[r-37] = int32(v)
	case 40, 41, 42, 43:
		g.L.M[r-40] = v
	case 44:
		g.L.M33 = int16(v)
	case 45, 46, 47:
		g.BK[r-45] = int32(v)
	case 48, 49, 50, 51:
		g.LR.M[r-48] = v
	case 52:
		g.LR.M33 = int16(v)
	case 53, 54, 55:
		g.FC[r-53] = int32(v)
	case 56:
		g.OFX = v
	case 57:
		g.OFY = v
	case 58:
		g.H = v
	case 59:
		g.DQA = int16(v)
	case 60:
		g.DQB = int32(v)
	case 61:
		g.ZSF3 = int16(v)
	case 62:
		g.ZSF4 = int16(v)
	case 63:
		g.Flag = v & 0x7ffff000
	}
}

// leadingSignBits counts the leading bits of v equal to its sign bit, which
// is what LZCR reports for a value written to LZCS.
func leadingSignBits(v int32) int32 {
	if v == 0 || v == -1 {
		return 32
	}
	if v < 0 {
		v = ^v
	}
	return int32(bits.LeadingZeros32(uint32(v)))
}
