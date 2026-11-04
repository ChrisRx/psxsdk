//go:build yaroze

package libps

import "unsafe"

// Files by name. The disc's path table is read once, into a table of its
// directories, and a directory's records are read when a file in it is
// looked for, into a table of its files; CdSearchFile walks a path
// through the one and finds the name in the other, and CdReadFile reads
// what it found.

// A cdDir is a directory from the path table: its number, its parent's,
// its first sector and its name.
//
// libps 0x80060CF8, 128 of them
type cdDir struct {
	id     int32
	parent int32
	lba    uint32
	name   [32]byte
}

var (
	cdFiles [64]CdlFILE // 0x800606F8: the files of the cached directory, ended by an empty name
	cdDirs  [128]cdDir  // 0x80060CF8: the directories, ended by a parent of 0
	cdBuf   [512]uint32 // 0x800622F8: the sector read
	cdFile  CdlFILE     // 0x80062AF8: the file CdReadFile found

	cdDirCached int32 = 0  // 0x8004D750: the directory whose files are cached
	cdMedia     int32 = -1 // 0x8004D754: the shell count the path table was read at

	// A component of the path and the name as CdReadFile completes it,
	// not on the stack since their addresses are handed on.
	cdPart [64]byte
	cdPath [96]byte
	cdLoc  CdlLOC
)

// cdBytes is the sector read as bytes.
func cdBytes() []byte { return unsafe.Slice((*byte)(unsafe.Pointer(&cdBuf[0])), 2048) }

// CdSearchFile finds the file at name, "\DIR\FILE.EXT;1", and copies its
// position, size and name to fp, returning the library's own record of
// it, or nil if there is no such file. The directories are read from the
// disc again after the shell has been opened.
//
// libps 0x8003B198, 736 bytes
//
//export CdSearchFile
func CdSearchFile(fp *CdlFILE, name *byte) *CdlFILE {
	cdStart()
	if cdMedia != cdOpens {
		if !cdNewMedia() {
			return nil
		}
		cdMedia = cdOpens
	}
	if *name != '\\' {
		return nil
	}
	// Each component up to a backslash names a directory under the one
	// before, the root being what the empty name before the first
	// backslash finds; the last names the file.
	p := name
	dir := int32(1)
	level := 0
	for level < 8 {
		n := 0
		for *p != 0 && *p != '\\' && n < len(cdPart)-1 {
			cdPart[n] = *p
			n++
			p = (*byte)(unsafe.Add(unsafe.Pointer(p), 1))
		}
		cdPart[n] = 0
		if *p == 0 {
			break
		}
		p = (*byte)(unsafe.Add(unsafe.Pointer(p), 1))
		if dir = cdFindDir(dir, &cdPart[0]); dir == -1 {
			cdPart[0] = 0
			break
		}
		level++
	}
	if level >= 8 {
		if cdDebug > 0 {
			kprintf("%s: path level (%d) error\n", cstr(name), int32(level))
		}
		return nil
	}
	if cdPart[0] == 0 {
		if cdDebug > 0 {
			kprintf("%s: dir was not found\n", cstr(name))
		}
		return nil
	}
	if cdCacheFile(dir) == 0 {
		if cdDebug > 0 {
			kprintf("CdSearchFile: disc error\n")
		}
		return nil
	}
	if cdDebug >= 2 {
		kprintf("CdSearchFile: searching %s...\n", cstr(&cdPart[0]))
	}
	for i := range cdFiles {
		if cdFiles[i].Name[0] == 0 {
			break
		}
		if cdStrncmp(&cdFiles[i].Name[0], &cdPart[0], 12) == 0 {
			if cdDebug >= 2 {
				kprintf("%s:  found\n", cstr(&cdPart[0]))
			}
			*fp = cdFiles[i]
			return &cdFiles[i]
		}
	}
	if cdDebug > 0 {
		kprintf("%s: not found\n", cstr(&cdPart[0]))
	}
	return nil
}

// cdFindDir finds the directory called name under the directory numbered
// parent and returns its number, or -1.
//
// libps 0x8003B778, 168 bytes
func cdFindDir(parent int32, name *byte) int32 {
	for i := range cdDirs {
		d := &cdDirs[i]
		if d.parent == 0 {
			break
		}
		if d.parent == parent && cdStrcmp(name, &d.name[0]) == 0 {
			return int32(i) + 1
		}
	}
	return -1
}

