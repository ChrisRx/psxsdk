//go:build yaroze

package cleanlib

import "unsafe"

// CdSearchFile and the tables it keeps: the disc's directories, from the
// ISO 9660 path table, read once, and the files of the directory last
// searched, from the first sector of its listing (probe
// spec/probes/cdsearch).

// CdlFILE is a file as CdSearchFile finds it: its first sector, its length
// and its name.
type CdlFILE struct {
	Pos  CdlLOC
	Size uint32
	Name [16]byte
}

// cdDir is a directory of the path table: its first sector, its parent's
// number (1 for the root, whose parent it is) and its name, NUL-ended,
// the root's empty.
type cdDir struct {
	sector uint32
	parent uint16
	name   [32]byte
}

// cdMaxDirs and cdMaxFiles are the sizes of the tables. Sony's library
// finds the first 128 entries of the path table and not the 129th (probe
// spec/probes/cdlimits), and every record of a listing's first sector, 59
// of them in one, as many as a sector can show with the names ISO 9660
// allows (probe spec/probes/cdmore); so its file table holds at least 59
// entries. No sector can fill more than 60, or run a name's copy past the
// 61st, so the clean table is never filled, and a copy stops at its end
// all the same.
const (
	cdMaxDirs  = 128
	cdMaxFiles = 64
)

// cdFilesT is CdSearchFile's state.
type cdFilesT struct {
	// loaded is set once the path table has been read.
	loaded bool
	dirs   [cdMaxDirs]cdDir
	ndirs  int
	// rec is a path table record, which may cross into the next sector.
	rec [8 + 32]byte

	// dir is the sector of the listing in table, 0 for none, and n how
	// many of table's entries it filled. The rest keep what earlier
	// listings left, and so do the bytes of a name past its NUL. The
	// table is last: a name copied past its entry runs over the entries
	// after it, as in Sony's library.
	dir   uint32
	n     int
	table [cdMaxFiles]CdlFILE
}

// cdNameCompare is how many bytes of the last path part CdSearchFile
// compares with an entry's name, as C's strncmp does: a part of that many
// bytes or more finds any name with the same first bytes, a shorter one
// must be the whole name, version included (probe spec/probes/cdlimits,
// "case compare").
const cdNameCompare = 12

var cdFiles cdFilesT

// CdSearchFile finds the file at name, an absolute path such as
// "\DATA\FILE.DAT;1", copies its entry to fp and returns the library's own
// entry for it, which holds the same. It returns nil if there is no such
// file, and -1 as a pointer if the disc could not be read. Directory parts
// are compared exactly, case and version included, and the last part by
// its first cdNameCompare bytes (see there). Directories are found
// through the path table, wherever they are; a file only in the first
// sector of its directory's listing, whose records the table holds in
// order, "." and ".." (as the names of the first two) included. A part of
// any length is searched as any other; Sony's library breaks on one of
// 32 bytes or more.
//
//export CdSearchFile
func CdSearchFile(fp *CdlFILE, name *byte) *CdlFILE {
	f := &cdFiles
	if !f.loaded {
		if !cd.loadPathTable() {
			return (*CdlFILE)(unsafe.Pointer(^uintptr(0)))
		}
		f.loaded = true
	}
	p := unsafe.Pointer(name)
	at := func(i int) byte { return *(*byte)(unsafe.Add(p, i)) }
	if at(0) != '\\' {
		return nil
	}
	i, dir := 1, 1
	for {
		// The next part runs to a separator or the end; a part a
		// separator ends is a directory.
		n := 0
		for at(i+n) != 0 && at(i+n) != '\\' {
			n++
		}
		part := unsafe.Slice((*byte)(unsafe.Add(p, i)), n)
		if at(i+n) == 0 {
			return f.find(dir, part, fp)
		}
		if dir = f.child(dir, part, true); dir == 0 {
			return nil
		}
		i += n + 1
	}
}

// search finds the file at path, an absolute path such as "\\DIR\\FILE;1",
// through the tables, as Sony's CdReadFile and CdReadExec do (spec 14):
// the path table, read once, gives the directories, and the listing of
// the file's directory is read only when it is not the one last searched,
// so a file looked up again costs no read. Each part up to a backslash names a
// directory under the one before, the last the file, compared ignoring
// case, the file's version optional; a path with no part at all, the
// empty string, names nothing. It returns the file's first sector and
// length.
func (f *cdFilesT) search(path *byte) (sector, size uint32, ok bool) {
	if !f.loaded {
		if !cd.loadPathTable() {
			return 0, 0, false
		}
		f.loaded = true
	}
	p := unsafe.Pointer(path)
	dir := 1
	for {
		for *(*byte)(p) == '\\' {
			p = unsafe.Add(p, 1)
		}
		n := 0
		for ch := *(*byte)(p); ch != 0 && ch != '\\'; ch = *(*byte)(unsafe.Add(p, n)) {
			n++
		}
		part := unsafe.Slice((*byte)(p), n)
		if *(*byte)(unsafe.Add(p, n)) == 0 {
			if n == 0 {
				return 0, 0, false
			}
			return f.lookup(dir, part)
		}
		if dir = f.child(dir, part, false); dir == 0 {
			return 0, 0, false
		}
		p = unsafe.Add(p, n)
	}
}

// child returns the number of the directory named name in the directory
// numbered parent, or 0: the name matched exactly, or, with exact false,
// ignoring case.
func (f *cdFilesT) child(parent int, name []byte, exact bool) int {
	for k := range f.ndirs {
		d := &f.dirs[k]
		if int(d.parent) == parent && cString(d.name[:], name, exact) {
			return k + 1
		}
	}
	return 0
}

