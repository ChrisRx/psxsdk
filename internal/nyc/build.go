package nyc

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"go.chrisrx.dev/psx/internal/flags"
	"go.chrisrx.dev/psx/internal/mkpsxiso"
	"go.chrisrx.dev/psx/internal/toolchain"
	"go.chrisrx.dev/psx/link"
)

// buildFlags are the flags of every command that compiles a program.
type buildFlags struct {
	runtime string
	opt     string
	tags    []string
	loads   flags.Loads
	verbose bool
	hot     bool

	// c is the compiler, found once and kept for every build of the
	// command: under wazero it stays loaded between builds.
	c *toolchain.Wasm
}

// find returns the compiler, under the runtime the flags name, found once.
func (f *buildFlags) find() (*toolchain.Wasm, error) {
	if f.c == nil {
		c, err := toolchain.New(f.runtime)
		if err != nil {
			return nil, err
		}
		f.c = c
	}
	return f.c, nil
}

// close lets the compiler go, once the command is done building.
func (f *buildFlags) close() {
	if f.c != nil {
		f.c.Close(context.Background())
		f.c = nil
	}
}

func (f *buildFlags) register(cmd *cobra.Command) {
	fl := cmd.Flags()
	runtimeFlag(fl, &f.runtime)
	fl.StringVar(&f.opt, "opt", "", "TinyGo's optimization level: 0, 1, 2, s or z; 2 if not given, for speed, since a frame has a budget, unless the WebAssembly compiler's cache holds only another")
	fl.StringSliceVar(&f.tags, "tags", nil, "extra build tags")
	fl.Var(&f.loads, "load", "file@addr: put a file into the executable at a hexadecimal address, as siocons' local dload did; may be repeated")
	fl.BoolVarP(&f.verbose, "verbose", "v", false, "say which compiler runs, and how long each step takes")
	fl.BoolVar(&f.hot, "hot", false, "build a program that can take patches from the patch command while it runs: its main package's functions are called through slots, which costs a little speed, and 256KB is reserved for the patches")
}

// logf prints a step's progress with -v.
func (f *buildFlags) logf(format string, args ...any) {
	if f.verbose {
		fmt.Fprintf(os.Stderr, "nyc: "+format+"\n", args...)
	}
}

// pkgName is the name outputs are given by default: the base name of the
// package's directory.
func pkgName(pkg string) (string, error) {
	abs, err := filepath.Abs(pkg)
	if err != nil {
		return "", err
	}
	return filepath.Base(abs), nil
}

// build compiles pkg and writes the PSX-EXE to exe and, if elf is not empty,
// the ELF, with its traps removed, to elf. Either may be empty.
func (f *buildFlags) build(ctx context.Context, pkg, exe, elf string) error {
	start := time.Now()
	c, err := f.find()
	if err != nil {
		return err
	}
	f.logf("runtime: %s, found in %s", c.Runtime(), time.Since(start).Round(time.Millisecond))

	tmp, err := os.MkdirTemp("", "nyc-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	b := &toolchain.Build{
		Package: pkg,
		Opt:     f.opt,
		Tags:    f.tags,
		Hot:     f.hot,
		Output:  filepath.Join(tmp, "prog.elf"),
		Stderr:  os.Stderr,
		Log:     f.logf,
	}
	return f.finish(ctx, c, b, exe, elf)
}

// finish compiles b and writes its outputs, as build does.
func (f *buildFlags) finish(ctx context.Context, c *toolchain.Wasm, b *toolchain.Build, exe, elf string) error {
	if err := c.Compile(ctx, b); err != nil {
		return err
	}

	start := time.Now()
	data, err := os.ReadFile(b.Output)
	if err != nil {
		return err
	}
	traps, err := link.MIPS1Traps(data)
	if err != nil {
		return err
	}
	link.RemoveTraps(data, traps)
	if elf != "" {
		if err := os.WriteFile(elf, data, 0o666); err != nil {
			return err
		}
	}
	if exe != "" {
		img, err := link.ReadImage(bytes.NewReader(data))
		if err != nil {
			return err
		}
		x, err := link.Executable(img, true, f.loads...)
		if err != nil {
			return err
		}
		if err := x.WriteFile(exe); err != nil {
			return err
		}
	}
	f.logf("removed %d trap instruction(s) and wrote the executable in %s", len(traps), time.Since(start).Round(time.Millisecond))
	return nil
}

