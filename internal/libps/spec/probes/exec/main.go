//go:build yaroze

// Command exec is a probe for the spec: LoadTest, Load, Exec and
// CdReadExec over the small executables mkchild writes (files/), each
// case between two GP1 marker words (10000100h plus the case number). It
// prints each result, the EXEC words, what each child recorded, and the
// extent and hash of the memory each call wrote.
package main

import (
	"runtime/volatile"
	"unsafe"

	"go.chrisrx.dev/psx/disc"
	"go.chrisrx.dev/psx/yaroze"
)

// The children go on both discs through package disc; printing where each
// landed keeps their records in the executable.
var files = []disc.File{
	disc.Register("CHILD.EXE", "files/child.exe"),
	disc.Register("STACK.EXE", "files/stack.exe"),
	disc.Register("JUNK.EXE", "files/junk.exe"),
	disc.Register("LONG.EXE", "files/long.exe"),
	disc.Register("ENTRY.EXE", "files/entry.exe"),
	disc.Register("SHORT.EXE", "files/short.exe"),
	disc.Register("NOBSS.EXE", "files/nobss.exe"),
	disc.Register("BAD.BIN", "files/bad.bin"),
}

// region is the memory the children occupy, text, bss and stacks.
const (
	regionLo = 0x80100000
	regionHi = 0x801b0000
	fill     = 0x5a5a5a5a
)

func mark(n uint32) {
	volatile.StoreUint32((*uint32)(yaroze.Pointer(0x1f801814)), 0x10000100|n)
}

func cstr(s string) *byte {
	b := append([]byte(s), 0)
	return &b[0]
}

var hex = "0123456789abcdef"

func word(w uint32) {
	var b [8]byte
	for i := range b {
		b[i] = hex[w>>(28-4*i)&15]
	}
	print(string(b[:]))
}

func peek(addr uint32) uint32 {
	return volatile.LoadUint32((*uint32)(yaroze.Pointer(uintptr(addr))))
}

func poke(addr, v uint32) {
	volatile.StoreUint32((*uint32)(yaroze.Pointer(uintptr(addr))), v)
}

func fillRegion() {
	for a := uint32(regionLo); a < regionHi; a += 4 {
		poke(a, fill)
	}
}

// extent prints the first and last words of the region not left at fill
// and a hash of every word between them.
func extent() {
	first, last := uint32(0), uint32(0)
	for a := uint32(regionLo); a < regionHi; a += 4 {
		if peek(a) != fill {
			if first == 0 {
				first = a
			}
			last = a
		}
	}
	print(" wrote ")
	word(first)
	print("..")
	word(last)
	h := uint32(2166136261)
	for a := first; first != 0 && a <= last; a += 4 {
		h = (h ^ peek(a)) * 16777619
	}
	print(" hash ")
	word(h)
}

func words(e *yaroze.EXEC) {
	p := (*[15]uint32)(unsafe.Pointer(e))
	print(" exec")
	for _, w := range p {
		print(" ")
		word(w)
	}
}

func fillExec(e *yaroze.EXEC) {
	p := (*[15]uint32)(unsafe.Pointer(e))
	for i := range p {
		p[i] = fill
	}
}

