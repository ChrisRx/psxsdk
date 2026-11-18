//go:build yaroze

// Command mathsingle is a third probe for spec 12: the single precision
// helpers (__truncdfsf2, __divsf3, __mulsf3) over a grid of results below
// the single range, walking the shortfall (how far the result's exponent
// is below the smallest normal's) from 0 past 64 for several mantissas,
// so that the packing of an underflowed result shows bit by bit; the
// double quotient's denormals over the same kind of grid; and strtod at
// the limits of its exponent. It follows the rules of spec/probes/math:
// no floating point of its own, nothing allocated, inputs through
// volatile memory so that the compiler cannot work a call out, interrupts
// off for the arithmetic.
package main

import (
	"math"
	"runtime/volatile"
	"unsafe"

	"go.chrisrx.dev/psx/yaroze"
)

//export __divdf3
func divdf3(a, b float64) float64

//export __divsf3
func divsf3(a, b float32) float32

//export __mulsf3
func mulsf3(a, b float32) float32

//export __addsf3
func addsf3(a, b float32) float32

//export __truncdfsf2
func truncdfsf2(a float64) float32

func mark(n uint32) {
	volatile.StoreUint32((*uint32)(yaroze.Pointer(0x1f801814)), 0x10000100|n)
}

const hexDigits = "0123456789abcdef"

var (
	line  [256]byte
	nline int
)

func putc(c byte) {
	if nline < len(line) {
		line[nline] = c
		nline++
	}
	if c == '\n' {
		yaroze.Write(1, &line[0], int32(nline))
		nline = 0
	}
}

func ps(s string) {
	for i := 0; i < len(s); i++ {
		putc(s[i])
	}
}

func pi(v int64) {
	if v < 0 {
		putc('-')
		v = -v
	}
	var d [20]byte
	k := 0
	for {
		d[k] = byte('0' + v%10)
		k++
		v /= 10
		if v == 0 {
			break
		}
	}
	for k > 0 {
		k--
		putc(d[k])
	}
}

func hex64(v uint64) {
	for i := range 16 {
		putc(hexDigits[v>>(60-4*i)&15])
	}
}

func hex32(v uint32) {
	for i := range 8 {
		putc(hexDigits[v>>(28-4*i)&15])
	}
}

// The inputs go through memory so that the compiler cannot fold a call.
var (
	cur64 uint64
	cur32 uint32
)

func d(b uint64) float64 {
	volatile.StoreUint64(&cur64, b)
	return math.Float64frombits(volatile.LoadUint64(&cur64))
}

func f(b uint32) float32 {
	volatile.StoreUint32(&cur32, b)
	return math.Float32frombits(volatile.LoadUint32(&cur32))
}

// Results are logged and printed in batches, with interrupts let in only
// to print.
type entry struct {
	a, b, r uint64
}

var (
	lg [20]entry
	n  int
	nm string
)

func record(a, b, r uint64) {
	lg[n] = entry{a, b, r}
	n++
	if n == len(lg) {
		flush()
	}
}

func flush() {
	yaroze.ExitCriticalSection()
	for i := range n {
		l := &lg[i]
		ps(nm)
		ps(" ")
		if nm == "trunc" || nm == "ddiv" {
			hex64(l.a)
		} else {
			hex32(uint32(l.a))
		}
		if nm != "trunc" {
			ps(" ")
			if nm == "ddiv" {
				hex64(l.b)
			} else {
				hex32(uint32(l.b))
			}
		}
		ps(" = ")
		if nm == "ddiv" {
			hex64(l.r)
		} else {
			hex32(uint32(l.r))
		}
		ps("\n")
	}
	n = 0
	yaroze.EnterCriticalSection()
}

var group uint32

func start(name string) {
	group++
	mark(group)
	nm = name
	yaroze.ExitCriticalSection()
	ps("== ")
	ps(name)
	ps("\n")
	yaroze.EnterCriticalSection()
}

