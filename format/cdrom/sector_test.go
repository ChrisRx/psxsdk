package cdrom

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"math/rand/v2"
	"testing"
)

func TestMSF(t *testing.T) {
	// Addressing starts two seconds (150 frames) before logical sector zero,
	// and there are 75 frames to the second.
	tests := []struct {
		lba     uint32
		m, s, f byte
	}{
		{0, 0x00, 0x02, 0x00},
		{74, 0x00, 0x02, 0x74},
		{75, 0x00, 0x03, 0x00},
		{4349, 0x00, 0x59, 0x74}, // 4349+150 = 4499, one frame short of a minute
		{4350, 0x01, 0x00, 0x00}, // 4350+150 = 4500 = exactly one minute
	}
	for _, tt := range tests {
		m, s, f := MSF(tt.lba)
		if m != tt.m || s != tt.s || f != tt.f {
			t.Errorf("MSF(%d) = %02x:%02x:%02x, want %02x:%02x:%02x",
				tt.lba, m, s, f, tt.m, tt.s, tt.f)
		}
	}
}

func TestForm1Layout(t *testing.T) {
	data := make([]byte, 100)
	for i := range data {
		data[i] = byte(i)
	}

	sector := make([]byte, SectorSize)
	sh := Subheader{File: 1, Channel: 2, Submode: SubmodeData, Coding: 4}
	if err := EncodeForm1(sector, 75, sh, data); err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(sector[:12], syncPattern[:]) {
		t.Errorf("sync = %x, want %x", sector[:12], syncPattern)
	}
	if got := sector[15]; got != 2 {
		t.Errorf("mode = %d, want 2", got)
	}
	if got := sector[12:15]; !bytes.Equal(got, []byte{0x00, 0x03, 0x00}) {
		t.Errorf("header address = %x, want 000300", got)
	}

	// The subheader is stored twice so either copy can be recovered.
	want := []byte{1, 2, byte(SubmodeData), 4}
	if got := sector[16:20]; !bytes.Equal(got, want) {
		t.Errorf("first subheader = %x, want %x", got, want)
	}
	if got := sector[20:24]; !bytes.Equal(got, want) {
		t.Errorf("second subheader = %x, want %x", got, want)
	}

	if got := sector[dataOffset : dataOffset+len(data)]; !bytes.Equal(got, data) {
		t.Error("user data was not copied verbatim")
	}
	// Everything past the payload must be zero padded.
	for i := dataOffset + len(data); i < form1EDCOffset; i++ {
		if sector[i] != 0 {
			t.Fatalf("byte %d past the payload is %#x, want 0", i, sector[i])
		}
	}
}

func TestEncodeRejectsOversizedData(t *testing.T) {
	sector := make([]byte, SectorSize)
	if err := EncodeForm1(sector, 0, DataSubheader, make([]byte, Form1DataSize+1)); err == nil {
		t.Error("EncodeForm1 accepted more than 2048 bytes")
	}
	if err := EncodeForm2(sector, 0, DataSubheader, make([]byte, Form2DataSize+1)); err == nil {
		t.Error("EncodeForm2 accepted more than 2324 bytes")
	}
	if err := EncodeForm1(make([]byte, 100), 0, DataSubheader, nil); err == nil {
		t.Error("EncodeForm1 accepted an undersized sector buffer")
	}
}

func TestForm2SetsFormBit(t *testing.T) {
	sector := make([]byte, SectorSize)
	if err := EncodeForm2(sector, 0, Subheader{Submode: SubmodeData}, nil); err != nil {
		t.Fatal(err)
	}
	if got := Submode(sector[18]); got&SubmodeForm2 == 0 {
		t.Errorf("submode = %#x, want the Form 2 bit set", got)
	}
}

// edcBitwise is a deliberately naive reference, shifting one bit at a time
// through the polynomial from ECMA-130:
//
//	(x^16 + x^15 + x^2 + 1)(x^16 + x^2 + x + 1)
//	  = x^32 + x^31 + x^16 + x^15 + x^4 + x^3 + x + 1
//
// which is 0x8001801b, or 0xd8018001 once reversed for a least significant
// bit first CRC. Agreeing with the table-driven version confirms both the
// table construction and the constant.
func edcBitwise(data []byte) uint32 {
	var crc uint32
	for _, b := range data {
		crc ^= uint32(b)
		for range 8 {
			if crc&1 != 0 {
				crc = (crc >> 1) ^ 0xd8018001
			} else {
				crc >>= 1
			}
		}
	}
	return crc
}

func TestEDCMatchesBitwiseReference(t *testing.T) {
	r := rand.New(rand.NewPCG(1, 2))
	for n := range 64 {
		data := make([]byte, n*7)
		for i := range data {
			data[i] = byte(r.UintN(256))
		}
		if got, want := edc(data), edcBitwise(data); got != want {
			t.Fatalf("edc(%d bytes) = %#08x, want %#08x", len(data), got, want)
		}
	}
}