// after prints the EXEC's fields and what the child at taddr recorded
// (sp, gp, ra, a0, a1, fp, a2, its run count, the bss word it sampled).
// The values that come from the caller's own registers (the saved Sp, Fp,
// Gp, Ret and Base, and the child's sp, fp, a1 and a2) are printed as
// differences or as set or not, since the two builds' stacks and code sit
// differently.
func after(e *yaroze.EXEC, taddr uint32) {
	p := (*[15]uint32)(unsafe.Pointer(e))
	print(" exec")
	for _, w := range p[:10] {
		print(" ")
		word(w)
	}
	r := (*[9]uint32)(yaroze.Pointer(uintptr(taddr + 0x100)))
	print(" saved gp ", e.Gp != 0, " ret ", e.Ret-0x80010000 < 0x100000, " base ", e.Base != 0)
	if e.S_addr != 0 {
		print(" rec sp ")
		word(r[0])
		print(" fp ")
		word(r[5])
	} else {
		print(" rec sp ", int32(r[0]-e.Sp), " fp ", int32(r[5]-e.Fp))
	}
	print(" gp ")
	word(r[1])
	print(" ra ")
	word(r[2])
	print(" a0 ", r[3], " a1 ")
	if r[4] == 0 {
		print("nil")
	} else {
		print(int32(r[4] - uint32(uintptr(unsafe.Pointer(&argv[0])))))
	}
	print(" a2-a1 ", int32(r[6]-r[4]), " count ", r[7], " bss0 ")
	word(r[8])
}

func clearRec(taddr uint32) {
	for i := uint32(0); i < 9; i++ {
		poke(taddr+0x100+4*i, 0)
	}
}

// names for the kernel's loading. On the emulator's BIOS a cdrom: name,
// even of a file that is not there, never comes back, so none is tried.
var names = []string{
	"",
	"CHILD.EXE",
	"\\CHILD.EXE;1",
	"bu00:CHILD",
}

var n uint32

func begin(label string) {
	mark(n)
	print("case ", n, " ", label, ":")
}

func end() {
	mark(n)
	println()
	n++
}

var argv = [...]*byte{cstr("one"), cstr("two"), nil}

func exec(e *yaroze.EXEC, taddr uint32, argc int32, args **byte, critical bool) {
	clearRec(taddr)
	if critical {
		yaroze.EnterCriticalSection()
	}
	r := yaroze.Exec(e, argc, args)
	if critical {
		yaroze.ExitCriticalSection()
	}
	print(" -> ", r)
	after(e, taddr)
	if e.B_size != 0 && e.B_size != fill {
		print(" bss ")
		word(peek(e.B_addr))
		print(" ")
		word(peek(e.B_addr + 4))
		print(" ")
		word(peek(e.B_addr + e.B_size - 4))
		print(" ")
		word(peek(e.B_addr + e.B_size))
	}
}

