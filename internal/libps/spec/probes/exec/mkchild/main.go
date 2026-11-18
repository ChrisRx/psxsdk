// Command mkchild writes the small PSX-EXE files the exec probe loads and
// runs, into the directory given (the probe's files/). Each one stores a
// GP1 marker word, records its registers at offset 100h of its text, bumps
// a counter there, samples its first bss word, writes its second, and
// returns 1234h. The headers differ: a stack of its own, an entry past the
// start, several sectors, a size that is not whole sectors, and a bad
// magic.
package main

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
)

const (
	zero = 0
	v0   = 2
	a0   = 4
	a1   = 5
	a2   = 6
	t0   = 8
	t1   = 9
	t2   = 10
	gp   = 28
	sp   = 29
	fp   = 30
	ra   = 31
)

func lui(rt, imm uint32) uint32       { return 0x3C000000 | rt<<16 | imm&0xffff }
func ori(rt, rs, imm uint32) uint32   { return 0x34000000 | rs<<21 | rt<<16 | imm&0xffff }
func addiu(rt, rs, imm uint32) uint32 { return 0x24000000 | rs<<21 | rt<<16 | imm&0xffff }
func sw(rt, base, off uint32) uint32  { return 0xAC000000 | base<<21 | rt<<16 | off&0xffff }
func lw(rt, base, off uint32) uint32  { return 0x8C000000 | base<<21 | rt<<16 | off&0xffff }
func la(rt, addr uint32) []uint32     { return []uint32{lui(rt, addr>>16), ori(rt, rt, addr)} }
func jr(rs uint32) uint32             { return 0x00000008 | rs<<21 }

type child struct {
	name  string
	id    uint32 // the marker's low byte
	taddr uint32
	tsize uint32 // bytes after the header
	entry uint32 // offset of the code from taddr
	gp0   uint32
	baddr uint32
	bsize uint32
	saddr uint32
	ssize uint32
	magic string
	junk  bool // trash sp, gp and fp before returning
}

// code is the child's routine; rec is where it records, bss its first
// bss word (0 for none).
func code(c child, rec, bss uint32) []uint32 {
	var w []uint32
	w = append(w, lui(t0, 0x1f80), lui(t1, 0x1000), ori(t1, t1, 0x180|c.id), sw(t1, t0, 0x1814))
	w = append(w, la(t2, rec)...)
	w = append(w, sw(sp, t2, 0), sw(gp, t2, 4), sw(ra, t2, 8), sw(a0, t2, 12), sw(a1, t2, 16), sw(fp, t2, 20), sw(a2, t2, 24))
	w = append(w, lw(t1, t2, 28), 0, addiu(t1, t1, 1), sw(t1, t2, 28))
	if bss != 0 {
		w = append(w, la(t0, bss)...)
		w = append(w, lw(t1, t0, 0), 0, sw(t1, t2, 32), ori(t1, zero, 0xbb55), sw(t1, t0, 4))
	}
	if c.junk {
		w = append(w, lui(sp, 0x1234), lui(gp, 0x5678), lui(fp, 0x9abc))
	}
	w = append(w, ori(v0, zero, 0x1234), jr(ra), 0)
	return w
}

func main() {
	dir := os.Args[1]
	children := []child{
		{name: "child.exe", id: 0, taddr: 0x80100000, tsize: 0x800, gp0: 0x80108000, baddr: 0x80101000, bsize: 0x400, magic: "PS-X EXE"},
		{name: "stack.exe", id: 1, taddr: 0x80110000, tsize: 0x800, gp0: 0x80118000, baddr: 0x80111000, bsize: 0x400, saddr: 0x80190000, ssize: 0x4000, magic: "PS-X EXE"},
		{name: "junk.exe", id: 2, taddr: 0x80120000, tsize: 0x800, gp0: 0x80128000, baddr: 0x80121000, bsize: 0x400, saddr: 0x801a0000, ssize: 0x100, magic: "PS-X EXE", junk: true},
		{name: "long.exe", id: 3, taddr: 0x80130000, tsize: 0x2800, gp0: 0, baddr: 0x80133000, bsize: 0x1000, magic: "PS-X EXE"},
		{name: "entry.exe", id: 4, taddr: 0x80140000, tsize: 0x800, entry: 0x200, gp0: 0x80148000, baddr: 0x80141000, bsize: 0x400, magic: "PS-X EXE"},
		{name: "short.exe", id: 5, taddr: 0x80150000, tsize: 0x900, gp0: 0x80158000, baddr: 0x80151000, bsize: 0x400, magic: "PS-X EXE"},
		{name: "nobss.exe", id: 6, taddr: 0x80160000, tsize: 0x800, gp0: 0x80168000, baddr: 0, bsize: 0, magic: "PS-X EXE"},
		{name: "bad.bin", id: 7, taddr: 0x80170000, tsize: 0x800, gp0: 0x80178000, baddr: 0x80171000, bsize: 0x400, magic: "NOT-AEXE"},
	}
	for _, c := range children {
		b := make([]byte, 0x800+c.tsize)
		copy(b, c.magic)
		put := func(off, v uint32) { binary.LittleEndian.PutUint32(b[off:], v) }
		put(0x10, c.taddr+c.entry)
		put(0x14, c.gp0)
		put(0x18, c.taddr)
		put(0x1c, c.tsize)
		put(0x28, c.baddr)
		put(0x2c, c.bsize)
		put(0x30, c.saddr)
		put(0x34, c.ssize)
		// The text past the code is a pattern, so a hash shows how much
		// of it was loaded.
		for i := uint32(0); i < c.tsize; i += 4 {
			put(0x800+i, 0xc0de0000|(c.id<<12)|i>>2)
		}
		for i, w := range code(c, c.taddr+0x100, c.baddr) {
			put(0x800+c.entry+uint32(i)*4, w)
		}
		if err := os.WriteFile(filepath.Join(dir, c.name), b, 0o666); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
}
