package cpu

import "math/bits"

// The GTE's commands, ported from psxe. psxe computes in C's mixed integer
// widths, and the conversions here are written out so that each value has
// the width psxe gives it: 16-bit matrix entries and vectors, 8-bit color
// components, 32-bit translations, 44-bit MAC accumulation.

// unrTable is the GTE's table for the first step of its reciprocal.
var unrTable = [257]uint8{
	0xff, 0xfd, 0xfb, 0xf9, 0xf7, 0xf5, 0xf3, 0xf1,
	0xef, 0xee, 0xec, 0xea, 0xe8, 0xe6, 0xe4, 0xe3,
	0xe1, 0xdf, 0xdd, 0xdc, 0xda, 0xd8, 0xd6, 0xd5,
	0xd3, 0xd1, 0xd0, 0xce, 0xcd, 0xcb, 0xc9, 0xc8,
	0xc6, 0xc5, 0xc3, 0xc1, 0xc0, 0xbe, 0xbd, 0xbb,
	0xba, 0xb8, 0xb7, 0xb5, 0xb4, 0xb2, 0xb1, 0xb0,
	0xae, 0xad, 0xab, 0xaa, 0xa9, 0xa7, 0xa6, 0xa4,
	0xa3, 0xa2, 0xa0, 0x9f, 0x9e, 0x9c, 0x9b, 0x9a,
	0x99, 0x97, 0x96, 0x95, 0x94, 0x92, 0x91, 0x90,
	0x8f, 0x8d, 0x8c, 0x8b, 0x8a, 0x89, 0x87, 0x86,
	0x85, 0x84, 0x83, 0x82, 0x81, 0x7f, 0x7e, 0x7d,
	0x7c, 0x7b, 0x7a, 0x79, 0x78, 0x77, 0x75, 0x74,
	0x73, 0x72, 0x71, 0x70, 0x6f, 0x6e, 0x6d, 0x6c,
	0x6b, 0x6a, 0x69, 0x68, 0x67, 0x66, 0x65, 0x64,
	0x63, 0x62, 0x61, 0x60, 0x5f, 0x5e, 0x5d, 0x5d,
	0x5c, 0x5b, 0x5a, 0x59, 0x58, 0x57, 0x56, 0x55,
	0x54, 0x53, 0x53, 0x52, 0x51, 0x50, 0x4f, 0x4e,
	0x4d, 0x4d, 0x4c, 0x4b, 0x4a, 0x49, 0x48, 0x48,
	0x47, 0x46, 0x45, 0x44, 0x43, 0x43, 0x42, 0x41,
	0x40, 0x3f, 0x3f, 0x3e, 0x3d, 0x3c, 0x3c, 0x3b,
	0x3a, 0x39, 0x39, 0x38, 0x37, 0x36, 0x36, 0x35,
	0x34, 0x33, 0x33, 0x32, 0x31, 0x31, 0x30, 0x2f,
	0x2e, 0x2e, 0x2d, 0x2c, 0x2c, 0x2b, 0x2a, 0x2a,
	0x29, 0x28, 0x28, 0x27, 0x26, 0x26, 0x25, 0x24,
	0x24, 0x23, 0x22, 0x22, 0x21, 0x20, 0x20, 0x1f,
	0x1e, 0x1e, 0x1d, 0x1d, 0x1c, 0x1b, 0x1b, 0x1a,
	0x19, 0x19, 0x18, 0x18, 0x17, 0x16, 0x16, 0x15,
	0x15, 0x14, 0x14, 0x13, 0x12, 0x12, 0x11, 0x11,
	0x10, 0x0f, 0x0f, 0x0e, 0x0e, 0x0d, 0x0d, 0x0c,
	0x0c, 0x0b, 0x0a, 0x0a, 0x09, 0x09, 0x08, 0x08,
	0x07, 0x07, 0x06, 0x06, 0x05, 0x05, 0x04, 0x04,
	0x03, 0x03, 0x02, 0x02, 0x01, 0x01, 0x00, 0x00,
	0x00,
}

// cmd is the state of one command: its fields, and psxe's unclamped MAC0
// and MAC3, which later steps of the same command read.
type cmd struct {
	sf, mx, v, cv int
	lm            bool
	smac0, smac3  int64
}

