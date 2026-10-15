package cpu

import (
	"encoding/binary"
	"os"
	"testing"

	"go.chrisrx.dev/psx/internal/gteprog"
)

// TestGTE runs the GTE exercise program (see internal/gteprog) and compares
// every register after every case with what psxe produced, which is kept in
// testdata/gte.golden.
//
// The golden file comes from psxe, not from this package, so it has to be
// made again whenever the program changes:
//
//	go run ./tools/gteprog gte.exe
//	PSXE_TRACE=/dev/null PSXE_TRACE_RAM=psxe.ram PSXE_TRACE_COUNT=4000000 psxe -a -b bios.bin -x gte.exe
//	dd if=psxe.ram of=emu/cpu/testdata/gte.golden bs=256 skip=4096 count=<cases>
//
// where the count is the number of cases gteprog reports, and 4096 blocks
// of 256 bytes is the program's output address, 80100000, in RAM.
func TestGTE(t *testing.T) {
	golden, err := os.ReadFile("testdata/gte.golden")
	if err != nil {
		t.Fatal(err)
	}
	p := gteprog.Build()
	if want := p.Cases * gteprog.Regs * 4; len(golden) != want {
		t.Fatalf("testdata/gte.golden is %d bytes, but the program has %d cases, for %d; it needs making again",
			len(golden), p.Cases, want)
	}

	bus := &testBus{}
	copy(bus.ram[gteprog.CodeAddr&0x1fffff:], p.Code)
	c := New(bus)
	c.PC, c.NextPC = p.Entry, p.Entry+4
	for i := 0; c.PC != p.Done; i++ {
		if i == 2_000_000 {
			t.Fatal("the program did not finish in 2M instructions")
		}
		c.Step()
	}

	out := bus.ram[gteprog.OutputAddr&0x1fffff:]
	failures := 0
	for i := range p.Cases {
		for r := range gteprog.Regs {
			off := (i*gteprog.Regs + r) * 4
			got := binary.LittleEndian.Uint32(out[off:])
			want := binary.LittleEndian.Uint32(golden[off:])
			if got == want {
				continue
			}
			op := binary.LittleEndian.Uint32(p.Code[caseOp(p, i):])
			t.Errorf("case %d, command %08x: register %d is %08x, psxe has %08x", i, op, r, got, want)
			if failures++; failures == 20 {
				t.Fatal("too many differences")
			}
		}
	}
}

// caseOp finds case i's command in the code: each case is nine
// instructions from the entry point, and the command is the fifth.
func caseOp(p *gteprog.Program, i int) int {
	return int(p.Entry-gteprog.CodeAddr) + (i*9+4)*4
}
