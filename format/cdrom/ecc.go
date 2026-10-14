package cdrom

// Error detection and correction for CD-ROM sectors, as specified by the
// Yellow Book and ECMA-130.
//
// Two independent mechanisms are involved. The EDC is a 32-bit CRC that only
// says whether a sector is intact. The ECC is a pair of Reed-Solomon codes, P
// and Q, that can actually repair one. They are generated in that order,
// because the ECC covers the EDC.

// edcPoly is the CRC polynomial x^32 + x^31 + x^16 + x^15 + x^4 + x^3 + x + 1
// in the reversed (least significant bit first) representation the CRC is
// evaluated in.
const edcPoly = 0xd8018001

var edcTable [256]uint32

// eccFLUT and eccBLUT multiply by and divide by x in GF(2^8), the field the
// Reed-Solomon parity is computed over. Its primitive polynomial is
// x^8 + x^4 + x^3 + x^2 + 1, so a shift that overflows is reduced by 0x11d.
var eccFLUT, eccBLUT [256]byte

func init() {
	for i := range edcTable {
		v := uint32(i)
		for range 8 {
			if v&1 != 0 {
				v = (v >> 1) ^ edcPoly
			} else {
				v >>= 1
			}
		}
		edcTable[i] = v
	}

	for i := range 256 {
		// Multiplying by x is a left shift, reduced by the primitive
		// polynomial when it overflows. Truncating to a byte already drops
		// the overflowing bit, so only the low 8 bits of 0x11d remain to
		// be applied.
		j := byte(i) << 1
		if i&0x80 != 0 {
			j ^= 0x1d
		}
		eccFLUT[i] = j
		eccBLUT[byte(i)^j] = byte(i)
	}
}

// edc computes the CRC stored at the end of a sector's user data.
func edc(data []byte) uint32 {
	var v uint32
	for _, b := range data {
		v = (v >> 8) ^ edcTable[byte(v)^b]
	}
	return v
}

// eccBlock computes one Reed-Solomon parity block.
//
// P and Q differ only in how they stride through the sector: P treats it as
// columns and Q as diagonals, so that a burst error damaging consecutive
// bytes lands in different codewords for each and can be corrected by one
// even when it defeats the other. Both produce two parity bytes per codeword,
// written as two consecutive runs of majorCount bytes.
func eccBlock(src []byte, majorCount, minorCount, majorMult, minorInc int, dst []byte) {
	size := majorCount * minorCount
	for major := range majorCount {
		index := (major>>1)*majorMult + (major & 1)
		var a, b byte
		for range minorCount {
			v := src[index]
			index += minorInc
			if index >= size {
				index -= size
			}
			a ^= v
			b ^= v
			a = eccFLUT[a]
		}
		a = eccBLUT[eccFLUT[a]^b]
		dst[major] = a
		dst[major+majorCount] = a ^ b
	}
}

// generateECC fills in the P and Q parity of a Mode 2 Form 1 sector.
//
// The parity is computed with the four address bytes of the header treated as
// zero. A Mode 2 sector's address is not protected by its own ECC, so a reader
// that has to correct a sector does not need to have trusted the header to
// find it in the first place.
func generateECC(sector []byte) {
	var addr [4]byte
	copy(addr[:], sector[headerOffset:headerOffset+4])
	clear(sector[headerOffset : headerOffset+4])

	src := sector[headerOffset:]
	eccBlock(src, 86, 24, 2, 86, sector[eccPOffset:])
	eccBlock(src, 52, 43, 86, 88, sector[eccQOffset:])

	copy(sector[headerOffset:headerOffset+4], addr[:])
}