// el returns a matrix element, rows and columns from 1, as int64.
func (m *matrix) el(row, col int) int64 {
	i := (row-1)*3 + (col - 1)
	if i == 8 {
		return int64(m.M33)
	}
	w := m.M[i/2]
	if i%2 == 1 {
		w >>= 16
	}
	return int64(int16(w))
}

// lo and hi are the two 16-bit halves of a register, signed.
func lo(v uint32) int64 { return int64(int16(v)) }
func hi(v uint32) int64 { return int64(int16(v >> 16)) }

// byteOf is one of a color register's bytes.
func byteOf(v uint32, i uint) int64 { return int64(uint8(v >> (8 * i))) }

func (g *GTE) clampMAC0(c *cmd, v int64) int64 {
	c.smac0 = v
	if v < -0x80000000 {
		g.Flag |= 0x8000
	} else if v > 0x7fffffff {
		g.Flag |= 0x10000
	}
	return v
}

func (g *GTE) checkMAC(i int, v int64) int64 {
	if v < -0x80000000000 {
		g.Flag |= 0x8000000 >> (i - 1)
	} else if v > 0x7ffffffffff {
		g.Flag |= 0x40000000 >> (i - 1)
	}
	return (v << 20) >> 20
}

func (g *GTE) clampMAC(c *cmd, i int, v int64) int32 {
	if i == 3 {
		c.smac3 = v
	}
	return int32(g.checkMAC(i, v) >> c.sf)
}

func (g *GTE) clampIR0(v int32) int16 {
	if v < 0 {
		g.Flag |= 0x1000
		return 0
	}
	if v > 0x1000 {
		g.Flag |= 0x1000
		return 0x1000
	}
	return int16(v)
}

func (g *GTE) clampSXY(i int, v int64) int16 {
	if v < -0x400 {
		g.Flag |= 0x4000 >> (i - 1)
		return -0x400
	}
	if v > 0x3ff {
		g.Flag |= 0x4000 >> (i - 1)
		return 0x3ff
	}
	return int16(v)
}

func (g *GTE) clampSZ3(v int32) uint16 {
	if v < 0 {
		g.Flag |= 0x40000
		return 0
	}
	if v > 0xffff {
		g.Flag |= 0x40000
		return 0xffff
	}
	return uint16(v)
}

func (g *GTE) clampRGB(i int, v int32) uint32 {
	if v < 0 {
		g.Flag |= 0x200000 >> (i - 1)
		return 0
	}
	if v > 0xff {
		g.Flag |= 0x200000 >> (i - 1)
		return 0xff
	}
	return uint32(v)
}

func (g *GTE) clampIR(i int, v int64, lm bool) int16 {
	switch {
	case lm && v < 0:
		g.Flag |= 0x1000000 >> (i - 1)
		return 0
	case !lm && v < -0x8000:
		g.Flag |= 0x1000000 >> (i - 1)
		return -0x8000
	case v > 0x7fff:
		g.Flag |= 0x1000000 >> (i - 1)
		return 0x7fff
	}
	return int16(v)
}

// clampIRZ is IR3 for perspective transformation, whose overflow flag is
// checked as if the shift were 12 whatever sf says.
func (g *GTE) clampIRZ(v int64, sf int, lm bool) int16 {
	vsf := int32(v >> sf)
	v12 := int32(v >> 12)
	lower := int32(0)
	if !lm {
		lower = -0x8000
	}
	if v12 < -0x8000 || v12 > 0x7fff {
		g.Flag |= 1 << 22
	}
	return int16(min(max(vsf, lower), 0x7fff))
}

// divide is the GTE's unsigned Newton-Raphson division of H by SZ3.
func (g *GTE) divide(n, d uint16) uint32 {
	if int(n) >= int(d)*2 {
		g.Flag |= 1<<31 | 1<<17
		return 0x1ffff
	}
	shift := bits.LeadingZeros32(uint32(d)) - 16
	r1 := int(d) << shift & 0x7fff
	r2 := int(unrTable[(r1+0x40)>>7]) + 0x101
	r3 := ((0x80 - r2*(r1+0x8000)) >> 8) & 0x1ffff
	recip := uint32((r2*r3 + 0x80) >> 8)
	res := (uint64(recip)*uint64(uint32(int(n)<<shift)) + 0x8000) >> 16
	return uint32(min(res, 0x1ffff))
}