func buildCommand() *cobra.Command {
	var f buildFlags
	var out string
	cmd := &cobra.Command{
		Use:   "build [flags] [package]",
		Short: "compile a program into a PSX-EXE",
		Long: `build compiles the main package in the directory given, the current one
if none is, into a PSX-EXE, which the BIOS of a console or an emulator
loads and runs: the program with the Net Yaroze library combined in, so it
needs no boot disc.

-o names the file, the package directory's name with .exe if not given.
With a name ending in .elf it writes the ELF TinyGo linked instead, with
the instructions an R3000A cannot run replaced, which has the symbols an
emulator's debugger wants.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			defer f.close()
			pkg := "."
			if len(args) == 1 {
				pkg = args[0]
			}
			if out == "" {
				name, err := pkgName(pkg)
				if err != nil {
					return err
				}
				out = name + ".exe"
			}
			if strings.EqualFold(filepath.Ext(out), ".elf") {
				return f.build(cmd.Context(), pkg, "", out)
			}
			return f.build(cmd.Context(), pkg, out, "")
		},
	}
	cmd.Flags().StringVarP(&out, "output", "o", "", "the file to write: a PSX-EXE, or an ELF if it ends in .elf")
	f.register(cmd)
	return cmd
}

// discFlags are the flags of the commands that write a disc.
type discFlags struct {
	buildFlags
	out      string
	volumeID string
	files    []string
	audio    []string
	license  string
	elf      string
}

func (f *discFlags) register(cmd *cobra.Command) {
	f.buildFlags.register(cmd)
	fl := cmd.Flags()
	fl.StringVar(&f.volumeID, "volume-id", "", "the disc label, the package directory's name with .exe if not given")
	fl.StringArrayVar(&f.files, "file", nil, "DISC=HOST: also place a host file at a path on the disc; may be repeated")
	fl.StringArrayVar(&f.audio, "audio", nil, "a WAV or raw 44.1kHz 16-bit stereo file to record as a CD audio track; may be repeated")
	fl.StringVar(&f.license, "license", "", "licence data for the first 32KB, which only unmodified hardware needs")
	fl.StringVar(&f.elf, "elf", "", "also write the ELF, for its symbols")
}

// disc builds pkg onto a disc image at bin, a .bin with its .cue beside it.
func (f *discFlags) disc(ctx context.Context, pkg, bin string, stdout io.Writer) error {
	name, err := pkgName(pkg)
	if err != nil {
		return err
	}
	tmp, err := os.MkdirTemp("", "nyc-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	exe := filepath.Join(tmp, "MAIN.EXE")
	if err := f.build(ctx, pkg, exe, f.elf); err != nil {
		return err
	}

	start := time.Now()
	opts := mkpsxiso.NewOptions()
	opts.Output = bin
	opts.VolumeID = f.volumeID
	if opts.VolumeID == "" {
		opts.VolumeID = name + ".exe"
	}
	opts.Boot = "MAIN.EXE"
	opts.Files = append([]string{"MAIN.EXE=" + exe}, f.files...)
	opts.Src = pkg
	opts.Audio = f.audio
	opts.License = f.license
	opts.Verbose = f.verbose
	opts.Stdout = stdout
	if err := mkpsxiso.Build(opts); err != nil {
		return err
	}
	f.logf("wrote the disc in %s", time.Since(start).Round(time.Millisecond))
	return nil
}

// binName is the .bin for an output named name, which may be the .cue.
func binName(name string) string {
	if strings.EqualFold(filepath.Ext(name), ".cue") {
		return strings.TrimSuffix(name, filepath.Ext(name)) + ".bin"
	}
	return name
}

func discCommand() *cobra.Command {
	var f discFlags
	cmd := &cobra.Command{
		Use:   "disc [flags] [package]",
		Short: "build a program onto a disc image",
		Long: `disc builds the main package in the directory given, the current one if
none is, as build does, and writes a disc image that boots it: a .bin of
raw sectors with a .cue sheet beside it, which an emulator opens, or which
burns to a CD-R for a modified console.

The executable goes on the disc as MAIN.EXE, with a SYSTEM.CNF naming it,
and the files the program registers with package disc go with it, from
their paths relative to the package directory, with the program's records
of them filled in with where they were put.

-o names the image, the package directory's name with .bin if not given;
a name ending in .cue names the .bin beside it.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			defer f.close()
			pkg := "."
			if len(args) == 1 {
				pkg = args[0]
			}
			out := f.out
			if out == "" {
				name, err := pkgName(pkg)
				if err != nil {
					return err
				}
				out = name + ".bin"
			}
			return f.disc(cmd.Context(), pkg, binName(out), cmd.OutOrStdout())
		},
	}
	cmd.Flags().StringVarP(&f.out, "output", "o", "", "the image to write, a .bin, with its .cue beside it")
	f.register(cmd)
	return cmd
}
