//go:build yaroze

// Command cfntclose is a probe for spec 17: KanjiFntPrint and FntPrint around a
// stream's closing. cfntk saw KanjiFntPrint go on counting after KanjiFntClose,
// where spec 13 has -1 with no stream open. Each line is a call and what it returned.
package main

import (
	"unsafe"

	"go.chrisrx.dev/psx/yaroze"
)

//export KanjiFntPrint
func kprint(a0, a1, a2, a3 uint32) int32

//export FntPrint
func fprint(a0, a1, a2, a3 uint32) int32

var (
	a   = [...]byte{'A', 0}
	bc  = [...]byte{'B', 'C', 0}
	fmt = [...]byte{'%', 'd', 0}
)

func addr(b *byte) uint32 { return uint32(uintptr(unsafe.Pointer(b))) }

func kopen(n int32) int32 {
	r := yaroze.KanjiFntOpen(0, 0, 320, 240, 640, 0, 0, 480, 1, n)
	yaroze.DrawSync(0)
	return r
}

func flush(id int32) bool {
	p := yaroze.KanjiFntFlush(id)
	yaroze.DrawSync(0)
	return p != nil
}

func main() {
	yaroze.ResetGraph(0)
	println("never opened, literal", kprint(addr(&a[0]), 0, 0, 0))
	println("never opened, id 0", kprint(0, addr(&a[0]), 0, 0))
	println("never opened, id 1", kprint(1, addr(&a[0]), 0, 0))
	println("never opened, flush", flush(0))
	yaroze.KanjiFntClose()
	println("closed unopened, literal", kprint(addr(&a[0]), 0, 0, 0))
	println("open", kopen(16))
	println("print", kprint(addr(&a[0]), 0, 0, 0))
	yaroze.KanjiFntClose()
	println("closed, literal", kprint(addr(&a[0]), 0, 0, 0))
	println("closed, two more", kprint(addr(&bc[0]), 0, 0, 0))
	println("closed, id 0", kprint(0, addr(&a[0]), 0, 0))
	println("closed, id 1", kprint(1, addr(&a[0]), 0, 0))
	println("closed, %d", kprint(addr(&fmt[0]), 42, 0, 0))
	println("closed, flush", flush(0))
	println("flushed, literal", kprint(addr(&a[0]), 0, 0, 0))
	println("open again", kopen(16))
	println("print", kprint(addr(&a[0]), 0, 0, 0))
	yaroze.KanjiFntClose()
	println("closed, literal", kprint(addr(&a[0]), 0, 0, 0))
	println("open two", kopen(4), kopen(4))
	println("print 1", kprint(1, addr(&bc[0]), 0, 0))
	yaroze.KanjiFntClose()
	println("closed, id 1", kprint(1, addr(&bc[0]), 0, 0))
	println("closed, literal", kprint(addr(&a[0]), 0, 0, 0))
	println("open small", kopen(1))
	println("print", kprint(addr(&bc[0]), 0, 0, 0))
	println("print", kprint(addr(&bc[0]), 0, 0, 0))
	yaroze.KanjiFntClose()
	println("closed past its n", kprint(addr(&a[0]), 0, 0, 0))

	println("FntPrint never opened", fprint(addr(&a[0]), 0, 0, 0))
	yaroze.FntLoad(960, 256)
	println("FntPrint after a load, none open", fprint(addr(&a[0]), 0, 0, 0))
	println("FntOpen", yaroze.FntOpen(0, 0, 320, 240, 0, 16))
	println("FntPrint", fprint(addr(&a[0]), 0, 0, 0))
	yaroze.FntLoad(960, 256)
	println("FntPrint, loaded again", fprint(addr(&a[0]), 0, 0, 0))
	println("END")
	for {
	}
}
