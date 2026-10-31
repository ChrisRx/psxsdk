package main

import "go.chrisrx.dev/psx/yaroze"

// The file commands: dir, cd, re, wr, ld, ex, ren, del and format. A file
// is named as the kernel names it, bu00:NAME for the card in slot 1, bu10:
// in slot 2, which go through the kernel's file functions, and
// cdrom:\NAME;1 on the disc, which go through the library's (see cdrom.go),
// where cd sets the directory a name without one is in.

// cwd is the disc's current directory, as cd sets it: a path from the root
// ending in a backslash.
var cwd [128]byte
var cwdLen = 0

// path and newPath are where C strings naming files are made for the
// kernel.
var path, newPath [160]byte

// cString returns s as a C string in dst.
func cString(dst []byte, s []byte) *byte {
	n := copy(dst[:len(dst)-1], s)
	dst[n] = 0
	return &dst[0]
}

// dirOf is the disc's current directory, a backslash for the root.
func dirOf(d []byte) []byte {
	if len(d) == 0 {
		return []byte{'\\'}
	}
	return d
}

func putName(s []byte) { line.putBytes(s) }

// cd is cd: with the disc and a path, it changes the disc's current
// directory; with nothing, or the disc alone, it says what it is. Other
// devices have no directories.
func cd(args [][]byte) {
	if len(args) > 0 && !(len(args[0]) >= 6 && is(args[0][:6], "cdrom:")) {
		return
	}
	if len(args) == 0 || len(args[0]) == 6 {
		line.puts("current directory : ")
		putName(dirOf(cwd[:cwdLen]))
		line.puts("\r\n")
		return
	}
	p := args[0][6:]
	line.puts("change directory : ")
	putName(p)
	line.puts("\r\n")
	cwdLen = 0
	if len(p) > 0 && p[0] == '\\' {
		p = p[1:]
	}
	if len(p) > 0 {
		cwd[0] = '\\'
		cwdLen = 1 + copy(cwd[1:len(cwd)-1], p)
		if cwd[cwdLen-1] != '\\' {
			cwd[cwdLen] = '\\'
			cwdLen++
		}
	}
}

// devices are the kernel's devices, which dir names for one it does not
// know.
const devices = "Known devices are:\r\n" +
	"        tty:    SIO console\r\n" +
	"        cdrom:  CD-ROM\r\n" +
	"        bu:     MEMORY CARD\r\n" +
	"        mem:    (reserved)\r\n" +
	"        cdx:    (reserved)\r\n"

const dirRule = "--------------------------------------------------------------------------\r\n"

var entry yaroze.DIRENTRY

// dir is dir: the files on a device, the disc's current directory if none
// is given, with each one's size and first sector or block.
func dir(args [][]byte) {
	dev := []byte("cdrom:")
	if len(args) > 0 {
		dev = args[0]
	}
	line.puts("List of files (")
	putName(dev)
	line.puts(")\r\n")
	name := dev
	if i := indexByte(dev, ':'); i >= 0 {
		name = dev[:i]
	}
	for len(name) > 0 && name[len(name)-1] >= '0' && name[len(name)-1] <= '9' {
		name = name[:len(name)-1]
	}
	if !is(name, "tty") && !is(name, "cdrom") && !is(name, "bu") && !is(name, "mem") && !is(name, "cdx") {
		putName(name)
		line.puts(" is not known device\r\n")
		line.puts(devices)
		line.puts("no file\r\n")
		return
	}
	var count int
	if is(name, "cdrom") {
		d := cwd[:cwdLen]
		if len(dev) > 6 {
			d = dev[6:]
		}
		count = cdList(d)
	} else {
		var pattern [64]byte
		n := copy(pattern[:len(pattern)-2], dev)
		pattern[n] = '*'
		pattern[n+1] = 0
		for e := yaroze.Firstfile(&pattern[0], &entry); e != nil; e = yaroze.Nextfile(&entry) {
			count++
			l := indexByte(entry.Name[:], 0)
			if l < 0 {
				l = len(entry.Name)
			}
			listEntry(count, entry.Name[:l], uint32(entry.Size), uint32(entry.Head))
		}
	}
	if count == 0 {
		line.puts("no file\r\n")
		return
	}
	line.puts(dirRule)
}

// listEntry shows a file in dir's list, the list's head before the first.
func listEntry(count int, name []byte, size, head uint32) {
	if count == 1 {
		line.puts(dirRule)
		line.puts(" #  name                          size      hex           head      hex \r\n")
		line.puts(dirRule)
	}
	putDecimalWidth(uint32(count), 3, '0')
	line.putc(':')
	putName(name)
	line.puts("                        "[min(len(name), 24):])
	putDecimalWidth(size, 10, ' ')
	line.putc('(')
	putHexWidth(size, 8)
	line.putc(')')
	putDecimalWidth(head, 14, ' ')
	line.putc('(')
	putHexWidth(head, 8)
	line.puts(")\r\n")
}

func indexByte(s []byte, c byte) int {
	for i, b := range s {
		if b == c {
			return i
		}
	}
	return -1
}

// putDecimalWidth sends v in decimal, at least width wide, padded on the
// left with pad.
func putDecimalWidth(v uint32, width int, pad byte) {
	i := len(digits)
	for {
		i--
		digits[i] = byte('0' + v%10)
		v /= 10
		if v == 0 {
			break
		}
	}
	for range width - (len(digits) - i) {
		line.putc(pad)
	}
	for _, c := range digits[i:] {
		line.putc(c)
	}
}