// cString reports whether the NUL-ended name in b is s, exactly or, with
// exact false, ignoring the case of letters.
func cString(b, s []byte, exact bool) bool {
	if len(s) >= len(b) || b[len(s)] != 0 {
		return false
	}
	for i := range s {
		if b[i] != s[i] && (exact || upper(b[i]) != upper(s[i])) {
			return false
		}
	}
	return true
}

// cName returns the NUL-ended name in b without its NUL.
func cName(b []byte) []byte {
	if i := indexByte(b, 0); i >= 0 {
		return b[:i]
	}
	return b
}

// find looks for the file named name in the directory numbered dir,
// reading its listing into the table unless it is there already.
func (f *cdFilesT) find(dir int, name []byte, fp *CdlFILE) *CdlFILE {
	if len(name) == 0 {
		return nil
	}
	if !f.listing(dir) {
		return (*CdlFILE)(unsafe.Pointer(^uintptr(0)))
	}
	for k := range f.n {
		e := &f.table[k]
		if cNamePrefix(e.Name[:], name) {
			*fp = *e
			return e
		}
	}
	return nil
}

// cNamePrefix reports whether the entry name in b, NUL-ended, matches the
// path part s as C's strncmp with cdNameCompare does: byte for byte, case
// included, stopping at a NUL on either side (s ends in one), and no byte
// past the cdNameCompare-th looked at.
func cNamePrefix(b, s []byte) bool {
	for i := range cdNameCompare {
		var c byte
		if i < len(s) {
			c = s[i]
		}
		if b[i] != c {
			return false
		}
		if c == 0 {
			break
		}
	}
	return true
}

// lookup finds the file named name in the directory numbered dir for
// search, ignoring case and with the version optional, and returns
// its first sector and length.
func (f *cdFilesT) lookup(dir int, name []byte) (sector, size uint32, ok bool) {
	if !f.listing(dir) {
		return 0, 0, false
	}
	for k := range f.n {
		e := &f.table[k]
		if sameName(cName(e.Name[:]), name) {
			return uint32(CdPosToInt(&e.Pos)), e.Size, true
		}
	}
	return 0, 0, false
}

// listing reads the listing of the directory numbered dir into the table,
// unless it is the one there already.
func (f *cdFilesT) listing(dir int) bool {
	sector := f.dirs[dir-1].sector
	if f.dir != sector {
		if !cd.readDir(sector) {
			return false
		}
		f.fill(sector)
	}
	return true
}

// dots holds the names of a listing's first two records.
var dots = [...]byte{'.', '.'}

// fill puts the records of the listing in cd.dir, read from sector, into
// the table: each one's first sector, length and name. The name is copied
// whole as a C string, its NUL included, over what the entry held: a name
// of 16 bytes or more runs on into the entries after it, which the records
// after overwrite in turn, so that the last record's long name leaves its
// tail in the entry after it (probe spec/probes/cdlimits, "case names" and
// "case long"). The copy stops at the table's end.
func (f *cdFilesT) fill(sector uint32) {
	b := unsafe.Slice((*byte)(unsafe.Pointer(&cd.dir[0])), sectorSize)
	all := unsafe.Slice((*byte)(unsafe.Pointer(&f.table[0])), unsafe.Sizeof(f.table))
	f.dir, f.n = sector, 0
	for i := 0; i+33 <= len(b) && b[i] != 0 && f.n < len(f.table); i += int(b[i]) {
		rec := b[i : i+int(b[i])]
		if len(rec) < 33 || 33+int(rec[32]) > len(rec) {
			break
		}
		e := &f.table[f.n]
		f.n++
		CdIntToPos(int32(le32(rec[2:])), &e.Pos)
		e.Size = le32(rec[10:])
		name := rec[33 : 33+int(rec[32])]
		switch {
		case len(name) == 1 && name[0] == 0:
			name = dots[:1]
		case len(name) == 1 && name[0] == 1:
			name = dots[:2]
		}
		dst := all[uintptr(unsafe.Pointer(&e.Name[0]))-uintptr(unsafe.Pointer(&f.table[0])):]
		n := copy(dst, name)
		if n < len(dst) {
			dst[n] = 0
		}
	}
}

// loadPathTable reads the disc's path table, found through the primary
// volume descriptor, into the directory table.
func (c *cdDriver) loadPathTable() bool {
	if !c.readDir(16) {
		return false
	}
	pvd := unsafe.Slice((*byte)(unsafe.Pointer(&c.dir[0])), sectorSize)
	size, sector := le32(pvd[132:]), le32(pvd[140:])
	f := &cdFiles
	f.ndirs = 0
	rec := &f.rec
	have, off := 0, uint32(0)
	for s := uint32(0); s*sectorSize < size; s++ {
		if !c.readDir(sector + s) {
			return false
		}
		b := unsafe.Slice((*byte)(unsafe.Pointer(&c.dir[0])), sectorSize)
		for i := range b {
			if off >= size {
				break
			}
			off++
			rec[have] = b[i]
			have++
			n := 8 + int(rec[0]) + int(rec[0]&1)
			if have < 8 || have < n && have < len(rec) {
				continue
			}
			if f.ndirs < len(f.dirs) {
				d := &f.dirs[f.ndirs]
				f.ndirs++
				d.sector = le32(rec[2:])
				d.parent = uint16(rec[6]) | uint16(rec[7])<<8
				l := min(int(rec[0]), len(d.name)-1)
				copy(d.name[:l], rec[8:8+l])
				d.name[l] = 0
			}
			have = 0
		}
	}
	return true
}
