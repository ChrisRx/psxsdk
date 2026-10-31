package mkdisc

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"go.chrisrx.dev/psx/internal/flags"
)

// Command returns the mkdisc command, as use.
func Command(use string) *cobra.Command {
	opts := NewOptions()
	var noRaw bool
	var date string
	cmd := &cobra.Command{
		Use:   use + " [flags] [directory]",
		Short: "build a PlayStation disc image",
		Long: use + ` writes a disc image: a .bin of raw CD-ROM XA sectors with a .cue
sheet beside it naming its tracks, which an emulator opens and which burns
to a CD-R, or with --iso plain 2048-byte blocks for ordinary ISO 9660 tools.
An output ending in .zip puts the .bin and .cue in one archive, stored as
they are so that an emulator reads the sectors in place, or compressed with
--deflate.

The directory given is placed on the disc as it is, with names folded to
the capitals, digits and underscores ISO 9660 allows and given the ";1"
version a program's paths carry. --file DISC=HOST places one more host file
at a path on the disc, making the directories on the way, so an image needs
no directory at all.

--boot names the executable on the disc. A SYSTEM.CNF naming it is written,
unless the directory has one, and the files the program registers with
package disc are placed from --src, each at the path its declaration gives,
with its first sector and length written into the executable before it goes
onto the disc; a program then reads each file from its sector, with no
search of the directory. Tracks the program declares with disc.Track are
recorded after those --audio gives, and their numbers written in the same
way.

Files named .xa or .str are streams of whole Mode 2 sectors, with their
subheaders in place, and are recorded as they stand; --raw adds patterns to
that set and --no-raw empties it. --audio appends a track of CD audio, from
a WAV file or raw 44.1kHz 16-bit stereo samples, with the two second pause
the standard asks for before it.

The image ends with two seconds of empty sectors, since the console reads
whole sectors past the end of a file and a drive reads ahead of what it was
asked for: a file that ends at the end of the track is one the console
cannot finish reading. --padding sets how many, and 0 leaves none.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 1 {
				opts.Dir = args[0]
			}
			if noRaw {
				opts.Raw = nil
			}
			if date != "" {
				t, err := parseDate(date)
				if err != nil {
					return err
				}
				opts.Date = t
			}
			opts.Stdout = cmd.OutOrStdout()
			return Build(opts)
		},
		SilenceUsage: true,
	}
	f := cmd.Flags()
	f.StringVarP(&opts.Output, "output", "o", "", "the image to write: a .bin, with its .cue beside it, a .zip holding the two, or with --iso a .iso")
	f.BoolVar(&opts.Deflate, "deflate", false, "compress the .bin and .cue in a .zip rather than storing them")
	f.BoolVar(&opts.ISO, "iso", false, "write 2048-byte blocks rather than raw sectors, which holds no streams or audio")
	f.StringVar(&opts.Boot, "boot", "", "the path on the disc of the executable that boots it: SYSTEM.CNF names it, and the files it registers are placed")
	f.StringArrayVar(&opts.Files, "file", nil, "DISC=HOST: place a host file at a path on the disc; may be repeated")
	f.StringVar(&opts.Src, "src", "", "the directory the registered files' paths are relative to; the current one if not given")
	f.Var((*flags.Address)(&opts.Stack), "stack", "the stack SYSTEM.CNF gives the BIOS, which is not the program's: below the program's, so that the BIOS's frame survives the program")
	f.StringArrayVar(&opts.Audio, "audio", nil, "a WAV or raw 44.1kHz 16-bit stereo file to record as a track of CD audio; may be repeated")
	f.StringArrayVar(&opts.Raw, "raw", opts.Raw, "a pattern of names on the disc that are streams of Mode 2 sectors; may be repeated")
	f.BoolVar(&noRaw, "no-raw", false, "no file is a stream, whatever it is named")
	f.StringVar(&opts.VolumeID, "volume-id", "", "the disc label, UNTITLED if not given")
	f.StringVar(&opts.SystemID, "system-id", "", "the system identifier, PLAYSTATION if not given")
	f.StringVar(&opts.Publisher, "publisher", "", "the publisher identifier")
	f.StringVar(&opts.Preparer, "preparer", "", "the data preparer identifier")
	f.StringVar(&opts.Application, "application", "", "the application identifier")
	f.StringVar(&date, "date", "", "the volume date, as 2006-01-02 or RFC 3339; none if not given, so the same tree gives the same image")
	f.BoolVar(&opts.FileTimes, "file-times", false, "record each file's own modification time rather than the volume date")
	f.BoolVar(&opts.NoXA, "no-xa", false, "leave out the CD-XA extension, for comparison with a plain ISO 9660 writer")
	f.Uint32Var(&opts.Padding, "padding", opts.Padding, "empty sectors after the last file")
	f.StringVar(&opts.License, "license", "", "a file of license data for the first 32KB, which only unmodified hardware needs")
	f.BoolVarP(&opts.Verbose, "verbose", "v", false, "list what was placed where")
	cmd.MarkFlagRequired("output")
	return cmd
}

// parseDate reads a date, as a day or to the second.
func parseDate(s string) (time.Time, error) {
	for _, layout := range []string{time.RFC3339, time.DateOnly} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("--date %q is not a date, as 2006-01-02 or RFC 3339", s)
}
