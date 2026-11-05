//go:build yaroze

package cleanlib

import (
	"device"
	"unsafe"
)

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

//go:extern GsLIGHT_MODE
var gsLightMode int32

// lights is the color of each light, a column of the GTE's light color
// matrix for each.
var lights MATRIX

// GsSetFlatLight sets parallel light id, 0 to 2, to the direction and
// color of lt. The light's row of GsLIGHTWSMATRIX is the direction's
// opposite, since a face is lit most when its normal points against the
// light, normalized to 4.12 fixed point, all of it in 32 bits. A direction of
// no length returns -1 and changes nothing, and any other id returns 0 having
// set no light, with the color matrix going to the GTE all the same. It returns 0.
//
//export GsSetFlatLight
func GsSetFlatLight(id int32, lt *GsF_LIGHT) int32 {
	x, y, z := lt.Vx, lt.Vy, lt.Vz
	space(flatLightSpace)
	d := sqrt0(x*x + y*y + z*z)
	if d == 0 {
		return -1
	}
	if id >= 0 && id <= 2 {
		gsLIGHTWSMATRIX.M[id] = [3]int16{int16((-x << 12) / d), int16((-y << 12) / d), int16((-z << 12) / d)}
		lights.M[0][id] = colorFraction(lt.R)
		lights.M[1][id] = colorFraction(lt.G)
		lights.M[2][id] = colorFraction(lt.B)
	}
	setColorMatrix(&lights)
	space(3)
	pace(gsSetFlatLightPad)
	return 0
}

// gsSetFlatLightPad is the rest of GsSetFlatLight's 463 cycles (spec 22).
const gsSetFlatLightPad = 26

// flatLightSpace is the time GsSetFlatLight spends before its square root.
const flatLightSpace = 36

// colorFraction returns the color byte c as a fraction of 255 in 4.12,
// rounded down: D0h is D0Dh, 8Ch is 8C8h and FFh is 1000h.
func colorFraction(c uint8) int16 {
	return int16(int32(c) * 4096 / 255)
}

// sqrt0 returns the square root of v as the library works it out, to about
// three digits: v shifted by an even number of bits into 64 to 255, the square
// root of that from a table with 9 fraction bits, and that shifted back by half
// as many bits and then right by 9. Shifting before dropping the fraction lets
// low table bits reach the result for large inputs, so 12 fraction bits would not
// match. Its lengths show it: 1000000 gives 999 (a light along z of length 1000
// normalizes to 4100) and 20400 gives 142.
//
// A sum of squares that wrapped to a negative v goes the same way, its shift by
// the count of leading ones and an arithmetic one. What it then reads is the sine
// table, which comes before the square roots in the library's memory: over 46,341
// apart, Sony's lengths come from there.
func sqrt0(v int32) int32 {
	if v == 0 {
		return 0
	}
	// A count of 24 puts the top bit at bit 7, and 25 at bit 6.
	s := (25 - leadingZeros(v)) &^ 1
	var m int32
	if s >= 0 {
		m = v >> s
	} else {
		m = v << -s
	}
	var t int32
	if v < 0 {
		t = int32(sineTable[sineBeforeSqrt+m])
	} else {
		t = int32(sqrtTable[m-64])
	}
	if s >= 0 {
		return t << (s / 2) >> 9
	}
	return t >> (-s / 2) >> 9
}

// sineBeforeSqrt is the entry of sineTable that a shifted v of 0 would read.
const sineBeforeSqrt = 970

// GsSetAmbient sets the ambient color, each component in 4.12 fixed point,
// 4096 being full brightness: the GTE's background color. As the
// reference's does, it keeps a component to a multiple of 16, an 8-bit
// brightness: 4096/3 becomes 550h.
//
//export GsSetAmbient
func GsSetAmbient(r, g, b int32) {
	setBK(r>>4<<4, g>>4<<4, b>>4<<4)
	space(2)
}

// GsSetLightMode sets the default light mode: 0 lights normally, 1 with
// fog. Above 3 or below 0 it has the kernel print a message and changes nothing.
//
//export GsSetLightMode
func GsSetLightMode(mode int32) {
	if mode < 0 || mode > 3 {
		libps_biosA(biosPrintf, uintptr(unsafe.Pointer(unsafe.StringData(badLightMode))), uintptr(mode), 0, 0)
		return
	}
	gsLightMode = mode
}

// badLightMode is what GsSetLightMode has the kernel print for a mode it does not
// know.
const badLightMode = "not supported light mode %d\n\x00"

// GsSetLightMatrix loads the GTE's light matrix with the light directions
// turned into the coordinates m maps from: GsLIGHTWSMATRIX times m's
// rotation. m is normally the local-to-world matrix GsGetLw gives. The
// GTE's rotation matrix is left as it was: a program may have loaded the matrix
// to draw with before this call (test-mot001-mot001 does). In assembly.
//
//export GsSetLightMatrix
func GsSetLightMatrix(m *MATRIX)

// SetLightMatrix loads the GTE's light matrix from m's rotation and nothing
// else: GsSetLightMatrix multiplies first.
//
//export SetLightMatrix
func SetLightMatrix(m *MATRIX) { setLightMatrix(m) }

// GsSetFogParam sets the depth cue: the GTE's DQA and DQB, and its far
// color from the fog's color, each component times 16.
//
//export GsSetFogParam
func GsSetFogParam(f *GsFOGPARAM) {
	setDQ(int32(f.Dqa), f.Dqb)
	setFC(int32(f.Rfc)<<4, int32(f.Gfc)<<4, int32(f.Bfc)<<4)
	space(2)
}

// The GTE's lighting control registers, each loaded with a ctc2 from $gp,
// which gets its own value back at once (see the comment in gte.go).

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
	move $$28, {gp}
	.set pop`, map[string]interface{}{"gp": readGP(), "a": w[0], "b": w[1], "c": w[2], "d": w[3], "e": w[4]})
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
	move $$28, {gp}
	.set pop`, map[string]interface{}{"gp": readGP(), "a": w[0], "b": w[1], "c": w[2], "d": w[3], "e": w[4]})
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
	move $$28, {gp}
	.set pop`, map[string]interface{}{"gp": readGP(), "r": r, "g": g, "b": b})
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
	move $$28, {gp}
	.set pop`, map[string]interface{}{"gp": readGP(), "r": r, "g": g, "b": b})
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
	move $$28, {gp}
	.set pop`, map[string]interface{}{"gp": readGP(), "a": a, "b": b})
}
