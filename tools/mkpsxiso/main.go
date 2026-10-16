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
	"go.chrisrx.dev/psx/disc"
	"go.chrisrx.dev/psx/format/iso9660"
	"go.chrisrx.dev/psx/format/psx"
	"go.chrisrx.dev/psx/format/psxdisc"
	"go.chrisrx.dev/psx/internal/flags"
)

var opts struct {
	Output      string
	Files       []string
	Src         string
	VolumeID    string
	SystemID    string
	Publisher   string
	Preparer    string
	Application string
	Date        string
	FileTimes   bool
	Boot        string
	Stack       flags.Address
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
		Use:   "mkpsxiso [flags] [directory]",
		Short: "build a PlayStation disc image from a directory",
		Long: `mkpsxiso builds a PlayStation disc image from a directory tree.

The tree is placed on the disc as it appears on disk, so the common case of
an executable plus the data it loads needs no layout file at all. Names are
folded to the restricted uppercase set ISO 9660 permits and given the ";1"
version suffix that PlayStation programs use when opening them. --file places
one more host file at a path on the disc, and may be repeated; with it an
image can be built with no directory at all.

A program built with the disc package registers the files it reads, each
with the host file to place at its path, and carries the registrations in its
executable. When --boot names an executable in the tree, the files it
registers are added to the image, from the directory --src names, and the
executable goes onto the disc with each registration filled in with the
sector and length its file was given, which is how the program then finds the
file without searching for it. --file MAIN.EXE=prog.exe is a whole image for a
program that registers everything else.

Files named .xa or .str are taken to be streamed media, which is produced as
whole 2336-byte Mode 2 sectors with their subheaders already in place, and are
written through untouched rather than packed into logical blocks. --raw adds
patterns to that set and --no-raw empties it.

--boot writes a SYSTEM.CNF naming the executable to start, unless the tree
already has one. Its stack starts at 801FF000 rather than the customary
801FFFF0, which leaves the BIOS the frame it needs for a program that returns
from main; --stack changes it.

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
		Args:         cobra.MaximumNArgs(1),
		RunE:         run,
		SilenceUsage: true,
	}

	f := cmd.Flags()
	f.StringVarP(&opts.Output, "output", "o", "psx.bin", "image to write")
	f.StringArrayVar(&opts.Files, "file", nil, "DISC=HOST: also place the host file at that path on the disc; may be repeated")
	f.StringVar(&opts.Src, "src", ".", "directory the files the boot executable registers are relative to")
	f.StringVar(&opts.VolumeID, "volume-id", "UNTITLED", "disc label")
	f.StringVar(&opts.SystemID, "system-id", "PLAYSTATION", "target system recorded in the volume descriptor")
	f.StringVar(&opts.Publisher, "publisher", "", "publisher recorded in the volume descriptor")
	f.StringVar(&opts.Preparer, "preparer", "", "data preparer recorded in the volume descriptor")
	f.StringVar(&opts.Application, "application", "", "application recorded in the volume descriptor")
	f.StringVar(&opts.Date, "date", "", `volume date, as a timestamp or "now"; unset records no date, which keeps the image reproducible`)
	f.BoolVar(&opts.FileTimes, "file-times", false, "record each file's modification time instead of the volume date")
	f.StringVar(&opts.Boot, "boot", "", "executable to boot, written into a generated SYSTEM.CNF")
	opts.Stack = defaultStack
	f.Var(&opts.Stack, "stack", "initial stack pointer written into a generated SYSTEM.CNF; values above the default grow over the BIOS's frame, which breaks returning from main")
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
// anything. BOOT names the executable; TCB and EVENT are the defaults every
// PlayStation title uses.
//
// STACK is not, and --stack sets it. Titles conventionally use 801FFFF0, but
// the BIOS that calls the executable keeps its own frame, with the registers
// it restores when the executable returns, at the top of RAM. A stack starting
// at 801FFFF0 grows straight over it, so a program that returns from main
// comes back to an address it wrote there itself. Commercial titles never
// return, so they never notice; a Net Yaroze program's crt0 does. Starting at
// 801FF000 leaves the BIOS its frame, and a returning program ends at the
// BIOS's "End of Main".
const systemCNFTemplate = `BOOT=cdrom:\%s
TCB=4
EVENT=10
STACK=%08X
`

// defaultStack is the STACK written when --stack is not given.
const defaultStack = 0x801ff000

// checkStack rejects a stack pointer the program could not run with: one
// outside the 2MB of main RAM, as the cached KSEG0 addresses programs use, or
// one not aligned to the 8 bytes the MIPS calling convention requires.
func checkStack(sp uint32) error {
	if sp <= 0x80000000 || sp > 0x80200000 {
		return fmt.Errorf("stack %#08x is outside main RAM (0x80000000 to 0x80200000)", sp)
	}
	if sp%8 != 0 {
		return fmt.Errorf("stack %#08x is not 8-byte aligned", sp)
	}
	return nil
}

// systemCNF returns the contents of a SYSTEM.CNF booting bootName, which is
// already a full identifier including its ";1" version suffix.
func systemCNF(bootName string, stack uint32) []byte {
	return fmt.Appendf(nil, systemCNFTemplate, strings.ToUpper(bootName), stack)
}

// hasSystemCNF reports whether the source tree provides its own SYSTEM.CNF.
func hasSystemCNF(root *iso9660.Dir) bool {
	for _, e := range root.Entries {
		if e.Name == "SYSTEM.CNF;1" {
			return true
		}
	}
	return false
}

// addSystemCNF adds a generated SYSTEM.CNF to the root of the tree.
func addSystemCNF(root *iso9660.Dir, content []byte) {
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
	root := &iso9660.Dir{}
	if len(args) == 1 {
		var err error
		if root, err = iso9660.FromDirectory(args[0]); err != nil {
			return err
		}
	} else if len(opts.Files) == 0 {
		return fmt.Errorf("nothing to put on the disc: give a directory, or --file")
	}
	for _, spec := range opts.Files {
		path, host, ok := strings.Cut(spec, "=")
		if !ok || path == "" || host == "" {
			return fmt.Errorf("--file %q: want a path on the disc, \"=\", and a host file", spec)
		}
		if _, err := place(root, path, host); err != nil {
			return err
		}
	}

	// The files the boot executable registers join the tree now, so that
	// they are laid out with everything else, and its records are filled in
	// once they have been.
	var prog *program
	if opts.Boot != "" {
		var err error
		if prog, err = loadProgram(root, filepath.Base(opts.Boot), opts.Src); err != nil {
			return err
		}
	}

	if !opts.NoRaw {
		if err := markRaw(root, "", append(defaultRawPatterns, opts.Raw...)); err != nil {
			return err
		}
	}

	// --stack only means something in a SYSTEM.CNF this command writes, so
	// asking for it where none is written is a mistake worth stopping on,
	// rather than an image that silently boots with some other stack.
	stackSet := cmd.Flags().Changed("stack")
	if err := checkStack(uint32(opts.Stack)); err != nil {
		return err
	}
	switch {
	case opts.Boot == "":
		if stackSet {
			return fmt.Errorf("--stack is written into the SYSTEM.CNF generated for --boot, and no --boot was given")
		}
	case hasSystemCNF(root):
		if stackSet {
			return fmt.Errorf("the tree provides its own SYSTEM.CNF, which is used as is, so --stack would have no effect; set STACK there instead")
		}
	default:
		addSystemCNF(root, systemCNF(iso9660.FileIdentifier(filepath.Base(opts.Boot)), uint32(opts.Stack)))
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
	if prog != nil {
		prog.patch()
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
		if prog != nil {
			prog.describe(os.Stdout)
		}
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

// splitDiscPath breaks a path on the disc, written from the root with either
// separator and with or without the ";1" suffix, into the directories on the
// way and the file's own name, neither yet folded to ISO 9660 form.
func splitDiscPath(path string) (dirs []string, name string, err error) {
	parts := strings.Split(strings.Trim(strings.ReplaceAll(path, "\\", "/"), "/"), "/")
	for _, p := range parts {
		if p == "" {
			return nil, "", fmt.Errorf("%q is not a path on the disc", path)
		}
	}
	name = strings.TrimSuffix(strings.ToUpper(parts[len(parts)-1]), ";1")
	return parts[:len(parts)-1], name, nil
}

// discPath is a path as it will read on the disc, for messages.
func discPath(path string) string {
	dirs, name, err := splitDiscPath(path)
	if err != nil {
		return path
	}
	out := ""
	for _, d := range dirs {
		out += "\\" + iso9660.NormalizeName(d)
	}
	return out + "\\" + iso9660.FileIdentifier(name)
}

// place adds the host file to the tree at path, making the directories on the
// way. A path something already occupies is an error rather than a choice
// between the two: whichever won, the program would be reading the other.
func place(root *iso9660.Dir, path, host string) (*iso9660.Entry, error) {
	dirs, name, err := splitDiscPath(path)
	if err != nil {
		return nil, err
	}
	d := root
	for _, dir := range dirs {
		d = d.Mkdir(dir)
	}
	if d.Find(name) != nil {
		return nil, fmt.Errorf("%s is already on the disc", discPath(path))
	}
	return d.AddFile(name, host)
}

// A program is the boot executable, held in memory so that the records of
// the files it registers, found as package disc describes, can be filled in
// once the image is laid out.
type program struct {
	entry *iso9660.Entry
	data  []byte // the PSX-EXE as a whole
	image []byte // its text, within data
	files []registered
}

// A registered file is one the program asked for, and where it went.
type registered struct {
	disc.Record
	entry *iso9660.Entry
}

// loadProgram reads the executable called name in the root of the tree and
// places the files it registers, each relative to src. It returns nil if
// the tree holds no such file, since --boot may name one it does not.
func loadProgram(root *iso9660.Dir, name, src string) (*program, error) {
	e := root.Find(name)
	if e == nil {
		return nil, nil
	}
	rc, err := e.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	data, err := io.ReadAll(rc)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", e.Name, err)
	}
	exe, err := psx.ParseFile(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("%s is not a PSX-EXE: %w", e.Name, err)
	}
	p := &program{entry: e, data: data}
	p.image = data[psx.HeaderSize : psx.HeaderSize+int(exe.TextSize)]

	// A file registered twice from the same host file, as two packages
	// might, is one file with two records to fill in. From different host
	// files it is a mistake, since only one can be at the path.
	placed := map[string]registered{}
	for _, r := range disc.Find(p.image, exe.TextAddr) {
		key := discPath(r.Name)
		if prev, ok := placed[key]; ok {
			if prev.Src != r.Src {
				return nil, fmt.Errorf("%s registers %s from both %s and %s", e.Name, key, prev.Src, r.Src)
			}
			p.files = append(p.files, registered{Record: r, entry: prev.entry})
			continue
		}
		entry, err := place(root, r.Name, filepath.Join(src, filepath.FromSlash(r.Src)))
		if err != nil {
			return nil, fmt.Errorf("%s registers %s: %w", e.Name, r.Name, err)
		}
		reg := registered{Record: r, entry: entry}
		placed[key] = reg
		p.files = append(p.files, reg)
	}
	return p, nil
}

// patch writes each registered file's sector and length into its record, and
// has the executable written to the disc from the patched copy. It is called
// after layout, when the entries know their addresses.
func (p *program) patch() {
	for _, f := range p.files {
		f.Patch(p.image, f.entry.LBA, f.entry.Size)
	}
	data := p.data
	p.entry.Open = func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(data)), nil
	}
}

// describe lists the files the program registered and where they came from.
func (p *program) describe(w io.Writer) {
	for _, f := range p.files {
		fmt.Fprintf(w, "  %-40s registered by \\%s from %s\n", discPath(f.Name), p.entry.Name, f.Src)
	}
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
