// Command mdecenc encodes pictures into a movie for the PlayStation's motion
// decoder.
package main

import (
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"log"
	"os"

	"github.com/spf13/cobra"

	"go.chrisrx.dev/psx/format/mdec"
)

var opts struct {
	Output string
	QScale int
}

func main() {
	cmd := &cobra.Command{
		Use:   "mdecenc [flags] <picture>...",
		Short: "encode pictures into an MDEC movie",
		Long: `mdecenc encodes PNG, JPEG or GIF pictures, in the order given, into a
movie of run-length codes the MDEC decodes as they are, laid out as
format/mdec describes. Every picture must be the same size; each is
extended to a multiple of 16 pixels each way. The mdec example plays one.`,
		Args:         cobra.MinimumNArgs(1),
		RunE:         run,
		SilenceUsage: true,
	}
	f := cmd.Flags()
	f.StringVarP(&opts.Output, "output", "o", "MOVIE.MDC", "where to write the movie")
	f.IntVarP(&opts.QScale, "qscale", "q", 8, "quantization scale, 1 to 63: higher is smaller and blurrier")

	if err := cmd.Execute(); err != nil {
		log.Fatal(err)
	}
}

func run(cmd *cobra.Command, args []string) error {
	if opts.QScale < 1 || opts.QScale > 63 {
		return fmt.Errorf("--qscale %d is not between 1 and 63", opts.QScale)
	}
	var m mdec.Movie
	var size image.Point
	for i, path := range args {
		img, err := readImage(path)
		if err != nil {
			return err
		}
		if i == 0 {
			size = img.Bounds().Size()
			m.Width, m.Height = mdec.Size(size.X, size.Y)
		} else if img.Bounds().Size() != size {
			return fmt.Errorf("%s is %v, not %v as %s is", path, img.Bounds().Size(), size, args[0])
		}
		m.Frames = append(m.Frames, mdec.Encode(img, opts.QScale))
	}

	f, err := os.Create(opts.Output)
	if err != nil {
		return err
	}
	n, err := m.WriteTo(f)
	if err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	fmt.Printf("created %q: %dx%d, %d frames, %d bytes\n", opts.Output, m.Width, m.Height, len(m.Frames), n)
	return nil
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
