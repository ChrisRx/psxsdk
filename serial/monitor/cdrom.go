package main

import (
	"unsafe"

	"go.chrisrx.dev/psx/yaroze"
)

// The disc, through the resident library's CD functions rather than the
// kernel's cdrom: device, which stops working once the library's CD driver
// has the drive, as it does from the disc's startup on: a read through it
// waits for an event the library's driver never lets through.

// isCD reports whether name is on the disc.
func isCD(name []byte) bool { return len(name) >= 6 && is(name[:6], "cdrom:") }

// cdPath makes the absolute path on the disc a cdrom: name is, as the
// library takes it, "\DIR\NAME;1", in dst as a C string: a name with no
// directory is in cwd, and one with no version is version 1, as the
// kernel's cdrom: device has it.
func cdPath(dst []byte, name []byte) *byte {
	name = name[6:]
	n := 0
	if len(name) == 0 || name[0] != '\\' {
		n += copy(dst[n:], dirOf(cwd[:cwdLen]))
	}
	n += copy(dst[n:len(dst)-3], name)
	if indexByte(name, ';') < 0 {
		n += copy(dst[n:], ";1")
	}
	dst[n] = 0
	return &dst[0]
}

var cdFile yaroze.CdlFILE

// cdRead reads a file off the disc into RAM at addr, and returns its size,
// or -1 if there is no such file.
func cdRead(name []byte, addr uint32) int32 {
	p := cdPath(path[:], name)
	if yaroze.CdSearchFile(&cdFile, p) == nil {
		return -1
	}
	if yaroze.CdReadFile(p, (*uint32)(yaroze.Pointer(uintptr(addr))), 0) == 0 {
		return -1
	}
	yaroze.CdReadSync(0, nil)
	return int32(cdFile.Size)
}

// cdLoad loads a PS-X EXE off the disc into RAM, as the header says, and
// returns its header, or nil if there is no such file.
func cdLoad(name []byte) *yaroze.EXEC {
	p := cdPath(path[:], name)
	if yaroze.CdSearchFile(&cdFile, p) == nil {
		return nil
	}
	e := yaroze.CdReadExec(p)
	if e == nil {
		return nil
	}
	yaroze.CdReadSync(0, nil)
	return e
}

// sector is where the disc's sectors are read, for its directories.
var sector [2048 / 4]uint32

// CdlSetloc is the drive's command to say where the next read starts.
const cdlSetloc = 0x02

// readSector reads the disc's sector lba into sector.
func readSector(lba int32) []byte {
	var loc yaroze.CdlLOC
	yaroze.CdIntToPos(lba, &loc)
	yaroze.CdControl(cdlSetloc, (*uint8)(unsafe.Pointer(&loc)), nil)
	yaroze.CdRead(1, &sector[0], 0x80)
	yaroze.CdReadSync(0, nil)
	return unsafe.Slice((*byte)(unsafe.Pointer(&sector[0])), 2048)
}

func le32(b []byte) uint32 {
	return uint32(b[0]) | uint32(b[1])<<8 | uint32(b[2])<<16 | uint32(b[3])<<24
}

// cdDir finds the directory dir, a path from the root ending in a
// backslash, and returns its first sector and size, or false.
func cdDir(dir []byte) (uint32, uint32, bool) {
	pvd := readSector(16)
	lba, size := le32(pvd[156+2:]), le32(pvd[156+10:])
	for len(dir) > 0 {
		if dir[0] == '\\' {
			dir = dir[1:]
			continue
		}
		end := indexByte(dir, '\\')
		if end < 0 {
			end = len(dir)
		}
		want := dir[:end]
		dir = dir[end:]
		found := false
		walkDir(lba, size, func(rec []byte) bool {
			id := rec[33 : 33+rec[32]]
			if rec[25]&2 != 0 && equalFold(id, want) {
				lba, size, found = le32(rec[2:]), le32(rec[10:]), true
				return false
			}
			return true
		})
		if !found {
			return 0, 0, false
		}
	}
	return lba, size, true
}

// equalFold reports whether a and b are the same, whatever their case.
func equalFold(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		x, y := a[i], b[i]
		if 'a' <= x && x <= 'z' {
			x -= 'a' - 'A'
		}
		if 'a' <= y && y <= 'z' {
			y -= 'a' - 'A'
		}
		if x != y {
			return false
		}
	}
	return true
}

// walkDir calls f with each record of the directory at lba, size bytes
// long, until f returns false.
func walkDir(lba, size uint32, f func(rec []byte) bool) {
	for s := uint32(0); s < (size+2047)/2048; s++ {
		buf := readSector(int32(lba + s))
		for off := 0; off < 2048 && buf[off] != 0; off += int(buf[off]) {
			if !f(buf[off : off+int(buf[off])]) {
				return
			}
		}
	}
}

// cdList lists a directory on the disc, a path from the root, as dir does:
// each file and directory with its size and first sector, the parent
// directory first with no name. It returns how many it listed.
func cdList(dir []byte) int {
	lba, size, ok := cdDir(dir)
	if !ok {
		return 0
	}
	count := 0
	walkDir(lba, size, func(rec []byte) bool {
		id := rec[33 : 33+rec[32]]
		if len(id) == 1 && id[0] == 0 {
			return true // the directory itself
		}
		if len(id) == 1 && id[0] == 1 {
			id = nil // its parent
		}
		count++
		listEntry(count, id, le32(rec[10:]), le32(rec[2:]))
		return true
	})
	return count
}
