package cdrom

// Error detection and correction for CD-ROM sectors, as ECMA-130 (the
// Yellow Book) specifies them: the EDC in its clause 14.3, and the ECC in
// its Annex A.
//
// The EDC is a 32-bit CRC that says whether a sector is intact. The ECC is
// two Reed-Solomon codes, P and Q, laid over the sector as a product code,
// which can repair one. The ECC covers the EDC, so the EDC is computed
// first.

import "hash/crc32"

// The EDC polynomial is P(x) = (x^16 + x^15 + x^2 + 1)(x^16 + x^2 + x + 1),
// and the CRC is taken least significant bit first, from zero, with no
// complementing at either end. This is the polynomial reversed for that bit
// order.
var edcTable = crc32.MakeTable(0xd8018001)

// edc computes the CRC stored at the end of a sector's user data.
func edc(data []byte) uint32 {
	// crc32 complements the running value on the way in and out, as the
	// usual CRC-32 does. Starting from a complemented zero undoes the first,
	// and complementing the result undoes the second.
	return ^crc32.Update(^uint32(0), edcTable, data)
}

// The ECC is computed in GF(2^8) with the primitive polynomial
// P(x) = x^8 + x^4 + x^3 + x^2 + 1, which is 0x11d with its top bit. gfExp
// holds the powers of the primitive element α, and gfLog their logarithms,
// which is how two elements are multiplied.
var gfExp, gfLog [256]byte

func init() {
	v := 1
	for i := range 255 {
		gfExp[i] = byte(v)
		gfLog[v] = byte(i)
		v <<= 1
		if v&0x100 != 0 {
			v ^= 0x11d
		}
	}
	gfInvAlphaPlusOne = gfExp[(255-int(gfLog[0x03]))%255]
}

func gfMul(a, b byte) byte {
	if a == 0 || b == 0 {
		return 0
	}
	return gfExp[(int(gfLog[a])+int(gfLog[b]))%255]
}

// rsParity returns the two parity symbols that complete a codeword of n
// symbols, the n-2 data symbols being symbol(0) to symbol(n-3). Annex A's
// parity check matrix has a row of ones and a row of descending powers of
// α, so the codeword c, data then parity, satisfies
//
//	Σ c[i] = 0  and  Σ c[i]·α^(n-1-i) = 0.
//
// With s and t the two sums over the data alone, the parity p0 and p1 that
// sit at the end solve p0 + p1 = s and p0·α + p1 = t, so p0 = (s + t)/(α + 1)
// and p1 = s + p0.
func rsParity(n int, symbol func(i int) byte) (p0, p1 byte) {
	var s, t byte
	for i := range n - 2 {
		d := symbol(i)
		s ^= d
		t ^= gfMul(d, gfExp[(n-1-i)%255])
	}
	p0 = gfMul(s^t, gfInvAlphaPlusOne)
	return p0, s ^ p0
}

// gfInvAlphaPlusOne is 1/(α + 1), and α + 1 is 0x03. It is set by init once
// the tables are, which a package-level initializer would run before.
var gfInvAlphaPlusOne byte

// The product code's geometry, from Annex A. The 2064 bytes from the header
// through the EDC are taken as 1032 16-bit words, and the low bytes and the
// high bytes of those words make two separate arrays, each of 24 rows of 43
// symbols. A P codeword is a column of one array: 24 symbols, with its two
// parity symbols in rows 24 and 25, which is where the P parity area lies
// in the sector. A Q codeword is a diagonal through the 26 rows that the P
// parity has made: 43 symbols, each one row down and one column to the
// right of the last, wrapping round, with its parity in the Q area after.
const (
	eccColumns  = 43
	eccRows     = 24
	eccPWords   = (eccRows + 2) * eccColumns // the words a Q codeword runs over
	eccDiagonal = eccColumns + 1             // the step from one Q symbol to the next
)

// generateECC fills in the P and Q parity of a Mode 2 Form 1 sector.
//
// The four bytes of the address header are taken as zero. In Mode 2 the
// ECC does not protect them, so that a reader can correct a sector before
// it has trusted the header to tell it which sector it is.
func generateECC(sector []byte) {
	// From the header through the P parity: everything a Q codeword can
	// run over, a word being two bytes, one in each array.
	area := sector[headerOffset:eccQOffset]
	symbol := func(word, array int) byte {
		i := 2*word + array
		if i < 4 {
			return 0
		}
		return area[i]
	}

	for array := range 2 {
		for col := range eccColumns {
			p0, p1 := rsParity(eccRows+2, func(row int) byte {
				return symbol(row*eccColumns+col, array)
			})
			area[2*(eccRows*eccColumns+col)+array] = p0
			area[2*((eccRows+1)*eccColumns+col)+array] = p1
		}
	}

	q := sector[eccQOffset:]
	for array := range 2 {
		for diag := range eccRows + 2 {
			p0, p1 := rsParity(eccColumns+2, func(i int) byte {
				return symbol((diag*eccColumns+i*eccDiagonal)%eccPWords, array)
			})
			q[2*diag+array] = p0
			q[2*(eccRows+2+diag)+array] = p1
		}
	}
}
