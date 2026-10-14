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
	"time"

	"github.com/spf13/cobra"
	"go.chrisrx.dev/psx/format/iso9660"
	"go.chrisrx.dev/psx/format/psxdisc"
)

var opts struct {
	Output      string
	VolumeID    string
	SystemID    string
	Publisher   string
	Preparer    string
	Application string
	Date        string
	FileTimes   bool
	Boot        string
	License     string
	Padding     uint32
	Audio       []string
	Raw         []string
	NoRaw       bool
	NoXA        bool
	ISO         bool
	Verbose     bool
}

// defaultRawPatterns are the files taken to hold whole Mode 2 sectors rather
// than a byte stream. Those extensions mean exactly that on a PlayStation
// disc: an .XA is interleaved ADPCM audio and a .STR is video, both produced
// as 2336-byte sectors with the subheaders already in place.
var defaultRawPatterns = []string{"*.XA", "*.STR"}

// markRaw marks the files whose contents are raw sectors. Patterns are
// matched against the path on the disc without its version suffix, so both
// "*.xa" and "data/music.xa" select the same file.
func markRaw(d *iso9660.Dir, prefix string, patterns []string) error {
	for _, e := range d.Entries {
		path := strings.ToUpper(prefix + strings.TrimSuffix(e.Name, ";1"))
		for _, pat := range patterns {
			ok, err := filepath.Match(strings.ToUpper(pat), path)
			if err != nil {
				return fmt.Errorf("pattern %q: %w", pat, err)
			}
			if ok {
				e.Raw = true
				break
			}
		}
	}
	for _, sub := range d.Dirs {
		if err := markRaw(sub, prefix+sub.Name+"/", patterns); err != nil {
			return err
		}
	}
	return nil
}