// putHexWidth sends v in hexadecimal, at least width wide, padded with
// spaces on the left.
func putHexWidth(v uint32, width int) {
	n := 1
	for x := v >> 4; x != 0; x >>= 4 {
		n++
	}
	for range width - n {
		line.putc(' ')
	}
	putHexN(v, n)
}

// at is RAM at addr, for the kernel to read into or write from.
func at(addr uint32) *byte { return (*byte)(yaroze.Pointer(uintptr(addr))) }

// readFile is re: a file read into RAM at an address.
func readFile(args [][]byte) {
	if len(args) < 2 {
		line.puts("usage: <command> file-name address\r\n")
		return
	}
	line.puts("Reading ")
	putName(args[0])
	line.puts("...")
	addr := hex(args[1])
	if isCD(args[0]) {
		n := cdRead(args[0], addr)
		if n < 0 {
			line.puts("failed.\r\n")
			return
		}
		readDone(uint32(n))
		return
	}
	fd := yaroze.Open(cString(path[:], args[0]), yaroze.O_RDONLY)
	if fd < 0 {
		line.puts("failed.\r\n")
		return
	}
	total := uint32(0)
	for {
		n := yaroze.Read(fd, at(addr+total), 2048)
		if n <= 0 {
			break
		}
		total += uint32(n)
		if n < 2048 {
			break
		}
	}
	yaroze.Close(fd)
	readDone(total)
}

func readDone(size uint32) {
	line.puts("done.\r\nsize=")
	line.putDecimal(size)
	line.puts("(hex:")
	line.putHex(size)
	line.puts(") bytes\r\n")
}

// writeFile is wr: RAM at an address written into a file, which on a card
// is made as many blocks long as it needs.
func writeFile(args [][]byte) {
	if len(args) < 3 {
		line.puts("usage: <command> file-name address length\r\n")
		return
	}
	line.puts("Writing ")
	putName(args[0])
	line.puts("...")
	addr, size := hex(args[1]), hex(args[2])
	if isCD(args[0]) {
		line.puts("failed.\r\n")
		return
	}
	name := cString(path[:], args[0])
	blocks := int32((size + yaroze.CardBlock - 1) / yaroze.CardBlock)
	if fd := yaroze.Open(name, yaroze.O_CREAT|blocks<<16); fd >= 0 {
		yaroze.Close(fd)
	}
	fd := yaroze.Open(name, yaroze.O_WRONLY)
	if fd < 0 {
		line.puts("failed.\r\n")
		return
	}
	// A card takes whole sectors.
	n := yaroze.Write(fd, at(addr), int32((size+yaroze.CardSector-1)/yaroze.CardSector*yaroze.CardSector))
	yaroze.Close(fd)
	if n < 0 {
		line.puts("failed.\r\n")
		return
	}
	line.puts("done.\r\nsize=")
	line.putDecimal(size)
	line.puts("(hex:")
	line.putHex(size)
	line.puts(") bytes\r\n")
}

var exec yaroze.EXEC

// loadFile is ld, and with run ex: a PS-X EXE loaded from a file, its
// registers set to start it, its stack where the second argument says or
// its header does, and with run, started. The boot disc's monitor tells a
// file that is not a PS-X EXE by its type, and stops; this one says it
// failed, as for a file that is not there.
func loadFile(args [][]byte, run bool) {
	if len(args) < 1 {
		line.puts("usage: <command> file-name [sp value]\r\n")
		return
	}
	if run {
		line.puts("LoadExec ")
		putName(args[0])
		line.puts("...")
	} else {
		line.puts("Load ")
		putName(args[0])
	}
	ok := false
	if isCD(args[0]) {
		if e := cdLoad(args[0]); e != nil {
			exec, ok = *e, true
		}
	} else {
		ok = yaroze.Load(cString(path[:], args[0]), &exec) != 0
	}
	if !ok {
		if run {
			line.puts("failed.\r\n")
		} else {
			line.puts(" failed\r\n")
		}
		return
	}
	if !run {
		line.puts(" DONE\r\n")
	}
	regs[regEPC] = exec.Pc0
	regs[regGP] = exec.Gp0
	switch {
	case len(args) > 1:
		regs[regSP] = hex(args[1])
	case exec.S_addr+exec.S_size != 0:
		regs[regSP] = exec.S_addr + exec.S_size
	default:
		regs[regSP] = defaultStack
	}
	if run {
		start()
	}
}

// rename is ren.
func rename(args [][]byte) {
	if len(args) < 2 {
		line.puts("usage: <command> old-file-name new-file-name\r\n")
		return
	}
	line.puts("Rename file : ")
	putName(args[0])
	line.puts(" -> ")
	putName(args[1])
	line.puts("\r\n")
	if isCD(args[0]) {
		result(0)
		return
	}
	result(yaroze.Rename(cString(path[:], args[0]), cString(newPath[:], args[1])))
}

// remove is del, which says nothing without a file.
func remove(args [][]byte) {
	if len(args) < 1 {
		return
	}
	line.puts("Delete file : ")
	putName(args[0])
	line.puts("\r\n")
	if isCD(args[0]) {
		result(0)
		return
	}
	result(yaroze.Delete(cString(path[:], args[0])))
}

// format is format.
func format(args [][]byte) {
	if len(args) < 1 {
		line.puts("usage: <command> device-name\r\n")
		return
	}
	line.puts("Format device : ")
	putName(args[0])
	line.puts("\r\n")
	if isCD(args[0]) {
		result(0)
		return
	}
	result(yaroze.Format(cString(path[:], args[0])))
}

// result says how a kernel call that returns 1 for success went.
func result(r int32) {
	if r == 1 {
		line.puts("complete\r\n")
		return
	}
	line.puts("failed(")
	line.putDecimal(uint32(r))
	line.puts(")\r\n")
}
