package cpu

import (
	"encoding/binary"
	"os"
	"testing"
)

// TestGTE runs the GTE exercise program (see gteprog_test.go) and compares
// every register after every case with what psxe produced, which is kept in
// testdata/gte.golden.
//
// The golden file comes from psxe, not from this package, so it has to be
// made again whenever the program changes. With PSX_GTE_EXE set, the test
// writes the program there as a PSX-EXE and logs its case count and where
// its results are:
//
//	PSX_GTE_EXE=$PWD/gte.exe go test ./emu/cpu -run TestGTE -v
//	PSXE_TRACE=/dev/null PSXE_TRACE_RAM=psxe.ram PSXE_TRACE_COUNT=4000000 psxe -a -b bios.bin -x gte.exe
//	dd if=psxe.ram of=emu/cpu/testdata/gte.golden bs=256 skip=4096 count=<cases>
//
// where 4096 blocks of 256 bytes is the program's output address, 80100000,
// in RAM. To find where the two GTEs part, trace both runs and compare them
// with psxtrace:
//
//	PSXE_TRACE=psxe.trace PSXE_TRACE_COUNT=4000000 psxe -a -b bios.bin -x gte.exe
//	psxemu --bios bios.bin --exe gte.exe --trace emu.trace --count 4000000
//	psxtrace diff psxe.trace emu.trace
func TestGTE(t *testing.T) {
	p := buildGTEProgram()
	if path := os.Getenv("PSX_GTE_EXE"); path != "" {
		if err := p.exe().WriteFile(path); err != nil {
			t.Fatal(err)
		}
		t.Logf("wrote %s: %d cases; done at %08x; results %08x-%08x", path,
			p.cases, p.done, gteOutputAddr, gteOutputAddr+uint32(p.cases*gteRegs*4))
	}

	golden, err := os.ReadFile("testdata/gte.golden")
	if err != nil {
		t.Fatal(err)
	}
	if want := p.cases * gteRegs * 4; len(golden) != want {
		t.Fatalf("testdata/gte.golden is %d bytes, but the program has %d cases, for %d; it needs making again",
			len(golden), p.cases, want)
	}

	bus := &testBus{}
	copy(bus.ram[gteCodeAddr&0x1fffff:], p.code)
	c := New(bus)
	c.PC, c.NextPC = p.entry, p.entry+4
	for i := 0; c.PC != p.done; i++ {
		if i == 2_000_000 {
			t.Fatal("the program did not finish in 2M instructions")
		}
		c.Step()
	}

	out := bus.ram[gteOutputAddr&0x1fffff:]
	failures := 0
	for i := range p.cases {
		for r := range gteRegs {
			off := (i*gteRegs + r) * 4
			got := binary.LittleEndian.Uint32(out[off:])
			want := binary.LittleEndian.Uint32(golden[off:])
			if got == want {
				continue
			}
			op := binary.LittleEndian.Uint32(p.code[caseOp(p, i):])
			t.Errorf("case %d, command %08x: register %d is %08x, psxe has %08x", i, op, r, got, want)
			if failures++; failures == 20 {
				t.Fatal("too many differences")
			}
		}
	}
}

// caseOp finds case i's command in the code: each case is nine
// instructions from the entry point, and the command is the fifth.
func caseOp(p *gteProgram, i int) int {
	return int(p.entry-gteCodeAddr) + (i*9+4)*4
}
