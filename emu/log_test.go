package emu

import (
	"bytes"
	"fmt"
	"log/slog"
	"strings"
	"testing"

	"go.chrisrx.dev/psx/emu/cpu"
	"go.chrisrx.dev/x/assert"
)

// loadProgram writes words of code into RAM at addr and starts the CPU
// there.
func loadProgram(m *Machine, addr uint32, code ...uint32) {
	for i, w := range code {
		m.Bus.Write32(addr+4*uint32(i), w)
	}
	m.CPU.PC, m.CPU.NextPC = addr, addr+4
}

// logTo has m log at level into the buffer returned.
func logTo(m *Machine, level slog.Level) *bytes.Buffer {
	var buf bytes.Buffer
	m.SetLogger(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: level})))
	return &buf
}

func TestLogMisalignedStore(t *testing.T) {
	m := newTestMachine(t)
	logs := logTo(m, slog.LevelError)
	loadProgram(m, 0x80010000,
		0x3c088002, // lui $t0, 0x8002
		0xad000001, // sw $zero, 1($t0)
		0x00000000,
	)
	_, err := m.Run(20)
	assert.NoError(t, err)
	assert.Equal(t, true, strings.Contains(logs.String(),
		"crash: address error on a store (AdES) at 80010004, address 80020001"), logs.String())
}

func TestLogPanicLoop(t *testing.T) {
	m := newTestMachine(t)
	m.SetSymbols([]Symbol{
		{Name: "main.main", Addr: 0x80010300, Size: 0x40},
		{Name: "runtime.lookupPanic", Addr: 0x80010200, Size: 0x20},
		{Name: "runtime.runtimePanicAt", Addr: 0x80010100, Size: 0x10},
	})
	m.Bus.Write32(0x80010200, 0x0c004040) // jal runtime.runtimePanicAt
	m.Bus.Write32(0x80010310, 0x0c004080) // jal runtime.lookupPanic
	// The stack holds the two return addresses.
	m.Bus.Write32(0x80100004, 0x80010208)
	m.Bus.Write32(0x80100008, 0x80010318)
	m.CPU.R[29] = 0x80100000
	loadProgram(m, 0x80010100,
		0x08004040, // j .
		0x00000000,
	)
	logs := logTo(m, slog.LevelError)

	_, err := m.RunFrame(1000)
	assert.NoError(t, err)
	out := logs.String()
	assert.Equal(t, true, strings.Contains(out,
		"crash: the program panicked (index out of range); the CPU is looping at 80010100 runtime.runtimePanicAt"), out)
	assert.Equal(t, true, strings.Contains(out,
		`stack="80010200 runtime.lookupPanic <- 80010310 main.main+0x10"`), out)

	_, err = m.RunFrame(1000)
	assert.NoError(t, err)
	assert.Equal(t, 1, strings.Count(logs.String(), "crash:"), "the loop is reported once")
}

func TestLogBIOSCall(t *testing.T) {
	m := newTestMachine(t)
	logs := logTo(m, slog.LevelDebug)
	loadProgram(m, 0x80010000,
		0x24040011, // li $a0, 0x11
		0x2409002a, // li $t1, 0x2a
		0x0c000028, // jal 0x800000a0
		0x00000000,
	)
	_, err := m.Run(10)
	assert.NoError(t, err)
	assert.Equal(t, true, strings.Contains(logs.String(),
		"BIOS A(2ah) memcpy(00000011, 00000000, 00000000, 00000000) from 80010008"), logs.String())
}

// TestTTYFileWrite checks that what the program writes to standard output
// with FileWrite, as the Net Yaroze library's putchar does, shows in the
// TTY.
func TestTTYFileWrite(t *testing.T) {
	m := newTestMachine(t)
	var tty bytes.Buffer
	m.TTY = &tty
	for i, b := range []byte("abc") {
		m.Bus.Write8(0x80020000+uint32(i), b)
	}
	loadProgram(m, 0x80010000,
		0x24040001, // li $a0, 1
		0x3c058002, // lui $a1, 0x8002
		0x24060003, // li $a2, 3
		0x24090035, // li $t1, 0x35
		0x0c00002c, // jal 0x800000b0
		0x00000000,
	)
	_, err := m.Run(20)
	assert.NoError(t, err)
	assert.Equal(t, "abc", tty.String())
}