// shortfalls are how far below the smallest single normal the result's
// exponent is put: every value to 70, then a few larger ones, including
// the shortfalls hypot's open cases take (871 for 1e-300, 896 and 897 for
// 2^-1022 and 2^-1023, 1074 for 2^-1074).
var shortfalls = [...]int{
	0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26,
	27, 28, 29, 30, 31, 32, 33, 34, 35, 36, 37, 38, 39, 40, 41, 42, 43, 44, 45, 46, 47, 48, 49, 50,
	51, 52, 53, 54, 55, 56, 57, 58, 59, 60, 61, 62, 63, 64, 65, 66, 67, 68, 69, 70,
	88, 89, 90, 91, 92, 93, 94, 95, 96, 97, 100, 120, 125, 126, 127, 128, 129, 150, 200, 255, 256,
	300, 500, 870, 871, 872, 895, 896, 897, 898, 1000, 1073, 1074,
}

// dmants are the double mantissas of the grid: 1, 1.5, 1.25, 1.75, 2-ulp,
// 1+ulp, 1.6, a mixed one, 1+2^-24 (the tie for 24 bits) and the 24-bit
// 1.9999999.
var dmants = [...]uint64{
	0, 0x8000000000000, 0x4000000000000, 0xc000000000000, 0xfffffffffffff, 1, 0x999999999999a,
	0x23456789abcde, 0x0000010000000, 0xffffff0000000,
}

// smants are the single mantissas: 1, 1.5, 1.25, 1.75, 2-ulp, 1+ulp, 1.6
// and a mixed one.
var smants = [...]uint32{0, 0x400000, 0x200000, 0x600000, 0x7fffff, 1, 0x4ccccd, 0x2345678}

func runTrunc() {
	start("trunc")
	for _, sh := range shortfalls {
		e := 1023 - 126 - sh
		if e < 0 {
			continue
		}
		for _, m := range dmants {
			x := uint64(e)<<52 | m
			record(x, 0, uint64(math.Float32bits(truncdfsf2(d(x)))))
		}
	}
	flush()
}

// runDiv divides a mantissa at the bottom of the single range by a power
// of two and by 1.5 and 1+ulp times that power, so that the quotient's
// exponent is the shortfall below the range, with exact, normalized and
// inexact quotients.
func runDiv() {
	start("sdiv")
	for _, sh := range shortfalls {
		if sh > 127 {
			continue
		}
		for _, m := range smants {
			a := uint32(1)<<23 | m
			for _, bm := range [...]uint32{0, 0x400000, 1} {
				b := uint32(127+sh)<<23 | bm
				record(uint64(a), uint64(b), uint64(math.Float32bits(divsf3(f(a), f(b)))))
			}
		}
	}
	flush()
}

// runMul multiplies a mantissa at the bottom of the single range by a
// power of two below one, so that the product underflows by the
// shortfall, and a mantissa at the top by a power above one, so that it
// overflows.
func runMul() {
	start("smul")
	for _, sh := range shortfalls {
		if sh > 126 {
			continue
		}
		for _, m := range smants {
			a := uint32(1)<<23 | m
			b := uint32(127-sh) << 23
			record(uint64(a), uint64(b), uint64(math.Float32bits(mulsf3(f(a), f(b)))))
			record(uint64(a), uint64(b|0x400000), uint64(math.Float32bits(mulsf3(f(a), f(b|0x400000)))))
		}
	}
	for sh := 0; sh <= 10; sh++ {
		for _, m := range smants {
			a := uint32(254)<<23 | m
			b := uint32(127+sh) << 23
			record(uint64(a), uint64(b), uint64(math.Float32bits(mulsf3(f(a), f(b)))))
		}
	}
	flush()
}

