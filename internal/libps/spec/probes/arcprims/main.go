//go:build yaroze

// Command arcprims is a probe for spec 20: the packets GsSortSprite,
// GsSortFastSprite, GsSortClear and GsSortFixBg16 make for what the web archive's
// programs pass them, and what GsGetTimInfo, GsClearOt, GsGetLs and LoadImage do
// with no memory behind a pointer. The ordering table is walked from its Tag, and
// each packet printed as its words without its link, since addresses differ.
package main

import (
	"unsafe"

	"go.chrisrx.dev/psx/yaroze"
)

var (
	packets [8192]uint32
	tags    [1 << 2]yaroze.GsOT_TAG
	ot      = yaroze.GsOT{Length: 2, Org: &tags[0]}
	bgWork  [4096]uint32
	cells   [4]yaroze.GsCELL
	index   [8 * 8]uint16
	bgMap   yaroze.GsMAP
)

func hex(w uint32) string {
	const digits = "0123456789abcdef"
	var b [8]byte
	for i := 7; i >= 0; i-- {
		b[i] = digits[w&15]
		w >>= 4
	}
	return string(b[:])
}

func start() {
	yaroze.GsClearOt(0, 0, &ot)
	yaroze.GsSetWorkBase((*yaroze.PACKET)(unsafe.Pointer(&packets[0])))
}

// dump prints the first packets of the table, and how many packets and words it
// holds in all.
func dump(first int) {
	a := uintptr(unsafe.Pointer(ot.Tag))
	count, total := 0, 0
	for {
		tag := *(*uint32)(unsafe.Pointer(a | 0x80000000))
		n := int(tag >> 24)
		if n > 0 {
			if count < first {
				print("  ", n, ":")
				for i := 1; i <= n; i++ {
					print(" ", hex(*(*uint32)(unsafe.Pointer(a | 0x80000000 + uintptr(i)*4))))
				}
				println()
			}
			count++
			total += n
		}
		if tag&0xffffff == 0xffffff {
			break
		}
		a = uintptr(tag & 0xffffff)
	}
	println("  packets", count, "words", total)
}

func sprite(name string, f func(*yaroze.GsSPRITE)) {
	sp := yaroze.GsSPRITE{
		X:      100,
		Y:      60,
		W:      32,
		H:      16,
		Tpage:  5,
		U:      8,
		V:      4,
		Cx:     32,
		Cy:     480,
		R:      10,
		G:      20,
		B:      30,
		Mx:     2,
		My:     3,
		Scalex: 4096,
		Scaley: 4096,
	}
	f(&sp)
	println(name)
	start()
	yaroze.GsSortSprite(&sp, &ot, 1)
	dump(2)
	start()
	yaroze.GsSortFastSprite(&sp, &ot, 1)
	dump(2)
}

func sprites() {
	const rotOff, hflip, vflip, semi, bright = 1 << 27, 1 << 23, 1 << 22, 1 << 30, 1 << 6
	for tp := range uint32(4) {
		for page := range uint16(3) {
			sprite("depth attr "+hex(tp)+" tpage "+hex(uint32(page)), func(s *yaroze.GsSPRITE) {
				s.Attribute = tp << 24
				s.Tpage = 5 | page<<7
			})
		}
	}
	sprite("rate in tpage", func(s *yaroze.GsSPRITE) { s.Tpage = 5 | 3<<5 })
	sprite("rate in attr", func(s *yaroze.GsSPRITE) { s.Attribute = 2 << 28 })
	sprite("semi", func(s *yaroze.GsSPRITE) { s.Attribute = semi })
	sprite("bright off", func(s *yaroze.GsSPRITE) { s.Attribute = bright })
	sprite("scale 0", func(s *yaroze.GsSPRITE) { s.Scalex, s.Scaley = 0, 0 })
	sprite("scale x 0", func(s *yaroze.GsSPRITE) { s.Scalex = 0 })
	sprite("scale twice", func(s *yaroze.GsSPRITE) { s.Scalex, s.Scaley = 8192, 8192 })
	sprite("hflip", func(s *yaroze.GsSPRITE) { s.Attribute = hflip })
	sprite("vflip", func(s *yaroze.GsSPRITE) { s.Attribute = vflip })
	sprite("both flips with depth", func(s *yaroze.GsSPRITE) { s.Attribute = hflip | vflip | 1<<24; s.Tpage = 5 | 2<<7 })
	sprite("rotoff with rotate and scale", func(s *yaroze.GsSPRITE) {
		s.Attribute = rotOff
		s.Rotate = 90 * 4096
		s.Scalex = 8192
	})
	sprite("rotoff with flip", func(s *yaroze.GsSPRITE) { s.Attribute = rotOff | hflip })
	sprite("rotate 90", func(s *yaroze.GsSPRITE) { s.Rotate = 90 * 4096 })
	sprite("rotate -30", func(s *yaroze.GsSPRITE) { s.Rotate = -30 * 4096 })
	sprite("rotate depth", func(s *yaroze.GsSPRITE) { s.Rotate = 45 * 4096; s.Attribute = 2 << 24; s.Tpage = 5 | 1<<7 })
	sprite("wide", func(s *yaroze.GsSPRITE) { s.W, s.H = 256, 256; s.Rotate = 4096 })
	sprite("cx low bits", func(s *yaroze.GsSPRITE) { s.Cx = 37 })
}

