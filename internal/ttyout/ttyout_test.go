package ttyout

import (
	"bytes"
	"testing"

	"go.chrisrx.dev/x/assert"
)

// openBIOSBoot is what OpenBIOS prints booting a disc, as it printed it,
// and then the start of what the program prints.
const openBIOSBoot = "PS-X Realtime Kernel OpenBios - build id unknown.\n" +
	"KERNEL SETUP!\n\nConfiguration : EvCB\t0x10\t\tTCB\t0x04\n\n" +
	"BOOTSTRAP LOADER\nsetup file    : cdrom:SYSTEM.CNF;1\n" +
	"boot file     : cdrom:\\MAIN.EXE;1\n" +
	"EXEC:PC0(80010000)  T_ADDR(80010000)  T_SIZE(00009000)\n" +
	"boot address  : 80010000 801ff000\nExecute !\n\n" +
	"                S_ADDR(801ff000)  S_SIZE(00000000)\n"

// sonyBoot is the same from Sony's BIOS, which differs in its banner.
const sonyBoot = "\nPS-X Realtime Kernel Ver.2.5\n" +
	"Copyright 1993,1994 (C) Sony Computer Entertainment Inc. \n" +
	"BOOTSTRAP LOADER Type C Ver 2.1   03-JUL-1994\n" +
	"EXEC:PC0(80010000)  T_ADDR(80010000)  T_SIZE(00009000)\n" +
	"boot address  : 80010000 801ff000\nExecute !\n\n" +
	"                S_ADDR(801ff000)  S_SIZE()\n"

// program is what a Go program on the Net Yaroze library prints first.
const program = "addr=8004d724\r\ndebug: start\r\r\nResetGraph:jtb=8004829c,env=800482e4\r\nPS-X Control PAD Driver\n"

const programPlain = "addr=8004d724\ndebug: start\nResetGraph:jtb=8004829c,env=800482e4\nPS-X Control PAD Driver\n"

func write(w *Writer, chunks ...string) {
	for _, c := range chunks {
		w.Write([]byte(c))
	}
}

func TestBootLeftOut(t *testing.T) {
	for name, boot := range map[string]string{"openbios": openBIOSBoot, "sony": sonyBoot} {
		var b bytes.Buffer
		write(New(&b, false), boot+program)
		assert.Equal(t, programPlain, b.String(), name)

		// A byte at a time, as the BIOS's putchar prints.
		b.Reset()
		w := New(&b, false)
		for _, c := range []byte(boot + program) {
			w.Write([]byte{c})
		}
		assert.Equal(t, programPlain, b.String(), name+", a byte at a time")
	}
}

func TestBootShown(t *testing.T) {
	var b bytes.Buffer
	write(New(&b, true), openBIOSBoot, program)
	assert.Equal(t, openBIOSBoot+programPlain, b.String())
}

func TestStart(t *testing.T) {
	// A program the emulator loads at the BIOS's shell: the BIOS printed
	// part of its log, and no S_ADDR line comes.
	var b bytes.Buffer
	w := New(&b, false)
	write(w, "PS-X Realtime Kernel OpenBios\nKERNEL SETUP!\nhalf a li")
	w.Start()
	write(w, program)
	assert.Equal(t, programPlain, b.String())
}

func TestLineEndings(t *testing.T) {
	for in, want := range map[string]string{
		"a\r\nb\n":      "a\nb\n",
		"a\r\r\nb":      "a\nb",
		"50%\r60%\r\n":  "50%\r60%\n",
		"a\r\r\rb":      "a\rb",
		"no ending":     "no ending",
		"\r\n\r\n":      "\n\n",
		"tab\tkept\r\n": "tab\tkept\n",
	} {
		var b bytes.Buffer
		write(Program(&b), in)
		assert.Equal(t, want, b.String(), in)
	}
	// A "\r" at the end of one write and the "\n" at the start of the next.
	var b bytes.Buffer
	write(Program(&b), "a\r", "\r", "\nb")
	assert.Equal(t, "a\nb", b.String())
}
