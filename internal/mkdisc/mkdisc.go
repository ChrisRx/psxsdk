// Package mkdisc builds a PlayStation disc image: a .bin of raw sectors with
// a .cue naming its tracks, or plain 2048-byte blocks. What goes on the disc
// is a directory placed as it is, host files placed at paths, the executable
// that boots it with a SYSTEM.CNF naming it, the files that executable
// registers with package disc, each placed where its declaration says and
// its sector written back into the record, and CD audio after the data
// track. The formats are format/iso9660's and format/psxdisc's; this package
// decides what goes where. It is the mkdisc command, and yaroze's disc and iso
// commands.
package mkdisc

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"go.chrisrx.dev/psx/disc"
	"go.chrisrx.dev/psx/format/iso9660"
	"go.chrisrx.dev/psx/format/psx"
	"go.chrisrx.dev/psx/format/psxdisc"
	"go.chrisrx.dev/psx/internal/discrecord"
)

// DefaultRaw names the streamed media: XA audio and STR video come as whole
// Mode 2 sectors, subheaders included, and go on the disc as they stand.
var DefaultRaw = []string{"*.xa", "*.str"}

// DefaultStack is the stack SYSTEM.CNF names. It is the BIOS's stack, not
// the program's: the BIOS runs on it while it loads and starts the program,
// and the frame it returns to when main returns sits just below it. The
// program's own stack is the one its header names, at the top of main RAM,
// 0x801FFF00 for a program yaroze builds, and grows down from there. The
// default is 4KB below that, so that the program's stack does not reach the
// BIOS's frame: with the two at the same address, Sony's BIOS takes an
// illegal instruction when the program returns, since its frame has been
// overwritten. OpenBIOS keeps its stack elsewhere and does not mind.
const DefaultStack = 0x801ff000

// Options say what goes on the disc and how it is written. NewOptions gives
// the defaults.
type Options struct {
	// Output is the image to write: a .bin, with its .cue beside it, or a
	// .cue, naming the .bin beside it. With ISO it is written as it is.
	Output string

	// ISO writes 2048-byte blocks rather than raw sectors: what ordinary
	// ISO 9660 tools read, and what cannot hold a stream or an audio track.
	ISO bool

	// Dir is a directory placed on the disc as it is, or empty for none.
	Dir string

	// Files are host files placed at paths on the disc, each DISC=HOST,
	// with the directories on the way made as needed.
	Files []string

	// Boot is the path on the disc of the executable that boots it. A
	// SYSTEM.CNF naming it is written unless the disc has one, and the
	// files the executable registers with package disc are placed from
	// Src, with their sectors and sizes written back into it.
	Boot string

	// Src is the directory the registered files' host paths are relative
	// to; the current directory if empty.
	Src string

	// Stack is the stack SYSTEM.CNF names for the BIOS, DefaultStack if
	// zero. See DefaultStack for why it is not the program's.
	Stack uint32

	// Audio are CD audio tracks recorded after the data track, WAV files
	// or raw 44.1kHz 16-bit stereo samples, before any the program
	// declares.
	Audio []string

	// Raw are patterns, as path.Match takes them, of the names on the disc
	// that are streams of Mode 2 sectors, matched without regard to case.
	// A stream is recorded sector for sector rather than packed into
	// blocks, and its record says so.
	Raw []string

	// The volume descriptor's fields. An empty VolumeID is UNTITLED and an
	// empty SystemID is PLAYSTATION.
	VolumeID    string
	SystemID    string
	Publisher   string
	Preparer    string
	Application string

	// Date is the volume date, and every file's unless FileTimes is set.
	// Zero records none, so the same tree gives the same bytes.
	Date time.Time

	// FileTimes records each file's own modification time.
	FileTimes bool

	// NoXA leaves out the CD-XA extension: the signature in the volume
	// descriptor and the attributes on every record, which a PlayStation
	// disc carries and a plain ISO 9660 writer does not.
	NoXA bool

	// Padding is the run-out after the last file, in sectors.
	Padding uint32

	// License is a file of licence data for the system area, the first 32KB
	// of the disc, in place of the identification string alone.
	License string

	// Verbose lists what was placed where on Stdout, before the summary.
	Verbose bool

	// Stdout is where the listing and the summary go; os.Stdout if nil.
	Stdout io.Writer
}

// NewOptions returns the defaults: the standard run-out and the usual
// stream names.
func NewOptions() Options {
	return Options{
		Raw:     slices.Clone(DefaultRaw),
		Padding: psxdisc.StandardPadding,
		Stack:   DefaultStack,
	}
}