func main() {
	yaroze.ResetGraph(0)
	yaroze.GsInitGraph(320, 240, yaroze.GsOFSGPU, 0, 0)
	yaroze.GsDefDispBuff(0, 0, 0, 240)
	yaroze.DrawSync(0)

	for i := range files {
		print(files[i].Name, " ", files[i].Sector(), " ", files[i].Size(), "\n")
	}
	var e yaroze.EXEC
	var res [8]uint8

	// read brings a child in with CdReadExec and waits for it, printing
	// what the EXEC held before and after the read, how many polls the
	// read took (as a bool) and what was written.
	read := func(name string) *yaroze.EXEC {
		fillRegion()
		p := yaroze.CdReadExec(cstr(name))
		print(" -> ", p != nil)
		if p == nil {
			extent()
			return nil
		}
		words(p)
		var polls int
		for yaroze.CdReadSync(1, &res[0]) > 0 {
			polls++
		}
		print(" sync ", polls > 0, " ", yaroze.CdReadSync(0, &res[0]))
		words(p)
		extent()
		return p
	}

	// CdReadExec: each child, then running it.
	var ps [2]*yaroze.EXEC
	for i, name := range []string{"\\CHILD.EXE;1", "\\LONG.EXE;1", "\\ENTRY.EXE;1", "\\SHORT.EXE;1", "\\NOBSS.EXE;1", "\\BAD.BIN;1", "\\NONE.EXE;1", "cdrom:\\CHILD.EXE;1", "\\CHILD.EXE", "\\child.exe;1", ""} {
		begin("CdReadExec " + name)
		p := read(name)
		if i < 2 {
			ps[i] = p
		}
		if p != nil && p.Pc0 != 0 && p.Pc0 != fill && name != "\\BAD.BIN;1" {
			exec(p, p.T_addr, 2, &argv[0], true)
		}
		end()
	}
	begin("CdReadExec same pointer")
	print(" ", ps[0] == ps[1], " ", uintptr(unsafe.Pointer(ps[0]))-0x80000000 < 0x200000)
	end()
	begin("CdReadExec then CdReadSync(0) only")
	fillRegion()
	p := yaroze.CdReadExec(cstr("\\LONG.EXE;1"))
	print(" ", p != nil, " ", yaroze.CdReadSync(0, &res[0]))
	extent()
	end()
	begin("CdReadExec then CdReadFile")
	fillRegion()
	p = yaroze.CdReadExec(cstr("\\CHILD.EXE;1"))
	print(" ", p != nil, " ", yaroze.CdReadFile(cstr("\\LONG.EXE;1"), (*uint32)(yaroze.Pointer(0x80130000)), 0))
	print(" ", yaroze.CdReadSync(0, &res[0]))
	extent()
	end()

	// Exec over copies of the EXEC, with its fields changed by hand.
	begin("Exec again")
	p = read("\\CHILD.EXE;1")
	exec(p, p.T_addr, 2, &argv[0], true)
	exec(p, p.T_addr, 0, nil, true)
	end()
	begin("Exec copy")
	e = *p
	exec(&e, e.T_addr, 1, &argv[1], true)
	end()
	begin("Exec not critical")
	exec(&e, e.T_addr, 2, &argv[0], false)
	end()
	begin("Exec stack")
	p = read("\\STACK.EXE;1")
	exec(p, p.T_addr, 2, &argv[0], true)
	end()
	begin("Exec stack S_addr 0")
	e = *p
	e.S_addr, e.S_size = 0, 0
	exec(&e, e.T_addr, 2, &argv[0], true)
	end()
	begin("Exec stack S_size 0")
	e = *p
	e.S_addr, e.S_size = 0x80190000, 0
	exec(&e, e.T_addr, 2, &argv[0], true)
	end()
	begin("Exec stack B_size 0")
	e = *p
	e.B_size = 0
	poke(0x80111000, 0x11111111)
	exec(&e, e.T_addr, 2, &argv[0], true)
	print(" ")
	word(peek(0x80111000))
	end()
	// A B_size that is not a multiple of 4 halts the emulator's kernel on
	// an unaligned store, so none is tried.
	begin("Exec hand-made")
	// The child's code copied by hand to another address, with a header
	// written here: Exec runs what the EXEC says.
	p = read("\\CHILD.EXE;1")
	for i := uint32(0); i < 0x800; i += 4 {
		poke(0x801a8000+i, peek(0x80100000+i))
	}
	e = yaroze.EXEC{Pc0: 0x801a8000, Gp0: 0x12340000, T_addr: 0x801a8000, T_size: 0x800, B_addr: 0x801a9000, B_size: 0x100}
	exec(&e, 0x80100000, 2, &argv[0], true)
	extent()
	end()
	begin("Exec junk")
	p = read("\\JUNK.EXE;1")
	exec(p, p.T_addr, 2, &argv[0], true)
	end()
	begin("Exec junk again, not critical")
	exec(p, p.T_addr, 1, &argv[1], false)
	end()

	// The kernel's loading, last: on the emulator's BIOS a cdrom: name
	// that exists never comes back, so those cases end the run.
	for _, name := range names {
		begin("LoadTest " + name)
		fillRegion()
		fillExec(&e)
		print(" -> ", yaroze.LoadTest(cstr(name), &e))
		words(&e)
		extent()
		end()
	}
	for _, name := range names {
		begin("Load " + name)
		fillRegion()
		fillExec(&e)
		print(" -> ", yaroze.Load(cstr(name), &e))
		words(&e)
		extent()
		end()
	}
	println("done")
	for {
		yaroze.VSync(0)
	}
}
