package emu

import (
	"bufio"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
)

// sectorSize is a raw CD sector: sync, header, subheader, data and error
// correction.
const sectorSize = 2352

// Sector kinds, as psxe classifies them.
const (
	sectorFar    = iota // past the end of the disc
	sectorData          // in a MODE2/2352 track
	sectorAudio         // in any other track
	sectorPregap        // before the disc's end but in no track
)

// Disc is a CD image as psxe reads one, addressed by LBA counted from the
// start of the disc, so the first track's data starts at 150 (00:02:00).
type Disc struct {
	files  []*discFile
	tracks []*discTrack
}

type discFile struct {
	r     io.ReaderAt
	c     io.Closer
	size  int64
	start uint32
}

type discTrack struct {
	number     int
	data       bool
	index      [2]int32 // INDEX 00 and 01, -1 if absent
	start, end uint32
	file       *discFile
}

// OpenDisc opens a cue sheet and the files it names.
//
// The parser takes the subset of the cue format a PlayStation disc uses:
// FILE, TRACK and INDEX, with paths relative to the cue sheet. The layout
// rules are psxe's. Files follow one another from 00:02:00. A file with a
// single track holds just that track, offset by its pregap when both of
// its indexes are given; a file with several holds them at their INDEX 01
// positions plus 150.
func OpenDisc(path string) (*Disc, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	return readDisc(f, path, func(name string) (*discFile, error) {
		if !filepath.IsAbs(name) {
			name = filepath.Join(filepath.Dir(path), name)
		}
		bin, err := os.Open(name)
		if err != nil {
			return nil, err
		}
		info, err := bin.Stat()
		if err != nil {
			bin.Close()
			return nil, err
		}
		return &discFile{r: bin, c: bin, size: info.Size()}, nil
	})
}

// OpenDiscFS is OpenDisc for a cue sheet in fsys, where the files it names
// are relative to it. Their files must implement io.ReaderAt, as the files
// of os.DirFS and testing/fstest.MapFS do.
func OpenDiscFS(fsys fs.FS, name string) (*Disc, error) {
	f, err := fsys.Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	return readDisc(f, name, func(file string) (*discFile, error) {
		bin, err := fsys.Open(path.Join(path.Dir(name), file))
		if err != nil {
			return nil, err
		}
		r, ok := bin.(io.ReaderAt)
		if !ok {
			bin.Close()
			return nil, fmt.Errorf("%s: %s cannot be read at an offset", name, file)
		}
		info, err := bin.Stat()
		if err != nil {
			bin.Close()
			return nil, err
		}
		return &discFile{r: r, c: bin, size: info.Size()}, nil
	})
}

// readDisc parses a cue sheet, named name for errors, opening the files it
// names with open.
func readDisc(cue io.Reader, name string, open func(file string) (*discFile, error)) (*Disc, error) {
	d := &Disc{}
	var cur *discFile
	var curTracks []*discTrack
	var groups [][]*discTrack

	sc := bufio.NewScanner(cue)
	for line := 1; sc.Scan(); line++ {
		fields := splitCueLine(sc.Text())
		if len(fields) == 0 {
			continue
		}
		bad := func(format string, args ...any) error {
			return fmt.Errorf("%s:%d: %s", name, line, fmt.Sprintf(format, args...))
		}
		switch strings.ToUpper(fields[0]) {
		case "FILE":
			if len(fields) < 2 {
				return nil, bad("FILE without a file name")
			}
			file, err := open(fields[1])
			if err != nil {
				d.Close()
				return nil, err
			}
			if cur != nil {
				groups = append(groups, curTracks)
			}
			cur = file
			curTracks = nil
			d.files = append(d.files, cur)
		case "TRACK":
			if cur == nil {
				return nil, bad("TRACK before FILE")
			}
			if len(fields) < 3 {
				return nil, bad("TRACK needs a number and a mode")
			}
			n, err := strconv.Atoi(fields[1])
			if err != nil {
				return nil, bad("track number %q: %v", fields[1], err)
			}
			t := &discTrack{
				number: n,
				data:   strings.EqualFold(fields[2], "MODE2/2352"),
				index:  [2]int32{-1, -1},
				file:   cur,
			}
			d.tracks = append(d.tracks, t)
			curTracks = append(curTracks, t)
		case "INDEX":
			if len(curTracks) == 0 {
				return nil, bad("INDEX before TRACK")
			}
			if len(fields) < 3 {
				return nil, bad("INDEX needs a number and a position")
			}
			i, err := strconv.Atoi(fields[1])
			if err != nil {
				return nil, bad("index number %q: %v", fields[1], err)
			}
			msf, err := parseMSF(fields[2])
			if err != nil {
				return nil, bad("%v", err)
			}
			if i <= 1 {
				curTracks[len(curTracks)-1].index[i] = int32(msf)
			}
		}
	}
	if err := sc.Err(); err != nil {
		d.Close()
		return nil, err
	}
	if cur != nil {
		groups = append(groups, curTracks)
	}
	if len(d.tracks) == 0 {
		d.Close()
		return nil, fmt.Errorf("%s: no tracks", name)
	}

	lba := uint32(150)
	for i, file := range d.files {
		file.start = lba
		lba = layoutTracks(file, groups[i], lba)
	}
	return d, nil
}

