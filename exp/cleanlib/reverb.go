//go:build yaroze

package cleanlib

// The reverb: the SPU's own, set to one of the library's presets.

// reverbTypes are the presets for SS_REV_TYPE_OFF to SS_REV_TYPE_PIPE:
// the 32 reverb registers from 1C0h, then the work area's start in 8-byte
// units, as the reference library writes them.
var reverbTypes = [10]struct {
	regs [32]uint16
	base uint16
}{
	{base: 0xfffe},
	{[32]uint16{0x007d, 0x005b, 0x6d80, 0x54b8, 0xbed0, 0x0000, 0x0000, 0xba80, 0x5800, 0x5300, 0x04d6, 0x0333, 0x03f0, 0x0227, 0x0374, 0x01ef, 0x0334, 0x01b5, 0x0000, 0x0000, 0x0000, 0x0000, 0x0000, 0x0000, 0x0000, 0x0000, 0x01b4, 0x0136, 0x00b8, 0x005c, 0x8000, 0x8000}, 0xfb28},
	{[32]uint16{0x0033, 0x0025, 0x70f0, 0x4fa8, 0xbce0, 0x4410, 0xc0f0, 0x9c00, 0x5280, 0x4ec0, 0x03e4, 0x031b, 0x03a4, 0x02af, 0x0372, 0x0266, 0x031c, 0x025d, 0x025c, 0x018e, 0x022f, 0x0135, 0x01d2, 0x00b7, 0x018f, 0x00b5, 0x00b4, 0x0080, 0x004c, 0x0026, 0x8000, 0x8000}, 0xfc18},
	{[32]uint16{0x00b1, 0x007f, 0x70f0, 0x4fa8, 0xbce0, 0x4510, 0xbef0, 0xb4c0, 0x5280, 0x4ec0, 0x0904, 0x076b, 0x0824, 0x065f, 0x07a2, 0x0616, 0x076c, 0x05ed, 0x05ec, 0x042e, 0x050f, 0x0305, 0x0462, 0x02b7, 0x042f, 0x0265, 0x0264, 0x01b2, 0x0100, 0x0080, 0x8000, 0x8000}, 0xf6f8},
	{[32]uint16{0x00e3, 0x00a9, 0x6f60, 0x4fa8, 0xbce0, 0x4510, 0xbef0, 0xa680, 0x5680, 0x52c0, 0x0dfb, 0x0b58, 0x0d09, 0x0a3c, 0x0bd9, 0x0973, 0x0b59, 0x08da, 0x08d9, 0x05e9, 0x07ec, 0x04b0, 0x06ef, 0x03d2, 0x05ea, 0x031d, 0x031c, 0x0238, 0x0154, 0x00aa, 0x8000, 0x8000}, 0xf204},
	{[32]uint16{0x01a5, 0x0139, 0x6000, 0x5000, 0x4c00, 0xb800, 0xbc00, 0xc000, 0x6000, 0x5c00, 0x15ba, 0x11bb, 0x14c2, 0x10bd, 0x11bc, 0x0dc1, 0x11c0, 0x0dc3, 0x0dc0, 0x09c1, 0x0bc4, 0x07c1, 0x0a00, 0x06cd, 0x09c2, 0x05c1, 0x05c0, 0x041a, 0x0274, 0x013a, 0x8000, 0x8000}, 0xea44},
	{[32]uint16{0x033d, 0x0231, 0x7e00, 0x5000, 0xb400, 0xb000, 0x4c00, 0xb000, 0x6000, 0x5400, 0x1ed6, 0x1a31, 0x1d14, 0x183b, 0x1bc2, 0x16b2, 0x1a32, 0x15ef, 0x15ee, 0x1055, 0x1334, 0x0f2d, 0x11f6, 0x0c5d, 0x1056, 0x0ae1, 0x0ae0, 0x07a2, 0x0464, 0x0232, 0x8000, 0x8000}, 0xe128},
	{[32]uint16{0x0001, 0x0001, 0x7fff, 0x7fff, 0x0000, 0x0000, 0x0000, 0x8100, 0x0000, 0x0000, 0x1fff, 0x0fff, 0x1005, 0x0005, 0x0000, 0x0000, 0x1005, 0x0005, 0x0000, 0x0000, 0x0000, 0x0000, 0x0000, 0x0000, 0x0000, 0x0000, 0x1004, 0x1002, 0x0004, 0x0002, 0x8000, 0x8000}, 0xcff8},
	{[32]uint16{0x0001, 0x0001, 0x7fff, 0x7fff, 0x0000, 0x0000, 0x0000, 0x0000, 0x0000, 0x0000, 0x1fff, 0x0fff, 0x1005, 0x0005, 0x0000, 0x0000, 0x1005, 0x0005, 0x0000, 0x0000, 0x0000, 0x0000, 0x0000, 0x0000, 0x0000, 0x0000, 0x1004, 0x1002, 0x0004, 0x0002, 0x8000, 0x8000}, 0xcff8},
	{[32]uint16{0x0017, 0x0013, 0x70f0, 0x4fa8, 0xbce0, 0x4510, 0xbef0, 0x8500, 0x5f80, 0x54c0, 0x0371, 0x02af, 0x02e5, 0x01df, 0x02b0, 0x01d7, 0x0358, 0x026a, 0x01d6, 0x011e, 0x012d, 0x00b1, 0x011f, 0x0059, 0x01a0, 0x00e3, 0x0058, 0x0040, 0x0028, 0x0014, 0x8000, 0x8000}, 0xf880},
}

