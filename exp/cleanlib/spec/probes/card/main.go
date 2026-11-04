//go:build yaroze

// Command card is a probe for the spec: what the memory card and file
// calls return, case by case, on whatever cards the harness puts in the
// slots. It tests both slots for a few fields, formats both, creates,
// writes, reads, seeks, lists, renames and deletes files on the card in
// slot 1, and does the same to files that are not there, printing each
// result to the TTY. Each case starts with a GP1 marker word (10000100h
// plus the case), so a trace of the kernel calls shows where each falls.
package main

import (
	"runtime/volatile"
	"unsafe"

	"go.chrisrx.dev/psx/yaroze"
)

func mark(n uint32) {
	volatile.StoreUint32((*uint32)(yaroze.Pointer(0x1f801814)), 0x10000100|n)
}

var (
	pathBuf  [32]byte
	pathBuf2 [32]byte
	buf      [3 * 128]byte
	in       [3 * 128]byte
	dir      yaroze.DIRENTRY
	step     uint32
)

// path makes a C string of s in b.
func path(b *[32]byte, s string) *byte {
	n := copy(b[:31], s)
	b[n] = 0
	return &b[0]
}

func next(what string) {
	step++
	mark(step)
	print("-- ", step, " ", what, "\n")
}

const hexDigits = "0123456789abcdef"

func hex(v uint32) {
	var s [8]byte
	for i := range 8 {
		s[i] = hexDigits[v>>(28-4*i)&15]
	}
	print(string(s[:]))
}

// sum prints a simple checksum of b, and its first bytes.
func sum(b []byte) {
	var s uint32
	for i, c := range b {
		s = s*31 + uint32(c) + uint32(i)
	}
	print(" sum ")
	hex(s)
	if len(b) >= 4 {
		print(" first ", b[0], " ", b[1], " ", b[2], " ", b[3])
	}
	print("\n")
}

func fill(seed byte) {
	for i := range buf {
		buf[i] = byte(i)*7 + seed
	}
}

func clearIn() {
	for i := range in {
		in[i] = 0xee
	}
}

func showDir(r *yaroze.DIRENTRY) {
	if r == nil {
		print(" nil\n")
		return
	}
	if r != &dir {
		print(" other pointer ")
		hex(uint32(uintptr(unsafe.Pointer(r))))
	}
	print(" name ")
	for i := range dir.Name {
		c := dir.Name[i]
		if c >= 0x20 && c < 0x7f {
			print(string(rune(c)))
		} else {
			print("\\", c)
		}
	}
	print(" attr ")
	hex(uint32(dir.Attr))
	print(" size ", dir.Size, " next ")
	hex(uint32(uintptr(unsafe.Pointer(dir.Next))))
	print(" head ")
	hex(uint32(dir.Head))
	print(" sys ")
	for _, b := range dir.System {
		print(b, " ")
	}
	print("\n")
}

func resetDir() {
	b := unsafe.Slice((*byte)(unsafe.Pointer(&dir)), unsafe.Sizeof(dir))
	for i := range b {
		b[i] = 0xaa
	}
}

func list(pattern string) {
	next("list " + pattern)
	resetDir()
	r := yaroze.Firstfile(path(&pathBuf, pattern), &dir)
	showDir(r)
	for i := 0; r != nil && i < 20; i++ {
		resetDir()
		r = yaroze.Nextfile(&dir)
		showDir(r)
	}
}

func test(n int) {
	next("test")
	for range n {
		print("test ", yaroze.TestCard(0), " ", yaroze.TestCard(1), "\n")
		yaroze.VSync(0)
	}
}

func open(name string, flags int32) int32 {
	fd := yaroze.Open(path(&pathBuf, name), flags)
	print("open ", name, " ")
	hex(uint32(flags))
	print(" = ", fd, "\n")
	return fd
}

func write(fd int32, n int32) {
	r := yaroze.Write(fd, &buf[0], n)
	print("write ", fd, " ", n, " = ", r, "\n")
}

func read(fd int32, n int32) {
	clearIn()
	r := yaroze.Read(fd, &in[0], n)
	print("read ", fd, " ", n, " = ", r)
	sum(in[:])
}

func seek(fd int32, off uint32, whence int32) {
	print("lseek ", fd, " ", off, " ", whence, " = ", yaroze.Lseek(fd, off, whence), "\n")
}

func closeFD(fd int32) {
	print("close ", fd, " = ", yaroze.Close(fd), "\n")
}

