// Package ttyout tidies what a machine prints for a terminal: the BIOS's
// boot log left out, and line endings made plain.
//
// Before a program runs, the BIOS prints a log of its start meant for a
// developer's serial console: the kernel's configuration, SYSTEM.CNF, the
// executable it loads. Both Sony's BIOS and OpenBIOS end it the same way,
// before they jump to the program:
//
//	Execute !
//
//	                S_ADDR(801ff000)  S_SIZE(00000000)
//
// A Writer drops what comes before that line, and the line, unless told
// to show the log. A program the emulator loads itself, not through the
// BIOS, prints no such line, so Start says it has started.
//
// Programs end lines with "\r\n", and the Net Yaroze library's putchar
// adds a "\r" before each "\n" it is given too, so a Go program's println
// ends with "\r\r\n", while the BIOS ends its own with "\n". A Writer
// passes on each as "\n", which a terminal shows as a new line and a file
// or a pipe keeps without a stray carriage return.
package ttyout

import (
	"bytes"
	"io"
	"sync"
)

// bootEnd is in the last line of the BIOS's boot log.
var bootEnd = []byte("S_ADDR(")

// A Writer passes on what a machine prints to W.
type Writer struct {
	w        io.Writer
	showBoot bool

	mu      sync.Mutex
	started bool
	line    []byte // the boot log's line so far, before the program starts
	cr      int    // carriage returns held back, which a "\n" may follow
	out     []byte
}

// New returns a Writer for a machine that has not started its program,
// which leaves out the BIOS's boot log unless showBoot is set.
func New(w io.Writer, showBoot bool) *Writer {
	return &Writer{w: w, showBoot: showBoot}
}

// Program returns a Writer for a program that is already running: it only
// makes the line endings plain.
func Program(w io.Writer) *Writer {
	return &Writer{w: w, started: true}
}

// Start says the program has started, for one loaded without the BIOS.
// What the BIOS printed is left out from here on, and the program's text
// goes through.
func (t *Writer) Start() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.started = true
	t.line = t.line[:0]
}

func (t *Writer) Write(p []byte) (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.out = t.out[:0]
	for _, c := range p {
		if !t.started {
			t.boot(c)
			continue
		}
		t.put(c)
	}
	if len(t.out) == 0 {
		return len(p), nil
	}
	if _, err := t.w.Write(t.out); err != nil {
		return 0, err
	}
	return len(p), nil
}

// boot takes a byte of the boot log, a line at a time: shown if asked for,
// and the program started after the line that ends the log.
func (t *Writer) boot(c byte) {
	if t.showBoot {
		t.put(c)
	}
	if c != '\n' {
		t.line = append(t.line, c)
		return
	}
	if bytes.Contains(t.line, bootEnd) {
		t.started = true
	}
	t.line = t.line[:0]
}

// put passes on a byte with the line endings made plain: carriage returns
// before a "\n" are dropped, and any other run of them is one.
func (t *Writer) put(c byte) {
	switch c {
	case '\r':
		t.cr++
		return
	case '\n':
		t.cr = 0
	default:
		if t.cr > 0 {
			t.out = append(t.out, '\r')
			t.cr = 0
		}
	}
	t.out = append(t.out, c)
}