// cdNewMedia reads the disc's volume descriptor and its path table into
// the table of directories.
//
// libps 0x8003B49C, 732 bytes
func cdNewMedia() bool {
	if cdReadNow(1, 16, &cdBuf[0]) != 1 {
		if cdDebug > 0 {
			kprintf("CD_newmedia: Read error in cd_read(PVD)\n")
		}
		return false
	}
	b := cdBytes()
	if string(b[1:6]) != "CD001" {
		if cdDebug > 0 {
			kprintf("CD_newmedia: Disc format error in cd_read(PVD)\n")
		}
		return false
	}
	table := int32(b[0x8c]) | int32(b[0x8d])<<8 | int32(b[0x8e])<<16 | int32(b[0x8f])<<24
	if cdReadNow(1, table, &cdBuf[0]) != 1 {
		if cdDebug > 0 {
			kprintf("CD_newmedia: Read error (PT:%08x)\n", uint32(table))
		}
		return false
	}
	if cdDebug >= 2 {
		kprintf("CD_newmedia: sarching dir..\n")
	}
	count := 0
	for i := 0; i < len(b) && b[i] != 0 && count < len(cdDirs); {
		n := int(b[i])
		d := &cdDirs[count]
		d.lba = uint32(b[i+2]) | uint32(b[i+3])<<8 | uint32(b[i+4])<<16 | uint32(b[i+5])<<24
		d.id = int32(count) + 1
		d.parent = int32(b[i+6])
		// The original copied the whole name into its 32 bytes, over the
		// next entry for a longer one; a disc's names are far shorter.
		m := min(n, len(d.name)-1)
		copy(d.name[:m], b[i+8:i+8+m])
		d.name[m] = 0
		if cdDebug >= 2 {
			kprintf("\t%08x,%04x,%04x,%s\n", d.lba, uint32(d.id), uint32(d.parent), cstr(&d.name[0]))
		}
		i += 8 + n + n&1
		count++
	}
	if count < len(cdDirs) {
		cdDirs[count].parent = 0
	}
	cdDirCached = 0
	if cdDebug >= 2 {
		kprintf("CD_newmedia: %d dir entries found\n", int32(count))
	}
	return true
}

// cdCacheFile reads the records of the directory numbered dir into the
// table of files, unless they are there already. It returns 1 when they
// are, and -1 when the directory could not be read.
//
// libps 0x8003B820, 676 bytes
func cdCacheFile(dir int32) int32 {
	if dir == cdDirCached {
		return 1
	}
	lba := cdDirs[dir-1].lba
	if cdReadNow(1, int32(lba), &cdBuf[0]) != 1 {
		if cdDebug > 0 {
			kprintf("CD_cachefile: dir not found\n")
		}
		return -1
	}
	if cdDebug >= 2 {
		kprintf("CD_cachefile: searching...\n")
	}
	b := cdBytes()
	count := 0
	for i := 0; i < len(b) && b[i] != 0 && count < len(cdFiles); {
		f := &cdFiles[count]
		CdIntToPos(int32(uint32(b[i+2])|uint32(b[i+3])<<8|uint32(b[i+4])<<16|uint32(b[i+5])<<24), &f.Pos)
		f.Size = uint32(b[i+10]) | uint32(b[i+11])<<8 | uint32(b[i+12])<<16 | uint32(b[i+13])<<24
		switch count {
		case 0:
			f.Name[0], f.Name[1] = '.', 0
		case 1:
			f.Name[0], f.Name[1], f.Name[2] = '.', '.', 0
		default:
			// The whole identifier and its NUL, as a C string copy: one
			// longer than fifteen bytes runs into the next entry, which the
			// next record then overwrites (spec 09, CdSearchFile; the probe
			// cdlimits shows the leftovers). The copy stops at the table's
			// end.
			table := unsafe.Slice((*byte)(unsafe.Pointer(&cdFiles[0])), unsafe.Sizeof(cdFiles))
			at := count*int(unsafe.Sizeof(cdFiles[0])) + int(unsafe.Offsetof(f.Name))
			m := min(int(b[i+32]), len(table)-1-at)
			copy(table[at:at+m], b[i+33:i+33+m])
			table[at+m] = 0
		}
		if cdDebug >= 2 {
			kprintf("\t(%02x:%02x:%02x) %8d %s\n", uint32(f.Pos.Minute), uint32(f.Pos.Second), uint32(f.Pos.Sector), int32(f.Size), cstr(&f.Name[0]))
		}
		i += int(b[i])
		count++
	}
	cdDirCached = dir
	if count < len(cdFiles) {
		cdFiles[count].Name[0] = 0
	}
	if cdDebug >= 2 {
		kprintf("CD_cachefile: %d files found\n", int32(count))
	}
	return 1
}