// pushRGB pushes a color onto the color FIFO, with the code byte of RGBC.
func (g *GTE) pushRGB(mac1, mac2, mac3 int32) {
	g.RGB[0] = g.RGB[1]
	g.RGB[1] = g.RGB[2]
	g.RGB[2] = g.clampRGB(1, mac1>>4) | g.clampRGB(2, mac2>>4)<<8 |
		g.clampRGB(3, mac3>>4)<<16 | g.RGBC&0xff000000
}

// setIR sets IR1 to IR3 from MAC1 to MAC3.
func (g *GTE) setIR(c *cmd) {
	g.IR[1] = g.clampIR(1, int64(g.MAC[1]), c.lm)
	g.IR[2] = g.clampIR(2, int64(g.MAC[2]), c.lm)
	g.IR[3] = g.clampIR(3, int64(g.MAC[3]), c.lm)
}

// vertex returns V0, V1 or V2.
func (g *GTE) vertex(i int) (x, y, z int64) {
	return lo(g.V[i].XY), hi(g.V[i].XY), int64(g.V[i].Z)
}

// mulAdd is the triple accumulation the matrix commands use: each partial
// sum checked for 44-bit overflow, the last clamped into a MAC.
func (g *GTE) mulAdd(c *cmd, i int, base, a, x, b, y, cc, z int64) int32 {
	return g.clampMAC(c, i, g.checkMAC(i, g.checkMAC(i, base+a*x)+b*y)+cc*z)
}

// rtp transforms vertex i by the rotation and translation and projects it,
// pushing the result onto the SZ and SXY FIFOs. With dq, it also works out
// the depth cueing factor.
func (g *GTE) rtp(c *cmd, i int, dq bool) {
	vx, vy, vz := g.vertex(i)
	rt := &g.RT
	g.MAC[1] = g.mulAdd(c, 1, int64(g.TR[0])<<12, rt.el(1, 1), vx, rt.el(1, 2), vy, rt.el(1, 3), vz)
	g.MAC[2] = g.mulAdd(c, 2, int64(g.TR[1])<<12, rt.el(2, 1), vx, rt.el(2, 2), vy, rt.el(2, 3), vz)
	g.MAC[3] = g.mulAdd(c, 3, int64(g.TR[2])<<12, rt.el(3, 1), vx, rt.el(3, 2), vy, rt.el(3, 3), vz)
	g.IR[1] = g.clampIR(1, int64(g.MAC[1]), c.lm)
	g.IR[2] = g.clampIR(2, int64(g.MAC[2]), c.lm)
	g.IR[3] = g.clampIRZ(c.smac3, c.sf, c.lm)

	g.SZ[0], g.SZ[1], g.SZ[2] = g.SZ[1], g.SZ[2], g.SZ[3]
	g.SZ[3] = g.clampSZ3(int32(c.smac3 >> 12))
	div := int64(g.divide(uint16(g.H), g.SZ[3]))

	g.SXY[0], g.SXY[1] = g.SXY[1], g.SXY[2]
	sx := g.clampSXY(1, g.clampMAC0(c, int64(int32(g.OFX))+int64(g.IR[1])*div)>>16)
	g.SXY[2] = g.SXY[2]&0xffff0000 | uint32(uint16(sx))
	sy := g.clampSXY(2, g.clampMAC0(c, int64(int32(g.OFY))+int64(g.IR[2])*div)>>16)
	g.SXY[2] = g.SXY[2]&0x0000ffff | uint32(uint16(sy))<<16

	if dq {
		g.MAC[0] = int32(g.clampMAC0(c, int64(g.DQB)+int64(g.DQA)*div))
		g.IR[0] = g.clampIR0(int32(c.smac0 >> 12))
	}
}

// lightColor is the light matrix stage the normal color commands share:
// the vertex's normal through the light matrix, then the light colors
// through the color matrix with the background color.
func (g *GTE) lightColor(c *cmd, i int) {
	vx, vy, vz := g.vertex(i)
	l := &g.L
	g.MAC[1] = g.clampMAC(c, 1, l.el(1, 1)*vx+l.el(1, 2)*vy+l.el(1, 3)*vz)
	g.MAC[2] = g.clampMAC(c, 2, l.el(2, 1)*vx+l.el(2, 2)*vy+l.el(2, 3)*vz)
	g.MAC[3] = g.clampMAC(c, 3, l.el(3, 1)*vx+l.el(3, 2)*vy+l.el(3, 3)*vz)
	g.setIR(c)
	g.colorMatrix(c)
}

