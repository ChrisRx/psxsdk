package main

import (
	"unsafe"

	"go.chrisrx.dev/psx/internal/volatile"
	"go.chrisrx.dev/psx/yaroze"
)

// The serial port's registers, SIO1, at 1F801050.
var (
	sioData = (*uint8)(yaroze.Pointer(0x1f801050))
	sioStat = (*uint32)(yaroze.Pointer(0x1f801054))
	sioMode = (*uint16)(yaroze.Pointer(0x1f801058))
	sioCtrl = (*uint16)(yaroze.Pointer(0x1f80105a))
	sioBaud = (*uint16)(yaroze.Pointer(0x1f80105e))
)

// SIO_STAT bits.
const (
	statTXReady = 1 << 0
	statRXReady = 1 << 1
	statDSR     = 1 << 7 // the host is there
	statCTS     = 1 << 8 // the host's RTS: it is ready for a byte
)

// SIO_CTRL bits.
const (
	ctrlTXEnable = 1 << 0
	ctrlDTR      = 1 << 1
	ctrlRXEnable = 1 << 2
	ctrlAck      = 1 << 4
	ctrlRTS      = 1 << 5
	ctrlReset    = 1 << 6
)

// modeLine is SIO_MODE for 8 data bits, no parity and a stop bit, with
// SIO_BAUD counting the clock divided by 16.
const modeLine = 0x4e

// sio is the serial port, as the monitor and the programs it runs use it.
// The port holds 8 bytes it has received, which the host can send in less
// time than the monitor takes to print a prompt, so whenever the monitor
// waits on the port it moves what has come into ring, and reads from
// there.
type sio struct {
	baud       uint32
	ring       [1024]byte
	head, tail uint32 // ring holds ring[head:tail], wrapped
}

var line sio

// open sets the port up at baud bits a second, with the console's RTS
// and DTR on, so that the host sends.
func (s *sio) open(baud uint32) {
	s.baud = baud
	volatile.StoreUint16(sioCtrl, ctrlReset)
	volatile.StoreUint16(sioMode, modeLine)
	volatile.StoreUint16(sioBaud, uint16(33868800/16/baud))
	volatile.StoreUint16(sioCtrl, ctrlTXEnable|ctrlDTR|ctrlRXEnable|ctrlAck|ctrlRTS)
}

func stat() uint32 { return volatile.LoadUint32(sioStat) }

// poll moves what the port has received into ring, and returns the port's
// status as it then is. A byte that would overfill ring is dropped.
func (s *sio) poll() uint32 {
	st := stat()
	for st&statRXReady != 0 {
		c := volatile.LoadUint8(sioData)
		if s.tail-s.head < uint32(len(s.ring)) {
			s.ring[s.tail%uint32(len(s.ring))] = c
			s.tail++
		}
		st = stat()
	}
	return st
}

// putc sends a byte once the port can take it and, while the host is
// there, it is ready for it. With no host, the byte goes nowhere rather
// than holding up a program that prints.
func (s *sio) putc(c byte) {
	for s.poll()&statTXReady == 0 {
	}
	if s.poll()&statDSR != 0 {
		for s.poll()&statCTS == 0 {
		}
	}
	volatile.StoreUint8(sioData, c)
}

// putAcked sends a byte of a binary read, then waits for the host to turn
// its RTS off and on again, which is how it says it has taken the byte.
// The host's RTS is off for a few microseconds, so the wait is a bare loop
// on the status: one that moved received bytes to ring as well could miss
// it, and the host sends nothing while it reads a block anyway.
func (s *sio) putAcked(c byte) {
	s.putc(c)
	for stat()&statCTS != 0 {
	}
	for stat()&statCTS == 0 {
	}
}

// putBytes sends b. The monitor writes out bytes it has with this rather
// than making them a string, which would take memory its heap, never
// collected, does not give back.
func (s *sio) putBytes(b []byte) {
	for _, c := range b {
		s.putc(c)
	}
}

func (s *sio) puts(str string) {
	for i := range len(str) {
		s.putc(str[i])
	}
}

// getc waits for a byte from the host.
func (s *sio) getc() byte {
	for s.head == s.tail {
		s.poll()
	}
	c := s.ring[s.head%uint32(len(s.ring))]
	s.head++
	return c
}

// The kernel's output functions while the monitor's hook is in (see
// installHook): what a program prints goes down the cable as it is. The
// library's putchar has already ended each line with a carriage return and
// a line feed.

func outPutchar(c int32) int32 {
	out(byte(c))
	return c
}

func outPutc(c, fd int32) int32 {
	out(byte(c))
	return c
}

func outPuts(s *byte) int32 {
	for p := s; *p != 0; p = (*byte)(unsafe.Add(unsafe.Pointer(p), 1)) {
		out(*p)
	}
	return 0
}

func outWrite(fd int32, src *byte, n int32) int32 {
	for _, c := range unsafe.Slice(src, n) {
		out(c)
	}
	return n
}

func out(c byte) {
	if debugging {
		sendChar(c)
		return
	}
	line.putc(c)
}

// digits is where numbers are written out, so that writing one allocates
// nothing: the monitor's heap is never collected.
var digits [10]byte

// putHex sends v as eight hexadecimal digits.
func (s *sio) putHex(v uint32) {
	for i := 7; i >= 0; i-- {
		digits[i] = "0123456789abcdef"[v&15]
		v >>= 4
	}
	for _, c := range digits[:8] {
		s.putc(c)
	}
}

// putDecimal sends v in decimal.
func (s *sio) putDecimal(v uint32) {
	i := len(digits)
	for {
		i--
		digits[i] = byte('0' + v%10)
		v /= 10
		if v == 0 {
			break
		}
	}
	for _, c := range digits[i:] {
		s.putc(c)
	}
}
