// Command mkpsxiso builds a PlayStation disc image from a directory.
package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"go.chrisrx.dev/psx/format/iso9660"
	"go.chrisrx.dev/psx/format/psxdisc"
)

var opts struct {
	Output   string
	VolumeID string
	Boot     string
	License  string
	ISO      bool
	Verbose  bool
}

func main() {
	cmd := &cobra.Command{
		Use:   "mkpsxiso [flags] <directory>",
		Short: "build a PlayStation disc image from a directory",
		Long: `mkpsxiso builds a PlayStation disc image from a directory tree.

The tree is placed on the disc as it appears on disk, so the common case of
an executable plus the data it loads needs no layout file at all. Names are
folded to the restricted uppercase set ISO 9660 permits and given the ";1"
version suffix that PlayStation programs use when opening them.

By default it writes a .bin of raw 2352-byte Mode 2 sectors along with a .cue
sheet. --iso writes plain 2048-byte blocks instead, which cannot represent
audio or streamed tracks but can be read by ordinary ISO 9660 tools.`,
		Args:         cobra.ExactArgs(1),
		RunE:         run,
		SilenceUsage: true,
	}

	f := cmd.Flags()
	f.StringVarP(&opts.Output, "output", "o", "psx.bin", "image to write")
	f.StringVar(&opts.VolumeID, "volume-id", "UNTITLED", "disc label")
	f.StringVar(&opts.Boot, "boot", "", "executable to boot, written into a generated SYSTEM.CNF")
	f.StringVar(&opts.License, "license", "", "licence data for the first 32KB, needed only by unmodified hardware")
	f.BoolVar(&opts.ISO, "iso", false, "write 2048-byte blocks instead of raw sectors")
	f.BoolVarP(&opts.Verbose, "verbose", "v", false, "list the files placed on the disc")

	if err := cmd.Execute(); err != nil {
		log.Fatal(err)
	}
}

// systemCNF is the boot configuration the console reads before loading
// anything. BOOT names the executable; the remaining values are the defaults
// every PlayStation title uses.
// bootName is already a full identifier including its ";1" version suffix.
const systemCNFTemplate = `BOOT=cdrom:\%s
TCB=4
EVENT=10
STACK=801FFFF0
`

// addSystemCNF synthesises a SYSTEM.CNF naming the boot executable, unless
// the source tree already provides one.
func addSystemCNF(root *iso9660.Dir, bootName string) {
	for _, e := range root.Entries {
		if e.Name == "SYSTEM.CNF;1" {
			return
		}
	}
	content := fmt.Appendf(nil, systemCNFTemplate, strings.ToUpper(bootName))
	root.Entries = append(root.Entries, &iso9660.Entry{
		Name: "SYSTEM.CNF;1",
		Size: uint32(len(content)),
		Open: func() (io.ReadCloser, error) {
			return io.NopCloser(bytes.NewReader(content)), nil
		},
	})
}

// describe prints the laid out image, which is the quickest way to see
// whether a file landed on the disc under the name a program will ask for.
func describe(w io.Writer, d *iso9660.Dir, prefix string) {
	for _, e := range d.Entries {
		fmt.Fprintf(w, "  %-40s %8d bytes  LBA %d\n", prefix+"\\"+e.Name, e.Size, e.LBA)
	}
	for _, sub := range d.Dirs {
		fmt.Fprintf(w, "  %-40s %8s        LBA %d\n", prefix+"\\"+sub.Name, "<dir>", sub.LBA)
		describe(w, sub, prefix+"\\"+sub.Name)
	}
}

func run(cmd *cobra.Command, args []string) error {
	root, err := iso9660.FromDirectory(args[0])
	if err != nil {
		return err
	}

	if opts.Boot != "" {
		addSystemCNF(root, iso9660.FileIdentifier(filepath.Base(opts.Boot)))
	}

	isoOpts := iso9660.Options{
		VolumeID: strings.ToUpper(opts.VolumeID),
		// The identification string alone is enough for an emulator to
		// recognise the image as a PlayStation disc.
		SystemArea: psxdisc.SystemArea(),
	}
	if opts.License != "" {
		// Booting real hardware also wants the licence data, which holds
		// the startup logo. It is Sony's and cannot be redistributed, so
		// it comes from a file the user supplies.
		data, err := os.ReadFile(opts.License)
		if err != nil {
			return fmt.Errorf("reading licence: %w", err)
		}
		isoOpts.SystemArea = data
	}

	w, err := iso9660.NewWriter(root, isoOpts)
	if err != nil {
		return err
	}

	out, err := os.Create(opts.Output)
	if err != nil {
		return err
	}
	defer out.Close()

	var bw iso9660.BlockWriter
	if opts.ISO {
		bw = psxdisc.NewISOWriter(out)
	} else {
		bw = psxdisc.NewRawWriter(out)
	}
	if err := w.WriteTo(bw); err != nil {
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}

	// A raw image needs a cue sheet to say what its single track is.
	cueName := ""
	if !opts.ISO {
		cueName = strings.TrimSuffix(opts.Output, filepath.Ext(opts.Output)) + ".cue"
		cue, err := os.Create(cueName)
		if err != nil {
			return err
		}
		if err := psxdisc.WriteCueSheet(cue, opts.Output); err != nil {
			cue.Close()
			return err
		}
		if err := cue.Close(); err != nil {
			return err
		}
	}

	if opts.Verbose {
		describe(os.Stdout, root, "")
	}
	fmt.Printf("created %q: %d blocks", opts.Output, w.TotalBlocks())
	if cueName != "" {
		fmt.Printf(", %q", cueName)
	}
	fmt.Println()
	return nil
}