// colorMatrix is the background color plus the color matrix applied to
// IR1 to IR3.
func (g *GTE) colorMatrix(c *cmd) {
	lr := &g.LR
	ir1, ir2, ir3 := int64(g.IR[1]), int64(g.IR[2]), int64(g.IR[3])
	g.MAC[1] = g.mulAdd(c, 1, int64(g.BK[0])<<12, lr.el(1, 1), ir1, lr.el(1, 2), ir2, lr.el(1, 3), ir3)
	g.MAC[2] = g.mulAdd(c, 2, int64(g.BK[1])<<12, lr.el(2, 1), ir1, lr.el(2, 2), ir2, lr.el(2, 3), ir3)
	g.MAC[3] = g.mulAdd(c, 3, int64(g.BK[2])<<12, lr.el(3, 1), ir1, lr.el(3, 2), ir2, lr.el(3, 3), ir3)
	g.setIR(c)
}

// depthCue interpolates the color in RGBC, scaled by IR1 to IR3, towards
// the far color by IR0, and pushes the result.
func (g *GTE) depthCue(c *cmd) {
	rc, gc, bc := byteOf(g.RGBC, 0)<<4, byteOf(g.RGBC, 1)<<4, byteOf(g.RGBC, 2)<<4
	ir1 := int64(g.clampIR(1, int64(g.clampMAC(c, 1, int64(g.FC[0])<<12-rc*int64(g.IR[1]))), false))
	ir2 := int64(g.clampIR(2, int64(g.clampMAC(c, 2, int64(g.FC[1])<<12-gc*int64(g.IR[2]))), false))
	ir3 := int64(g.clampIR(3, int64(g.clampMAC(c, 3, int64(g.FC[2])<<12-bc*int64(g.IR[3]))), false))
	ir0 := int64(g.IR[0])
	g.MAC[1] = g.clampMAC(c, 1, rc*int64(g.IR[1])+ir0*ir1)
	g.MAC[2] = g.clampMAC(c, 2, gc*int64(g.IR[2])+ir0*ir2)
	g.MAC[3] = g.clampMAC(c, 3, bc*int64(g.IR[3])+ir0*ir3)
	g.setIR(c)
	g.pushRGB(g.MAC[1], g.MAC[2], g.MAC[3])
}

// colorScale multiplies the color in RGBC by IR1 to IR3 and pushes it,
// then sets IR1 to IR3 from the result.
func (g *GTE) colorScale(c *cmd) {
	g.MAC[1] = g.clampMAC(c, 1, (byteOf(g.RGBC, 0)*int64(g.IR[1]))<<4)
	g.MAC[2] = g.clampMAC(c, 2, (byteOf(g.RGBC, 1)*int64(g.IR[2]))<<4)
	g.MAC[3] = g.clampMAC(c, 3, (byteOf(g.RGBC, 2)*int64(g.IR[3]))<<4)
	g.pushRGB(g.MAC[1], g.MAC[2], g.MAC[3])
	g.setIR(c)
}

// interpolate moves the values a towards the far color by IR0 and pushes
// the result: the depth cueing of DPCS, DPCT and INTPL.
func (g *GTE) interpolate(c *cmd, a1, a2, a3 int64) {
	ir1 := int64(g.clampIR(1, int64(g.clampMAC(c, 1, int64(g.FC[0])<<12-a1)), false))
	ir2 := int64(g.clampIR(2, int64(g.clampMAC(c, 2, int64(g.FC[1])<<12-a2)), false))
	ir3 := int64(g.clampIR(3, int64(g.clampMAC(c, 3, int64(g.FC[2])<<12-a3)), false))
	ir0 := int64(g.IR[0])
	g.MAC[1] = g.clampMAC(c, 1, a1+ir0*ir1)
	g.MAC[2] = g.clampMAC(c, 2, a2+ir0*ir2)
	g.MAC[3] = g.clampMAC(c, 3, a3+ir0*ir3)
	g.setIR(c)
	g.pushRGB(g.MAC[1], g.MAC[2], g.MAC[3])
}

// gteCycles is what each command costs, by its function field.
var gteCycles = map[uint32]uint32{
	0x01: 15, 0x06: 8, 0x0c: 6, 0x10: 8, 0x11: 8, 0x12: 8, 0x13: 19,
	0x14: 13, 0x16: 44, 0x1b: 17, 0x1c: 11, 0x1e: 14, 0x20: 30, 0x28: 5,
	0x29: 8, 0x2a: 17, 0x2d: 5, 0x2e: 6, 0x30: 23, 0x3d: 5, 0x3e: 5,
	0x3f: 39,
}

