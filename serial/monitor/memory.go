package main

import "go.chrisrx.dev/psx/yaroze"

// dumpRegister is dr with a register's name: the register and its value,
// or the list of names with their numbers if there is no such register.
func dumpRegister(name []byte) {
	r := register(name)
	if r < 0 {
		line.puts("Register ")
		line.putBytes(name)
		line.puts(" is not exist. Check register list.\r\n")
		for i, label := range regLabels {
			label = label[:len(label)-1] // without its =
			if label[len(label)-1] == ' ' {
				label = label[:len(label)-1]
			}
			line.puts(label)
			line.puts("        "[len(label):])
			if i%8 == 7 || i == len(regLabels)-1 {
				line.puts("\r\n")
			}
		}
		return
	}
	line.puts(regNames[r])
	line.putc('=')
	line.putHex(regs[r])
	line.puts("\r\n")
}

// excNames are the CPU's exception codes' names, which ds shows.
var excNames = [16]string{"Int", "Mod", "TLBL", "TLBS", "AdEL", "AdES", "IBE", "DBE", "Sys", "Bp", "RI", "CpU", "Ov", "???", "???", "???"}

// showStatus is ds: the stored status and cause registers, field by field.
func showStatus() {
	sr, cr := regs[regSR], regs[regCR]
	bit := func(n uint) { line.putc(byte('0' + sr>>n&1)) }
	line.puts("sr: CU   RE  BEV TS  PE  CM  PZ SwC IsC Intr   Sw  KUo IEo KUp IEp KUc IEc\r\n    ")
	putBits(sr>>28, 4)
	for _, n := range []uint{25, 22, 21, 20, 19, 18, 17} {
		line.putc(' ')
		bit(n)
		line.puts("  ")
	}
	bit(16)
	line.puts("   ")
	putBits(sr>>10, 6)
	line.putc(' ')
	putBits(sr>>8, 2)
	for i, n := range []uint{5, 4, 3, 2, 1, 0} {
		if i == 0 {
			line.puts("  ")
		} else {
			line.puts("   ")
		}
		bit(n)
	}
	line.puts("\r\ncr: BD   CE  IP      Sw  ExcCode\r\n    ")
	line.putc(byte('0' + cr>>31))
	line.puts("    ")
	putBits(cr>>28, 2)
	line.puts("  ")
	putBits(cr>>10, 6)
	line.puts("  ")
	putBits(cr>>8, 2)
	line.puts("  ")
	code := cr >> 2 & 15
	line.putc(byte('0' + code/10))
	line.putc(byte('0' + code%10))
	line.puts(" (")
	line.puts(excNames[code])
	line.puts(")\r\n")
}

// putBits sends the low n bits of v in binary.
func putBits(v uint32, n int) {
	for i := n - 1; i >= 0; i-- {
		line.putc(byte('0' + v>>i&1))
	}
}

// putHexN sends the low n digits of v in hexadecimal.
func putHexN(v uint32, n int) {
	for i := n - 1; i >= 0; i-- {
		line.putc("0123456789abcdef"[v>>(4*i)&15])
	}
}

// next is where each size of dump goes on from without an address: a
// word's, a half-word's and a byte's.
var next [3]uint32

// sizeIndex is next's index for a size in bytes.
func sizeIndex(size uint32) int { return int(size / 2) }

func load(addr, size uint32) uint32 {
	switch size {
	case 4:
		return *(*uint32)(yaroze.Pointer(uintptr(addr)))
	case 2:
		return uint32(*(*uint16)(yaroze.Pointer(uintptr(addr))))
	}
	return uint32(*(*uint8)(yaroze.Pointer(uintptr(addr))))
}

func store(addr, size, v uint32) {
	switch size {
	case 4:
		*(*uint32)(yaroze.Pointer(uintptr(addr))) = v
	case 2:
		*(*uint16)(yaroze.Pointer(uintptr(addr))) = uint16(v)
	default:
		*(*uint8)(yaroze.Pointer(uintptr(addr))) = uint8(v)
	}
}

// dump is dw, dh or db: from the address, or where the last dump of the
// size stopped, as many bytes as the count says, in hexadecimal, rounded
// up to whole items, or 100h, sixteen bytes to a line.
func dump(size uint32, args [][]byte) {
	addr := next[sizeIndex(size)]
	if len(args) > 0 {
		addr = hex(args[0])
	}
	addr &^= size - 1
	n := uint32(0x100)
	if len(args) > 1 {
		n = hex(args[1])
	}
	items := (n + size - 1) / size
	perLine := 16 / size
	gap := "  "
	if size == 1 {
		gap = " "
	}
	for i := uint32(0); i < items; i++ {
		if i%perLine == 0 {
			line.putHex(addr)
			line.putc(':')
		}
		putHexN(load(addr, size), int(size*2))
		line.puts(gap)
		addr += size
		if i%perLine == perLine-1 || i == items-1 {
			line.puts("\r\n")
		}
	}
	next[sizeIndex(size)] = addr
}

// set is sw, sh or sb: with an address and a value, it stores the value
// there; otherwise it asks for a value for each item from the address on,
// storing what is typed, going on to the next on an empty line, asking
// again for a word that is not a number, and stopping at a dot.
func set(size uint32, args [][]byte) {
	var addr uint32
	if len(args) > 0 {
		addr = hex(args[0]) &^ (size - 1)
	}
	if len(args) == 2 {
		store(addr, size, hex(args[1]))
		return
	}
	var p [9]byte
	for {
		for i := 7; i >= 0; i-- {
			p[i] = "0123456789abcdef"[addr>>(4*(7-i))&15]
		}
		p[8] = ':'
		line.putBytes(p[:])
		words := split(readLine(p[:]))
		switch {
		case len(words) == 0:
			addr += size
		case equal(words[0], "."):
			return
		case isHex(words[0]):
			store(addr, size, hex(words[0]))
			addr += size
		default:
			line.puts(" ?")
			line.putBytes(words[0])
			line.puts("?\r\n")
		}
	}
}

// isHex reports whether word is all hexadecimal digits.
func isHex(word []byte) bool {
	for _, c := range word {
		if !('0' <= c && c <= '9' || 'a' <= c && c <= 'f' || 'A' <= c && c <= 'F') {
			return false
		}
	}
	return len(word) > 0
}

// fill is fw, fh or fb: the value stored in as many items from the address
// as the count says, or four.
func fill(size uint32, args [][]byte) {
	if len(args) < 2 {
		line.puts("?\r\n")
		return
	}
	addr := hex(args[0]) &^ (size - 1)
	v := hex(args[1])
	n := uint32(4)
	if len(args) > 2 {
		n = hex(args[2])
	}
	for range n {
		store(addr, size, v)
		addr += size
	}
}
