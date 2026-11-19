package main

import (
	"os"
	"slices"
	"strings"
	"testing"

	"go.chrisrx.dev/x/assert"
)

func TestDecode(t *testing.T) {
	for _, tc := range []struct {
		word uint32
		want string
	}{
		{0x27bdffe8, "addiu sp, sp, -24"},
		{0xafbf0014, "sw ra, 20(sp)"},
		{0x0c00c1e4, "jal 0x80030790"},
		{0x3c048006, "lui a0, 0x8006"},
		{0x8c820000, "lw v0, 0(a0)"},
		{0x03e00008, "jr ra"},
		{0x10400003, "beq v0, zero, 0x80010010"},
		{0x48c80000, "ctc2 t0, $0"},
		{0x4a280030, "cop2 0x280030"},
		{0x0000000c, "syscall 0"},
	} {
		got := Decode(0x80010000, tc.word)
		assert.Equal(t, tc.want, got.Mnemonic+" "+got.Operands, "%08x", tc.word)
	}
}

// TestImageFunctions walks the resident image: the named entry points and
// every jal target are functions, each ending at the next one.
func TestImageFunctions(t *testing.T) {
	img := loadImage(t)
	fns := img.Functions()
	gs := fns.Named("GsSortSprite")
	assert.Equal(t, uint32(0x800208C4), gs.Start)
	assert.Equal(t, uint32(1276), gs.Size, "ends at the next function start")
	assert.Equal(t, true, slices.Contains(gs.Calls, "RotMatrix"))
	assert.Equal(t, "func_8002048c", fns.At(0x8002048c).Name, "an internal function is named by its address")
	assert.Equal(t, nil, fns.Named("PSDCNT"), "a data symbol is not a function")
	assert.Equal(t, "longjmp", fns.Named("longjmp").Name, "a function nobody calls, after the last call target, is still a function")
	assert.Equal(t, 758, len(fns), "named entry points, jal targets, and functions reached through pointers")
	// Functions only reached through pointers are found where a frame
	// opens after a return: SetIntrMask's neighbor at 0x8003CDE4, and
	// SetGraphReverse after ResetGraph, which ends at 344 bytes, not where
	// the next call target is.
	assert.Equal(t, "func_8003cde4", fns.At(0x8003CDE4).Name)
	assert.Equal(t, uint32(344), fns.Named("ResetGraph").Size)
	// The GPU's driver table holds pointers to functions nothing calls by
	// name: a data word that addresses code right after a return is one.
	assert.Equal(t, "func_8002a8f4", fns.At(0x8002A8F4).Name)
	last := fns[len(fns)-1]
	assert.Equal(t, "longjmp", last.Name)
	assert.Equal(t, true, last.Size <= 256, "the last function ends at its return, not at the end of the image: %d", last.Size)
}

// TestDescribe resolves what an address reaches: a data symbol with its
// value, a string, and an offset into a function.
func TestDescribe(t *testing.T) {
	img := loadImage(t)
	fns := img.Functions()
	assert.Equal(t, "PSDCNT = 0x00000000", img.describe(0x800661E0, fns))
	assert.Equal(t, "putchar+0x4", img.describe(0x80043794, fns))
	assert.Equal(t, `0x800455a0 "This is not SEQ Data.\n"`, img.describe(0x800455A0, fns), "a string is quoted")
}

// TestDump prints words of data with what each addresses: the GPU driver
// table's first entries are functions.
func TestDump(t *testing.T) {
	img := loadImage(t)
	fns := img.Functions()
	var buf strings.Builder
	img.Dump(&buf, 0x8004829C, 2, fns)
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	assert.Equal(t, 2, len(lines))
	assert.Equal(t, true, strings.HasPrefix(lines[0], `8004829C: 80045DB4  0x80045db4 "$Id: sys.c`), lines[0])
	assert.Equal(t, "800482A0: 80029ECC  func_80029ecc", lines[1])
}

// TestBIOSNote names a BIOS call made the library's way: the table in a
// register, jr through it, the function number in the delay slot.
func TestBIOSNote(t *testing.T) {
	img := loadImage(t)
	fns := img.Functions()
	var buf strings.Builder
	img.Write(&buf, fns.Named("write"), fns)
	assert.Equal(t, true, strings.Contains(buf.String(), "BIOS B(0x35) write"), buf.String())
}

func loadImage(t *testing.T) *Image {
	t.Helper()
	if os.Getenv("PSX_SONY_LIBPS") == "" {
		t.Skip("PSX_SONY_LIBPS names the private copy of Sony's image")
	}
	img, err := LoadImage("", "../../internal/toolchain/_tinygo/targets/playstation-yaroze-symusr.S")
	if !assert.NoError(t, err) {
		t.FailNow()
	}
	return img
}