// Command runs a GTE command and returns what it cost in cycles, or 0 for
// one the GTE does not have, which leaves the registers alone.
func (g *GTE) Command(op uint32) uint32 {
	fn := op & 0x3f
	cycles, ok := gteCycles[fn]
	if !ok {
		return 0
	}
	c := &cmd{
		sf: int(op>>19&1) * 12,
		lm: op&0x400 != 0,
		cv: int(op >> 13 & 3),
		v:  int(op >> 15 & 3),
		mx: int(op >> 17 & 3),
	}
	g.Flag = 0

	switch fn {
	case 0x01: // RTPS
		g.rtp(c, 0, true)
	case 0x30: // RTPT
		g.rtp(c, 0, false)
		g.rtp(c, 1, false)
		g.rtp(c, 2, true)
	case 0x06: // NCLIP
		sx0, sy0 := lo(g.SXY[0]), hi(g.SXY[0])
		sx1, sy1 := lo(g.SXY[1]), hi(g.SXY[1])
		sx2, sy2 := lo(g.SXY[2]), hi(g.SXY[2])
		v := sx0*(sy1-sy2) + sx1*(sy2-sy0) + sx2*(sy0-sy1)
		g.MAC[0] = int32(g.clampMAC0(c, v))
	case 0x0c: // OP
		rt := &g.RT
		ir1, ir2, ir3 := int64(g.IR[1]), int64(g.IR[2]), int64(g.IR[3])
		g.MAC[1] = g.clampMAC(c, 1, rt.el(2, 2)*ir3-rt.el(3, 3)*ir2)
		g.MAC[2] = g.clampMAC(c, 2, rt.el(3, 3)*ir1-rt.el(1, 1)*ir3)
		g.MAC[3] = g.clampMAC(c, 3, rt.el(1, 1)*ir2-rt.el(2, 2)*ir1)
		g.setIR(c)
	case 0x10: // DPCS
		g.interpolate(c, byteOf(g.RGBC, 0)<<16, byteOf(g.RGBC, 1)<<16, byteOf(g.RGBC, 2)<<16)
	case 0x2a: // DPCT, on each color of the FIFO in turn
		for range 3 {
			g.interpolate(c, byteOf(g.RGB[0], 0)<<16, byteOf(g.RGB[0], 1)<<16, byteOf(g.RGB[0], 2)<<16)
		}
	case 0x11: // INTPL
		g.interpolate(c, int64(g.IR[1])<<12, int64(g.IR[2])<<12, int64(g.IR[3])<<12)
	case 0x12:
		g.mvmva(c)
	case 0x13: // NCDS
		g.lightColor(c, 0)
		g.depthCue(c)
	case 0x16: // NCDT
		for i := range 3 {
			g.lightColor(c, i)
			g.depthCue(c)
		}
	case 0x14: // CDP
		g.colorMatrix(c)
		g.depthCue(c)
	case 0x1b: // NCCS
		g.lightColor(c, 0)
		g.colorScale(c)
	case 0x3f: // NCCT
		for i := range 3 {
			g.lightColor(c, i)
			g.colorScale(c)
		}
	case 0x1c: // CC
		g.colorMatrix(c)
		g.colorScale(c)
	case 0x1e: // NCS
		g.ncs(c, 0)
	case 0x20: // NCT
		for i := range 3 {
			g.ncs(c, i)
		}
	case 0x28: // SQR
		ir1, ir2, ir3 := int64(g.IR[1]), int64(g.IR[2]), int64(g.IR[3])
		g.MAC[1] = g.clampMAC(c, 1, ir1*ir1)
		g.MAC[2] = g.clampMAC(c, 2, ir2*ir2)
		g.MAC[3] = g.clampMAC(c, 3, ir3*ir3)
		g.setIR(c)
	case 0x29: // DCPL
		// psxe stores these shifted, wrapping, and overwrites them below;
		// only the flags of the clamps survive.
		g.MAC[1] = g.clampMAC(c, 1, byteOf(g.RGBC, 0)*int64(g.IR[1])) << 4
		g.MAC[2] = g.clampMAC(c, 2, byteOf(g.RGBC, 1)*int64(g.IR[2])) << 4
		g.MAC[3] = g.clampMAC(c, 3, byteOf(g.RGBC, 2)*int64(g.IR[3])) << 4
		g.depthCue(c)
	case 0x2d: // AVSZ3
		avg := int64(g.ZSF3) * int64(int(g.SZ[1])+int(g.SZ[2])+int(g.SZ[3]))
		g.MAC[0] = int32(g.clampMAC0(c, avg))
		g.OTZ = g.clampSZ3(int32(avg >> 12))
	case 0x2e: // AVSZ4
		avg := int64(g.ZSF4) * int64(int(g.SZ[0])+int(g.SZ[1])+int(g.SZ[2])+int(g.SZ[3]))
		g.MAC[0] = int32(g.clampMAC0(c, avg))
		g.OTZ = g.clampSZ3(int32(avg >> 12))
	case 0x3d: // GPF
		ir0 := int64(g.IR[0])
		g.MAC[1] = g.clampMAC(c, 1, ir0*int64(g.IR[1]))
		g.MAC[2] = g.clampMAC(c, 2, ir0*int64(g.IR[2]))
		g.MAC[3] = g.clampMAC(c, 3, ir0*int64(g.IR[3]))
		g.setIR(c)
		g.pushRGB(g.MAC[1], g.MAC[2], g.MAC[3])
	case 0x3e: // GPL
		ir0 := int64(g.IR[0])
		g.MAC[1] = g.clampMAC(c, 1, int64(g.MAC[1])<<c.sf+ir0*int64(g.IR[1]))
		g.MAC[2] = g.clampMAC(c, 2, int64(g.MAC[2])<<c.sf+ir0*int64(g.IR[2]))
		g.MAC[3] = g.clampMAC(c, 3, int64(g.MAC[3])<<c.sf+ir0*int64(g.IR[3]))
		g.setIR(c)
		g.pushRGB(g.MAC[1], g.MAC[2], g.MAC[3])
	}
	return cycles
}

