package gdbserver

import (
	"fmt"
	"strings"
)

// breakInsn is break 5, GDB's breakpoint on a MIPS.
const breakInsn = 0x0005000d

// isBranch reports whether insn is a jump or a branch, whose delay slot
// runs with it.
func isBranch(insn uint32) bool {
	switch op := insn >> 26; {
	case op == 0:
		f := insn & 0x3f
		return f == 0x08 || f == 0x09 // jr, jalr
	case op == 1: // bltz, bgez, bltzal, bgezal
		return true
	case op >= 2 && op <= 7: // j, jal, beq, bne, blez, bgtz
		return true
	}
	return false
}

// A region is memory a debugger may read and write.
type region struct {
	start, size uint32
}

// regions is the console's memory: RAM, at each of the addresses it is
// seen through, the scratchpad, the I/O ports and the BIOS ROM. A read
// anywhere else is a bus error on a console, which the monitor's debugger
// would take itself, so the server refuses it.
var regions = []region{
	{start: 0x00000000, size: 0x200000},
	{start: 0x80000000, size: 0x200000},
	{start: 0xa0000000, size: 0x200000},
	{start: 0x1f800000, size: 0x400},
	{start: 0x1f801000, size: 0x2000},
	{start: 0x1fc00000, size: 0x80000},
	{start: 0x9fc00000, size: 0x80000},
	{start: 0xbfc00000, size: 0x80000},
}

// accessible reports whether the n bytes at addr are all in one region.
func accessible(addr uint32, n int) bool {
	for _, r := range regions {
		if addr >= r.start && uint64(addr)+uint64(n) <= uint64(r.start)+uint64(r.size) {
			return true
		}
	}
	return false
}

// inRAM reports whether the word at addr is in RAM, where a break
// instruction can be written.
func inRAM(addr uint32) bool {
	return addr&3 == 0 && addr&0x1fffffff < 0x200000 && accessible(addr, 4)
}

// fpuRegs is how many registers the target description gives the floating
// point unit an R3000A in a PlayStation does not have: f0 to f31, fcsr and
// fir, numbered after the 38 there are. GDB takes no description of a MIPS
// without them, so they are described, and each read as unavailable.
const fpuRegs = 34

// targetXML is the target description: an R3000 with the system
// coprocessor's registers a debugger shows.
var targetXML = func() string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0"?>
<!DOCTYPE target SYSTEM "gdb-target.dtd">
<target version="1.0">
<architecture>mips:3000</architecture>
<feature name="org.gnu.gdb.mips.cpu">
`)
	for i := range 32 {
		fmt.Fprintf(&b, "<reg name=\"r%d\" bitsize=\"32\" regnum=\"%d\"/>\n", i, i)
	}
	fmt.Fprintf(&b, `<reg name="lo" bitsize="32" regnum="%d"/>
<reg name="hi" bitsize="32" regnum="%d"/>
<reg name="pc" bitsize="32" regnum="%d"/>
</feature>
<feature name="org.gnu.gdb.mips.cp0">
<reg name="status" bitsize="32" regnum="%d"/>
<reg name="badvaddr" bitsize="32" regnum="%d"/>
<reg name="cause" bitsize="32" regnum="%d"/>
</feature>
<feature name="org.gnu.gdb.mips.fpu">
`, RegLO, RegHI, RegPC, RegSR, RegBadVAddr, RegCause)
	for i := range 32 {
		fmt.Fprintf(&b, "<reg name=\"f%d\" bitsize=\"32\" type=\"ieee_single\" regnum=\"%d\"/>\n", i, NumRegs+i)
	}
	fmt.Fprintf(&b, `<reg name="fcsr" bitsize="32" group="float" regnum="%d"/>
<reg name="fir" bitsize="32" group="float" regnum="%d"/>
</feature>
</target>
`, NumRegs+32, NumRegs+33)
	return b.String()
}()
