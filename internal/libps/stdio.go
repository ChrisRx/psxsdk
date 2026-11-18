//go:build yaroze

package libps

import (
	"unsafe"

	"go.chrisrx.dev/psx/internal/volatile"
)

// The C library's character input and output, through the kernel's read and
// write (spec 18). Sony's keep the byte on the stack; a local whose address goes
// to the kernel would go on the heap here, so it is ioChar, and a read that
// writes nothing leaves the last byte rather than a stale one from the stack,
// which no program can rely on.
var ioChar byte

// putc writes the low byte of c to file fd, once, with no expansion of a
// newline, and returns what the write returned.
//
//export putc
func libPutc(c int32, fd uintptr) int32 {
	volatile.StoreUint8(&ioChar, byte(c))
	return libps_biosB(biosWrite, fd, uintptr(unsafe.Pointer(&ioChar)), 1, 0)
}

// ioRead reads a byte of file fd into ioChar and returns what the read returned.
func ioRead(fd uintptr) int32 {
	return libps_biosB(biosRead, fd, uintptr(unsafe.Pointer(&ioChar)), 1, 0)
}

// getc returns a byte of file fd, or 255 if the read gave none.
//
//export getc
func libGetc(fd uintptr) int32 {
	if ioRead(fd) <= 0 {
		return 255
	}
	return int32(volatile.LoadUint8(&ioChar))
}

// getchar reads a byte of file 0, whatever the read says of it, and returns it
// with bit 7 cleared. There is no echo.
//
//export getchar
func libGetchar() int32 {
	ioRead(0)
	return int32(volatile.LoadUint8(&ioChar) & 0x7f)
}

// getsMax is the most bytes gets stores, so a buffer needs one more.
const getsMax = 125

// gets reads a line into buf, echoing what it stores through putchar, ringing for
// what it does not, and taking back a byte for a backspace.
// 16h stores the next byte, whatever it is.
//
//export gets
//go:nobounds
func libGets(buf uintptr) uintptr {
	b := cBytes(buf)
	n := 0
	for {
		c := byte(libGetchar())
		quoted := false
		if c == 0x16 {
			c = byte(libGetchar())
			quoted = true
		}
		if !quoted {
			switch c {
			case '\n', '\r':
				putchar('\n')
				b[n] = 0
				return buf
			case 8, 0x7f:
				if n > 0 {
					n--
					putchar(8)
					putchar(' ')
					putchar(8)
				}
				continue
			case '\t':
				c = ' '
			}
		}
		if !quoted && (c < ' ' || c > 0x7e) || n == getsMax {
			putchar(7)
			continue
		}
		b[n] = c
		n++
		putchar(int32(c))
	}
}