func clears() {
	println("clear")
	start()
	yaroze.GsSortClear(1, 2, 3, &ot)
	dump(2)
	yaroze.GsDRAWENV().Clip = yaroze.RECT{X: 10, Y: 20, W: 30, H: 40}
	println("clear with a clip of its own")
	start()
	yaroze.GsSortClear(1, 2, 3, &ot)
	dump(2)
	yaroze.GsSetDrawBuffClip()
}

func background(name string, f func(*yaroze.GsBG)) {
	bg := yaroze.GsBG{
		W:      320,
		H:      240,
		R:      128,
		G:      128,
		B:      128,
		Map:    &bgMap,
		Scalex: 4096,
		Scaley: 4096,
	}
	f(&bg)
	println(name)
	yaroze.GsInitFixBg16(&bg, &bgWork[0])
	for range 2 {
		start()
		yaroze.GsSortFixBg16(&bg, &bgWork[0], &ot, 1)
		dump(4)
		yaroze.GsSwapDispBuff()
	}
}

func backgrounds() {
	for i := range cells {
		cells[i] = yaroze.GsCELL{
			U:     uint8(16 * i),
			V:     uint8(8 * i),
			Cba:   uint16(0x7800 + i),
			Flag:  uint16(i),
			Tpage: uint16(5 + i),
		}
	}
	for i := range index {
		index[i] = uint16(i % 4)
	}
	index[1] = 0xffff
	bgMap = yaroze.GsMAP{
		Cellw:  16,
		Cellh:  16,
		Ncellw: 8,
		Ncellh: 8,
		Base:   &cells[0],
		Index:  &index[0],
	}
	background("bg", func(b *yaroze.GsBG) {})
	background("bg at 160,120 and smaller", func(b *yaroze.GsBG) { b.X, b.Y, b.W, b.H = 160, 120, 64, 32 })
	background("bg dark", func(b *yaroze.GsBG) { b.R, b.G, b.B = 40, 50, 60 })
	background("bg scrolled", func(b *yaroze.GsBG) { b.Scrollx, b.Scrolly = 37, -5 })
	background("bg semi", func(b *yaroze.GsBG) { b.Attribute = 1 << 30 })
	background("bg bright off", func(b *yaroze.GsBG) { b.Attribute = 1 << 6 })
	background("bg depth and rate", func(b *yaroze.GsBG) { b.Attribute = 1<<24 | 2<<28 })
}

func tim() {
	im := yaroze.GsIMAGE{
		Cx:   11,
		Cy:   22,
		Cw:   33,
		Ch:   44,
		Clut: (*uint32)(unsafe.Pointer(uintptr(0x12345678))),
	}
	raw := [...]uint32{2, 12 + 4, 0x00100020, 0x00040002, 1, 2}
	yaroze.GsGetTimInfo(&raw[0], &im)
	println("tim with no palette", hex(im.Pmode), im.Px, im.Py, im.Pw, im.Ph, im.Pixel == (*uint32)(unsafe.Pointer(&raw[4])))
	println("  palette fields", im.Cx, im.Cy, im.Cw, im.Ch, hex(uint32(uintptr(unsafe.Pointer(im.Clut)))))
}

func nulls() {
	low := (*[32]uint32)(unsafe.Pointer(uintptr(0x80000000)))
	saved := *low
	var empty yaroze.GsOT
	yaroze.GsClearOt(1, 2, &empty)
	println("GsClearOt with no memory", hex(uint32(uintptr(unsafe.Pointer(empty.Tag)))), empty.Offset, empty.Point, hex(low[0]))
	*low = saved
	var m yaroze.MATRIX
	yaroze.GsGetLs(nil, &m)
	println("GsGetLs of WORLD", hex(low[0x48/4]))
	*low = saved
	r := yaroze.RECT{X: 960, Y: 500, W: 2, H: 1}
	println("LoadImage from address 0", yaroze.LoadImage(&r, nil))
	yaroze.DrawSync(0)
	r = yaroze.RECT{X: 960, Y: 500, W: -2, H: 4}
	println("LoadImage negative width", yaroze.LoadImage(&r, nil))
	yaroze.DrawSync(0)
	r = yaroze.RECT{X: 960, Y: 500, W: 0, H: 22}
	println("LoadImage no width", yaroze.LoadImage(&r, nil))
	yaroze.DrawSync(0)
}

func main() {
	yaroze.GsInitGraph(320, 240, 4, 0, 0)
	yaroze.GsDefDispBuff(0, 0, 0, 240)
	yaroze.GsInit3D()
	sprites()
	clears()
	backgrounds()
	tim()
	nulls()
	println("END")
	for {
		yaroze.VSync(0)
	}
}
