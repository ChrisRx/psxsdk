//go:build yaroze

package libps

import "unsafe"

// The library's allocator, which nothing the Go library covers allocates
// from: the crt0 and the runtime hand it a region at startup, which is
// recorded and left alone. A program's own allocations are TinyGo's. The
// original keeps a free list from head; a function of this library that
// turns out to need it brings the allocator with it.

// heapHead and heapSize are what InitHeap was last given.
//
// libps 0x8004F5F4 and 0x8004F5F8
var heapHead unsafe.Pointer
var heapSize uint32

// InitHeap gives the allocator size bytes at head.
//
// libps 0x80010700, 84 bytes
//
//export InitHeap
func InitHeap(head unsafe.Pointer, size uint32) {
	heapHead, heapSize = head, size&^3
}

// malloc allocates nothing: the library's own allocator is not here, and
// no caller in the Go library asks it for anything.
//
// libps 0x80010754, 464 bytes
//
//export malloc
func malloc(n uint32) unsafe.Pointer {
	return nil
}

// free frees what malloc gave, which is nothing.
//
// libps 0x80010924, 56 bytes
//
//export free
func free(p unsafe.Pointer) {
}
