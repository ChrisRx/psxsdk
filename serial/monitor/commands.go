package main

import (
	"unsafe"

	"go.chrisrx.dev/psx/yaroze"
)

// The stored registers, which go starts a program with: the 32 general
// registers, then these.
const (
	regGP = 28
	regSP = 29
)

const (
	regEPC = 32 + iota
	regHI
	regLO
	regSR
	regCR
	numRegs
)

var regs [numRegs]uint32

// regNames are the registers' names, as sr takes them.
var regNames = [numRegs]string{
	"zero", "at", "v0", "v1", "a0", "a1", "a2", "a3",
	"t0", "t1", "t2", "t3", "t4", "t5", "t6", "t7",
	"s0", "s1", "s2", "s3", "s4", "s5", "s6", "s7",
	"t8", "t9", "k0", "k1", "gp", "sp", "fp", "ra",
	"epc", "hi", "lo", "sr", "cr",
}

// regLabels are the registers' labels in dr's dump.
var regLabels = [numRegs]string{
	"zero(0)=", "at( 1) =", "v0( 2) =", "v1( 3) =", "a0( 4) =", "a1( 5) =", "a2( 6) =", "a3( 7) =",
	"t0( 8) =", "t1( 9) =", "t2(10) =", "t3(11) =", "t4(12) =", "t5(13) =", "t6(14) =", "t7(15) =",
	"s0(16) =", "s1(17) =", "s2(18) =", "s3(19) =", "s4(20) =", "s5(21) =", "s6(22) =", "s7(23) =",
	"t8(24) =", "t9(25) =", "k0(26) =", "k1(27) =", "gp(28) =", "sp(29) =", "fp(30) =", "ra(31) =",
	"pc(epc)=", "hi     =", "lo     =", "sr     =", "cr     =",
}

// defaultStack is where go points a program's stack if nothing has set sp.
const defaultStack = 0x801ffff0

// fields is where a command line is split into words.
var fields [8][]byte

// split splits l at spaces and tabs into fields, and returns as many as
// there are, up to len(fields).
func split(l []byte) [][]byte {
	n := 0
	for i := 0; i < len(l) && n < len(fields); {
		for i < len(l) && (l[i] == ' ' || l[i] == '\t') {
			i++
		}
		start := i
		for i < len(l) && l[i] != ' ' && l[i] != '\t' {
			i++
		}
		if i > start {
			fields[n] = l[start:i]
			n++
		}
	}
	return fields[:n]
}

// equal reports whether word is s.
func equal(word []byte, s string) bool {
	if len(word) != len(s) {
		return false
	}
	for i := range word {
		if word[i] != s[i] {
			return false
		}
	}
	return true
}

// The prompts readLine redraws a line after, made once.
var (
	commandPrompt  = []byte(">>")
	registerPrompt = []byte("Register: ")
	valuePrompt    = []byte("New value:      ")
)

// is reports whether word is name, whatever its case.
func is(word []byte, name string) bool {
	if len(word) != len(name) {
		return false
	}
	for i := range word {
		c := word[i]
		if 'A' <= c && c <= 'Z' {
			c += 'a' - 'A'
		}
		if c != name[i] {
			return false
		}
	}
	return true
}

// hex reads the hexadecimal number at the start of word, stopping at the
// first character that is not a digit, so that a word with none is 0.
func hex(word []byte) uint32 {
	var v uint32
	for _, c := range word {
		switch {
		case '0' <= c && c <= '9':
			c -= '0'
		case 'a' <= c && c <= 'f':
			c -= 'a' - 10
		case 'A' <= c && c <= 'F':
			c -= 'A' - 10
		default:
			return v
		}
		v = v<<4 | uint32(c)
	}
	return v
}

// decimal reads the decimal number at the start of word.
func decimal(word []byte) uint32 {
	var v uint32
	for _, c := range word {
		if c < '0' || c > '9' {
			break
		}
		v = v*10 + uint32(c-'0')
	}
	return v
}

