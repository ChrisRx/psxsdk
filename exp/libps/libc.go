//go:build yaroze

package libps

import "unsafe"

// ptr is the address of p, for the kernel calls.
func ptr(p *byte) uintptr { return uintptr(unsafe.Pointer(p)) }

// column is where the cursor is on the TTY's line, which putchar keeps
// for expanding tabs.
//
// libps 0x80062CE0
var column int32

// putchar writes c to the TTY through the kernel. A tab is written as
// spaces to the next column that is a multiple of eight, a newline is
// preceded by a carriage return and starts the column over, and any other
// printable character advances it. The emulator captures what the kernel
// writes as the program's output.
//
// libps 0x80043790, 180 bytes
//
//export putchar
func putchar(c int32) int32 {
	b := byte(c)
	switch b {
	case '\t':
		for {
			putchar(' ')
			if column&7 == 0 {
				break
			}
		}
		return c
	case '\n':
		putchar('\r')
		column = 0
	default:
		if printable(b) {
			column++
		}
	}
	fileWrite(1, &b, 1)
	return c
}

// printable is the original's test, through its ctype table masked with
// 0x97: letters, digits, punctuation and the space class, which takes in
// the carriage return, vertical tab and form feed too.
func printable(c byte) bool {
	return c >= 0x20 && c < 0x7f || c == '\r' || c == '\v' || c == '\f'
}
