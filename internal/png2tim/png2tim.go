// Package png2tim converts pictures into TIM files, the PlayStation's image
// format, which a program loads into video memory as sprites and textures:
// what Sony's TIMUTIL did for BMP and PICT files on Windows. With -r it
// reads TIM files back into PNG pictures. It is the png2tim tool and
// yaroze's tim command.
package png2tim

import (
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"go.chrisrx.dev/psx/format/tim"
	_ "go.chrisrx.dev/psx/internal/bmp"
)

type options struct {
	Output string
	Bits   int
	At     string
	CLUT   string
	Black  string
	Semi   bool

	Read    bool
	Palette int
	Offset  int
}

// Command returns the command, as use: png2tim as a tool of its own, and
// tim in yaroze.
func Command(use string) *cobra.Command {
	var opts options
	cmd := &cobra.Command{
		Use:   use + " [flags] <picture>...",
		Short: "convert pictures into TIM files, or TIM files back into PNGs",
		Long: use + ` converts PNG, BMP, GIF or JPEG pictures into TIM files, each
beside its picture with the extension .TIM unless -o names the one output.

An indexed picture (a PNG or GIF with a palette, or a BMP of 8 bits or
fewer) keeps its palette in order and becomes a 4-bit TIM when it uses at
most 16 colors and an 8-bit one otherwise. Any other picture becomes 4- or
8-bit when it has that few colors and 16-bit when it has more; --bits
chooses, reducing the colors by median cut for an indexed mode.

A pixel with alpha under half is transparent, 0x0000, which the GPU does not
draw. Opaque black becomes 0x8000, black that is drawn, unless --black
transparent asks for the Net Yaroze convention of black as the transparent
color. --semi sets STP on every other color, so that the image blends
where a sprite or polygon turns semitransparency on.

--at and --clut place the pixels and the palette in video memory, which is
1024x512 16-bit units. A texture must sit inside one 64x256 texture page,
and the CLUT's x must be a multiple of 16.

-r reads TIM files instead and writes each as a PNG beside it, or to -o for
one TIM, printing where the image and its CLUT sit in video memory. An
indexed TIM becomes an indexed PNG, its palette in order, with the CLUT's
row --palette when it holds more than one; 0x0000 is transparent.
--offset reads the TIM that starts that many bytes into the file, for a
game that packs its images into one file.`,
		Example: "  " + use + " -r CUBE.TIM",
		Args:    cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if opts.Read {
				return read(&opts, args)
			}
			return run(&opts, args)
		},
		SilenceUsage: true,
	}
	f := cmd.Flags()
	f.StringVarP(&opts.Output, "output", "o", "", "where to write the TIM, for one picture")
	f.IntVarP(&opts.Bits, "bits", "b", 0, "bits per pixel, 4, 8, 16 or 24: 0 chooses")
	f.StringVar(&opts.At, "at", "640,0", "x,y of the pixels in video memory")
	f.StringVar(&opts.CLUT, "clut", "0,480", "x,y of the CLUT in video memory")
	f.StringVar(&opts.Black, "black", "opaque", "what opaque black becomes: opaque or transparent")
	f.BoolVar(&opts.Semi, "semi", false, "set STP on every drawn color")
	f.BoolVarP(&opts.Read, "read", "r", false, "read TIM files and write them as PNG pictures")
	f.IntVarP(&opts.Palette, "palette", "p", 0, "with -r, the CLUT row to color an indexed image with")
	f.IntVar(&opts.Offset, "offset", 0, "with -r, where in the file the TIM starts, in bytes")
	return cmd
}

func run(opts *options, args []string) error {
	if opts.Output != "" && len(args) > 1 {
		return fmt.Errorf("-o names one output, for one picture, not %d", len(args))
	}
	o := tim.Options{Mode: tim.Auto}
	switch opts.Bits {
	case 0:
	case 4, 8, 16, 24:
		o.Mode = map[int]tim.Mode{4: tim.Mode4, 8: tim.Mode8, 16: tim.Mode16, 24: tim.Mode24}[opts.Bits]
	default:
		return fmt.Errorf("--bits %d is not 4, 8, 16 or 24", opts.Bits)
	}
	var err error
	if o.X, o.Y, err = point(opts.At); err != nil {
		return fmt.Errorf("--at: %w", err)
	}
	if o.CLUTX, o.CLUTY, err = point(opts.CLUT); err != nil {
		return fmt.Errorf("--clut: %w", err)
	}
	switch opts.Black {
	case "opaque":
		o.Black = tim.BlackOpaque
	case "transparent":
		o.Black = tim.BlackTransparent
	default:
		return fmt.Errorf("--black %q is not opaque or transparent", opts.Black)
	}
	o.Semi = opts.Semi

	for _, path := range args {
		img, err := readImage(path)
		if err != nil {
			return err
		}
		t, err := tim.Encode(img, o)
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		out := opts.Output
		if out == "" {
			out = strings.TrimSuffix(path, filepath.Ext(path)) + ".TIM"
		}
		if err := os.WriteFile(out, t.Bytes(), 0o644); err != nil {
			return err
		}
		u, v := t.UV()
		fmt.Printf("created %q: %dx%d %s at (%d,%d), tpage %#04x, u,v %d,%d", out, t.Width(), t.Height(), t.Mode, t.Pixels.X, t.Pixels.Y, t.TPage(), u, v)
		if t.CLUT != nil {
			fmt.Printf(", CLUT at (%d,%d), cba %#04x", t.CLUT.X, t.CLUT.Y, t.CBA())
		}
		fmt.Println()
	}
	return nil
}

func point(s string) (x, y int, err error) {
	a, b, ok := strings.Cut(s, ",")
	if !ok {
		return 0, 0, fmt.Errorf("%q is not x,y", s)
	}
	if x, err = strconv.Atoi(strings.TrimSpace(a)); err == nil {
		y, err = strconv.Atoi(strings.TrimSpace(b))
	}
	return x, y, err
}

func readImage(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return img, nil
}

// read writes each TIM as a PNG.
func read(opts *options, args []string) error {
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