// command carries out a line the host typed.
func command(l []byte) {
	args := split(l)
	if len(args) == 0 {
		return
	}
	name, args := args[0], args[1:]
	switch {
	case is(name, "help"), is(name, "?"):
		help(args)
	case is(name, "dr"):
		if len(args) > 0 {
			dumpRegister(args[0])
		} else {
			dumpRegisters()
		}
	case is(name, "ds"):
		showStatus()
	case is(name, "sr"):
		setRegister(args)
	case is(name, "go"):
		if len(args) > 0 {
			regs[regEPC] = hex(args[0])
		}
		start()
	// The host says it has a byte of a binary read by turning its RTS off
	// for a few microseconds, which an interrupt handler running then would
	// hide, so a binary session runs with interrupts off.
	case is(name, "bwr"):
		yaroze.EnterCriticalSection()
		binaryWrite()
		yaroze.ExitCriticalSection()
	case is(name, "brd"):
		yaroze.EnterCriticalSection()
		binaryRead()
		yaroze.ExitCriticalSection()
	case is(name, "baud"):
		if len(args) > 0 {
			changeBaud(decimal(args[0]))
			return
		}
		line.putDecimal(line.baud)
		line.puts(" bps\r\n")
	case is(name, "dw"):
		dump(4, args)
	case is(name, "dh"):
		dump(2, args)
	case is(name, "db"):
		dump(1, args)
	case is(name, "sw"):
		set(4, args)
	case is(name, "sh"):
		set(2, args)
	case is(name, "sb"):
		set(1, args)
	case is(name, "fw"):
		fill(4, args)
	case is(name, "fh"):
		fill(2, args)
	case is(name, "fb"):
		fill(1, args)
	case is(name, "dir"):
		dir(args)
	case is(name, "cd"):
		cd(args)
	case is(name, "re"), is(name, "read"):
		readFile(args)
	case is(name, "wr"), is(name, "write"):
		writeFile(args)
	case is(name, "ld"), is(name, "load"):
		loadFile(args, false)
	case is(name, "ex"), is(name, "exec"):
		loadFile(args, true)
	case is(name, "ren"):
		rename(args)
	case is(name, "del"):
		remove(args)
	case is(name, "format"):
		format(args)
	case is(name, "war"):
		waveTransfer(args, true)
	case is(name, "waw"):
		waveTransfer(args, false)
	case is(name, "var"):
		frameTransfer(args, true)
	case is(name, "vaw"):
		frameTransfer(args, false)
	case is(name, "cb"):
		colorBar()
	case is(name, "play"):
		play(args)
	case is(name, "rdb"):
		rdb()
	case is(name, "dwc"):
		dumpContinuously(4, args)
	case is(name, "dhc"):
		dumpContinuously(2, args)
	case is(name, "dbc"):
		dumpContinuously(1, args)
	case is(name, "swc"):
		setContinuously(4, args)
	case is(name, "shc"):
		setContinuously(2, args)
	case is(name, "sbc"):
		setContinuously(1, args)
	case is(name, "di"), is(name, "dis"):
		dis(args)
	case is(name, "ad"):
		addDriver(args)
	case is(name, "dd"):
		deleteDriver(args)
	case is(name, "sc"):
		showUser()
	case is(name, "ac"):
		addUser(args)
	case is(name, "dc"):
		deleteUser(args)
	case is(name, "cls"):
	default:
		if !runUser(split(l)) {
			line.puts("?\r\n")
		}
	}
}

func dumpRegisters() {
	for i, label := range regLabels {
		line.puts(label)
		line.putHex(regs[i])
		line.puts("  ")
		if i%4 == 3 || i == len(regLabels)-1 {
			line.puts("\r\n")
		}
	}
}

