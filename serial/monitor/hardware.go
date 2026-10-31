package main

import (
	"go.chrisrx.dev/psx/internal/volatile"
	"go.chrisrx.dev/psx/yaroze"
)

// The hardware commands: war and waw, between RAM and sound memory; var
// and vaw, between RAM and the frame buffer; cb, the color bars; play, a
// CD audio track; and the continuous dumps and sets, dwc, dhc, dbc, swc,
// shc and sbc, which read or write one address over and over, for a
// hardware register.

// The SPU's transfer registers, and DMA channel 4's, which moves the data.
var (
	spuTransferAddr = (*uint16)(yaroze.Pointer(0x1f801da6))
	spuControl      = (*uint16)(yaroze.Pointer(0x1f801daa))
	spuTransferCtrl = (*uint16)(yaroze.Pointer(0x1f801dac))
	spuStatus       = (*uint16)(yaroze.Pointer(0x1f801dae))
	dmaSPUAddr      = (*uint32)(yaroze.Pointer(0x1f8010c0))
	dmaSPUBlock     = (*uint32)(yaroze.Pointer(0x1f8010c4))
	dmaSPUCtrl      = (*uint32)(yaroze.Pointer(0x1f8010c8))
	dmaControl      = (*uint32)(yaroze.Pointer(0x1f8010f0))
)

// SPUCNT's transfer modes.
const (
	spuTransferStop     = 0 << 4
	spuTransferDMAWrite = 2 << 4
	spuTransferDMARead  = 3 << 4
	spuTransferMask     = 3 << 4
)

// spuTransfer moves length bytes between RAM at addr and sound memory at
// sbaddr by DMA, into sound memory if write, in 64-byte blocks.
func spuTransfer(addr, sbaddr, length uint32, write bool) {
	mode, ctrl := uint16(spuTransferDMARead), uint32(0x01000200)
	if write {
		mode, ctrl = spuTransferDMAWrite, 0x01000201
	}
	setSPUMode(spuTransferStop)
	volatile.StoreUint16(spuTransferCtrl, 0x0004)
	volatile.StoreUint16(spuTransferAddr, uint16(sbaddr/8))
	setSPUMode(mode)
	volatile.StoreUint32(dmaControl, volatile.LoadUint32(dmaControl)|0x8<<16) // channel 4 on
	volatile.StoreUint32(dmaSPUAddr, addr&0xffffff)
	volatile.StoreUint32(dmaSPUBlock, (length+63)/64<<16|16)
	volatile.StoreUint32(dmaSPUCtrl, ctrl)
	for volatile.LoadUint32(dmaSPUCtrl)&0x01000000 != 0 {
	}
	for volatile.LoadUint16(spuStatus)&0x400 != 0 {
	}
	setSPUMode(spuTransferStop)
}

// setSPUMode sets SPUCNT's transfer mode and waits for SPUSTAT to show it.
func setSPUMode(mode uint16) {
	volatile.StoreUint16(spuControl, volatile.LoadUint16(spuControl)&^spuTransferMask|mode)
	for volatile.LoadUint16(spuStatus)&spuTransferMask != mode {
	}
}

// waveTransfer is war, into sound memory, and waw, out of it.
func waveTransfer(args [][]byte, write bool) {
	if len(args) < 3 {
		line.puts("usage: <command> <address> <sbaddr> <length>\r\n")
		return
	}
	n := hex(args[2])
	spuTransfer(hex(args[0]), hex(args[1]), n, write)
	line.putDecimal(n)
	line.putc('(')
	putHexN(n, 5)
	if write {
		line.puts("H) bytes transfer\r\n")
	} else {
		line.puts("H) bytes received\r\n")
	}
}

// The frame buffer's rectangle and the RAM address var and vaw last used,
// which either goes on using until told otherwise.
var (
	frameRect yaroze.RECT
	frameAddr uint32
)

