package main

// rdb: the remote debugger, which the Net Yaroze's gdb drives with its
// "target hss", over the serial line, in HSS packets: a 12-byte header,
// the magic "HSs", a code, an address, a length and a checksum, all
// little-endian, then the data, padded to an even length. Each packet but
// an ACK or NAK is answered with one. It was written from the protocol as
// gdb speaks it, and what the boot disc's debugger is seen to answer.

// The codes of HSS packets.
const (
	hssAck    = 0
	hssNak    = 1
	hssOK     = 2
	hssRdReg  = 4
	hssWrReg  = 5
	hssRdMem  = 6
	hssWrMem  = 7
	hssCont   = 8
	hssStep   = 9
	hssSignal = 10
	hssQuery  = 11
	hssSend   = 12
	hssUnimp  = 13
)

// hssMax is the most data a packet carries.
const hssMax = 1500

// The stopped program's registers, as HSS numbers them: the general
// registers, then sr, lo, hi, a gap, the cause register and epc. The
// exception handler saves them here, and the resume code loads them back,
// both in hook_yaroze.go, at these offsets.
const (
	dbgSR    = 32
	dbgLO    = 33
	dbgHI    = 34
	dbgCause = 36
	dbgEPC   = 37
)

var dbgRegs [38]uint32

// The signal the program last stopped for, in gdb's numbers, and whether
// the debugger has the line, so that what the program prints goes to the
// host in SEND packets.
var (
	dbgSignal uint32
	debugging bool
)

// dbgStack is the stack the debugger runs on when the program stops.
var dbgStack [1024]uint32

// stepBreaks are the breakpoints a step plants, at the instructions the
// one stepped can go on to, and what they replace.
var stepBreaks [2]struct {
	addr, orig uint32
	set        bool
}

// breakInsn is break 5, gdb's breakpoint.
const breakInsn = 0x0005000d

// rdb starts the remote debugger: with the stored registers, stopped, for a
// trap, as if at a breakpoint before the program's first instruction.
func rdb() {
	line.puts("Loading Remote Debugger [monitor]...done.\r\nExec Remote Debugger (epc:")
	line.putHex(funcAddr(debugLoop))
	line.puts(")\r\n")
	for r := 1; r < 32; r++ {
		dbgRegs[r] = regs[r]
	}
	dbgRegs[dbgLO], dbgRegs[dbgHI] = regs[regLO], regs[regHI]
	// Interrupts stay off until the program runs, when the resume code's
	// rfe takes IEp, set here, into IEc.
	dbgRegs[dbgSR] = currentSR()&^0xf | 0x4
	dbgRegs[dbgCause] = 9 << 2 // a break
	dbgRegs[dbgEPC] = regs[regEPC]
	dbgSignal = 5
	installVector()
	debugging = true
	debugLoop()
}

// debugStopped is where the exception handler goes when the program stops,
// on dbgStack: it takes out a step's breakpoints, tells the host why the
// program stopped, and waits for the host again.
func debugStopped() {
	for i := range stepBreaks {
		if b := &stepBreaks[i]; b.set {
			store(b.addr, 4, b.orig)
			b.set = false
		}
	}
	flushCache()
	switch dbgRegs[dbgCause] >> 2 & 31 {
	case 4, 5, 6, 7: // address and bus errors
		dbgSignal = 10
	case 10, 11: // a reserved instruction, or a coprocessor's
		dbgSignal = 4
	case 12: // overflow
		dbgSignal = 8
	default:
		dbgSignal = 5
	}
	var w [4]byte
	putWord(w[:], dbgSignal)
	hssPut(hssSignal, 0, w[:])
	debugLoop()
}

// debugLoop answers the host's requests until it says to go on, when it
// starts the program, and does not return.
func debugLoop() {
	var w [4]byte
	for {
		code, addr, data := hssReceive()
		switch code {
		case hssRdReg:
			putWord(w[:], dbgReg(addr))
			hssPut(hssOK, 0, w[:])
		case hssWrReg:
			if addr > 0 && addr < uint32(len(dbgRegs)) && len(data) >= 4 {
				dbgRegs[addr] = getWord(data)
			}
			hssPut(hssOK, 0, nil)
		case hssRdMem:
			n := uint32(0)
			if len(data) >= 4 {
				n = min(getWord(data), hssMax)
			}
			for i := range n {
				sendData[i] = byte(load(addr+i, 1))
			}
			hssPut(hssOK, 0, sendData[:n])
		case hssWrMem:
			for i, b := range data {
				store(addr+uint32(i), 1, uint32(b))
			}
			flushCache()
			hssPut(hssOK, 0, nil)
		case hssQuery:
			putWord(w[:], dbgSignal)
			hssPut(hssOK, 0, w[:])
		case hssCont:
			resume()
		case hssStep:
			plantStep()
			resume()
		case hssAck, hssNak:
		default:
			hssPut(hssUnimp, 0, nil)
		}
	}
}