func main() {
	yaroze.ResetGraph(0)
	// A first test, unprinted, so that the card driver has started
	// before the first marker in both libraries: Sony's has the boot
	// start it, the clean one at its first card call. The harness
	// counts the frames a card goes in or out from that marker.
	yaroze.TestCard(0)
	print("card probe\n")
	test(6)

	next("format")
	print("format bu00: = ", yaroze.Format(path(&pathBuf, "bu00:")), "\n")
	print("format bu10: = ", yaroze.Format(path(&pathBuf, "bu10:")), "\n")
	print("format bu20: = ", yaroze.Format(path(&pathBuf, "bu20:")), "\n")
	test(4)

	list("bu00:*")

	next("create")
	fd := open("bu00:ALPHA", yaroze.O_CREAT|1<<16)
	closeFD(fd)
	fd = open("bu00:ALPHA", yaroze.O_CREAT|1<<16)
	closeFD(fd)
	fd = open("bu00:BETA", yaroze.O_CREAT|2<<16)
	closeFD(fd)
	fd = open("bu00:GAMMA", yaroze.O_CREAT)
	closeFD(fd)
	fd = open("bu00:NONE", yaroze.O_RDONLY)
	closeFD(fd)
	fd = open("bu10:ALPHA", yaroze.O_CREAT|1<<16)
	closeFD(fd)

	next("write")
	fill(1)
	fd = open("bu00:ALPHA", yaroze.O_WRONLY)
	write(fd, 128)
	write(fd, 256)
	write(fd, 100)
	write(fd, 0)
	seek(fd, 0, yaroze.SEEK_CUR)
	seek(fd, 1024, yaroze.SEEK_SET)
	fill(2)
	write(fd, 128)
	seek(fd, 100, yaroze.SEEK_SET)
	seek(fd, 128, yaroze.SEEK_CUR)
	seek(fd, 8192, yaroze.SEEK_SET)
	write(fd, 128)
	seek(fd, 8192-128, yaroze.SEEK_SET)
	fill(3)
	write(fd, 256)
	seek(fd, 3*8192, yaroze.SEEK_SET)
	seek(fd, 0, 2)
	read(fd, 128)
	closeFD(fd)
	write(fd, 128)
	closeFD(fd)
	closeFD(9)
	closeFD(-1)

	next("read")
	fd = open("bu00:ALPHA", yaroze.O_RDONLY)
	read(fd, 128)
	read(fd, 256)
	read(fd, 100)
	read(fd, 0)
	seek(fd, 1024, yaroze.SEEK_SET)
	read(fd, 128)
	seek(fd, 8192-128, yaroze.SEEK_SET)
	read(fd, 256)
	read(fd, 128)
	write(fd, 128)
	closeFD(fd)
	read(fd, 128)

	next("rdwr")
	fd = open("bu00:BETA", yaroze.O_RDWR)
	fill(4)
	write(fd, 384)
	seek(fd, 128, yaroze.SEEK_SET)
	read(fd, 256)
	seek(fd, 8192, yaroze.SEEK_SET)
	write(fd, 128)
	seek(fd, 8192, yaroze.SEEK_SET)
	read(fd, 128)
	closeFD(fd)

	next("two open")
	fd = open("bu00:ALPHA", yaroze.O_RDONLY)
	fd2 := open("bu00:BETA", yaroze.O_RDONLY)
	read(fd2, 128)
	read(fd, 128)
	closeFD(fd2)
	closeFD(fd)

	list("bu00:*")
	list("bu00:ALPHA")
	list("bu00:B*")
	list("bu00:?ETA")
	list("bu00:Z*")
	list("bu00:")
	list("bu10:*")

	next("rename")
	print("rename ALPHA DELTA = ", yaroze.Rename(path(&pathBuf, "bu00:ALPHA"), path(&pathBuf2, "bu00:DELTA")), "\n")
	print("rename NONE OMEGA = ", yaroze.Rename(path(&pathBuf, "bu00:NONE"), path(&pathBuf2, "bu00:OMEGA")), "\n")
	print("rename DELTA BETA = ", yaroze.Rename(path(&pathBuf, "bu00:DELTA"), path(&pathBuf2, "bu00:BETA")), "\n")
	print("rename BETA bu10:BETA = ", yaroze.Rename(path(&pathBuf, "bu00:BETA"), path(&pathBuf2, "bu10:BETA")), "\n")
	list("bu00:*")

	next("delete")
	print("delete BETA = ", yaroze.Delete(path(&pathBuf, "bu00:BETA")), "\n")
	print("delete BETA = ", yaroze.Delete(path(&pathBuf, "bu00:BETA")), "\n")
	print("delete NONE = ", yaroze.Delete(path(&pathBuf, "bu00:NONE")), "\n")
	print("delete bu10:ALPHA = ", yaroze.Delete(path(&pathBuf, "bu10:ALPHA")), "\n")
	list("bu00:*")
	fd = open("bu00:DELTA", yaroze.O_RDONLY)
	read(fd, 384)
	closeFD(fd)

	test(3)
	next("done")
	print("card probe done\n")
	for {
		yaroze.VSync(0)
	}
}