// frameTransfer is var, into the frame buffer, and vaw, out of it: -s
// with the rectangle, x,y,w,h in decimal, and -a with the address.
func frameTransfer(args [][]byte, write bool) {
	if len(args) == 0 {
		line.puts("usage: <command> [-s< x,y,w,h >] [-a<address>]\r\n")
		return
	}
	for _, a := range args {
		switch {
		case len(a) > 2 && is(a[:2], "-s"):
			var v [4]int16
			i := 0
			for _, c := range a[2:] {
				switch {
				case c == ',' && i < 3:
					i++
				case '0' <= c && c <= '9':
					v[i] = v[i]*10 + int16(c-'0')
				}
			}
			frameRect = yaroze.RECT{X: v[0], Y: v[1], W: v[2], H: v[3]}
		case len(a) > 2 && is(a[:2], "-a"):
			frameAddr = hex(a[2:])
		}
	}
	p := (*uint32)(yaroze.Pointer(uintptr(frameAddr)))
	if write {
		yaroze.LoadImage(&frameRect, p)
	} else {
		yaroze.StoreImage(&frameRect, p)
	}
	yaroze.DrawSync(0)
}

// bars are cb's colors, left to right.
var bars = [8][3]uint8{
	{255, 255, 255}, {255, 255, 0}, {0, 255, 255}, {0, 255, 0},
	{255, 0, 255}, {255, 0, 0}, {0, 0, 255}, {0, 0, 0},
}

// colorBar is cb: eight bars of color across a 256 by 240 screen.
func colorBar() {
	env := yaroze.DISPENV{Disp: yaroze.RECT{W: 256, H: 240}, Screen: yaroze.RECT{W: 256, H: 240}}
	yaroze.PutDispEnv(&env)
	for i, c := range bars {
		r := yaroze.RECT{X: int16(i * 32), W: 32, H: 240}
		yaroze.ClearImage(&r, c[0], c[1], c[2])
	}
	yaroze.DrawSync(0)
	yaroze.SetDispMask(1)
}

// The drive's commands play uses.
const (
	cdlPlay    = 0x03
	cdlStop    = 0x08
	cdlSetmode = 0x0e
	cdlGetlocP = 0x11
	cdlGetTN   = 0x13
	cdlGetTD   = 0x14
)

// The drive's registers, and the interrupt controller's, which play talks
// to the drive through directly: the resident library has no call for the
// table of contents or the position, and its own command call does not
// hand the drive's reply back.
var (
	cdIndex  = (*uint8)(yaroze.Pointer(0x1f801800))
	cdReg1   = (*uint8)(yaroze.Pointer(0x1f801801))
	cdReg2   = (*uint8)(yaroze.Pointer(0x1f801802))
	cdReg3   = (*uint8)(yaroze.Pointer(0x1f801803))
	irqStat  = (*uint32)(yaroze.Pointer(0x1f801070))
	irqMask  = (*uint32)(yaroze.Pointer(0x1f801074))
	cdResult [8]uint8
)

const irqCD = 1 << 2

// cdCommand gives the drive a command and waits for its reply, in
// cdResult, and for a command that finishes later, such as Stop, for its
// second, with the drive's interrupt masked, so that the library's handler
// leaves the replies alone. It reports false if the drive answered with
// an error.
func cdCommand(com uint8, second bool, params ...uint8) bool {
	mask := volatile.LoadUint32(irqMask)
	volatile.StoreUint32(irqMask, mask&^irqCD)
	for volatile.LoadUint8(cdIndex)&0x80 != 0 { // busy
	}
	volatile.StoreUint8(cdIndex, 1)
	volatile.StoreUint8(cdReg3, 0x1f) // acknowledge anything waiting
	volatile.StoreUint8(cdIndex, 0)
	for _, p := range params {
		volatile.StoreUint8(cdReg2, p)
	}
	volatile.StoreUint8(cdReg1, com)
	ok := cdReply()
	if ok && second {
		ok = cdReply()
	}
	volatile.StoreUint32(irqStat, ^uint32(irqCD))
	volatile.StoreUint32(irqMask, mask)
	return ok
}

