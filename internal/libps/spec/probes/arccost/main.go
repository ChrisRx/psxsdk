//go:build yaroze

// Command arccost is a probe for spec 21: how the time of the calls that take much
// longer on Sony's library depends on what they are given, and what GsLinkObject4
// leaves in a model and the image calls in a caller's rectangle. Run it with
// PROBE_WATCH set to the entries, which gives each call's cycles; each is called
// after a line naming the case.
package main

import (
	"unsafe"

	"go.chrisrx.dev/psx/yaroze"
)

var (
	buf   [64 << 10]uint32
	prims [32 << 10]uint32
	table [7]uint32
	obj   yaroze.GsDOBJ2
	pix   [512]uint32
	sink  float64
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

// model writes n primitives with the headers f gives, and links them.
func model(name string, n int, f func(i int) uint32) {
	p := 0
	for i := range n {
		h := f(i)
		prims[p] = h
		code, grd := h>>24&^2, h>>16&4 != 0
		size := 0
		switch code {
		case 0x20:
			size = 4
			if grd {
				size = 6
			}
		case 0x21, 0x29:
			size = 4
		case 0x24, 0x31, 0x38:
			size = 6
		case 0x25, 0x34, 0x39:
			size = 7
		case 0x28:
			size = 5
		case 0x2c, 0x2d:
			size = 8
		case 0x30:
			size = 5
			if grd {
				size = 7
			}
		case 0x35, 0x3c:
			size = 9
		case 0x3d:
			size = 11
		}
		for j := 1; j < size; j++ {
			prims[p+j] = 0
		}
		p += size
	}
	table = [7]uint32{
		0: uint32(uintptr(unsafe.Pointer(&pix[0]))),
		2: uint32(uintptr(unsafe.Pointer(&pix[0]))),
		4: uint32(uintptr(unsafe.Pointer(&prims[0]))),
		5: uint32(n),
	}
	println(name, n)
	yaroze.GsLinkObject4(&table[0], &obj, 0)
	print("  low halves:")
	p = 0
	for i := range min(n, 8) {
		print(" ", prims[p]&0xffff)
		h := f(i)
		switch h >> 24 &^ 2 {
		case 0x20:
			p += 4
			if h>>16&4 != 0 {
				p += 2
			}
		case 0x28:
			p += 5
		case 0x30:
			p += 5
			if h>>16&4 != 0 {
				p += 2
			}
		case 0x2c:
			p += 8
		}
	}
	println()
}

func models() {
	const f3, f4, g3, ft4 = uint32(0x20000304), uint32(0x28000304), uint32(0x30000406), uint32(0x2c000607)
	same := func(h uint32) func(int) uint32 { return func(int) uint32 { return h } }
	for _, n := range []int{1, 10, 100, 1000} {
		model("same f3", n, same(f3&^4))
	}
	model("same g3", 100, same(g3&^(4<<16)))
	model("same ft4", 100, same(ft4&^(4<<16)))
	model("alternate f3 f4", 100, func(i int) uint32 {
		if i&1 == 0 {
			return f3 &^ 4
		}
		return f4 &^ 4
	})
	model("flag changes", 100, func(i int) uint32 { return f3&^4 | uint32(i&1)<<17 })
	model("high flag bits change", 100, func(i int) uint32 { return f3&^4 | uint32(i&1)<<25 })
	model("semi bit changes", 100, func(i int) uint32 { return f3&^4 | uint32(i&1)<<25 })
	model("grd flag", 100, same(0x20040000))
	model("olen ilen vary", 8, func(i int) uint32 { return 0x20000000 | uint32(i) })
}

func rects() {
	r := yaroze.RECT{X: 0, Y: 0, W: 2000, H: 512}
	yaroze.DrawSync(0)
	yaroze.ClearImage(&r, 0, 0, 0)
	println("ClearImage idle leaves", r.X, r.Y, r.W, r.H)
	yaroze.DrawSync(0)
	r = yaroze.RECT{X: 0, Y: 0, W: -3, H: -4}
	yaroze.ClearImage(&r, 0, 0, 0)
	println("ClearImage negative leaves", r.X, r.Y, r.W, r.H)
	yaroze.DrawSync(0)
	r = yaroze.RECT{X: 0, Y: 0, W: 2000, H: 600}
	yaroze.LoadImage(&r, &pix[0])
	println("LoadImage leaves", r.X, r.Y, r.W, r.H)
	yaroze.DrawSync(0)
	r = yaroze.RECT{X: 0, Y: 0, W: 2000, H: 600}
	yaroze.StoreImage(&r, &pix[0])
	println("StoreImage leaves", r.X, r.Y, r.W, r.H)
	yaroze.DrawSync(0)
	r = yaroze.RECT{X: 0, Y: 0, W: 2000, H: 600}
	yaroze.MoveImage(&r, 0, 0)
	println("MoveImage leaves", r.X, r.Y, r.W, r.H)
	yaroze.DrawSync(0)
	// With the queue busy, the rectangle is taken as a copy.
	a := yaroze.RECT{X: 0, Y: 0, W: 512, H: 256}
	yaroze.ClearImage(&a, 1, 2, 3)
	r = yaroze.RECT{X: 0, Y: 0, W: 2000, H: 512}
	yaroze.ClearImage(&r, 0, 0, 0)
	println("ClearImage queued leaves", r.X, r.Y, r.W, r.H)
	yaroze.DrawSync(0)
}

func floats() {
	for _, x := range []float64{0, 1e-9, 0.25, 1, 2, 3.5, 10, 1234.5, 1e6, 1e12} {
		println("value", int(x*1000))
		sink += yaroze.Sqrt(x)
		sink += yaroze.Sin(x)
		sink += yaroze.Cos(x)
	}
}

func kanji() {
	for _, s := range []string{"\x00", "\x82\xa0\x00", "\x82\xa0\x82\xa2\x82\xa4\x82\xa6\x00", "ABCD\x00", "\x82\xa0\x82\xa2\x82\xa4\x82\xa6\x82\xa8\x82\xa9\x82\xab\x82\xad\x82\xaf\x82\xb1\x82\xb3\x82\xb5\x82\xb7\x82\xb9\x82\xbb\x82\xbd\x00"} {
		b := []byte(s)
		println("krom bytes", len(b)-1)
		yaroze.Krom2Tim(&b[0], &buf[0], 640, 0, 640, 480, 0xffff, 0)
	}
	println("kanji open")
	yaroze.KanjiFntOpen(0, 0, 256, 64, 640, 0, 640, 256, 0, 32)
}

func main() {
	yaroze.GsInitGraph(320, 240, 4, 0, 0)
	yaroze.GsDefDispBuff(0, 0, 0, 240)
	println("gettpage")
	yaroze.GetTPage(1, 0, 320, 0)
	models()
	rects()
	floats()
	kanji()
	println(int(sink))
	println("END")
	for {
		yaroze.VSync(0)
	}
}