// tieProducts are operand pairs whose product's bits below the 24-bit
// mantissa take a chosen shape on the six guard bits and the rest: an
// exact tie (the half bit alone), a near tie (the half bit, five zeros,
// then nonzero bits far down), just below and just above the half, at
// exponents from the underflow range up; and the squares hypot takes of
// its truncated ratios, where Sony's rounding of a near tie differs.
var tieProducts = [...][2]uint32{
	{0x00134800, 0x00328800}, // exact tie e=-127
	{0x00535800, 0x00062800}, // exact tie e=-127
	{0x00094800, 0x00691800}, // exact tie e=-127
	{0x00449800, 0x000c0800}, // exact tie e=-127
	{0x002a40f0, 0x001d5de0}, // near tie small rest e=-127
	{0x00351c84, 0x0076ed58}, // near tie small rest e=-127
	{0x0054ed15, 0x005dbf09}, // near tie small rest e=-127
	{0x003e2f18, 0x00123bb6}, // near tie small rest e=-127
	{0x007e0710, 0x0036c29f}, // near tie big rest e=-127
	{0x001ff5ce, 0x002bb23a}, // near tie big rest e=-127
	{0x0028dc28, 0x00737304}, // near tie big rest e=-127
	{0x00167ddc, 0x003d4332}, // near tie big rest e=-127
	{0x0014721f, 0x004c7d1d}, // just below e=-127
	{0x00273c29, 0x005baa72}, // just below e=-127
	{0x001e8def, 0x00216006}, // just below e=-127
	{0x0011488c, 0x000de9d1}, // just below e=-127
	{0x0053f83c, 0x00288d2c}, // just above e=-127
	{0x004edd5b, 0x00131440}, // just above e=-127
	{0x001fc73f, 0x00121a1d}, // just above e=-127
	{0x005f1ddd, 0x006048b5}, // just above e=-127
	{0x00498800, 0x0098e800}, // exact tie e=-126
	{0x002d3800, 0x00818800}, // exact tie e=-126
	{0x00410800, 0x009e6800}, // exact tie e=-126
	{0x001d1800, 0x0095b800}, // exact tie e=-126
	{0x0072fe7e, 0x00a91e80}, // near tie small rest e=-126
	{0x0033995c, 0x00e059b6}, // near tie small rest e=-126
	{0x0008386b, 0x00a7b6d5}, // near tie small rest e=-126
	{0x000d8721, 0x009e7080}, // near tie small rest e=-126
	{0x00317b97, 0x00ec1697}, // near tie big rest e=-126
	{0x00654040, 0x008a22da}, // near tie big rest e=-126
	{0x004cd453, 0x00e5d4fd}, // near tie big rest e=-126
	{0x0008a990, 0x00cef54d}, // near tie big rest e=-126
	{0x003a8561, 0x00b793db}, // just below e=-126
	{0x00654964, 0x00a80bcc}, // just below e=-126
	{0x003ea6cf, 0x00d8b988}, // just below e=-126
	{0x004fbc84, 0x00acb977}, // just below e=-126
	{0x002090fe, 0x0085942f}, // just above e=-126
	{0x001d25d9, 0x00803e0e}, // just above e=-126
	{0x006671ca, 0x00f1665a}, // just above e=-126
	{0x00002820, 0x00aa7c78}, // just above e=-126
	{0x00e6e800, 0x0d01c800}, // exact tie e=-100
	{0x0086e800, 0x0d416800}, // exact tie e=-100
	{0x00e1a800, 0x0d07f800}, // exact tie e=-100
	{0x00946800, 0x0d0d0800}, // exact tie e=-100
	{0x009a261d, 0x0d26dd14}, // near tie small rest e=-100
	{0x00d818aa, 0x0d727979}, // near tie small rest e=-100
	{0x00dd3ef2, 0x0d53ce73}, // near tie small rest e=-100
	{0x00cf0f87, 0x0d18f416}, // near tie small rest e=-100
	{0x00e2fd29, 0x0d200fe5}, // near tie big rest e=-100
	{0x00917e6d, 0x0d5102bb}, // near tie big rest e=-100
	{0x009211d7, 0x0d2e6d39}, // near tie big rest e=-100
	{0x00850c0d, 0x0d38da6d}, // near tie big rest e=-100
	{0x00e90a58, 0x0d4ddfea}, // just below e=-100
	{0x00f18dd1, 0x0d253b45}, // just below e=-100
	{0x0082d909, 0x0d099806}, // just below e=-100
	{0x009377fb, 0x0d26926a}, // just below e=-100
	{0x00e7c3af, 0x0d12865c}, // just above e=-100
	{0x0098940b, 0x0d48dcef}, // just above e=-100
	{0x0092f35d, 0x0d51d0b1}, // just above e=-100
	{0x00937070, 0x0d540c94}, // just above e=-100
	{0x00fb1800, 0x2b025800}, // exact tie e=-40
	{0x008cd800, 0x2b0f6800}, // exact tie e=-40
	{0x00c1d800, 0x2b0f2800}, // exact tie e=-40
	{0x00852800, 0x2b05c800}, // exact tie e=-40
	{0x00cbe052, 0x2b553e0f}, // near tie small rest e=-40
	{0x008d8e7a, 0x2b573fd4}, // near tie small rest e=-40
	{0x00e780e4, 0x2b3b1289}, // near tie small rest e=-40
	{0x008faae3, 0x2b39ab32}, // near tie small rest e=-40
	{0x008fdb2f, 0x2b3dd8e4}, // near tie big rest e=-40
	{0x00d05a9d, 0x2b0e4e86}, // near tie big rest e=-40
	{0x00fe6e19, 0x2b1542e8}, // near tie big rest e=-40
	{0x00d1645f, 0x2b7503a8}, // near tie big rest e=-40
	{0x00e6e41e, 0x2b629a5e}, // just below e=-40
	{0x008487a5, 0x2b086a89}, // just below e=-40
	{0x00b8b840, 0x2b17b4a9}, // just below e=-40
	{0x00a631ea, 0x2b7a0e3e}, // just below e=-40
	{0x0084dcf9, 0x2b2bb942}, // just above e=-40
	{0x00872e89, 0x2b19f267}, // just above e=-40
	{0x0087319d, 0x2b15c018}, // just above e=-40
	{0x00b82088, 0x2b66a27a}, // just above e=-40
	{0x008b6800, 0x3ea6f800}, // exact tie e=-1
	{0x00ee0800, 0x3e823800}, // exact tie e=-1
	{0x00881800, 0x3ebfd800}, // exact tie e=-1
	{0x0086b800, 0x3ea9a800}, // exact tie e=-1
	{0x00bd0964, 0x3e94f14e}, // near tie small rest e=-1
	{0x00b8586f, 0x3efe38e1}, // near tie small rest e=-1
	{0x00d2800a, 0x3ed53378}, // near tie small rest e=-1
	{0x008b6acb, 0x3edd80c1}, // near tie small rest e=-1
	{0x00983f20, 0x3ed05dcc}, // near tie big rest e=-1
	{0x00ff4c07, 0x3eebd816}, // near tie big rest e=-1
	{0x00bc0b60, 0x3eecff05}, // near tie big rest e=-1
	{0x00b89ecb, 0x3eb36ab4}, // near tie big rest e=-1
	{0x00a4c40c, 0x3eacf95e}, // just below e=-1
	{0x00ba8f50, 0x3ef99d2d}, // just below e=-1
	{0x00f914d6, 0x3eec5b51}, // just below e=-1
	{0x008c0164, 0x3ef21c3c}, // just below e=-1
	{0x00d3c30d, 0x3ea1a2cb}, // just above e=-1
	{0x00a283c7, 0x3ebf95af}, // just above e=-1
	{0x0086f84b, 0x3ec64568}, // just above e=-1
	{0x0092e486, 0x3ea2f5c8}, // just above e=-1
	{0x00896800, 0x3f547800}, // exact tie e=0
	{0x00888800, 0x3f11c800}, // exact tie e=0
	{0x00933800, 0x3f3bf800}, // exact tie e=0
	{0x00e77800, 0x3f011800}, // exact tie e=0
	{0x0095c3d5, 0x3f369f0e}, // near tie small rest e=0
	{0x00a3e68b, 0x3f2db5ea}, // near tie small rest e=0
	{0x009748a0, 0x3f3b2234}, // near tie small rest e=0
	{0x00fb2a00, 0x3f2325bd}, // near tie small rest e=0
	{0x009dd722, 0x3f5e64fd}, // near tie big rest e=0
	{0x0083791b, 0x3f2aef82}, // near tie big rest e=0
	{0x00a02669, 0x3f7be658}, // near tie big rest e=0
	{0x0096bb79, 0x3f65d96f}, // near tie big rest e=0
	{0x00b11845, 0x3f042933}, // just below e=0
	{0x00dc4373, 0x3f7f5ee4}, // just below e=0
	{0x00c536b4, 0x3f652559}, // just below e=0
	{0x00d71268, 0x3f407eeb}, // just below e=0
	{0x00dd3828, 0x3f105482}, // just above e=0
	{0x00aa5114, 0x3f37ef2c}, // just above e=0
	{0x0081176a, 0x3f5420f8}, // just above e=0
	{0x00b8a041, 0x3f54517a}, // just above e=0
	{0x00817800, 0x3f9e8800}, // exact tie e=1
	{0x008b5800, 0x3fdcf800}, // exact tie e=1
	{0x009da800, 0x3fa3a800}, // exact tie e=1
	{0x009e9800, 0x3f835800}, // exact tie e=1
	{0x00afad3b, 0x3f9006c4}, // near tie small rest e=1
	{0x00a107ae, 0x3fa0f2b8}, // near tie small rest e=1
	{0x0097426b, 0x3fceeb39}, // near tie small rest e=1
	{0x00845143, 0x3fb5fbbe}, // near tie small rest e=1
	{0x00cd240d, 0x3fefa852}, // near tie big rest e=1
	{0x0094ba30, 0x3ffe5175}, // near tie big rest e=1
	{0x00b49d71, 0x3fd696fc}, // near tie big rest e=1
	{0x00af3159, 0x3fb682cb}, // near tie big rest e=1
	{0x00fdf9ff, 0x3fe86226}, // just below e=1
	{0x00911e77, 0x3fe5df2d}, // just below e=1
	{0x00ea52ce, 0x3f8a68ce}, // just below e=1
	{0x0097712d, 0x3fa4585d}, // just below e=1
	{0x00ba93d6, 0x3fbb35d0}, // just above e=1
	{0x00c55f69, 0x3fff6bcd}, // just above e=1
	{0x0098d7cd, 0x3fd62a24}, // just above e=1
	{0x008bb6c5, 0x3fb0bb07}, // just above e=1
	{0x00c6e800, 0x4022a800}, // exact tie e=2
	{0x00884800, 0x402aa800}, // exact tie e=2
	{0x0087a800, 0x40502800}, // exact tie e=2
	{0x00a88800, 0x403ad800}, // exact tie e=2
	{0x00817378, 0x4018c308}, // near tie small rest e=2
	{0x009778da, 0x401da8c7}, // near tie small rest e=2
	{0x00c6c363, 0x407487fa}, // near tie small rest e=2
	{0x00a3bd2b, 0x40339463}, // near tie small rest e=2
	{0x00e4cfcc, 0x40151f7e}, // near tie big rest e=2
	{0x00e65c52, 0x406a4091}, // near tie big rest e=2
	{0x00e0231e, 0x401982e3}, // near tie big rest e=2
	{0x00f45b99, 0x404b2e32}, // near tie big rest e=2
	{0x00b6681e, 0x40307e46}, // just below e=2
	{0x00ea138c, 0x4010975b}, // just below e=2
	{0x0092678a, 0x405618e8}, // just below e=2
	{0x00be4085, 0x405f1a3e}, // just below e=2
	{0x00b6093a, 0x40448b64}, // just above e=2
	{0x00d8b1f8, 0x4000f69e}, // just above e=2
	{0x00ca0bf8, 0x400366ce}, // just above e=2
	{0x00b20226, 0x40451b76}, // just above e=2
	{0x009c4800, 0x53090800}, // exact tie e=40
	{0x00b21800, 0x531f8800}, // exact tie e=40
	{0x00cdd800, 0x531da800}, // exact tie e=40
	{0x00cb2800, 0x5305e800}, // exact tie e=40
	{0x00bb3499, 0x537981cc}, // near tie small rest e=40
	{0x009dec98, 0x534bc67e}, // near tie small rest e=40
	{0x00973e7e, 0x535e2776}, // near tie small rest e=40
	{0x0089340c, 0x5330c131}, // near tie small rest e=40
	{0x00fad5eb, 0x5306614d}, // near tie big rest e=40
	{0x0081a634, 0x5329710a}, // near tie big rest e=40
	{0x00a6faac, 0x53639584}, // near tie big rest e=40
	{0x00bfa71b, 0x53730d4a}, // near tie big rest e=40
	{0x00f393bc, 0x53114c64}, // just below e=40
	{0x00fb3d5f, 0x53565cd6}, // just below e=40
	{0x008ef73d, 0x5360390e}, // just below e=40
	{0x00d7cbc0, 0x53219d41}, // just below e=40
	{0x00ff4dd4, 0x5370e1e8}, // just above e=40
	{0x00f95844, 0x534b6c8c}, // just above e=40
	{0x00982950, 0x533e8077}, // just above e=40
	{0x00ced4f5, 0x5329509b}, // just above e=40
	{0x00c29800, 0x71260800}, // exact tie e=100
	{0x00dd4800, 0x710e2800}, // exact tie e=100
	{0x00a1e800, 0x71457800}, // exact tie e=100
	{0x008b3800, 0x713e5800}, // exact tie e=100
	{0x00be62d1, 0x710bd393}, // near tie small rest e=100
	{0x00a121c3, 0x713e6665}, // near tie small rest e=100
	{0x00a81381, 0x71015472}, // near tie small rest e=100
	{0x00d2f548, 0x715072f0}, // near tie small rest e=100
	{0x00f52aa6, 0x710dc2ab}, // near tie big rest e=100
	{0x00b1edcf, 0x716e28dc}, // near tie big rest e=100
	{0x00920611, 0x7157035b}, // near tie big rest e=100
	{0x00fe93c7, 0x71042066}, // near tie big rest e=100
	{0x00be3628, 0x7165b9a9}, // just below e=100
	{0x00efd1c2, 0x7169c499}, // just below e=100
	{0x00bc3ed7, 0x7141e2e3}, // just below e=100
	{0x00e8d21c, 0x71142c09}, // just below e=100
	{0x00b14e1d, 0x7114364e}, // just above e=100
	{0x00930ecb, 0x714fbbf6}, // just above e=100
	{0x0093068a, 0x71249f04}, // just above e=100
	{0x00be58a4, 0x7149a452}, // just above e=100
	{0x000156e2, 0x000156e2}, // hypot t*t e=-127
	{0x0000a9b1, 0x0000a9b1}, // hypot t*t e=-127
	{0x0000546a, 0x0000546a}, // hypot t*t e=-127
	{0x00005512, 0x00005512}, // hypot t*t e=-127
	{0x00002a6d, 0x00002a6d}, // hypot t*t e=-127
	{0x0000152f, 0x0000152f}, // hypot t*t e=-127
	{0x000011e9, 0x000011e9}, // hypot t*t e=-127
	{0x000023dd, 0x000023dd}, // hypot t*t e=-127
	{0x000047e2, 0x000047e2}, // hypot t*t e=-127
}

