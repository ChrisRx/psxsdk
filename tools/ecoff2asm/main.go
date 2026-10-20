// Command ecoff2asm converts ECOFF objects, or every object in an ar
// archive, into GNU as source a current mipsel-none-elf toolchain
// assembles, and with --ld-script writes the absolute symbols as PROVIDE
// definitions: how the Net Yaroze library was relinked as ELF.
package main

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"go.chrisrx.dev/psx/format/ecoff"
)

var opts struct {
	OutputDir string
	Annotate  bool
	LdScript  bool
	Exclude   []string
}

func NewEcoff2AsmCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ecoff2asm [flags] <file>",
		Short: "convert ECOFF objects to GNU as source",
		Long: `ecoff2asm converts ECOFF objects, or every object in an ar archive,
into GNU as source that a modern mipsel-none-elf toolchain can assemble.

Section contents are emitted as .word directives except where a relocation
applies, in which case the storage unit is respelled symbolically so that the
assembler recreates the original reference. This is what makes the output
usable: a plain disassembly of a relocatable object renders every external
call as "jal 0" and every symbol reference as 0, because ECOFF keeps those
addresses in its relocation tables rather than in the instruction stream.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			objects, err := load(args[0])
			if err != nil {
				return err
			}

			excluded := make(map[string]bool)
			for _, e := range opts.Exclude {
				excluded[e] = true
			}
			kept := objects[:0:0]
			for _, o := range objects {
				if !excluded[o.name] {
					kept = append(kept, o)
				}
			}

			if opts.LdScript {
				return runLdScript(kept)
			}

			for _, o := range kept {
				var buf bytes.Buffer
				err := WriteAsm(&buf, o.file, AsmOptions{
					Source:   fmt.Sprintf("%s(%s)", filepath.Base(args[0]), o.name),
					Annotate: opts.Annotate,
				})
				if err != nil {
					return fmt.Errorf("%s: %v", o.name, err)
				}
				if err := writeOrPrint(asmName(o.name), buf.Bytes()); err != nil {
					return err
				}
			}
			return nil
		},
	}

	f := cmd.PersistentFlags()
	f.StringVarP(&opts.OutputDir, "output-dir", "o", "", "write output files to this directory instead of stdout")
	f.BoolVarP(&opts.Annotate, "annotate", "a", false, "comment each relocated instruction with its original encoding")
	f.BoolVar(&opts.LdScript, "ld-script", false, "emit ld PROVIDE definitions for absolute symbols instead of assembly")
	f.StringSliceVarP(&opts.Exclude, "exclude", "x", nil, "archive members to skip")
	return cmd
}

func main() {
	if err := NewEcoff2AsmCommand().Execute(); err != nil {
		log.Fatal(err)
	}
}

// asmName derives the assembly file name for an archive member.
func asmName(member string) string {
	base := strings.TrimSuffix(strings.TrimSuffix(member, ".o"), ".O")
	return strings.ToLower(base) + ".S"
}

func writeOrPrint(path string, data []byte) error {
	if opts.OutputDir == "" {
		_, err := os.Stdout.Write(data)
		return err
	}
	if err := os.MkdirAll(opts.OutputDir, 0o755); err != nil {
		return err
	}
	out := filepath.Join(opts.OutputDir, path)
	if err := os.WriteFile(out, data, 0o644); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "wrote %s (%d bytes)\n", out, len(data))
	return nil
}

// runLdScript emits ld PROVIDE definitions for every absolute symbol in the
// input, which is how the libps API entry points are described.
func runLdScript(objects []namedFile) error {
	var buf bytes.Buffer
	var total int
	for _, o := range objects {
		syms := AbsoluteSymbols(o.file)
		if len(syms) == 0 {
			continue
		}
		fmt.Fprintf(&buf, "  /* %d symbols from %s */\n", len(syms), o.name)
		if err := WriteSymbolDefinitions(&buf, syms); err != nil {
			return err
		}
		total += len(syms)
	}
	if total == 0 {
		return fmt.Errorf("no absolute symbols found")
	}
	fmt.Fprintf(os.Stderr, "%d absolute symbols\n", total)
	return writeOrPrint("libps.ld", buf.Bytes())
}

type namedFile struct {
	name string
	file *ecoff.File
}

// load reads the input as an archive, falling back to a single object.
func load(path string) ([]namedFile, error) {
	if a, err := ecoff.OpenArchive(path); err == nil {
		defer a.Close()
		var objects []namedFile
		for _, m := range a.Objects() {
			objects = append(objects, namedFile{name: m.Name, file: m.File})
		}
		if len(objects) == 0 {
			return nil, fmt.Errorf("%s: archive contains no ECOFF objects", path)
		}
		return objects, nil
	} else if err != ecoff.ErrNotArchive {
		return nil, err
	}
	f, err := ecoff.Open(path)
	if err != nil {
		return nil, err
	}
	return []namedFile{{name: filepath.Base(path), file: f}}, nil
}