// cdReply waits for one of the drive's replies, reads it into cdResult and
// acknowledges it.
func cdReply() bool {
	volatile.StoreUint8(cdIndex, 1)
	var kind uint8
	for kind == 0 {
		kind = volatile.LoadUint8(cdReg3) & 7
	}
	for i := range cdResult {
		cdResult[i] = 0
		if volatile.LoadUint8(cdIndex)&0x20 != 0 { // a byte to read
			cdResult[i] = volatile.LoadUint8(cdReg1)
		}
	}
	volatile.StoreUint8(cdReg3, 0x07)
	volatile.StoreUint8(cdIndex, 0)
	return kind != 5
}

func fromBCD(b uint8) uint32 { return uint32(b>>4)*10 + uint32(b&15) }
func toBCD(n uint32) uint8   { return uint8(n/10<<4 | n%10) }

func put2(n uint32) {
	line.putc(byte('0' + n/10%10))
	line.putc(byte('0' + n%10))
}

// play is play: a track of CD audio, the first if none is given, with the
// time it ends at, then how far into it it is, until a key is pressed,
// which goes no further.
func play(args [][]byte) {
	track := uint32(1)
	if len(args) > 0 {
		track = decimal(args[0])
	}
	cdCommand(cdlGetTN, false)
	last := fromBCD(cdResult[2])
	if track == 0 || track > last {
		line.puts("Invalid parameter\r\n")
		return
	}
	// Where the track ends: where the next starts, or the lead-out.
	end := uint8(0)
	if track < last {
		end = toBCD(track + 1)
	}
	cdCommand(cdlGetTD, false, end)
	line.puts("track=")
	line.putDecimal(track)
	line.putc('/')
	line.putDecimal(last)
	line.puts(" [")
	put2(fromBCD(cdResult[1]))
	line.putc(':')
	put2(fromBCD(cdResult[2]))
	line.puts("]\r\n")

	cdCommand(cdlSetmode, false, 0x01) // CD-DA
	cdCommand(cdlPlay, false, toBCD(track))
	for {
		line.poll()
		if line.head != line.tail {
			line.getc()
			break
		}
		cdCommand(cdlGetlocP, false)
		line.puts("\r ")
		put2(fromBCD(cdResult[2]))
		line.putc(':')
		put2(fromBCD(cdResult[3]))
		yaroze.VSync(8)
	}
	cdCommand(cdlStop, true)
}

// dumpContinuously is dwc, dhc or dbc: the item at the address read again
// and again, as many times as the count says, or until Control-C.
func dumpContinuously(size uint32, args [][]byte) {
	if len(args) == 0 {
		line.puts("?\r\n")
		return
	}
	addr := hex(args[0]) &^ (size - 1)
	n := ^uint32(0)
	if len(args) > 1 {
		n = hex(args[1])
	}
	perLine := 16 / size
	gap := "  "
	if size == 1 {
		gap = " "
	}
	for i := uint32(0); i < n; i++ {
		if i%perLine == 0 {
			if stopped() {
				line.puts("\r\n")
				return
			}
			line.putHex(addr)
			line.putc(':')
		}
		putHexN(load(addr, size), int(size*2))
		line.puts(gap)
		if i%perLine == perLine-1 || i == n-1 {
			line.puts("\r\n")
		}
	}
}

// stopped reports whether Control-C has come from the host, dropping
// anything else that has.
func stopped() bool {
	line.poll()
	for line.head != line.tail {
		if line.getc() == 0x03 {
			return true
		}
	}
	return false
}

// setContinuously is swc, shc or sbc: values typed stored at one address,
// which does not move on, until a dot. An empty line is asked again.
func setContinuously(size uint32, args [][]byte) {
	if len(args) == 0 {
		line.puts("?\r\n")
		return
	}
	addr := hex(args[0]) &^ (size - 1)
	var p [9]byte
	for i := 7; i >= 0; i-- {
		p[i] = "0123456789abcdef"[addr>>(4*(7-i))&15]
	}
	p[8] = ':'
	for {
		line.putBytes(p[:])
		words := split(readLine(p[:]))
		switch {
		case len(words) == 0:
			line.puts(" ??\r\n")
		case equal(words[0], "."):
			return
		case isHex(words[0]):
			store(addr, size, hex(words[0]))
		default:
			line.puts(" ?")
			line.putBytes(words[0])
			line.puts("?\r\n")
		}
	}
}
