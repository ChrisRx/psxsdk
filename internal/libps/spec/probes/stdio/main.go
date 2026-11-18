//go:build yaroze

// Command stdio is a probe for spec 18: putc against the library's putchar, over
// characters and files, with each result printed and the TTY and the kernel
// calls (.build/stdio-cimg/out.kernel) as the record of what each wrote.
package main

import (
	"runtime/volatile"
	"unsafe"

	"go.chrisrx.dev/psx/yaroze"
)

//export putc
func putc(c, fd int32) int32

//export putchar
func putchar(c int32) int32

var (
	count = (*uint32)(unsafe.Pointer(uintptr(0x1f801120)))
	mode  = (*uint32)(unsafe.Pointer(uintptr(0x1f801124)))
)

func main() {
	println("putc over characters, file 1")
	for _, c := range [...]int32{'A', '\n', '\r', 0, 0x7f, 0x80, 0xff, 0x100, 0x141, -1, -191} {
		print("[")
		r := putc(c, 1)
		println("] putc", c, "returns", r)
	}
	println("putchar over the same")
	for _, c := range [...]int32{'A', '\n', '\r', 0, 0x7f, 0x80, 0xff, 0x100, 0x141, -1, -191} {
		print("[")
		r := putchar(c)
		println("] putchar", c, "returns", r)
	}
	println("putc over files")
	for _, fd := range [...]int32{0, 1, 2, 3, 15, 16, 17, 100, -1} {
		print("[")
		r := putc('B', fd)
		println("] file", fd, "returns", r)
	}
	// What the kernel's write returns, for the same files and for a buffer of
	// more than one byte.
	println("write over files")
	for _, fd := range [...]int32{0, 1, 2, 3, 100, -1} {
		var b = [4]byte{'C', 'D', 'E', 'F'}
		print("[")
		r := yaroze.Write(fd, &b[0], 1)
		println("] write file", fd, "returns", r)
		print("[")
		r = yaroze.Write(fd, &b[0], 3)
		println("] write 3 bytes, file", fd, "returns", r)
	}
	// What each costs, from the second counter in system clock mode, the
	// least of a few runs. A write to the TTY is much of it.
	yaroze.VSync(0)
	for _, c := range [...]int32{'x', '\n'} {
		best := uint32(0xffffffff)
		for range 5 {
			volatile.StoreUint32(mode, 0)
			t0 := volatile.LoadUint32(count)
			putc(c, 1)
			t1 := volatile.LoadUint32(count)
			if t1-t0 < best {
				best = t1 - t0
			}
		}
		println("cost putc", c, best)
		best = 0xffffffff
		for range 5 {
			volatile.StoreUint32(mode, 0)
			t0 := volatile.LoadUint32(count)
			putchar(c)
			t1 := volatile.LoadUint32(count)
			if t1-t0 < best {
				best = t1 - t0
			}
		}
		println("cost putchar", c, best)
	}
	end()
}

// libps_biosB calls function fn of the kernel's B0 table.
//
//export libps_biosB
func libps_biosB(fn, a0, a1, a2, a3 uintptr) int32

// end prints the line that stops the run, through the kernel's putchar, since
// write would put a "\r" before the "\n" and the harness looks for "END\n".
func end() {
	for _, c := range [...]byte{'E', 'N', 'D', '\n'} {
		libps_biosB(0x3d, uintptr(c), 0, 0, 0)
	}
	for {
	}
}