func layoutTracks(file *discFile, tracks []*discTrack, lba uint32) uint32 {
	sectors := uint32(file.size / sectorSize)
	if len(tracks) == 1 {
		t := tracks[0]
		var pregap uint32
		if t.index[0] != -1 && t.index[1] != -1 {
			pregap = uint32(t.index[1])
		}
		t.start = lba + pregap
		t.end = t.start + sectors
		return t.end
	}
	for i, t := range tracks {
		t.start = uint32(t.index[1]) + 150
		if i == len(tracks)-1 {
			// psxe ends the last track at the file's length in sectors,
			// not counting from where the file starts, and leaves the LBA
			// for the next file where it was.
			t.end = sectors
		} else {
			t.end = uint32(tracks[i+1].index[1]) + 150 - 1
		}
	}
	return lba
}

// splitCueLine splits a cue sheet line into words, keeping a quoted file
// name as one.
func splitCueLine(s string) []string {
	var fields []string
	s = strings.TrimSpace(s)
	for s != "" {
		if s[0] == '"' {
			end := strings.IndexByte(s[1:], '"')
			if end < 0 {
				fields = append(fields, s[1:])
				break
			}
			fields = append(fields, s[1:1+end])
			s = strings.TrimSpace(s[2+end:])
			continue
		}
		end := strings.IndexAny(s, " \t")
		if end < 0 {
			fields = append(fields, s)
			break
		}
		fields = append(fields, s[:end])
		s = strings.TrimSpace(s[end:])
	}
	return fields
}

func parseMSF(s string) (uint32, error) {
	parts := strings.Split(s, ":")
	if len(parts) != 3 {
		return 0, fmt.Errorf("position %q is not mm:ss:ff", s)
	}
	var v [3]uint32
	for i, p := range parts {
		n, err := strconv.ParseUint(p, 10, 32)
		if err != nil {
			return 0, fmt.Errorf("position %q: %v", s, err)
		}
		v[i] = uint32(n)
	}
	return v[0]*4500 + v[1]*75 + v[2], nil
}

// Close closes the image's files.
func (d *Disc) Close() error {
	for _, f := range d.files {
		f.c.Close()
	}
	return nil
}

func (d *Disc) track(lba uint32) *discTrack {
	for _, t := range d.tracks {
		if lba >= t.start && lba < t.end {
			return t
		}
	}
	return nil
}

func (d *Disc) query(lba uint32) int {
	if lba >= d.tracks[len(d.tracks)-1].end {
		return sectorFar
	}
	t := d.track(lba)
	switch {
	case t == nil:
		return sectorPregap
	case t.data:
		return sectorData
	}
	return sectorAudio
}

// read reads a raw sector. A pregap sector reads as empty apart from its
// sync pattern.
func (d *Disc) read(lba uint32, buf []byte) int {
	kind := d.query(lba)
	switch kind {
	case sectorFar:
		return kind
	case sectorPregap:
		clear(buf[:sectorSize])
		for i := 1; i <= 10; i++ {
			buf[i] = 0xff
		}
		return kind
	}
	t := d.track(lba)
	// A short read is treated as psxe treats it, by leaving what was there.
	t.file.r.ReadAt(buf[:sectorSize], int64(lba-t.file.start)*sectorSize)
	return kind
}

// trackNumber finds the track an LBA belongs to, counting the gap before a
// track as part of the one before it, and comparing only whole seconds.
func (d *Disc) trackNumber(lba uint32) int {
	for i, t := range d.tracks {
		if i == len(d.tracks)-1 {
			return t.number
		}
		next := d.tracks[i+1]
		cur := int(t.start - t.start%75)
		nxt := int(next.start - next.start%75)
		if int(lba) >= cur && int(lba) < nxt {
			return t.number
		}
	}
	return 0
}

// subqTrack is the track number the disc's subchannel gives for an LBA, as
// the hardware senses track changes: exactly the track the sector is in, a
// gap before a track counting as that track. Past the end, it is 0.
func (d *Disc) subqTrack(lba uint32) int {
	for _, t := range d.tracks {
		if lba < t.end {
			return t.number
		}
	}
	return 0
}

// trackLBA is where a track starts, or for track 0, where the disc ends. A
// track past the last reads as 0, which is sectorFar.
func (d *Disc) trackLBA(track int) uint32 {
	if track == 0 {
		return d.tracks[len(d.tracks)-1].end
	}
	if track > len(d.tracks) {
		return sectorFar
	}
	return d.tracks[track-1].start
}

// Disc types, which decide what GetID reports.
const (
	discNone = iota
	discLicensed
	discAudio
	discUnknown
)

// kind classifies the disc by its primary volume descriptor: one that names
// the system as PLAYSTATION is licensed, one that does not is taken to be
// an audio disc, and a disc too short to have one is unknown.
func (d *Disc) kind() int {
	var buf [sectorSize]byte
	if d.read(150+16, buf[:]) == sectorFar {
		return discUnknown
	}
	if string(buf[0x20:0x2b]) != "PLAYSTATION" {
		return discAudio
	}
	return discLicensed
}