func runMulTies() {
	start("smult")
	for _, p := range tieProducts {
		record(uint64(p[0]), uint64(p[1]), uint64(math.Float32bits(mulsf3(f(p[0]), f(p[1])))))
	}
	flush()
}

// runMulGrid multiplies by 1+2^-23, which puts the other operand's
// fraction below the product's last place as it is (the product of 1.f
// and 1+2^-23 is 1.f + 2^-23 + f*2^-23, so the dropped bits are f), so
// that the bits below the half bit can be walked exactly: the half bit
// set with the next five clear and the low sixteen bits over a grid, the
// half bit with bit 16 set, and the half bit clear with everything below
// set; at an exponent in the underflow range and in the normal range,
// in both operand orders.
func runMulGrid() {
	start("smulg")
	for _, e := range [...]int{-127, 10} {
		ea := 1
		if e < 0 {
			ea = 0
		}
		eb := e + 127 - ea
		for _, top := range [...]uint32{0x400000, 0x410000, 0x3f0000} {
			for low := uint32(0); low < 0x10000; low += 0x800 {
				a := uint32(ea)<<23 | top | low
				b := uint32(eb)<<23 | 1
				record(uint64(a), uint64(b), uint64(math.Float32bits(mulsf3(f(a), f(b)))))
				record(uint64(b), uint64(a), uint64(math.Float32bits(mulsf3(f(b), f(a)))))
			}
		}
		for low := uint32(0xb000); low < 0xf800; low += 0x200 {
			a := uint32(ea)<<23 | 0x400000 | low
			b := uint32(eb)<<23 | 1
			record(uint64(a), uint64(b), uint64(math.Float32bits(mulsf3(f(a), f(b)))))
		}
		for _, low := range [...]uint32{0xbea1, 0xbfff, 0xc000, 0xc001, 0xcfff, 0xd000, 0xdfff, 0xe000, 0xecac, 0xf544, 0xffff, 0x7fff, 0x8000, 0x8001, 0x9f84} {
			a := uint32(ea)<<23 | 0x400000 | low
			b := uint32(eb)<<23 | 1
			record(uint64(a), uint64(b), uint64(math.Float32bits(mulsf3(f(a), f(b)))))
		}
	}
	flush()
}

