package yarozecmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"go.chrisrx.dev/psx/internal/flags"
	"go.chrisrx.dev/psx/internal/hot"
	"go.chrisrx.dev/psx/internal/toolchain"
)

// patch compiles pkg as a hot reload patch for the program whose ELF base
// names, linked at at, or at the start of the program's region when at is
// zero, and writes the patch's ELF, with its traps removed, to out.
func (f *buildFlags) patch(ctx context.Context, pkg, base string, at uint32, out string) error {
	c, err := f.find()
	if err != nil {
		return err
	}
	data, err := os.ReadFile(base)
	if err != nil {
		return err
	}
	start, end, err := hot.Region(data)
	if err != nil {
		return err
	}
	if at == 0 {
		at = start
	}
	if end == start {
		return fmt.Errorf("the program is too large to leave room for patches")
	}
	if at < start || at >= end {
		return fmt.Errorf("%08x is outside the program's region for patches, %08x to %08x", at, start, end)
	}
	abs, err := filepath.Abs(base)
	if err != nil {
		return err
	}
	tmp, err := os.MkdirTemp("", "yaroze-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	b := &toolchain.Build{
		Package: pkg,
		Opt:     f.opt,
		Tags:    f.tags,
		HotBase: abs,
		HotAt:   at,
		Output:  filepath.Join(tmp, "patch.elf"),
		Stderr:  os.Stderr,
		Log:     f.logf,
	}
	return f.finish(ctx, c, b, "", out)
}

func patchCommand() *cobra.Command {
	var f buildFlags
	var base, out string
	var at flags.Address
	cmd := &cobra.Command{
		Use:   "patch [flags] --base prog.elf [package]",
		Short: "compile a patch for a running program built with --hot",
		Long: `patch compiles the main package in the directory given, the current one
if none is, as a patch for the program whose ELF --base names, which build
--hot or disc --hot wrote with --elf: the main package's functions
compiled afresh, bound by address to the running program's variables and
library code, which an emulator loads into the running program between
frames. The package's other packages, its variables and its types must be
as they were when the program was built; a variable that changed size is
an error, and the program has to be built and started again.

--at is the address to link the patch at, inside the region the program
reserved; the region's start if not given. A patch is linked at an address
no patch before it covers, since the program may still be running their
code, so a second patch goes after the first. -o names the ELF to write,
patch.elf if not given.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			defer f.close()
			pkg := "."
			if len(args) == 1 {
				pkg = args[0]
			}
			if out == "" {
				out = "patch.elf"
			}
			return f.patch(cmd.Context(), pkg, base, uint32(at), out)
		},
	}
	cmd.Flags().StringVar(&base, "base", "", "the ELF of the program the patch is for, built with --hot")
	cmd.Flags().Var(&at, "at", "the address to link the patch at, hexadecimal with 0x")
	cmd.Flags().StringVarP(&out, "output", "o", "", "the ELF to write")
	cmd.MarkFlagRequired("base")
	f.register(cmd)
	return cmd
}
