// Command monitor is the console's end of a Net Yaroze serial cable: a
// monitor that takes commands from the host a line at a time, uploads into
// RAM and reads RAM back in binary transfers, starts the program it was
// given, and sends what that program prints down the cable.
//
// It stands in for the monitor on Sony's boot disc, and was written from
// what that one is seen to do, not from its code: the transfer protocol in
// the readme of Sony's SIOCONS, the commands in the Net Yaroze User Guide,
// and the replies of the real monitor running in the emulator, which the
// tests in the tests package check both against. It has the commands
// package serial and SIOCONS use, help, dr, sr, go, bwr, brd and baud, and
// ds; the memory commands dw, dh, db, sw, sh, sb, fw, fh and fb; the file
// commands dir, cd, re, wr, ld, ex, ren, del and format; the hardware
// commands war, waw, var, vaw, cb and play, and the continuous dbc, dhc,
// dwc, sbc, shc and swc; the disassembler, di and dis; the user commands,
// sc, ac and dc; the device drivers, ad and dd; baud with a new speed;
// cls; and rdb, the remote debugger, in the HSS protocol of the Net
// Yaroze's gdb, written from that gdb's source.
//
// It lives where the boot disc's monitor does, between the resident library
// and the program, at 80068000, which target.json links it at, with its
// heap below 80090000, and goes onto a disc with the library:
//
//	yaroze disc --target serial/monitor/target.json -o monitor.bin ./serial/monitor
//
// target.json also links it with --nmagic, without the segments' page
// alignment, which would otherwise load the ELF headers into the library
// below it.
//
// The line runs at the speed the memory card in either slot gives in the
// file the boot disc keeps it in, BWDTLS-03030CIP, and 9600 bps without
// one.
package main

import "go.chrisrx.dev/psx/yaroze"

// The file the line's speed is kept in, and where in it.
const (
	cipFile     = "BWDTLS-03030CIP"
	cipBaudAt   = 0x22c
	defaultBaud = 9600
)

// cip is where the file is read into: as many of its first sectors as
// hold the speed.
var cip [(cipBaudAt + 4 + yaroze.CardSector - 1) / yaroze.CardSector * yaroze.CardSector]byte

// savedBaud is the line's speed once the card has been read, kept for when
// the monitor starts again after a program returns. It starts as a value
// so that it is in .data, which the monitor's startup leaves alone, rather
// than .bss, which it clears.
var savedBaud = ^uint32(0)

func main() {
	if savedBaud == ^uint32(0) {
		savedBaud = cardBaud()
	}
	line.open(savedBaud)
	installHook()
	line.puts("\r\nConnected CIP ver2.0\r\nCommunication baud rate ")
	line.putDecimal(savedBaud)
	line.puts(" bps\r\nOK\r\n")
	for {
		prompt()
		command(readLine(commandPrompt))
	}
}

// cardBaud reads the line's speed from the card in either slot, or
// returns defaultBaud.
func cardBaud() uint32 {
	for slot := range 2 {
		if yaroze.CardRead(slot, cipFile, cip[:]) >= cipBaudAt+4 {
			b := uint32(cip[cipBaudAt]) | uint32(cip[cipBaudAt+1])<<8 | uint32(cip[cipBaudAt+2])<<16 | uint32(cip[cipBaudAt+3])<<24
			if b != 0 {
				return b
			}
		}
	}
	return defaultBaud
}

// prompt is the monitor's: the cursor back over a cleared line, then ">>".
func prompt() {
	line.puts("\r                                                    \r>>")
}

// input is the line being typed.
var input [256]byte

// recent is the last lines typed, which Control-B brings back, the most
// recent first: kept in recent[recentN%len(recent)] on back, recentN in
// all.
var (
	recent    [10][len(input)]byte
	recentLen [len(recent)]int
	recentN   int
)

// readLine reads a line from the host at a prompt, echoing it as it comes,
// and returns it once a carriage return ends it, which it echoes as a new
// line. A backspace takes back a character, Control-B brings back the
// lines typed before, one further back at each press, redrawing the line
// after the prompt, and Control-C ends the line with nothing on it.
func readLine(prompt []byte) []byte {
	n := 0
	back := 0
	for {
		c := line.getc()
		switch {
		case c == '\r':
			line.puts("\r\n")
			if n > 0 {
				i := recentN % len(recent)
				copy(recent[i][:], input[:n])
				recentLen[i] = n
				recentN++
			}
			return input[:n]
		case c == 0x03:
			line.puts("\r\n")
			return input[:0]
		case c == 0x02:
			if back < min(recentN, len(recent)) {
				back++
				i := (recentN - back) % len(recent)
				n = copy(input[:], recent[i][:recentLen[i]])
				line.puts("\r                                                           \r")
				line.putBytes(prompt)
				for _, c := range input[:n] {
					line.putc(c)
				}
			}
		case c == '\b':
			if n > 0 {
				n--
				line.puts("\b \b")
			}
		case c == '\n':
		case n < len(input):
			input[n] = c
			n++
			line.putc(c)
		}
	}
}