// ncs is NCS for one vertex: the light matrix and color matrix stages, and
// the result pushed as a color.
func (g *GTE) ncs(c *cmd, i int) {
	g.lightColor(c, i)
	g.pushRGB(g.MAC[1], g.MAC[2], g.MAC[3])
	g.setIR(c)
}

// mvmva multiplies a vector by a matrix and adds a translation, each chosen
// by the command's fields.
func (g *GTE) mvmva(c *cmd) {
	var mx matrix
	switch c.mx {
	case 0:
		mx = g.RT
	case 1:
		mx = g.L
	case 2:
		mx = g.LR
	case 3:
		// There is no fourth matrix; what the hardware uses instead, and
		// psxe copies, is made of odd pieces of other registers.
		rc := int16(byteOf(g.RGBC, 0) << 4)
		rt13 := uint16(g.RT.el(1, 3))
		rt22 := uint16(g.RT.el(2, 2))
		mx.M[0] = uint32(uint16(-rc)) | uint32(uint16(rc))<<16
		mx.M[1] = uint32(uint16(g.IR[0])) | uint32(rt13)<<16
		mx.M[2] = uint32(rt13) | uint32(rt13)<<16
		mx.M[3] = uint32(rt22) | uint32(rt22)<<16
		mx.M33 = int16(rt22)
	}

	var vx, vy, vz int64
	if c.v < 3 {
		vx, vy, vz = g.vertex(c.v)
	} else {
		vx, vy, vz = int64(g.IR[1]), int64(g.IR[2]), int64(g.IR[3])
	}

	var cv [3]int32
	switch c.cv {
	case 0:
		cv = g.TR
	case 1:
		cv = g.BK
	case 2:
		cv = g.FC
	}

	if c.cv == 2 {
		// With the far color as the translation the hardware gets it
		// wrong: the first column is computed only for its flags.
		for i := 1; i <= 3; i++ {
			g.MAC[i] = g.clampMAC(c, i, g.checkMAC(i, mx.el(i, 2)*vy)+mx.el(i, 3)*vz)
		}
		for i := 1; i <= 3; i++ {
			mac := g.clampMAC(c, i, int64(cv[i-1])<<12+mx.el(i, 1)*vx)
			g.clampIR(i, int64(mac), false)
		}
	} else {
		for i := 1; i <= 3; i++ {
			g.MAC[i] = g.mulAdd(c, i, int64(cv[i-1])<<12, mx.el(i, 1), vx, mx.el(i, 2), vy, mx.el(i, 3), vz)
		}
	}
	g.setIR(c)
}
