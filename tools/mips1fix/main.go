// Command mips1fix removes instructions from an object that the R3000A
// cannot execute.
//
// LLVM's MIPS-I support is marked highly experimental, and it shows: when it
// compiles an integer division it emits a trap instruction to catch division
// by zero.
//
//	div  $zero, $a0, $v0
//	teq  $v0, $zero, 7      <- MIPS II and later only
//	mfhi $v0
//
// The trap family (teq, tne, tge, tgeu, tlt, tltu) arrived with MIPS II. On a
// MIPS I core the encoding is not a valid instruction at all, so the console
// takes a reserved-instruction exception and the program stops dead. The
// failure is silent and total: a black screen, with the fault several frames
// before anything was drawn.
//
// GCC does not have the problem. For the same division it emits a branch
// around a break, which is MIPS I throughout, and clang avoids it too when
// given -mno-check-zero-division. That option is an LLVM backend flag rather
// than a target feature, so it cannot be reached through TinyGo, which passes
// only features through to the code generator.
//
// Patching the object afterwards works because the replacement is the same
// width: a trap becomes a nop, nothing moves, and no relocation or symbol
// needs adjusting. What is lost is the division-by-zero check itself, so a
// division by zero now has an undefined result rather than trapping. That is
// the same trade -mno-check-zero-division makes.
package main

import (
	"fmt"
	"log"
	"os"

	"github.com/spf13/cobra"

	"go.chrisrx.dev/psx/binutils"
)

var opts struct {
	DryRun bool
	Quiet  bool
}

// patch rewrites trap instructions in the executable sections of an object,
// returning how many it replaced.
func patch(path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	traps, err := binutils.MIPS1Traps(data)
	if err != nil {
		return 0, fmt.Errorf("%s: %v", path, err)
	}
	if !opts.Quiet {
		for _, t := range traps {
			fmt.Printf("  %s+0x%x: %s (%#08x) -> nop\n", t.Section, t.Offset, t.Name, t.Word)
		}
	}
	if len(traps) > 0 && !opts.DryRun {
		binutils.RemoveTraps(data, traps)
		if err := os.WriteFile(path, data, 0o644); err != nil {
			return len(traps), err
		}
	}
	return len(traps), nil
}

func main() {
	cmd := &cobra.Command{
		Use:   "mips1fix [flags] <object>...",
		Short: "replace instructions an R3000A cannot execute",
		Long: `mips1fix rewrites MIPS II trap instructions as nops, so that an object
built by LLVM for mips1 will run on a PlayStation.

LLVM emits teq after an integer division to catch division by zero. That
instruction does not exist on a MIPS I core, which takes a reserved
instruction exception instead, and the program stops with no indication of
why. Replacing the trap with a nop is the same trade clang's
-mno-check-zero-division makes, and that option cannot be reached through
TinyGo. Division by zero is undefined afterwards rather than trapping.`,
		Args:         cobra.MinimumNArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			total := 0
			for _, path := range args {
				n, err := patch(path)
				if err != nil {
					return err
				}
				total += n
			}
			if !opts.Quiet {
				verb := "patched"
				if opts.DryRun {
					verb = "found"
				}
				fmt.Printf("%s %d trap instruction(s)\n", verb, total)
			}
			return nil
		},
	}
	f := cmd.Flags()
	f.BoolVarP(&opts.DryRun, "dry-run", "n", false, "report what would change without writing")
	f.BoolVarP(&opts.Quiet, "quiet", "q", false, "only report errors")

	if err := cmd.Execute(); err != nil {
		log.Fatal(err)
	}
}
