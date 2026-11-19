//go:build yaroze

package libps

import (
	"device"

	"go.chrisrx.dev/psx/internal/volatile"
)

// The C library's heap (spec 18): first fit from a place that moves, with
// blocks joined only while a request needs them, and nothing checked. Programs
// can see all of it, down to the headers left in memory, so the rules below are
// followed exactly, the ones that look like bugs among them: realloc taking a
// free block in place does not correct where the next search starts.
//
// A block is a header word and its bytes. The header holds the byte count, with
// bit 0 set if the block is free. heapEndMark is normally at the top.

const heapEndMark = 0xfffffffe

// What Sony's calls take beyond the same work here, in stores (spec 18): a block
// from the top, one taken whole or split, and each block passed or joined on the
// way to it.
const (
	heapTopSpace    = 39
	heapTakenSpace  = 18
	heapSplitSpace  = 20
	heapPassedSpace = 13
	heapJoinSpace   = 15
	heapNoneSpace   = 13
)

// cHeap is the head and end of the area InitHeap gave, where the next search
// starts, and the top, where the next block made from unused memory goes.
var cHeap struct {
	head, end, search, top uint32
}

// InitHeap gives the C library's heap the size bytes at head, forgetting all
// that went before. Size may be anything. The start-up of a program built for
// the resident library calls it, and the runtime before main.
//
//export InitHeap
//go:nobounds
func InitHeap(head, size uint32) {
	cHeap.head, cHeap.end, cHeap.search, cHeap.top = head, head+size, head, head
	cWords(uintptr(head))[0] = heapEndMark
}

// heapWord reads the word at p, and heapSet writes it.
//
//go:inline
//go:nobounds
func heapWord(p uint32) uint32 { return cWords(uintptr(p))[0] }

//go:inline
//go:nobounds
func heapSet(p, v uint32) { cWords(uintptr(p))[0] = v }

// heapRound rounds n up to whole words, in 32 bits.
//
//go:inline
func heapRound(n uint32) uint32 { return (n + 3) &^ 3 }

//export libps_c_malloc
//go:nobounds
func libMalloc(n uint32) uint32 {
	if n == 0 {
		space(heapNoneSpace)
		return 0
	}
	n = heapRound(n)
	p := cHeap.search
	for {
		h := heapWord(p)
		if h == heapEndMark {
			return heapFromTop(n)
		}
		if h&1 == 0 {
			p += 4 + h
			continue
		}
		size := h &^ 1
		for size < n {
			w := heapWord(p + 4 + size)
			if w == heapEndMark {
				// A free run reaching the end mark gives its memory back,
				// whether or not the request then fits.
				heapSet(p, heapEndMark)
				cHeap.top = p
				return heapFromTop(n)
			}
			if w&1 == 0 {
				break
			}
			size += w&^1 + 4
			heapSet(p, size|1)
			space(heapJoinSpace)
		}
		if size >= n {
			if size-n >= 4 {
				heapSet(p+4+n, (size-n-4)|1)
				heapSet(p, n)
				space(heapSplitSpace)
			} else {
				heapSet(p, size)
				space(heapTakenSpace)
			}
			return p + 4
		}
		p += 4 + size
		space(heapPassedSpace)
	}
}

// heapFromTop makes a block at the top, or sets the search back to the head
// and returns 0 if there is no room. The room is 8 bytes short of the end, with
// the subtraction wrapping: a top within 4 of the end gives a huge room.
func heapFromTop(n uint32) uint32 {
	t := cHeap.top
	if n > cHeap.end-t-8 {
		cHeap.search = cHeap.head
		return 0
	}
	heapSet(t, n)
	next := t + 4 + n
	cHeap.top, cHeap.search = next, next
	heapSet(next, heapEndMark)
	space(heapTopSpace)
	return t + 4
}

// free marks any pointer's word before it free, and moves the search down to
// it. It checks nothing but null.
//
//export libps_c_free
//go:nobounds
func libFree(p uint32) {
	if p == 0 {
		return
	}
	q := p - 4
	heapSet(q, heapWord(q)|1)
	if q < cHeap.search {
		cHeap.search = q
	}
}

// realloc(p, 0) frees p and leaves in v0 the s1 of its caller: Sony's returns a
// value it never set (probe spec/probes/heapv0, with spec 18's "0" being what
// that s1 held in the heap probe). So it is kept as a small function, which holds
// nothing in s1 itself, around the rest.
//
//export libps_c_realloc
func libRealloc(p, n uint32) uint32 {
	if p != 0 && n == 0 {
		libFree(p)
		return uint32(device.AsmFull(`.set push
	.set noat
	move {}, $$17
	.set pop`, nil))
	}
	return heapResize(p, n)
}

//go:noinline
//go:nobounds
func heapResize(p, n uint32) uint32 {
	if p == 0 {
		return libMalloc(n)
	}
	r := heapRound(n)
	h := heapWord(p-4) &^ 1
	if int32(r) <= int32(h) {
		// Shrunk in place. A negative n is a shrink too.
		if h-r >= 4 {
			heapSet(p+r, (h-r-4)|1)
			heapSet(p-4, r)
		}
		return p
	}
	size := h
	for {
		w := heapWord(p + size)
		if w == heapEndMark {
			if r <= cHeap.end-(p-4)-8 {
				heapSet(p-4, r)
				if cHeap.search == cHeap.top {
					cHeap.search = p + r
				}
				cHeap.top = p + r
				heapSet(p+r, heapEndMark)
				return p
			}
			break
		}
		if w&1 == 0 {
			break
		}
		size += w&^1 + 4
		heapSet(p-4, size)
		if size >= r {
			if size-r >= 4 {
				heapSet(p+r, (size-r-4)|1)
				heapSet(p-4, r)
			}
			return p
		}
	}
	// Moved, with the block still marked used. It copies the n asked for and
	// not the old size, so it reads past the old block.
	q := libMalloc(n)
	if q == 0 {
		return 0
	}
	copyForward(uintptr(q), uintptr(p), int32(n))
	libFree(p)
	return q
}

// calloc clears exactly the product's bytes, which overflows unnoticed, and not
// the rounded size.
//
//export libps_c_calloc
//go:nobounds
func libCalloc(n, m uint32) uint32 {
	total := n * m
	q := libMalloc(total)
	if q == 0 {
		return 0
	}
	b := cBytes(uintptr(q))
	for i := uint32(0); i < total; i++ {
		// Volatile, or the compiler would call memset for the loop.
		volatile.StoreUint8(&b[i], 0)
	}
	return q
}
