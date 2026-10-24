// Command tim2png converts TIM files into PNG pictures, to look at a game's
// sprites and textures or to edit them and convert them back with png2tim.
package main

import (
	"fmt"
	"image/png"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"go.chrisrx.dev/psx/format/tim"
)

var opts struct {
	Output  string
	Palette int
	Offset  int
}

func main() {
	cmd := &cobra.Command{
		Use:   "tim2png [flags] <tim>...",
		Short: "convert TIM files into PNG pictures",
		Long: `tim2png writes each TIM as a PNG beside it, or to -o for one TIM, and
prints where the image and its CLUT sit in video memory. An indexed TIM
becomes an indexed PNG, its palette in order, with the CLUT's row --palette
when it holds more than one; 0x0000, which the GPU does not draw, is
transparent.

--offset reads the TIM that starts that many bytes into the file, for a game
that packs its images into one file.`,
		Args:         cobra.MinimumNArgs(1),
		RunE:         run,
		SilenceUsage: true,
	}
	f := cmd.Flags()
	f.StringVarP(&opts.Output, "output", "o", "", "where to write the PNG, for one TIM")
	f.IntVarP(&opts.Palette, "palette", "p", 0, "the CLUT row to color an indexed image with")
	f.IntVar(&opts.Offset, "offset", 0, "where in the file the TIM starts, in bytes")

	if err := cmd.Execute(); err != nil {
		log.Fatal(err)
	}
}

func run(cmd *cobra.Command, args []string) error {
	if opts.Output != "" && len(args) > 1 {
		return fmt.Errorf("-o names one output, for one TIM, not %d", len(args))
	}
	for _, path := range args {
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if opts.Offset < 0 || opts.Offset >= len(b) {
			return fmt.Errorf("%s: --offset %d is outside the file's %d bytes", path, opts.Offset, len(b))
		}
		t, err := tim.Decode(b[opts.Offset:])
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		if t.Mode.Indexed() && opts.Palette >= t.CLUT.H {
			return fmt.Errorf("%s: the CLUT has %d rows, not %d", path, t.CLUT.H, opts.Palette+1)
		}
		out := opts.Output
		if out == "" {
			out = strings.TrimSuffix(path, filepath.Ext(path)) + ".png"
		}
		f, err := os.Create(out)
		if err != nil {
			return err
		}
		if err := png.Encode(f, t.Image(opts.Palette)); err != nil {
			f.Close()
			return err
		}
		if err := f.Close(); err != nil {
			return err
		}
		fmt.Printf("created %q: %dx%d %s at (%d,%d)", out, t.Width(), t.Height(), t.Mode, t.Pixels.X, t.Pixels.Y)
		if t.CLUT != nil {
			fmt.Printf(", CLUT of %d row(s) at (%d,%d)", t.CLUT.H, t.CLUT.X, t.CLUT.Y)
		}
		fmt.Println()
	}
	return nil
}
