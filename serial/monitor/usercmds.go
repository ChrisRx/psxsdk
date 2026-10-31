package main

import (
	"unsafe"

	"go.chrisrx.dev/psx/yaroze"
)

// The commands that manage the monitor itself: user commands (sc, ac and
// dc), the kernel's device drivers (ad and dd), and the line's speed
// (baud).

// call1 and call2 call the code at addr with one argument, or two, as C
// does: a func value is its context and its code's address, and the code
// takes the context as an extra last argument, which C code ignores.
func call1(addr, a0 uint32) int32 {
	var f func(uint32) int32
	(*[2]uintptr)(unsafe.Pointer(&f))[1] = uintptr(addr)
	return f(a0)
}

func call2(addr, a0, a1 uint32) int32 {
	var f func(uint32, uint32) int32
	(*[2]uintptr)(unsafe.Pointer(&f))[1] = uintptr(addr)
	return f(a0, a1)
}

// kernelB calls the kernel's B-function number fn with an argument,
// through a thunk that puts the number in t1 and jumps to the kernel's
// stub at B0h. The thunks are made once, in .data, a slot each.
var thunks = [2][4]uint32{{1}}

func kernelB(slot int, fn, a0 uint32) int32 {
	t := &thunks[slot]
	if t[3] != addiu(t1, zero, int32(fn)) {
		*t = [4]uint32{lui(t0, 0x8000), addiu(t0, t0, 0xb0), jr(t0), addiu(t1, zero, int32(fn))}
		flushCache()
	}
	return call1(uint32(uintptr(unsafe.Pointer(t))), a0)
}

// addDriver is ad: the device driver whose control block is at an address
// added to the kernel's, with its AddDevice, B(47h).
func addDriver(args [][]byte) {
	if len(args) == 0 {
		line.puts("Usage: <command> driver-addres\r\n")
		return
	}
	dcb := hex(args[0])
	line.puts("Add driver ")
	line.putHex(dcb)
	line.puts(" (name:")
	n := len("(name:")
	if dcb != 0 {
		name := *(*uint32)(yaroze.Pointer(uintptr(dcb)))
		for p := name; load(p, 1) != 0 && n < 40; p++ {
			line.putc(byte(load(p, 1)))
			n++
		}
	}
	line.putc(')')
	n++
	for ; n < 12; n++ {
		line.putc(' ')
	}
	if kernelB(0, 0x47, dcb) == 1 {
		line.puts("done\r\n")
	} else {
		line.puts("fault\r\n")
	}
}

// deleteDriver is dd: a device driver taken out of the kernel's by name, as
// its RemoveDevice does, B(48h).
func deleteDriver(args [][]byte) {
	if len(args) == 0 {
		line.puts("Usage: <command> device-name\r\n")
		return
	}
	line.puts("Delete driver ")
	putName(args[0])
	for n := len(args[0]); n < 10; n++ {
		line.putc(' ')
	}
	if kernelB(1, 0x48, uint32(uintptr(unsafe.Pointer(cString(path[:], args[0]))))) == 1 {
		line.puts("done\r\n")
	} else {
		line.puts("fault\r\n")
	}
}

// A user command: a name typed at the prompt that calls code in memory,
// with the words after it, C's argc and argv.
type userCommand struct {
	name  [32]byte
	n     int
	entry uint32
}

var userCommands [16]userCommand

func findUser(name []byte) int {
	for i := range userCommands {
		u := &userCommands[i]
		if u.n > 0 && u.n == len(name) && equal(name, string(u.name[:u.n])) {
			return i
		}
	}
	return -1
}

// showUser is sc: the user commands, each by its slot, with its entry.
func showUser() {
	line.puts("Show user commands\r\n")
	for i := range userCommands {
		u := &userCommands[i]
		if u.n == 0 {
			continue
		}
		put2(uint32(i))
		line.putc(':')
		line.putBytes(u.name[:u.n])
		col := 3 + u.n
		for ; col < 15; col++ {
			line.putc(' ')
		}
		// A tab, to the next stop every eight columns.
		for line.putc(' '); (col+1)%8 != 0; col++ {
			line.putc(' ')
		}
		line.putHex(u.entry)
		line.puts("\r\n")
	}
}

// addUser is ac: a user command added, or one of the name pointed at a new
// entry.
func addUser(args [][]byte) {
	if len(args) < 2 {
		line.puts("usage:<command> command-name entry-address\r\n")
		return
	}
	i := findUser(args[0])
	if i < 0 {
		for j := range userCommands {
			if userCommands[j].n == 0 {
				i = j
				break
			}
		}
	}
	if i < 0 {
		return
	}
	u := &userCommands[i]
	u.n = copy(u.name[:], args[0])
	u.entry = hex(args[1])
}

// deleteUser is dc.
func deleteUser(args [][]byte) {
	if len(args) < 1 {
		line.puts("usage:<command> command-name\r\n")
		return
	}
	i := findUser(args[0])
	if i < 0 {
		line.puts("no such a command\r\n")
		return
	}
	userCommands[i].n = 0
}

// argv is the C strings a user command is called with.
var argv [len(fields) + 1]uint32

// runUser runs a user command, if the line names one, and reports whether
// it did. Its words are made C strings where they are, in the line.
func runUser(words [][]byte) bool {
	i := findUser(words[0])
	if i < 0 {
		return false
	}
	for j, w := range words {
		if end := cap(w) - len(w); end > 0 {
			w[:len(w)+1][len(w)] = 0
		}
		argv[j] = uint32(uintptr(unsafe.Pointer(&w[0])))
	}
	argv[len(words)] = 0
	call2(userCommands[i].entry, uint32(len(words)), uint32(uintptr(unsafe.Pointer(&argv[0]))))
	return true
}

// changeBaud is baud with a speed: the line set to it, if it is one the
// line takes, after saying how to follow on the host; then, once the host
// is back, whether to keep it on the card, in the file the monitor reads
// it from as it starts.
func changeBaud(b uint32) {
	if b < 2400 || b > 115200 {
		line.puts("Invalid baud rate value.\r\n")
		return
	}
	line.puts("Baud rate has been changed to ")
	line.putDecimal(b)
	line.puts(" bps.\r\nHit F10 F2 Key to exit, then run SIOCONS again \r\n" +
		"specifying the new baud rate option. \r\nAnd hit RETURN key.\r\n")
	line.open(b)
	savedBaud = b
	for line.getc() != '\r' {
	}
	line.puts("                               \rSave the new baud rate into the memory card ? ( y/n ) ")
	c := line.getc()
	line.putc(c)
	line.puts("\r\n")
	if c != 'y' && c != 'Y' {
		return
	}
	// The file is kept whole, with only the speed changed in it.
	if yaroze.CardRead(0, cipFile, cipFull[:]) < 0 {
		cipFull = [len(cipFull)]byte{}
	}
	cipFull[cipBaudAt], cipFull[cipBaudAt+1], cipFull[cipBaudAt+2], cipFull[cipBaudAt+3] = byte(b), byte(b>>8), byte(b>>16), byte(b>>24)
	yaroze.CardWrite(0, cipFile, cipFull[:])
}

// cipFull is the whole of the file the speed is kept in, a block.
var cipFull [yaroze.CardBlock]byte
