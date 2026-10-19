// Command wav2vag encodes sounds into VAG files, the SPU's ADPCM, which a
// program plays as sound effects or puts in a VAB as an instrument: what
// Sony's aiff2vag did.
package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"go.chrisrx.dev/psx/format/vag"
	"go.chrisrx.dev/psx/internal/pcm"
)

var opts struct {
	Output string
	NoLoop bool
	Loop   bool
	Rate   int
	Big    bool
	Name   string
}

func main() {
	cmd := &cobra.Command{
		Use:   "wav2vag [flags] <sound>...",
		Short: "encode sounds into VAG files",
		Long: `wav2vag encodes WAV or AIFF files, 8 or 16 bits, into VAG files, each
beside its sound with the extension .VAG unless -o names the one output.
Stereo is mixed down to mono, since a voice plays one channel. The rate is
the file's: the SPU plays a VAG at any rate, set by the tone's pitch, but a
VAB plays it at its own rate only at the tone's centre note.

Each block of 28 samples is encoded with the filter and shift that decode
closest to it, worked out with the SPU's own arithmetic, which comes out a
few decibels cleaner than Sony's encoder.

A loop the file marks, a WAV's smpl chunk or an AIFF's sustain loop, is
kept: the sound plays to the loop's end and then repeats from its start, a
whole block at a time, so loops sound right when their ends are multiples
of 28 samples. -1 leaves the loop out and -L loops the whole sound.

-R reads headerless 16-bit samples at that rate, little-endian unless -E.`,
		Args:         cobra.MinimumNArgs(1),
		RunE:         run,
		SilenceUsage: true,
	}
	f := cmd.Flags()
	f.StringVarP(&opts.Output, "output", "o", "", "where to write the VAG, for one sound")
	f.BoolVarP(&opts.NoLoop, "no-loop", "1", false, "play once, whatever loop the file marks")
	f.BoolVarP(&opts.Loop, "loop", "L", false, "loop the whole sound")
	f.IntVarP(&opts.Rate, "rate", "R", 0, "read headerless 16-bit samples at this rate")
	f.BoolVarP(&opts.Big, "big-endian", "E", false, "headerless samples are big-endian")
	f.StringVar(&opts.Name, "name", "", "the name in the header, 16 bytes: the file's name without its extension unless given")

	if err := cmd.Execute(); err != nil {
		log.Fatal(err)
	}
}

func run(cmd *cobra.Command, args []string) error {
	if opts.Output != "" && len(args) > 1 {
		return fmt.Errorf("-o names one output, for one sound, not %d", len(args))
	}
	if opts.Loop && opts.NoLoop {
		return fmt.Errorf("-1 and -L contradict each other")
	}
	for _, path := range args {
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		var snd *pcm.Sound
		if opts.Rate > 0 {
			snd = pcm.Raw(b, opts.Rate, opts.Big)
		} else if snd, err = pcm.Read(b); err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		var o vag.Options
		switch {
		case opts.Loop:
			o.Loop = &[2]int{0, len(snd.Samples)}
		case !opts.NoLoop:
			o.Loop = snd.Loop
		}
		stem := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
		name := opts.Name
		if name == "" {
			name = strings.ToLower(stem)
		}
		if len(name) > 16 {
			name = name[:16]
		}
		v := &vag.VAG{Rate: snd.Rate, Name: name, Data: vag.Encode(snd.Samples, o)}
		out := opts.Output
		if out == "" {
			out = strings.TrimSuffix(path, filepath.Ext(path)) + ".VAG"
		}
		if err := os.WriteFile(out, v.Bytes(), 0o644); err != nil {
			return err
		}
		loop := ""
		if o.Loop != nil {
			loop = fmt.Sprintf(", looping samples %d-%d", o.Loop[0], o.Loop[1])
		}
		fmt.Printf("created %q: %d samples at %d Hz, %d bytes%s\n", out, len(snd.Samples), snd.Rate, len(v.Data), loop)
	}
	return nil
}
