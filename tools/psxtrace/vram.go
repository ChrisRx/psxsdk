package main

import (
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"

	"github.com/spf13/cobra"
)

const (
	vramWidth  = 1024
	vramHeight = 512
)

func newVRAMCommand() *cobra.Command {
	var pngPath string
	cmd := &cobra.Command{
		Use:   "vram [flags] <vram-a> [vram-b]",
		Short: "compare VRAM dumps, or convert one to an image",
		Long: `vram reads VRAM dumps, as psxe writes with PSXE_TRACE_VRAM and psxemu
with --vram: 1024x512 16-bit pixels.

Given two, it reports how many pixels differ and where, and exits 1 if any
do. --png writes an image: of the dump, given one, or given two, of the
first with the differing pixels in magenta.`,
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			a, err := readVRAM(args[0])
			if err != nil {
				return err
			}
			if len(args) == 1 {
				if pngPath == "" {
					return fmt.Errorf("nothing to do with one dump without --png")
				}
				return writePNG(pngPath, a, nil)
			}
			b, err := readVRAM(args[1])
			if err != nil {
				return err
			}

			var diff []bool
			n, x0, y0, x1, y1 := 0, vramWidth, vramHeight, -1, -1
			for i := range a {
				if a[i] == b[i] {
					continue
				}
				if diff == nil {
					diff = make([]bool, len(a))
				}
				diff[i] = true
				n++
				x, y := i%vramWidth, i/vramWidth
				x0, y0 = min(x0, x), min(y0, y)
				x1, y1 = max(x1, x), max(y1, y)
			}
			if pngPath != "" {
				if err := writePNG(pngPath, a, diff); err != nil {
					return err
				}
			}
			if n == 0 {
				fmt.Println("VRAM matches")
				return nil
			}
			fmt.Printf("%d pixels differ, within (%d,%d)-(%d,%d)\n", n, x0, y0, x1, y1)
			shown := 0
			for i := range a {
				if diff[i] && shown < 10 {
					fmt.Printf("  (%d,%d): %04x %04x\n", i%vramWidth, i/vramWidth, a[i], b[i])
					shown++
				}
			}
			os.Exit(1)
			return nil
		},
	}
	cmd.Flags().StringVar(&pngPath, "png", "", "write an image here")
	return cmd
}

func readVRAM(path string) ([]uint16, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(data) != 2*vramWidth*vramHeight {
		return nil, fmt.Errorf("%s: %d bytes, not a VRAM dump", path, len(data))
	}
	v := make([]uint16, vramWidth*vramHeight)
	for i := range v {
		v[i] = binary.LittleEndian.Uint16(data[2*i:])
	}
	return v, nil
}

// writePNG writes VRAM as 15-bit colour, marking the pixels in diff.
func writePNG(path string, v []uint16, diff []bool) error {
	img := image.NewRGBA(image.Rect(0, 0, vramWidth, vramHeight))
	for i, p := range v {
		c := color.RGBA{
			R: uint8(p&0x1f) << 3,
			G: uint8(p>>5&0x1f) << 3,
			B: uint8(p>>10&0x1f) << 3,
			A: 0xff,
		}
		if diff != nil && diff[i] {
			c = color.RGBA{R: 0xff, B: 0xff, A: 0xff}
		}
		img.SetRGBA(i%vramWidth, i/vramWidth, c)
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	if err := png.Encode(f, img); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}