// cdReadNow reads sectors from sector lba into buf, waiting for them, and
// returns 1 when they came.
//
// libps 0x8003BAC4, 108 bytes
func cdReadNow(sectors, lba int32, buf *uint32) int32 {
	CdIntToPos(lba, &cdLoc)
	CdControl(cdlSetloc, &cdLoc.Minute, nil)
	CdRead(sectors, buf, 0x80)
	return b2i(CdReadSync(0, nil) == 0)
}

// CdReadFile reads the file at name, "\DIR\FILE.EXT;1", into memory at
// addr, nbyte bytes of it or the whole file for 0, and returns how many
// bytes the read will deliver, or 0 if the file was not found or the read
// could not start. The name is folded to upper case and given its
// version if it lacks one. The read goes on in the background, and
// CdReadSync waits for it. With a nil name the file last found is read.
//
// libps 0x8003C14C, 360 bytes
//
//export CdReadFile
func CdReadFile(name *byte, addr *uint32, nbyte int32) int32 {
	cdStart()
	if name != nil {
		n := cdStrcpy(&cdPath[0], name, len(cdPath)-3)
		for i := 0; i < n; i++ {
			if cdPath[i] >= 'a' && cdPath[i] <= 'z' {
				cdPath[i] -= 'a' - 'A'
			}
		}
		if n < 2 || cdPath[n-2] != ';' {
			cdPath[n], cdPath[n+1], cdPath[n+2] = ';', '1', 0
		}
		if CdSearchFile(&cdFile, &cdPath[0]) == nil {
			return 0
		}
	}
	if nbyte == 0 {
		nbyte = int32(cdFile.Size)
	}
	if CdControl(cdlSetloc, &cdFile.Pos.Minute, nil) == 0 {
		return 0
	}
	sectors := (nbyte + 2047) / 2048
	if CdRead(sectors, addr, 0x80) == 0 {
		return 0
	}
	CdIntToPos(CdPosToInt(&cdFile.Pos)+sectors, &cdFile.Pos)
	cdFile.Size -= uint32(sectors)
	return nbyte
}

// The C strings the file calls take.

// cstr is the string at p, for a message.
func cstr(p *byte) string {
	n := 0
	for *(*byte)(unsafe.Add(unsafe.Pointer(p), n)) != 0 {
		n++
	}
	return unsafe.String(p, n)
}

// cdStrcpy copies the string at src to dst, at most max bytes of it, and
// returns its length.
func cdStrcpy(dst, src *byte, max int) int {
	n := 0
	for n < max {
		c := *(*byte)(unsafe.Add(unsafe.Pointer(src), n))
		*(*byte)(unsafe.Add(unsafe.Pointer(dst), n)) = c
		if c == 0 {
			return n
		}
		n++
	}
	*(*byte)(unsafe.Add(unsafe.Pointer(dst), n)) = 0
	return n
}

// cdStrcmp compares two strings as strcmp does: 0 when they are the same.
func cdStrcmp(a, b *byte) int32 {
	for i := 0; ; i++ {
		x := *(*byte)(unsafe.Add(unsafe.Pointer(a), i))
		y := *(*byte)(unsafe.Add(unsafe.Pointer(b), i))
		if x != y {
			return int32(x) - int32(y)
		}
		if x == 0 {
			return 0
		}
	}
}

// cdStrncmp compares at most n bytes of two strings as strncmp does.
func cdStrncmp(a, b *byte, n int) int32 {
	for i := 0; i < n; i++ {
		x := *(*byte)(unsafe.Add(unsafe.Pointer(a), i))
		y := *(*byte)(unsafe.Add(unsafe.Pointer(b), i))
		if x != y {
			return int32(x) - int32(y)
		}
		if x == 0 {
			return 0
		}
	}
	return 0
}
