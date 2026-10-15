package emutest

import (
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"go.chrisrx.dev/x/assert"
)

// Instructions for the test ROMs.
const (
	nop = 0x00000000
)

func lui(rt uint32, imm uint16) uint32             { return 0x0f<<26 | rt<<16 | uint32(imm) }
func ori(rt, rs uint32, imm uint16) uint32         { return 0x0d<<26 | rs<<21 | rt<<16 | uint32(imm) }
func addiu(rt, rs uint32, imm uint16) uint32       { return 0x09<<26 | rs<<21 | rt<<16 | uint32(imm) }
func sw(rt uint32, off uint16, base uint32) uint32 { return 0x2b<<26 | base<<21 | rt<<16 | uint32(off) }
func jalr(rs uint32) uint32                        { return rs<<21 | 31<<11 | 0x09 }
func beqSelf() uint32                              { return 0x04<<26 | 0xffff } // beq zero, zero, -1

// Registers.
const (
	a0 = 4
	t0 = 8
	t1 = 9
	t2 = 10
)

// writeROM writes a BIOS image holding program at the reset vector.
func writeROM(t *testing.T, program ...uint32) string {
	t.Helper()
	rom := make([]byte, 512<<10)
	for i, insn := range program {
		binary.LittleEndian.PutUint32(rom[4*i:], insn)
	}
	path := filepath.Join(t.TempDir(), "rom.bin")
	if !assert.NoError(t, os.WriteFile(path, rom, 0o644)) {
		t.FailNow()
	}
	return path
}

// printing is a ROM that installs a B-function handler at 0xb4, which only
// returns, and calls it as putchar for each byte of s, before looping.
func printing(s string) []uint32 {
	p := []uint32{
		// The handler: jr ra; nop, at 800000B4.
		lui(t0, 0x8000),
		lui(t1, 0x03e0), ori(t1, t1, 0x0008), sw(t1, 0xb4, t0),
		sw(0, 0xb8, t0),
		ori(t2, t0, 0x00b4),
	}
	for _, c := range []byte(s) {
		p = append(p, addiu(t1, 0, 0x3d), addiu(a0, 0, uint16(c)), jalr(t2), nop)
	}
	return append(p, beqSelf(), nop)
}

func TestRunUntilTTY(t *testing.T) {
	m := Boot(t, BIOS(writeROM(t, printing("hello\n")...)))
	m.RunUntilTTY("hello", 10*Millisecond)
	assert.Equal(t, "hello", m.TTY(), "it stops as soon as the text is there")
}

func TestRunFrames(t *testing.T) {
	m := Boot(t, BIOS(writeROM(t)))
	m.RunFrames(2)
	assert.Equal(t, uint64(2), m.Frames())
}

// failure runs f with a testing.TB that records the first Fatal instead of
// failing the test, and returns its message.
func failure(t *testing.T, f func(tb testing.TB)) string {
	t.Helper()
	tb := &recorder{TB: t}
	done := make(chan struct{})
	go func() {
		defer close(done)
		f(tb)
	}()
	<-done
	if tb.msg == "" {
		t.Fatal("expected the test to fail")
	}
	return tb.msg
}

type recorder struct {
	testing.TB
	msg string
}

func (r *recorder) Helper() {}

func (r *recorder) Fatal(args ...any) {
	r.msg = fmt.Sprint(args...)
	runtime.Goexit()
}

func (r *recorder) Fatalf(format string, args ...any) {
	r.msg = fmt.Sprintf(format, args...)
	runtime.Goexit()
}

func TestIllegalInstructionFails(t *testing.T) {
	rom := writeROM(t, lui(t0, 0x8000), nop, 0x00000001)
	msg := failure(t, func(tb testing.TB) {
		m := Boot(tb, BIOS(rom))
		m.Run(Millisecond)
	})
	assert.Equal(t, "illegal instruction 00000001 at bfc00008", strings.SplitN(msg, "\n", 2)[0])
	for _, want := range []string{"last 3 instructions:", "bfc00008: 00000001", "pc   80000080"} {
		if !strings.Contains(msg, want) {
			t.Errorf("report is missing %q:\n%s", want, msg)
		}
	}
}

func TestBudget(t *testing.T) {
	rom := writeROM(t, printing("waiting\n")...)
	msg := failure(t, func(tb testing.TB) {
		m := Boot(tb, BIOS(rom))
		m.RunUntilTTY("never", Millisecond)
	})
	assert.Equal(t, `printing "never" did not happen within 0.001s`, strings.SplitN(msg, "\n", 2)[0])
	if !strings.Contains(msg, "TTY, last lines:\nwaiting") {
		t.Errorf("report should show the TTY:\n%s", msg)
	}
}

func TestGolden(t *testing.T) {
	t.Chdir(t.TempDir())
	img := image.NewRGBA(image.Rect(0, 0, 4, 2))
	img.SetRGBA(1, 1, color.RGBA{R: 0xff, A: 0xff})

	// Missing, then written by -update-golden, then matching.
	msg := failure(t, func(tb testing.TB) { Golden(tb, img, "x.png") })
	assert.Equal(t, "no golden image testdata/x.png; run with -update-golden to create it", msg)

	*updateGolden = true
	Golden(t, img, "x.png")
	*updateGolden = false
	Golden(t, img, "x.png")

	// A changed pixel fails, and leaves the image produced beside the
	// golden one.
	img.SetRGBA(3, 0, color.RGBA{G: 0xff, A: 0xff})
	msg = failure(t, func(tb testing.TB) { Golden(tb, img, "x.png") })
	assert.Equal(t, "1 pixels differ from testdata/x.png, the first at (3,0); the image is in testdata/x.actual.png", msg)
	_, err := os.Stat("testdata/x.actual.png")
	assert.NoError(t, err)
}