// spuReverbOn is SPUCNT's reverb master enable.
const spuReverbOn = 1 << 7

var reverbType int16

// SsUtSetReverbType sets the reverb to one of the presets with its depth
// at zero, turning it off for SS_REV_TYPE_OFF, and returns the type, or -1
// for none. A negative type names the preset of its size, and is what
// SsUtGetReverbType reports with bit 8 set (spec 23).
//
//export SsUtSetReverbType
func SsUtSetReverbType(t int16) int32 {
	soundStart()
	kept := t
	if t < 0 {
		t = -t
		kept = t | reverbNegative
	}
	if t < 0 || int(t) >= len(reverbTypes) {
		return -1
	}
	reverbType = kept
	if t == 0 {
		spuSet(spuCtrl, spuGet(spuCtrl)&^spuReverbOn)
	}
	spuSet(spuRevL, 0)
	spuSet(spuRevR, 0)
	p := &reverbTypes[t]
	for i, r := range p.regs {
		spuSet(spuRevRegs+uint32(i)*2, r)
	}
	spuSet(spuRevBase, p.base)
	return int32(t)
}

// reverbNegative marks a type SsUtSetReverbType was given as a negative.
const reverbNegative = 1 << 8

// SsUtGetReverbType returns the type SsUtSetReverbType last set.
//
//export SsUtGetReverbType
func SsUtGetReverbType() int32 { return int32(reverbType) }

// SsUtReverbOn turns the reverb on.
//
//export SsUtReverbOn
func SsUtReverbOn() {
	soundStart()
	spuSet(spuCtrl, spuGet(spuCtrl)|spuReverbOn)
	pace(ssUtReverbOnPad)
}

// SsUtReverbOff turns the reverb off.
//
//export SsUtReverbOff
func SsUtReverbOff() {
	soundStart()
	spuSet(spuCtrl, spuGet(spuCtrl)&^spuReverbOn)
}

// SsUtSetReverbDepth sets the reverb's depth, left and right, 0 to 127.
//
//export SsUtSetReverbDepth
func SsUtSetReverbDepth(l, r int16) {
	soundStart()
	spuSet(spuRevL, uint16(int32(l)*0x7fff/127))
	spuSet(spuRevR, uint16(int32(r)*0x7fff/127))
	pace(ssUtSetReverbDepthPad)
}

// The rest of these calls' cycles on Sony's, for spec 21: SsUtReverbOn's 137,
// SsUtSetReverbDepth's 248, SsUtSetReverbDelay's 827 and SsUtSetReverbFeedback's
// 782, and 188 for either on a type they write nothing for.
const (
	ssUtReverbOnPad          = 98
	ssUtSetReverbDepthPad    = 158
	ssUtSetReverbDelayPad    = 746
	ssUtSetReverbFeedbackPad = 722
	ssUtSetReverbNoTypePad   = 146
)

// The echo and delay types are the ones whose delay and feedback can be
// set; for the others SsUtSetReverbDelay and SsUtSetReverbFeedback write
// nothing.
const (
	revTypeEcho  = 7
	revTypeDelay = 8
)

// SsUtSetReverbDelay sets the echo and delay types' delay, 0 to 127: u =
// d·4096/127 rounded toward zero, and from it six of the reverb's
// registers: 1D4h = 2u-1, 1D6h = u-1, 1D8h and 1E0h = u+5, 1F4h = u+4,
// 1F6h = u+2 (127 gives the presets' own values).
//
//export SsUtSetReverbDelay
func SsUtSetReverbDelay(d int16) {
	soundStart()
	if reverbType != revTypeEcho && reverbType != revTypeDelay {
		pace(ssUtSetReverbNoTypePad)
		return
	}
	u := int32(d) * 4096 / 127
	spuSet(0x1d4, uint16(2*u-1))
	spuSet(0x1d6, uint16(u-1))
	spuSet(0x1d8, uint16(u+5))
	spuSet(0x1e0, uint16(u+5))
	spuSet(0x1f4, uint16(u+4))
	spuSet(0x1f6, uint16(u+2))
	pace(ssUtSetReverbDelayPad)
}

// SsUtSetReverbFeedback sets the echo and delay types' feedback, 0 to
// 127: register 1CEh = f·8100h/127, rounded toward zero (127 gives the
// echo preset's 8100h).
//
//export SsUtSetReverbFeedback
func SsUtSetReverbFeedback(f int16) {
	soundStart()
	if reverbType != revTypeEcho && reverbType != revTypeDelay {
		pace(ssUtSetReverbNoTypePad)
		return
	}
	spuSet(0x1ce, uint16(int32(f)*0x8100/127))
	pace(ssUtSetReverbFeedbackPad)
}
