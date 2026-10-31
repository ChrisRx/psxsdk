package siocons

import (
	"io"
	"time"
)

// Key is a key pressed on the host's keyboard: a byte as typed, below
// 100h, or one of the keys below, which SIOCONS's own key codes order.
type Key int

// The keys that are not bytes.
const (
	KeyUp Key = 0x100 + iota
	KeyDown
	KeyLeft
	KeyRight
	KeyInsert
	KeyDelete
	KeyHome
	KeyEnd
	KeyPageUp
	KeyPageDown
)

const (
	KeyF1 Key = 0x110 + iota
	KeyF2
	KeyF3
	KeyF4
	KeyF5
	KeyF6
	KeyF7
	KeyF8
	KeyF9
	KeyF10
	KeyF11
	KeyF12
)

// Control characters that mean something to SIOCONS.
const (
	keyCtrlC  Key = 0x03
	keyEscape Key = 0x1b
)

// function reports whether k is a function or cursor key rather than a
// byte, which SIOCONS handles itself rather than sending to the console.
func (k Key) function() bool { return k >= 0x100 }

// escWait is how long an escape waits for the rest of a sequence before it
// is taken as the Escape key on its own.
const escWait = 50 * time.Millisecond

// readKeys reads the terminal's bytes from r and sends them to keys,
// decoding the escape sequences xterm and the Linux console send for the
// cursor and function keys. It closes keys when r ends.
func readKeys(r io.Reader, keys chan<- Key) {
	bytes := make(chan byte, 64)
	go func() {
		defer close(bytes)
		var buf [64]byte
		for {
			n, err := r.Read(buf[:])
			for _, b := range buf[:n] {
				bytes <- b
			}
			if err != nil {
				return
			}
		}
	}()
	defer close(keys)
	for b := range bytes {
		if b != 0x1b {
			keys <- Key(b)
		} else if !escape(bytes, keys) {
			return
		}
	}
}

// escape reads the rest of an escape sequence from bytes and sends the key
// it is to keys. An escape with nothing after it in time is the Escape key,
// and whatever comes after it, as typed. It reports whether bytes is still
// open.
func escape(bytes <-chan byte, keys chan<- Key) bool {
	seq := []byte{0x1b}
	for {
		select {
		case c, ok := <-bytes:
			if !ok {
				keys <- keyEscape
				return false
			}
			seq = append(seq, c)
			if k, done := escapeSequence(seq); done {
				if k != 0 {
					keys <- k
				}
				return true
			}
		case <-time.After(escWait):
			keys <- keyEscape
			for _, c := range seq[1:] {
				keys <- Key(c)
			}
			return true
		}
	}
}

// escapeSequence decodes seq, an escape and what has come after it so far,
// and reports whether it is complete, with the key it is, or 0 if it is
// complete but not a key SIOCONS knows.
func escapeSequence(seq []byte) (Key, bool) {
	if len(seq) < 2 {
		return 0, false
	}
	switch seq[1] {
	case 'O': // xterm's F1 to F4, and the cursor keys in application mode
		if len(seq) < 3 {
			return 0, false
		}
		switch seq[2] {
		case 'P', 'Q', 'R', 'S':
			return KeyF1 + Key(seq[2]-'P'), true
		}
		return cursor(seq[2]), true
	case '[':
	default:
		return 0, true
	}
	if len(seq) < 3 {
		return 0, false
	}
	if seq[2] == '[' { // the Linux console's F1 to F5
		if len(seq) < 4 {
			return 0, false
		}
		if seq[3] >= 'A' && seq[3] <= 'E' {
			return KeyF1 + Key(seq[3]-'A'), true
		}
		return 0, true
	}
	last := seq[len(seq)-1]
	if last < 0x40 || last > 0x7e {
		return 0, false // parameters still coming
	}
	if last != '~' {
		return cursor(last), true
	}
	n := 0
	for _, c := range seq[2 : len(seq)-1] {
		if c < '0' || c > '9' {
			break // a modifier after ';'
		}
		n = n*10 + int(c-'0')
	}
	switch {
	case n == 1 || n == 7:
		return KeyHome, true
	case n == 2:
		return KeyInsert, true
	case n == 3:
		return KeyDelete, true
	case n == 4 || n == 8:
		return KeyEnd, true
	case n == 5:
		return KeyPageUp, true
	case n == 6:
		return KeyPageDown, true
	case n >= 11 && n <= 15:
		return KeyF1 + Key(n-11), true
	case n >= 17 && n <= 21:
		return KeyF6 + Key(n-17), true
	case n == 23 || n == 24:
		return KeyF11 + Key(n-23), true
	}
	return 0, true
}

// cursor is the key a sequence's final letter names, or 0.
func cursor(c byte) Key {
	switch c {
	case 'A':
		return KeyUp
	case 'B':
		return KeyDown
	case 'C':
		return KeyRight
	case 'D':
		return KeyLeft
	case 'H':
		return KeyHome
	case 'F':
		return KeyEnd
	}
	return 0
}