// Build writes the disc the options describe.
func Build(o Options) error {
	if o.Output == "" {
		return errors.New("no output to write")
	}
	out := o.Stdout
	if out == nil {
		out = os.Stdout
	}

	root := &iso9660.Dir{}
	if o.Dir != "" {
		var err error
		if root, err = iso9660.FromDirectory(o.Dir); err != nil {
			return err
		}
	}
	for _, spec := range o.Files {
		on, host, ok := strings.Cut(spec, "=")
		if !ok || on == "" || host == "" {
			return fmt.Errorf("%q is not DISC=HOST", spec)
		}
		if _, err := place(root, on, host); err != nil {
			return err
		}
	}

	var tracks []*psxdisc.AudioTrack
	for _, name := range o.Audio {
		t, err := psxdisc.AudioTrackFile(name)
		if err != nil {
			return err
		}
		tracks = append(tracks, t)
	}

	var prog *program
	if o.Boot != "" {
		var err error
		if prog, err = loadProgram(root, o.Boot, o.Src, len(tracks)); err != nil {
			return err
		}
		tracks = append(tracks, prog.tracks...)
		if root.Find("SYSTEM.CNF") == nil {
			stack := o.Stack
			if stack == 0 {
				stack = DefaultStack
			}
			cnf := systemCNF(prog.path, stack)
			root.Entries = append(root.Entries, &iso9660.Entry{
				Name: iso9660.FileIdentifier("SYSTEM.CNF"),
				Size: uint32(len(cnf)),
				Open: func() (io.ReadCloser, error) {
					return io.NopCloser(bytes.NewReader(cnf)), nil
				},
			})
		}
	}

	if err := markRaw(root, o.Raw); err != nil {
		return err
	}

	system := psxdisc.SystemArea()
	if o.License != "" {
		data, err := os.ReadFile(o.License)
		if err != nil {
			return err
		}
		system = data
	}
	w, err := iso9660.NewWriter(root, iso9660.Options{
		VolumeID:    o.VolumeID,
		SystemID:    o.SystemID,
		Publisher:   o.Publisher,
		Preparer:    o.Preparer,
		Application: o.Application,
		Created:     o.Date,
		FileTimes:   o.FileTimes,
		OmitXA:      o.NoXA,
		Padding:     o.Padding,
		SystemArea:  system,
	})
	if err != nil {
		return err
	}
	// The layout is known, so the program can be told where its files are
	// before it goes onto the disc.
	if prog != nil {
		prog.patch()
	}

	if o.Verbose {
		list(out, root, prog, tracks)
	}
	if o.ISO {
		if len(tracks) > 0 {
			return fmt.Errorf("%d audio track(s) given, which a .iso of 2048-byte blocks cannot hold", len(tracks))
		}
		if err := writeFile(o.Output, func(w2 io.Writer) error { return w.WriteTo(psxdisc.NewISOWriter(w2)) }); err != nil {
			return err
		}
		fmt.Fprintf(out, "wrote %s: %d blocks\n", o.Output, w.TotalBlocks())
		return nil
	}

	bin, cue := names(o.Output)
	var layout psxdisc.Layout
	err = writeFile(bin, func(w2 io.Writer) error {
		img := psxdisc.Image{Data: w, Audio: tracks}
		var err error
		layout, err = img.Write(w2)
		return err
	})
	if err != nil {
		return err
	}
	if err := writeFile(cue, func(w2 io.Writer) error {
		return psxdisc.WriteCueSheet(w2, filepath.Base(bin), layout)
	}); err != nil {
		return err
	}

	fmt.Fprintf(out, "wrote %s: %d blocks", bin, w.TotalBlocks())
	if o.Padding > 0 {
		fmt.Fprintf(out, ", %d of them run-out", o.Padding)
	}
	if n := len(tracks); n > 0 {
		fmt.Fprintf(out, ", and %d audio track(s), %d sectors in all", n, layout.Sectors())
	}
	fmt.Fprintf(out, "; %s beside it\n", filepath.Base(cue))
	return nil
}

// names returns the .bin and the .cue for an output, which may name either.
func names(output string) (bin, cue string) {
	ext := filepath.Ext(output)
	stem := strings.TrimSuffix(output, ext)
	if strings.EqualFold(ext, ".cue") {
		return stem + ".bin", output
	}
	return output, stem + ".cue"
}

// writeFile writes name through fill, buffered, and removes it if fill
// fails, so that a disc that could not be finished is not left looking
// like one that was.
func writeFile(name string, fill func(io.Writer) error) (err error) {
	f, err := os.Create(name)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			f.Close()
			os.Remove(name)
		}
	}()
	bw := bufio.NewWriterSize(f, 1<<20)
	if err = fill(bw); err != nil {
		return fmt.Errorf("writing %s: %w", name, err)
	}
	if err = bw.Flush(); err != nil {
		return err
	}
	return f.Close()
}

