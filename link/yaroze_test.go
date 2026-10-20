package link

import (
	"crypto/md5"
	"encoding/binary"
	"fmt"
	"testing"

	"go.chrisrx.dev/x/assert"

	"go.chrisrx.dev/psx/format/ecoff"
	"go.chrisrx.dev/psx/format/psx"
)

func TestYarozeConvertEcoffToEXE(t *testing.T) {
	input, err := ecoff.Open("../format/ecoff/testdata/main-ecoff")
	input = assert.Ok(t, input, err)

	exe := &psx.File{
		FileHeader: psx.FileHeader{
			Magic:     psx.ExecutableSignature,
			PC0:       input.Entry,
			TextAddr:  input.Entry,
			TextSize:  input.Size(),
			StackAddr: 0x801fff00,
		},
		Sections: []*psx.Section{
			&psx.Section{
				Name: "text",
				Addr: 0x80010000,
				Data: input.Data(),
			},
		},
	}

	if !assert.NoError(t, PatchExecutable(exe)) {
		t.FailNow()
	}

	psx.AlignTextData(exe, 2048)

	exe, err = Combine(exe)
	exe = assert.Ok(t, exe, err)
	assert.Equal(t, "ABF4D710C68B1E59A7BF4AB6CF0A9586", fmt.Sprintf("%X", md5.Sum(exe.Bytes())))
}

// TestPatchExecutablePreservesReturnAddress runs the startup stub and checks
// that the program is entered with the caller's $ra, not one left behind by
// the call to the library. crt0 returns through that $ra when main returns,
// and an address inside the stub is, by then, part of the program's .bss.
func TestPatchExecutablePreservesReturnAddress(t *testing.T) {
	const (
		addr  = 0x80140000
		entry = 0x80140000
		size  = 0x3aa8

		callerRA = 0xbfc06ff0
		callerSP = 0x801ff000
	)
	exe := psx.New(addr, entry, make([]byte, size))

	if !assert.NoError(t, PatchExecutable(exe)) {
		t.FailNow()
	}
	assert.Equal(t, uint32(addr+size), exe.PC0, "PC0 should point at the stub, at the end of the original image")
	assert.Equal(t, uint32(len(exe.Section("text").Data)), exe.TextSize)

	got, err := runStub(exe.Section("text").Data[size:], exe.PC0, callerRA, callerSP)
	if !assert.NoError(t, err) {
		t.FailNow()
	}
	assert.Equal(t, stubExit{
		Called: []uint32{libpsInit},
		Jump:   entry,
		RA:     callerRA,
		SP:     callerSP,
	}, got)
}

// stubExit is the machine state at the stub's final jump.
type stubExit struct {
	Called []uint32 // targets of every jal, in order
	Jump   uint32   // target of the j that leaves the stub
	RA, SP uint32
}

// runStub executes the stub with just enough of the instruction set to follow
// $sp, $ra and the stack it uses. A jal is recorded and stepped over rather
// than followed into the library; it is the clobbering of $ra that matters.
func runStub(stub []byte, pc, ra, sp uint32) (stubExit, error) {
	const rSP, rRA = 29, 31
	var exit stubExit
	regs := [32]uint32{rSP: sp, rRA: ra}
	stack := map[uint32]uint32{}

	for i := 0; i+4 <= len(stub); i += 4 {
		insn := binary.LittleEndian.Uint32(stub[i:])
		op, rs, rt := insn>>26, (insn>>21)&31, (insn>>16)&31
		imm := uint32(int32(int16(insn)))
		target := pc&0xf0000000 | (insn&0x03ffffff)<<2
		pc += 4

		switch {
		case insn == 0:
		case op == 0x09: // addiu
			regs[rt] = regs[rs] + imm
		case op == 0x2b: // sw
			stack[regs[rs]+imm] = regs[rt]
		case op == 0x23: // lw
			regs[rt] = stack[regs[rs]+imm]
		case op == 0x03: // jal
			exit.Called = append(exit.Called, target)
			regs[rRA] = pc + 4
		case op == 0x02: // j
			exit.Jump, exit.RA, exit.SP = target, regs[rRA], regs[rSP]
			return exit, nil
		default:
			return exit, fmt.Errorf("unexpected instruction %08x at %#x", insn, pc-4)
		}
	}
	return exit, fmt.Errorf("stub never jumps to the program")
}
