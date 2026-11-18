//go:build yaroze

// Command vabsize is a probe for the spec: how much of a bank's body
// SsVabTransfer sends, when the header's total size (fsize) and the VAG
// size table disagree. Each case changes a copy of the header, sends the
// bank and closes it again, between GP1 marker words (10000100h plus the
// case number), so a stream of GPU and SPU writes shows each case's
// upload.
package main

import (
	"encoding/binary"
	"runtime/volatile"
	"unsafe"

	"go.chrisrx.dev/psx/disc"
	"go.chrisrx.dev/psx/yaroze"
)

var (
	vhData [(3616 + 2047) / 2048 * 512]uint32
	vbData [(1008 + 2047) / 2048 * 512]uint32
	vhCopy [3616 / 4]uint32
	vbBig  [4096 / 4]uint32

	vh = disc.Register("ASSETS/PROBE.VH", "assets/PROBE.VH").Into(vhData[:])
	vb = disc.Register("ASSETS/PROBE.VB", "assets/PROBE.VB").Into(vbData[:])
)

func mark(n uint32) {
	volatile.StoreUint32((*uint32)(yaroze.Pointer(0x1f801814)), 0x10000100|n)
}

// The VAG size table follows the header, the 128 programs and the two
// programs' tones: 32 + 2048 + 2*512 bytes in. Entries are sizes in 8s.
const table = 32 + 2048 + 2*512

type change struct {
	fsize      int // added to the header's total size
	tab1, tab2 int // added to the table's first and second sizes, in 8s
}

var cases = []change{
	{},
	{fsize: -64},
	{fsize: 64},
	{fsize: 16},
	{fsize: -16},
	{fsize: 1000},
	{tab1: 2},
	{tab1: -2},
	{tab2: 8},
	{fsize: 64, tab1: -8},
	{fsize: -1008},
	{fsize: -1000},
}

func main() {
	yaroze.ResetGraph(0)
	yaroze.GsInitGraph(320, 240, yaroze.GsOFSGPU, 0, 0)
	yaroze.GsDefDispBuff(0, 0, 0, 240)
	if f := disc.LoadAll(300); f != nil {
		println("load failed")
	}
	src := unsafe.Slice(vh.Bytes(), 3616)
	big := unsafe.Slice((*byte)(unsafe.Pointer(&vbBig[0])), len(vbBig)*4)
	copy(big, unsafe.Slice(vb.Bytes(), 1008))
	dst := unsafe.Slice((*byte)(unsafe.Pointer(&vhCopy[0])), len(vhCopy)*4)
	for i := range cases {
		c := cases[i]
		copy(dst, src)
		binary.LittleEndian.PutUint32(dst[12:], uint32(int(binary.LittleEndian.Uint32(dst[12:]))+c.fsize))
		t1 := binary.LittleEndian.Uint16(dst[table+2:])
		t2 := binary.LittleEndian.Uint16(dst[table+4:])
		binary.LittleEndian.PutUint16(dst[table+2:], uint16(int(t1)+c.tab1))
		binary.LittleEndian.PutUint16(dst[table+4:], uint16(int(t2)+c.tab2))
		yaroze.VSync(0)
		mark(uint32(i))
		id := yaroze.SsVabTransfer(&dst[0], &big[0], -1, 1)
		yaroze.VSync(0)
		yaroze.VSync(0)
		println("case", i, "vab", id)
		if i == 0 || i == 8 {
			// What SsVabTransfer changed in the header it was given.
			for j := 0; j+4 <= len(src); j += 4 {
				was := binary.LittleEndian.Uint32(src[j:])
				now := binary.LittleEndian.Uint32(dst[j:])
				if j == 12 || j == table || j == table+4 {
					continue // changed by this case
				}
				if was != now {
					println("  changed", j, hex(was), hex(now))
				}
			}
		}
		if id >= 0 {
			yaroze.SsVabClose(id)
		}
	}
	yaroze.VSync(0)
	mark(0xff)
	println("done")
	for {
		yaroze.VSync(0)
	}
}

func hex(v uint32) string {
	const d = "0123456789abcdef"
	var b [8]byte
	for i := 7; i >= 0; i-- {
		b[i] = d[v&15]
		v >>= 4
	}
	return string(b[:])
}