// runAdd adds the garbage patterns hypot's steps make (a negative
// exponent run into the sign) to 4, and sums at every exponent gap.
func runAdd() {
	start("sadd")
	for _, a := range [...]uint32{
		0xc0800000, 0xc1000000, 0xc1800000, 0xe0800000, 0xff800000, 0x81000000, 0x80400000,
		0x00400000, 0x00000001, 0x007fffff, 0xfecccccd, 0xff7fffff, 0x7f800000, 0x7f7fffff,
	} {
		record(uint64(0x40800000), uint64(a), uint64(math.Float32bits(addsf3(f(0x40800000), f(a)))))
		record(uint64(a), uint64(0x40800000), uint64(math.Float32bits(addsf3(f(a), f(0x40800000)))))
	}
	flush()
}

// runDdiv is the double quotient's denormal grid: a mantissa at the
// bottom of the normal range over a power of two, 1.5 and 1+ulp times
// it, for shortfalls to 70.
func runDdiv() {
	start("ddiv")
	for _, sh := range shortfalls {
		if sh > 70 {
			continue
		}
		for _, m := range dmants {
			a := uint64(1)<<52 | m
			for _, bm := range [...]uint64{0, 0x8000000000000, 1} {
				b := uint64(1023+sh)<<52 | bm
				record(a, b, math.Float64bits(divdf3(d(a), d(b))))
			}
		}
	}
	flush()
}