// Both the EDC and the Reed-Solomon parity are linear codes over GF(2), so
// encoding the XOR of two payloads must give the XOR of their encodings. That
// is a strong check on the stride arithmetic and the field tables: almost any
// indexing mistake breaks it.
func TestEDCIsLinear(t *testing.T) {
	r := rand.New(rand.NewPCG(3, 4))
	a := make([]byte, 2056)
	b := make([]byte, 2056)
	for i := range a {
		a[i] = byte(r.UintN(256))
		b[i] = byte(r.UintN(256))
	}
	x := make([]byte, len(a))
	for i := range x {
		x[i] = a[i] ^ b[i]
	}
	if got, want := edc(x), edc(a)^edc(b); got != want {
		t.Errorf("edc(a^b) = %#08x, want edc(a)^edc(b) = %#08x", got, want)
	}
}

func TestECCIsLinear(t *testing.T) {
	r := rand.New(rand.NewPCG(5, 6))

	parity := func(data []byte) []byte {
		sector := make([]byte, SectorSize)
		if err := EncodeForm1(sector, 0, Subheader{}, data); err != nil {
			t.Fatal(err)
		}
		// The address bytes are not part of the encoded codeword, and the
		// sync pattern is constant, so only the parity itself is compared.
		return append([]byte(nil), sector[eccPOffset:]...)
	}

	a := make([]byte, Form1DataSize)
	b := make([]byte, Form1DataSize)
	for i := range a {
		a[i] = byte(r.UintN(256))
		b[i] = byte(r.UintN(256))
	}
	x := make([]byte, len(a))
	for i := range x {
		x[i] = a[i] ^ b[i]
	}

	pa, pb, px := parity(a), parity(b), parity(x)
	for i := range px {
		if px[i] != pa[i]^pb[i] {
			t.Fatalf("parity byte %d is %#x, want %#x", i, px[i], pa[i]^pb[i])
		}
	}
}

// A linear code maps the zero codeword to zero parity, so a sector whose
// subheader and data are entirely zero must carry no EDC and no ECC. The
// address bytes do not disturb this because they are excluded from both.
func TestZeroSectorHasZeroParity(t *testing.T) {
	sector := make([]byte, SectorSize)
	if err := EncodeForm1(sector, 1234, Subheader{}, nil); err != nil {
		t.Fatal(err)
	}
	for i := form1EDCOffset; i < SectorSize; i++ {
		if sector[i] != 0 {
			t.Fatalf("byte %d of an empty sector is %#x, want 0", i, sector[i])
		}
	}
}

// The P and Q codewords must cover every byte they are specified to protect:
// P over the header, subheader, data and EDC, and Q over all of that plus the
// P parity. Flipping any single byte in range therefore has to change the
// parity. This catches a stride that silently skips part of the sector.
func TestECCCoversEveryProtectedByte(t *testing.T) {
	base := make([]byte, SectorSize)
	if err := EncodeForm1(base, 0, DataSubheader, nil); err != nil {
		t.Fatal(err)
	}

	for _, off := range []int{subheaderOffset, dataOffset, dataOffset + 1000, form1EDCOffset - 1} {
		sector := append([]byte(nil), base...)
		sector[off] ^= 0xff
		generateECC(sector)
		if bytes.Equal(sector[eccPOffset:], base[eccPOffset:]) {
			t.Errorf("flipping byte %d left the ECC unchanged", off)
		}
	}
}

// TestForm1KnownAnswer encodes one fixed sector and checks its EDC and the
// digest of the whole. The other tests check properties every linear code
// has, which a wrong field constant can keep, so this pins the bytes: the
// digest is of the sector as the encoder wrote it before it was rewritten
// from the standard, which is what every disc the emulator tests boot was
// written with.
func TestForm1KnownAnswer(t *testing.T) {
	r := rand.New(rand.NewPCG(9, 10))
	data := make([]byte, Form1DataSize)
	for i := range data {
		data[i] = byte(r.UintN(256))
	}
	sector := make([]byte, SectorSize)
	if err := EncodeForm1(sector, 1234, DataSubheader, data); err != nil {
		t.Fatal(err)
	}
	if got := binary.LittleEndian.Uint32(sector[form1EDCOffset:]); got != 0x0f19563b {
		t.Errorf("EDC is %#08x, want 0x0f19563b", got)
	}
	sum := sha256.Sum256(sector)
	if got := hex.EncodeToString(sum[:]); got != "0d75cfaeda92d15f79fd72e2d41df68f01bb55464f79f0d8ab66b9dcb8281ea8" {
		t.Errorf("sector digest is %s, want 0d75cfae…", got)
	}
}

// The inverse the parity is scaled by has to be one: a wrong element there
// keeps the code linear and still breaks every sector.
func TestFieldInverse(t *testing.T) {
	if got := gfMul(gfInvAlphaPlusOne, 0x03); got != 1 {
		t.Errorf("(α+1)·1/(α+1) = %#x, want 1", got)
	}
	if gfExp[1] != 0x02 || gfExp[8] != 0x1d {
		t.Errorf("α = %#x and α^8 = %#x, want 0x02 and 0x1d", gfExp[1], gfExp[8])
	}
}
