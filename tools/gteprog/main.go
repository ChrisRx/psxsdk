// Command gteprog writes the GTE exercise program (see internal/gteprog) as
// a PSX-EXE, to run in another emulator for comparison:
//
//	gteprog gte.exe
//	PSXE_TRACE=psxe.trace PSXE_TRACE_RAM=psxe.ram PSXE_TRACE_COUNT=... psxe -a -b bios.bin -x gte.exe
//	psxemu --bios bios.bin --exe gte.exe --trace emu.trace --count ...
//	psxtrace diff psxe.trace emu.trace
//
// It prints the address the program finishes at and where its results are.
package main

import (
	"fmt"
	"log"
	"os"

	"go.chrisrx.dev/psx/internal/gteprog"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: gteprog <output.exe>")
		os.Exit(2)
	}
	p := gteprog.Build()
	if err := p.EXE().WriteFile(os.Args[1]); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%d cases; done at %08x; results %08x-%08x\n",
		p.Cases, p.Done, gteprog.OutputAddr, gteprog.OutputAddr+uint32(p.Cases*gteprog.Regs*4))
}