// TestTTYInterrupted checks that a character is printed once when an interrupt lands
// on any instruction of the call, with either shape of the stub at 0xb0: returning from
// the interrupt runs that instruction again. The interrupt makes a B-function call of its
// own, as a kernel's does, and in a delay slot it returns to the jump.
func TestTTYInterrupted(t *testing.T) {
	const (
		handler = 0x80020010
		vector  = 0x80030000
	)
	for _, stub := range []struct {
		name string
		code []uint32
		at   []uint32
	}{
		{
			name: "jump at b4",
			code: []uint32{
				0x3c088002, // lui $t0, 0x8002
				0x01000008, // jr $t0
				0x00000000,
			},
			at: []uint32{0xb0, 0xb4, 0xb8, 0x20000},
		},
		{
			name: "jump at b8",
			code: []uint32{
				0x3c088002, // lui $t0, 0x8002
				0x25080010, // addiu $t0, $t0, 0x10
				0x01000008, // jr $t0
				0x00000000,
			},
			at: []uint32{0xb0, 0xb4, 0xb8, 0xbc, 0x20010, 0x20014},
		},
	} {
		for _, at := range stub.at {
			t.Run(fmt.Sprintf("%s/%x", stub.name, at), func(t *testing.T) {
				m := newTestMachine(t)
				var tty bytes.Buffer
				m.TTY = &tty
				for i, op := range stub.code {
					m.Bus.Write32(0x800000b0+uint32(i)*4, op)
				}
				for _, addr := range []uint32{0x80020000, handler} {
					m.Bus.Write32(addr, 0x03e00008) // jr $ra
					m.Bus.Write32(addr+4, 0)
				}
				m.Bus.Write32(0x80000080, 0x0800c000) // j vector
				m.Bus.Write32(0x80000084, 0)
				for i, op := range []uint32{
					0xac080100, // sw $t0, 0x100($zero)
					0xac090104, // sw $t1, 0x104($zero)
					0xac1f0108, // sw $ra, 0x108($zero)
					0x24090017, // li $t1, 0x17
					0x0c00002c, // jal 0xb0
					0x00000000,
					0x8c080100, // lw $t0, 0x100($zero)
					0x8c090104, // lw $t1, 0x104($zero)
					0x8c1f0108, // lw $ra, 0x108($zero)
					0x401a7000, // mfc0 $k0, EPC
					0x00000000,
					0x03400008, // jr $k0
					0x42000010, // rfe
				} {
					m.Bus.Write32(vector+uint32(i)*4, op)
				}
				loadProgram(m, 0x80010000,
					0x24040078, // li $a0, 'x'
					0x2409003d, // li $t1, 0x3d
					0x0c00002c, // jal 0xb0
					0x00000000,
					0x08004004, // j .
					0x00000000,
				)
				m.CPU.COP0[cpu.SR] |= 0x401
				fired := false
				for range 200 {
					if !fired && m.CPU.PC&0x3fffffff == at {
						fired = true
						m.CPU.SetIRQ(true)
					}
					_, err := m.Run(1)
					assert.NoError(t, err)
					m.CPU.SetIRQ(false)
				}
				assert.Equal(t, true, fired)
				assert.Equal(t, "x", tty.String())
			})
		}
	}
}

func TestDescribeDisplayMode(t *testing.T) {
	assert.Equal(t, "320x240 NTSC 15-bit", describeDisplayMode(0x01))
	assert.Equal(t, "640x480 NTSC 15-bit interlaced", describeDisplayMode(0x27))
	assert.Equal(t, "256x240 PAL 24-bit", describeDisplayMode(0x18))
	assert.Equal(t, "368x240 NTSC 15-bit", describeDisplayMode(0x41))
}