func dbgReg(n uint32) uint32 {
	if n == 0 || n >= uint32(len(dbgRegs)) {
		return 0
	}
	return dbgRegs[n]
}

// plantStep plants breakpoints at the instructions the one at epc can go on
// to: the next, or for a jump or branch, past its delay slot or where it
// goes, or both.
func plantStep() {
	pc := dbgRegs[dbgEPC]
	w := load(pc, 4)
	op, rs := w>>26, w>>21&31
	target := pc + 4 + uint32(int32(int16(w)))<<2
	next := [2]uint32{pc + 4}
	switch {
	case op == 0 && (w&63 == 8 || w&63 == 9): // jr, jalr
		next = [2]uint32{dbgReg(rs)}
	case op == 2 || op == 3: // j, jal
		next = [2]uint32{(pc+4)&0xf0000000 | w&0x3ffffff<<2}
	case op == 1 || (op >= 4 && op <= 7): // the branches
		next = [2]uint32{pc + 8, target}
	}
	for i, a := range next {
		if a == 0 || (i == 1 && a == next[0]) {
			continue
		}
		stepBreaks[i] = struct {
			addr, orig uint32
			set        bool
		}{a, load(a, 4), true}
		store(a, 4, breakInsn)
	}
	flushCache()
}

func putWord(b []byte, v uint32) {
	b[0], b[1], b[2], b[3] = byte(v), byte(v>>8), byte(v>>16), byte(v>>24)
}

func getWord(b []byte) uint32 {
	return uint32(b[0]) | uint32(b[1])<<8 | uint32(b[2])<<16 | uint32(b[3])<<24
}

// The packets are made and taken here.
var (
	sendBuf  [12 + hssMax + 1]byte
	sendData [hssMax]byte
	recvBuf  [12 + hssMax + 1]byte
)

// hssSum is the ones' complement sum of b's half-words.
func hssSum(b []byte) uint16 {
	var v uint32
	for i := 0; i+1 < len(b); i += 2 {
		v += uint32(b[i]) | uint32(b[i+1])<<8
	}
	for v>>16 != 0 {
		v = v&0xffff + v>>16
	}
	return uint16(v)
}

// hssPacket makes a packet in buf and returns it.
func hssPacket(buf []byte, code uint8, addr uint32, data []byte) []byte {
	n := len(data)
	p := buf[:12+n+n&1]
	p[0], p[1], p[2], p[3] = 'H', 'S', 's', code
	putWord(p[4:], addr)
	p[8], p[9], p[10], p[11] = byte(n), byte(n>>8), 0, 0
	copy(p[12:], data)
	if n&1 != 0 {
		p[12+n] = 0
	}
	s := ^hssSum(p)
	p[10], p[11] = byte(s), byte(s>>8)
	return p
}

// hssSend sends a packet, again for as long as the host answers NAK.
func hssPut(code uint8, addr uint32, data []byte) {
	p := hssPacket(sendBuf[:], code, addr, data)
	for {
		line.putBytes(p)
		if c, _, _ := hssReceive(); c != hssNak {
			return
		}
	}
}

// hssAnswer sends an ACK or a NAK.
func hssAnswer(code uint8) {
	var b [12]byte
	line.putBytes(hssPacket(b[:], code, 0, nil))
}

// hssReceive waits for a packet from the host and returns it, answering it
// with an ACK, or with a NAK and waiting for it again if its sum is wrong.
// An ACK or NAK is returned as it is.
func hssReceive() (uint8, uint32, []byte) {
	for {
		p := recvBuf[:12]
		for i := 0; i < 12; {
			c := line.getc()
			if i < 3 && c != "HSs"[i] {
				i = 0
				if c == 'H' {
					p[0], i = c, 1
				}
				continue
			}
			p[i] = c
			i++
		}
		n := int(p[8]) | int(p[9])<<8
		if n > hssMax {
			continue
		}
		p = recvBuf[:12+n+n&1]
		for i := 12; i < len(p); i++ {
			p[i] = line.getc()
		}
		code := p[3]
		if code == hssAck || code == hssNak {
			return code, 0, nil
		}
		if hssSum(p) != 0xffff {
			hssAnswer(hssNak)
			continue
		}
		hssAnswer(hssAck)
		return code, getWord(p[4:]), p[12 : 12+n]
	}
}

// sendChar sends a character the program prints to the host, in a SEND
// packet of its own, as the boot disc's debugger does.
func sendChar(c byte) {
	var d [2]byte
	d[0] = c
	hssPut(hssSend, 0, d[:])
}
