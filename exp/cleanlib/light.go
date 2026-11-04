//go:build yaroze

package cleanlib

import "device"

// Lighting: the parallel lights, the ambient color, fog, and the light
// mode GsSortObject4 lights polygons with.

// GsF_LIGHT is a parallel light: the direction it shines in and its color.
type GsF_LIGHT struct {
	Vx, Vy, Vz int32
	R, G, B    uint8
	_          uint8
}

// GsFOGPARAM is the depth cue: the GTE's DQA and DQB, and the far color
// a polygon fades into.
type GsFOGPARAM struct {
	Dqa           int16
	Dqb           int32
	Rfc, Gfc, Bfc uint8
	_             uint8
}

// The library's lighting variables programs reach by name: the light
// directions in world coordinates, a row for each light, and the light
// mode, which GsSetLightMode sets.

//go:extern GsLIGHTWSMATRIX
var gsLIGHTWSMATRIX MATRIX

//go:extern GsLMODE
var gsLMODE int32

// lights is the color of each light, a column of the GTE's light color
// matrix for each.
var lights MATRIX

// GsSetFlatLight sets parallel light id, 0 to 2, to the direction and
// color of lt. The light's row of GsLIGHTWSMATRIX is the direction's
// opposite, since a face is lit most when its normal points against the
// light, normalized to 4.12 fixed point. It returns 0.
//
//export GsSetFlatLight
func GsSetFlatLight(id int32, lt *GsF_LIGHT) int32 {
	if id < 0 || id > 2 {
		return 1
	}
	x, y, z := normalize(-lt.Vx, -lt.Vy, -lt.Vz)
	gsLIGHTWSMATRIX.M[id] = [3]int16{int16(x), int16(y), int16(z)}
	lights.M[0][id] = colorFraction(lt.R)
	lights.M[1][id] = colorFraction(lt.G)
	lights.M[2][id] = colorFraction(lt.B)
	setColorMatrix(&lights)
	return 0
}

// colorFraction returns the color byte c as a fraction of 255 in 4.12,
// rounded down: D0h is D0Dh, 8Ch is 8C8h and FFh is 1000h.
func colorFraction(c uint8) int16 {
	return int16(int32(c) * 4096 / 255)
}

// normalize returns x, y, z scaled to a length of 4096, by the length
// sqrt0 gives, each component rounded towards zero.
func normalize(x, y, z int32) (int32, int32, int32) {
	n := int64(sqrt0(int64(x)*int64(x) + int64(y)*int64(y) + int64(z)*int64(z)))
	if n == 0 {
		return 0, 0, 0
	}
	return int32(div64(int64(x)*4096, n)), int32(div64(int64(y)*4096, n)), int32(div64(int64(z)*4096, n))
}

// div64 returns a/b rounded toward zero, as a 32-bit division when both
// fit, since the 64-bit one is a slow library routine on this CPU.
func div64(a, b int64) int64 {
	if int64(int32(a)) == a && int64(int32(b)) == b && (a != -1<<31 || b != -1) {
		return int64(int32(a) / int32(b))
	}
	return a / b
}

// sqrt0 returns the square root of v as the reference library works it
// out, to about three digits: v shifted by an even number of bits into
// 64 to 255, the square root of that from a table with 9 fraction bits
// (floor(sqrt(m)*512)), and that shifted back by half as many bits and
// then right by 9. Shifting before dropping the fraction lets low table
// bits reach the result for large inputs, so 12 fraction bits would not
// match. Its lengths show it: 1000000 gives 999 (a light along z of
// length 1000 normalizes to 4100) and 20400 gives 142.
func sqrt0(v int64) int32 {
	if v <= 0 {
		return 0
	}
	s := 0
	for v>>s >= 256 {
		s += 2
	}
	for s <= 0 && s > -64 && v<<-s < 64 {
		s -= 2
	}
	var m int64
	if s >= 0 {
		m = v >> s
	} else {
		m = v << -s
	}
	r := isqrt(m << 18) // the square root of m with 9 fraction bits
	if s >= 0 {
		return int32(r << (s / 2) >> 9)
	}
	return int32(r >> (-s / 2) >> 9)
}

// isqrt returns the square root of v rounded down.
func isqrt(v int64) int64 {
	if v <= 0 {
		return 0
	}
	r := int64(0)
	bit := int64(1) << 62
	for bit > v {
		bit >>= 2
	}
	for bit != 0 {
		if v >= r+bit {
			v -= r + bit
			r = r>>1 + bit
		} else {
			r >>= 1
		}
		bit >>= 2
	}
	return r
}

