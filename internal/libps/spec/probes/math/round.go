//go:build yaroze

package main

import (
	"math"
	"runtime/volatile"
)

var curN uint32

// Targeted tests of the arithmetic primitives' rounding: operands lined
// up at every distance, products that land on a tie, quotients below
// the normal range, and conversions at their edges.

func mantBits(e int) uint64 { return uint64(e)<<52 | mant() }

func runAddRound() {
	if !start("addr") {
		return
	}
	for d := 0; d <= 60; d++ {
		for k := range 3 {
			a := mantBits(1023)
			b := mantBits(1023 - d)
			switch k {
			case 1:
				// Only the low bits of b set below a's last place.
				b = b&^0xff | 0x80
			case 2:
				b = uint64(1023-d)<<52 | 1
			}
			for signs := range 4 {
				x := a | uint64(signs&1)<<63
				y := b | uint64(signs>>1)<<63
				record(bits(x), bits(y), adddf3(bits(x), bits(y)), 0)
				record(bits(y), bits(x), adddf3(bits(y), bits(x)), 0)
			}
		}
		flush("add", true, 0)
		for k := range 3 {
			a := mantBits(1023)
			b := mantBits(1023 - d)
			switch k {
			case 1:
				b = b&^0xff | 0x80
			case 2:
				b = uint64(1023-d)<<52 | 1
			}
			for signs := range 4 {
				x := a | uint64(signs&1)<<63
				y := b | uint64(signs>>1)<<63
				record(bits(x), bits(y), subdf3(bits(x), bits(y)), 0)
				record(bits(y), bits(x), subdf3(bits(y), bits(x)), 0)
			}
		}
		flush("sub", true, 0)
	}
}

// runAddGuard pins how many bits below the last place a sum keeps: a
// small negative addend of just under half the larger's last place,
// short of it by 2^-k of a place, ties only where k passes the guard
// bits.
func runAddGuard() {
	if !start("addg") {
		return
	}
	for k := 2; k <= 16; k++ {
		for _, low := range [...]uint64{0, 1} {
			a := uint64(1023)<<52 | 0x123456789a00 | low
			// Half a place less 2^-k of one, in units of 2^-80: a place
			// is 2^-52.
			m := uint64(1)<<(80-53) - uint64(1)<<(80-52-k)
			e := 1023 - 80 + 52
			for m < 1<<52 {
				m <<= 1
				e--
			}
			b := uint64(e)<<52 | m&(1<<52-1)
			for signs := range 4 {
				x := a | uint64(signs&1)<<63
				y := b | uint64(signs>>1)<<63
				record(bits(x), bits(y), adddf3(bits(x), bits(y)), 0)
				record(bits(y), bits(x), adddf3(bits(y), bits(x)), 0)
			}
			flush("add", true, 0)
			for signs := range 4 {
				x := a | uint64(signs&1)<<63
				y := b | uint64(signs>>1)<<63
				record(bits(x), bits(y), subdf3(bits(x), bits(y)), 0)
				record(bits(y), bits(x), subdf3(bits(y), bits(x)), 0)
			}
			flush("sub", true, 0)
		}
	}
}

func runMulRound() {
	if !start("mulr") {
		return
	}
	for range 300 {
		// 27-bit significands, whose product's last bits are often a tie.
		a := uint64(1023+int(rnd()%5)-2)<<52 | uint64(rnd()&0x3ffffff)<<26 | uint64(rnd()&1)<<63
		b := uint64(1023+int(rnd()%5)-2)<<52 | uint64(rnd()&0x3ffffff)<<26 | uint64(rnd()&1)<<63
		record(bits(a), bits(b), muldf3(bits(a), bits(b)), 0)
	}
	flush("mul", true, 0)
	// Products near the bottom and the top of the range.
	for range 200 {
		a := mantBits(int(rnd() % 40))
		b := mantBits(1023 - 1000 + int(rnd()%60) - 30)
		if rnd()&1 == 0 {
			a = mantBits(2040 + int(rnd()%7))
			b = mantBits(1023 + int(rnd()%30))
		}
		record(bits(a), bits(b), muldf3(bits(a), bits(b)), 0)
	}
	flush("mul", true, 0)
}

func runDivRound() {
	if !start("divr") {
		return
	}
	// Quotients from just above the normal range down past the last
	// denormal, by a power of two (exact) and by a random divisor.
	for sh := -2; sh <= 56; sh++ {
		for k := range 4 {
			a := mantBits(1023)
			switch k {
			case 1:
				a = a&^0xfff | 0x800
			case 2:
				a = uint64(1023)<<52 | 1
			}
			b := uint64(1023+1022+sh) << 52
			if k == 3 {
				b |= mant()
			}
			record(bits(a), bits(b), divdf3(bits(a), bits(b)), 0)
		}
	}
	flush("div", true, 0)
}

var fixMore = [...]uint64{
	0x8000000000000001, 0x8010000000000000, 0xbcb0000000000000, 0xbfefffffffffffff, 0x800fffffffffffff,
	0x0008000000000000, 0x8008000000000000, 0x3ff0000000000001, 0xbff0000000000001, 0xc1dfffffffe00000,
	0xc1e0000000000001, 0xc1e0000000100000, 0x41dfffffffe00000, 0x41e0000000100000, 0xfff8000000000000,
	0xffefffffffffffff, 0x4320000000000000, 0xc320000000000000, 0x7ff0000000000001, 0x0000000000000000,
}

func runFixMore() {
	if !start("fixr") {
		return
	}
	for _, a := range fixMore {
		ps("fix ")
		hex64(a)
		ps(" = ")
		pi(int64(fixdfsi(bits(a))))
		ps("\n")
	}
	for range 200 {
		a := mantBits(1023+int(rnd()%40)-6) | uint64(rnd()&1)<<63
		ps("fix ")
		hex64(a)
		ps(" = ")
		pi(int64(fixdfsi(bits(a))))
		ps("\n")
	}
	for range 60 {
		a := uint64(rnd())<<32 | uint64(rnd())
		ps("fix ")
		hex64(a)
		ps(" = ")
		pi(int64(fixdfsi(bits(a))))
		ps("\n")
	}
	for range 60 {
		a := uint64(rnd())<<32 | uint64(rnd())
		ps("neg ")
		hex64(a)
		ps(" = ")
		hex64(math.Float64bits(negdf2(bits(a))))
		ps("\n")
	}
}

// runLdexpGrid covers ldexp's powers below -1023 against the exponent
// of the number, where it changes its method.
func runLdexpGrid() {
	if !start("ldexpg") {
		return
	}
	es := [...]int{0, 1, 2, 3, 10, 26, 50, 51, 52, 53, 100, 500, 1000, 1022, 1023, 1024, 1500, 2046}
	ns := [...]int32{-1023, -1024, -1025, -1030, -1050, -1070, -1072, -1073, -1074, -1075, -1076, -1100,
		-1500, -2000, -2046, -2047, -2048, -2100, -3000, -100000}
	for _, e := range es {
		for k := range 2 {
			x := uint64(e)<<52 | 0x23456789abcde
			if k == 1 {
				x |= 1 << 63
			}
			for _, n := range ns {
				// Through memory, so that the compiler cannot work the
				// call out itself.
				volatile.StoreUint64(&cur, x)
				volatile.StoreUint32(&curN, uint32(n))
				r := ldexpRaw(bits(volatile.LoadUint64(&cur)), int32(volatile.LoadUint32(&curN)))
				l := record(bits(x), 0, r, 0)
				l.x[0] = uint32(n)
			}
			flush("ldexp", false, 1)
		}
	}
}