// register returns the index of the register named word, or -1.
func register(word []byte) int {
	for i, n := range regNames {
		if equal(word, n) {
			return i
		}
	}
	return -1
}

// setRegister is sr: with a register and a value it sets the register,
// with only a register it shows its value and asks for the new one, and
// with nothing it asks for the register first.
func setRegister(args [][]byte) {
	if len(args) == 0 {
		line.puts("Register: ")
		args = split(readLine(registerPrompt))
		if len(args) == 0 {
			return
		}
		args = args[:1]
	}
	r := register(args[0])
	if r < 0 {
		line.puts("Register ")
		line.putBytes(args[0])
		line.puts(" is not exist. Check register list.\r\n")
		for i, n := range regNames {
			line.puts(n)
			line.puts("        "[len(n):])
			if i%8 == 7 || i == len(regNames)-1 {
				line.puts("\r\n")
			}
		}
		return
	}
	var v uint32
	if len(args) > 1 {
		v = hex(args[1])
	} else {
		line.puts("Register: ")
		line.puts(regNames[r])
		line.puts("\r\nNow value:      ")
		line.putHex(regs[r])
		line.puts("\r\nNew value:      ")
		value := split(readLine(valuePrompt))
		if len(value) == 0 {
			return
		}
		v = hex(value[0])
	}
	if r != 0 {
		regs[r] = v
	}
	line.puts("Set value:      ")
	line.putHex(v)
	line.puts("\r\n")
}

// start is go: it starts the program at the stored epc, with the stored
// gp and sp, and starts the monitor again if the program returns.
func start() {
	sp := regs[regSP]
	if sp == 0 {
		sp = defaultStack
	}
	flushCache()
	run(regs[regEPC], regs[regGP], sp)
}

// The bytes of a binary transfer.
const (
	transferStart = 0x01
	blockStart    = 0x02
	blockSize     = 2048
)

var block [blockSize]byte

// memory returns n bytes of RAM at addr.
func memory(addr, n uint32) []byte {
	return unsafe.Slice((*byte)(yaroze.Pointer(uintptr(addr))), n)
}

// get32 reads a big-endian word from the host.
func get32() uint32 {
	var v uint32
	for range 4 {
		v = v<<8 | uint32(line.getc())
	}
	return v
}

// binaryWrite is bwr: after "binary", the host sends 01h with an address
// and a size, then the data in 2048-byte blocks, each 02h, the block and
// its sum, which is answered Y and the block stored, or N if the sum is
// wrong. Then another 01h starts another write, and anything else ends the
// session.
func binaryWrite() {
	line.puts("binary\r")
	for line.getc() == transferStart {
		addr, size := get32(), get32()
		for size > 0 {
			if line.getc() != blockStart {
				end()
				return
			}
			var sum byte
			for i := range block {
				block[i] = line.getc()
				sum += block[i]
			}
			if line.getc() != sum {
				line.putc('N')
				continue
			}
			n := min(size, blockSize)
			copy(memory(addr, n), block[:n])
			line.putc('Y')
			addr += n
			size -= n
		}
	}
	end()
}

// binaryRead is brd, which goes as bwr does the other way: for each 02h
// the host sends, a block of RAM and its sum, each byte waiting for the
// host to say it has it, then the host's Y, or N to have it again. A short
// block is made up with what is in RAM after it.
func binaryRead() {
	line.puts("binary\r")
	for line.getc() == transferStart {
		addr, size := get32(), get32()
		for size > 0 {
			if line.getc() != blockStart {
				end()
				return
			}
			var sum byte
			for _, b := range memory(addr, blockSize) {
				sum += b
				line.putAcked(b)
			}
			line.putc(sum)
			if line.getc() == 'Y' {
				n := min(size, blockSize)
				addr += n
				size -= n
			}
		}
	}
	end()
}

// end ends a binary session, once what was written is where the CPU will
// fetch it.
func end() {
	flushCache()
	line.puts("end binary\r")
}