// split breaks a path on the disc into its directories and its file name,
// taking either separator, a leading one, and a ";1" on the name, which is
// how a program writes the paths it opens. An empty component is not a
// path.
func split(name string) (dirs []string, base string, err error) {
	s := strings.ReplaceAll(name, `\`, "/")
	s = strings.TrimPrefix(s, "/")
	parts := strings.Split(s, "/")
	base = parts[len(parts)-1]
	if strings.HasSuffix(strings.ToUpper(base), ";1") {
		base = base[:len(base)-2]
	}
	if base == "" || slices.Contains(parts, "") {
		return nil, "", fmt.Errorf("%q is not a path on the disc", name)
	}
	return parts[:len(parts)-1], base, nil
}

// discPath is how a path on the disc is written in messages and in
// SYSTEM.CNF: from the root, with backslashes, in the form the directory
// records hold.
func discPath(dirs []string, id string) string {
	var b strings.Builder
	for _, d := range dirs {
		b.WriteByte('\\')
		b.WriteString(iso9660.NormalizeName(d))
	}
	b.WriteByte('\\')
	b.WriteString(id)
	return b.String()
}

// place puts the host file at name on the disc, making the directories on
// the way, and returns its entry. A name already on the disc is an error:
// two files cannot be at one path, and choosing one would be choosing
// wrong half the time.
func place(root *iso9660.Dir, name, host string) (*iso9660.Entry, error) {
	dirs, base, err := split(name)
	if err != nil {
		return nil, err
	}
	d := root
	for _, dir := range dirs {
		d = d.Mkdir(dir)
	}
	if d.Find(base) != nil {
		return nil, fmt.Errorf("%s is already on the disc", discPath(dirs, iso9660.FileIdentifier(base)))
	}
	e, err := d.AddFile(base, host)
	if err != nil {
		return nil, err
	}
	return e, nil
}

// lookup finds the file at name on the disc, or nil, with the directory it
// is in and the directories on the way.
func lookup(root *iso9660.Dir, name string) (*iso9660.Entry, []string, error) {
	dirs, base, err := split(name)
	if err != nil {
		return nil, nil, err
	}
	d := root
	for _, dir := range dirs {
		id := iso9660.NormalizeName(dir)
		i := slices.IndexFunc(d.Dirs, func(sub *iso9660.Dir) bool { return sub.Name == id })
		if i < 0 {
			return nil, dirs, nil
		}
		d = d.Dirs[i]
	}
	return d.Find(base), dirs, nil
}

// markRaw marks every file whose name on the disc matches one of the
// patterns as a stream of Mode 2 sectors.
func markRaw(root *iso9660.Dir, patterns []string) error {
	for _, p := range patterns {
		// A pattern that cannot match anything is a mistake, not an
		// empty set.
		if _, err := path.Match(p, ""); err != nil {
			return fmt.Errorf("--raw %q: %w", p, err)
		}
	}
	var walk func(d *iso9660.Dir)
	walk = func(d *iso9660.Dir) {
		for _, e := range d.Entries {
			name := strings.ToLower(strings.TrimSuffix(e.Name, ";1"))
			for _, p := range patterns {
				if ok, _ := path.Match(strings.ToLower(p), name); ok {
					e.Raw = true
					break
				}
			}
		}
		for _, sub := range d.Dirs {
			walk(sub)
		}
	}
	walk(root)
	return nil
}

// systemCNF is the file the BIOS reads to find the executable, in the form
// Sony's documentation gives: the path, the task and event counts the
// kernel is set up with, and the stack the BIOS runs on, see DefaultStack.
func systemCNF(bootPath string, stack uint32) []byte {
	return fmt.Appendf(nil, "BOOT = cdrom:%s\r\nTCB = 4\r\nEVENT = 10\r\nSTACK = %08X\r\n", bootPath, stack)
}

// A program is the executable that boots the disc and what it registered:
// the records package disc's Register and Track left in it, with the file
// or the track placed for each, which patch writes back once the disc is
// laid out.
type program struct {
	entry *iso9660.Entry // the executable on the disc
	path  string         // its path, as SYSTEM.CNF names it
	image []byte         // the whole executable, which the records are patched in
	text  []byte         // the loadable part of image, where the records are

	placed []placement
	tracks []*psxdisc.AudioTrack // the tracks it declared, in order
}

// A placement is what one record was given: a file on the disc, or a track
// by its number and length.
type placement struct {
	rec     discrecord.Record
	entry   *iso9660.Entry // the file, or nil for a track
	track   uint32         // the track's number, as CdPlay takes it
	sectors uint32         // the track's length
}

// loadProgram reads the executable at boot on the disc and places what it
// registered: each file from src at its path, and each track after the
// given ones, which are numbered from 2.
func loadProgram(root *iso9660.Dir, boot, src string, given int) (*program, error) {
	e, dirs, err := lookup(root, boot)
	if err != nil {
		return nil, err
	}
	if e == nil {
		return nil, fmt.Errorf("%s is not on the disc, so there is nothing to boot", boot)
	}
	rc, err := e.Open()
	if err != nil {
		return nil, err
	}
	image, err := io.ReadAll(rc)
	rc.Close()
	if err != nil {
		return nil, err
	}
	hdr, err := psx.ParseFile(bytes.NewReader(image))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", boot, err)
	}
	p := &program{
		entry: e,
		path:  discPath(dirs, e.Name),
		image: image,
	}
	if len(image) > psx.HeaderSize {
		p.text = image[psx.HeaderSize:]
	}
	// The executable on the disc is the patched one, so its contents come
	// from memory from here on.
	e.Open = func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(p.image)), nil
	}

	// A path registered twice from the same host file is one file, as two
	// packages declaring the same asset would have it; from two it is a
	// contradiction.
	files := map[string]*placement{}
	trackOf := map[string]int{}
	for _, rec := range discrecord.Find(p.text, hdr.TextAddr) {
		host := filepath.Join(src, filepath.FromSlash(rec.Src))
		if rec.Name == disc.TrackName {
			i, ok := trackOf[rec.Src]
			if !ok {
				t, err := psxdisc.AudioTrackFile(host)
				if err != nil {
					return nil, err
				}
				i = len(p.tracks)
				trackOf[rec.Src] = i
				p.tracks = append(p.tracks, t)
			}
			t := p.tracks[i]
			p.placed = append(p.placed, placement{
				rec:     rec,
				track:   uint32(2 + given + i),
				sectors: uint32((t.Size + psxdisc.AudioSectorSize - 1) / psxdisc.AudioSectorSize),
			})
			continue
		}

		dirs, base, err := split(rec.Name)
		if err != nil {
			return nil, err
		}
		key := discPath(dirs, iso9660.FileIdentifier(base))
		var entry *iso9660.Entry
		if prev, ok := files[key]; ok {
			if prev.rec.Src != rec.Src {
				return nil, fmt.Errorf("%s is registered from both %s and %s", key, prev.rec.Src, rec.Src)
			}
			entry = prev.entry
		} else {
			if entry, err = place(root, rec.Name, host); err != nil {
				return nil, err
			}
		}
		if !rec.Fits(entry.Size) {
			return nil, fmt.Errorf("%s is %d bytes, %d sectors, but the buffer it is read into holds %d",
				rec.Src, entry.Size, (entry.Size+iso9660.LogicalBlockSize-1)/iso9660.LogicalBlockSize, rec.BufferSize)
		}
		pl := placement{rec: rec, entry: entry}
		files[key] = &pl
		p.placed = append(p.placed, pl)
	}
	return p, nil
}

// patch writes into the executable where each registered file was placed,
// and which track each registered track became.
func (p *program) patch() {
	for _, pl := range p.placed {
		if pl.entry != nil {
			pl.rec.Patch(p.text, pl.entry.LBA, pl.entry.Size)
		} else {
			pl.rec.Patch(p.text, pl.track, pl.sectors)
		}
	}
}

// list writes what is on the disc and where: every file and directory, the
// registered files with what registered them, and the audio tracks.
func list(w io.Writer, root *iso9660.Dir, prog *program, tracks []*psxdisc.AudioTrack) {
	registered := map[*iso9660.Entry]string{}
	if prog != nil {
		for _, pl := range prog.placed {
			if pl.entry != nil {
				registered[pl.entry] = pl.rec.Src
			}
		}
	}
	var walk func(d *iso9660.Dir, prefix string)
	walk = func(d *iso9660.Dir, prefix string) {
		for _, e := range d.Entries {
			note := ""
			if e.Raw {
				note = "  raw sectors"
			}
			if src, ok := registered[e]; ok {
				note += "  registered from " + src
			}
			fmt.Fprintf(w, "  %-40s %8d bytes  LBA %d%s\n", prefix+"\\"+e.Name, e.Size, e.LBA, note)
		}
		for _, sub := range d.Dirs {
			fmt.Fprintf(w, "  %-40s %8s        LBA %d\n", prefix+"\\"+sub.Name, "<dir>", sub.LBA)
			walk(sub, prefix+"\\"+sub.Name)
		}
	}
	walk(root, "")
	for i, t := range tracks {
		fmt.Fprintf(w, "  track %-34d %8d bytes  %s\n", i+2, t.Size, t.Name)
	}
}