// parseDate reads the --date flag. An image is reproducible by default, so an
// unset date means the descriptor records no date at all rather than the time
// of the build.
func parseDate(s string) (time.Time, error) {
	switch s {
	case "":
		return time.Time{}, nil
	case "now":
		return time.Now(), nil
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02 15:04:05", "2006-01-02"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("date %q is not \"now\", a date, or an RFC 3339 timestamp", s)
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

Files named .xa or .str are taken to be streamed media, which is produced as
whole 2336-byte Mode 2 sectors with their subheaders already in place, and are
written through untouched rather than packed into logical blocks. --raw adds
patterns to that set and --no-raw empties it.

--audio appends a CD audio track, which a program plays by its track number
rather than opening as a file. The two second pause the standard requires
between the data track and the audio is inserted automatically.

The image ends with two seconds of empty sectors, because a drive reads ahead
of the sector it was asked for and a file ending at the very end of the track
is one the console cannot finish reading. --padding sets how many, and 0
leaves none.

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
	f.StringVar(&opts.SystemID, "system-id", "PLAYSTATION", "target system recorded in the volume descriptor")
	f.StringVar(&opts.Publisher, "publisher", "", "publisher recorded in the volume descriptor")
	f.StringVar(&opts.Preparer, "preparer", "", "data preparer recorded in the volume descriptor")
	f.StringVar(&opts.Application, "application", "", "application recorded in the volume descriptor")
	f.StringVar(&opts.Date, "date", "", `volume date, as a timestamp or "now"; unset records no date, which keeps the image reproducible`)
	f.BoolVar(&opts.FileTimes, "file-times", false, "record each file's modification time instead of the volume date")
	f.StringVar(&opts.Boot, "boot", "", "executable to boot, written into a generated SYSTEM.CNF")
	f.StringVar(&opts.License, "license", "", "licence data for the first 32KB, needed only by unmodified hardware")
	f.Uint32Var(&opts.Padding, "padding", psxdisc.StandardPadding, "empty sectors to leave after the last file, so that reading it does not run into the lead-out; 0 for none")
	f.StringArrayVar(&opts.Audio, "audio", nil, "WAV or raw 44.1kHz 16-bit stereo file to record as a CD audio track; may be repeated")
	f.StringArrayVar(&opts.Raw, "raw", nil, "also treat files matching this pattern as whole Mode 2 sectors; may be repeated")
	f.BoolVar(&opts.NoRaw, "no-raw", false, "treat every file as a byte stream, including .xa and .str")
	f.BoolVar(&opts.NoXA, "no-xa", false, "leave out the CD-XA signature and directory attributes")
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
		note := ""
		if e.Raw {
			note = "  raw sectors"
		}
		fmt.Fprintf(w, "  %-40s %8d bytes  LBA %d%s\n", prefix+"\\"+e.Name, e.Size, e.LBA, note)
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

	if !opts.NoRaw {
		if err := markRaw(root, "", append(defaultRawPatterns, opts.Raw...)); err != nil {
			return err
		}
	}

	if opts.Boot != "" {
		addSystemCNF(root, iso9660.FileIdentifier(filepath.Base(opts.Boot)))
	}

	created, err := parseDate(opts.Date)
	if err != nil {
		return err
	}

	isoOpts := iso9660.Options{
		VolumeID:    opts.VolumeID,
		SystemID:    opts.SystemID,
		Publisher:   opts.Publisher,
		Preparer:    opts.Preparer,
		Application: opts.Application,
		Created:     created,
		FileTimes:   opts.FileTimes,
		OmitXA:      opts.NoXA,
		Padding:     opts.Padding,
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

	tracks, err := audioTracks(opts.Audio)
	if err != nil {
		return err
	}
	if opts.ISO {
		// Both of these would otherwise fail part way through writing,
		// leaving an image that is neither finished nor absent.
		if len(tracks) > 0 {
			return fmt.Errorf("a 2048-byte image holds only the filesystem, so it cannot carry the %d audio track(s) given", len(tracks))
		}
		if name := firstRaw(root, ""); name != "" {
			return fmt.Errorf("%s holds raw Mode 2 sectors, which a 2048-byte image cannot represent; --no-raw writes it as ordinary data", name)
		}
	}

	out, err := os.Create(opts.Output)
	if err != nil {
		return err
	}
	defer out.Close()

	var layout psxdisc.Layout
	if opts.ISO {
		if err := w.WriteTo(psxdisc.NewISOWriter(out)); err != nil {
			return err
		}
	} else {
		img := &psxdisc.Image{Data: w, Audio: tracks}
		if layout, err = img.Write(out); err != nil {
			return err
		}
	}
	if err := out.Close(); err != nil {
		return err
	}

	// A raw image needs a cue sheet to say where its tracks are.
	cueName := ""
	if !opts.ISO {
		cueName = strings.TrimSuffix(opts.Output, filepath.Ext(opts.Output)) + ".cue"
		cue, err := os.Create(cueName)
		if err != nil {
			return err
		}
		if err := psxdisc.WriteCueSheet(cue, opts.Output, layout); err != nil {
			cue.Close()
			return err
		}
		if err := cue.Close(); err != nil {
			return err
		}
	}

	if opts.Verbose {
		describe(os.Stdout, root, "")
		for _, t := range layout.Tracks {
			if t.Audio {
				fmt.Printf("  %-40s %8d sectors  track %d\n", t.Name, t.Sectors, t.Number)
			}
		}
	}
	fmt.Printf("created %q: %d blocks", opts.Output, w.TotalBlocks())
	if opts.Padding > 0 {
		fmt.Printf(" (%d of run-out)", opts.Padding)
	}
	if len(tracks) > 0 {
		fmt.Printf(" and %d audio track(s), %d sectors in all", len(tracks), layout.Sectors())
	}
	if cueName != "" {
		fmt.Printf(", %q", cueName)
	}
	fmt.Println()
	return nil
}

// firstRaw returns the path of the first file of raw sectors in the tree, or
// an empty string if there is none.
func firstRaw(d *iso9660.Dir, prefix string) string {
	for _, e := range d.Entries {
		if e.Raw {
			return prefix + "\\" + e.Name
		}
	}
	for _, sub := range d.Dirs {
		if name := firstRaw(sub, prefix+"\\"+sub.Name); name != "" {
			return name
		}
	}
	return ""
}

// audioTracks reads the files to be recorded as CD audio tracks.
func audioTracks(paths []string) ([]*psxdisc.AudioTrack, error) {
	var out []*psxdisc.AudioTrack
	for _, p := range paths {
		t, err := psxdisc.AudioTrackFile(p)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, nil
}