// GsSetAmbient sets the ambient color, each component in 4.12 fixed point,
// 4096 being full brightness: the GTE's background color. As the
// reference's does, it keeps a component to a multiple of 16, an 8-bit
// brightness: 4096/3 becomes 550h.
//
//export GsSetAmbient
func GsSetAmbient(r, g, b int32) {
	setBK(r>>4<<4, g>>4<<4, b>>4<<4)
}

// GsSetLightMode sets the default light mode: 0 lights normally, 1 with
// fog.
//
//export GsSetLightMode
func GsSetLightMode(mode int32) {
	gsLMODE = mode
}

// GsSetLightMatrix loads the GTE's light matrix with the light directions
// turned into the coordinates m maps from: GsLIGHTWSMATRIX times m's
// rotation. m is normally the local-to-world matrix GsGetLw gives. The
// GTE's rotation matrix is left holding GsLIGHTWSMATRIX.
//
//export GsSetLightMatrix
func GsSetLightMatrix(m *MATRIX) {
	var l MATRIX
	MulMatrix0(&gsLIGHTWSMATRIX, m, &l)
	setLightMatrix(&l)
}

// GsSetFogParam sets the depth cue: the GTE's DQA and DQB, and its far
// color from the fog's color, each component times 16.
//
//export GsSetFogParam
func GsSetFogParam(f *GsFOGPARAM) {
	setDQ(int32(f.Dqa), f.Dqb)
	setFC(int32(f.Rfc)<<4, int32(f.Gfc)<<4, int32(f.Bfc)<<4)
}

// The GTE's lighting control registers, each loaded with a ctc2 from $gp,
// which the compiled code does not use.

// setLightMatrix loads the light matrix, control registers 8 to 12.
func setLightMatrix(m *MATRIX) {
	w := matrixWords(m)
	device.AsmFull(`.set push
	.set noat
	move $$28, {a}
	.word 0x48dc4000
	move $$28, {b}
	.word 0x48dc4800
	move $$28, {c}
	.word 0x48dc5000
	move $$28, {d}
	.word 0x48dc5800
	move $$28, {e}
	.word 0x48dc6000
	.set pop`, map[string]interface{}{"a": w[0], "b": w[1], "c": w[2], "d": w[3], "e": w[4]})
}

// setColorMatrix loads the light color matrix, control registers 16 to 20.
func setColorMatrix(m *MATRIX) {
	w := matrixWords(m)
	device.AsmFull(`.set push
	.set noat
	move $$28, {a}
	.word 0x48dc8000
	move $$28, {b}
	.word 0x48dc8800
	move $$28, {c}
	.word 0x48dc9000
	move $$28, {d}
	.word 0x48dc9800
	move $$28, {e}
	.word 0x48dca000
	.set pop`, map[string]interface{}{"a": w[0], "b": w[1], "c": w[2], "d": w[3], "e": w[4]})
}

// matrixWords packs a matrix's rotation the way the GTE's registers hold
// it: two elements to a word, row by row, the ninth alone.
func matrixWords(m *MATRIX) [5]uint32 {
	e := func(i, j int) uint32 { return uint32(uint16(m.M[i][j])) }
	return [5]uint32{
		e(0, 0) | e(0, 1)<<16,
		e(0, 2) | e(1, 0)<<16,
		e(1, 1) | e(1, 2)<<16,
		e(2, 0) | e(2, 1)<<16,
		uint32(int32(m.M[2][2])),
	}
}

// setBK loads the background color, control registers 13 to 15.
func setBK(r, g, b int32) {
	device.AsmFull(`.set push
	.set noat
	move $$28, {r}
	.word 0x48dc6800
	move $$28, {g}
	.word 0x48dc7000
	move $$28, {b}
	.word 0x48dc7800
	.set pop`, map[string]interface{}{"r": r, "g": g, "b": b})
}

// setFC loads the far color, control registers 21 to 23.
func setFC(r, g, b int32) {
	device.AsmFull(`.set push
	.set noat
	move $$28, {r}
	.word 0x48dca800
	move $$28, {g}
	.word 0x48dcb000
	move $$28, {b}
	.word 0x48dcb800
	.set pop`, map[string]interface{}{"r": r, "g": g, "b": b})
}

// setDQ loads the depth cue coefficients DQA and DQB, control registers
// 27 and 28.
func setDQ(a, b int32) {
	device.AsmFull(`.set push
	.set noat
	move $$28, {a}
	.word 0x48dcd800
	move $$28, {b}
	.word 0x48dce000
	.set pop`, map[string]interface{}{"a": a, "b": b})
}