// strs are strtod's strings at the limits of its exponent: where the
// result turns into an infinity or zero, whether that is judged on the
// written exponent or on the exponent less the digits after the point,
// an exponent with many digits, and the digit count at which the value's
// accumulation stops being exact.
var strs = [...]string{
	"1e254", "1e255", "1e256", "1e257", "1e-254", "1e-255", "1e-256", "1e-257",
	"0e255", "0e256", "0e-255", "0e-256", "5e255", "5e-255", "9e255", "9e-255",
	"0.00000001e260", "0.0000000001e-250", "100000000000e250", "10000000000000000000e240",
	"1234567.e255", "1.234567e261", "0.000001e261",
	"1e4294967297", "1e4294967296", "1e2147483648", "1e2147483647", "1e-2147483648", "1e-4294967295",
	"1e00000000000000000001", "1e-00000000000000000001", "1e+000255", "1e+000256",
	"1e32", "1e64", "1e128", "1e96", "1e192", "1e-32", "1e-64", "1e-128", "1e-96", "1e-192",
	"3e64", "3e128", "7e-64", "7e-128", "1e127", "1e-127", "1e129", "1e-129",
	"12345678901234567890123", "1234567890123456789012345", "0.1234567890123456789012345",
}

var sbuf [80]byte

func cstr(s string) *byte {
	k := copy(sbuf[:79], s)
	sbuf[k] = 0
	return &sbuf[0]
}

func runParse() {
	start("strtod")
	yaroze.ExitCriticalSection()
	for _, s := range strs {
		p := cstr(s)
		end := (*byte)(unsafe.Pointer(uintptr(0xdeadbeef)))
		r := yaroze.Strtod(p, &end)
		ps("strtod \"")
		ps(s)
		ps("\" = ")
		hex64(math.Float64bits(r))
		ps(" end ")
		pi(int64(int32(uintptr(unsafe.Pointer(end)) - uintptr(unsafe.Pointer(p)))))
		ps("\n")
	}
	yaroze.EnterCriticalSection()
}

func main() {
	yaroze.ResetGraph(0)
	ps("mathsingle probe\n")
	yaroze.EnterCriticalSection()
	runTrunc()
	runDiv()
	runMul()
	runMulTies()
	runMulGrid()
	runAdd()
	runDdiv()
	runParse()
	yaroze.ExitCriticalSection()
	group++
	mark(group)
	ps("mathsingle probe done\n")
	for {
		yaroze.VSync(0)
	}
}
