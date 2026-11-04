//go:build yaroze

// Command image is a prototype: the Go library laid out as the resident
// image, for Net Yaroze programs to be combined with in place of Sony's.
// keep.go calls every export, so that the link keeps them all.
package main

import (
	"unsafe"

	"go.chrisrx.dev/psx/exp/libps"
	"go.chrisrx.dev/psx/internal/volatile"
)

var sink uint32

// opaque returns a T the compiler cannot know, so that no call in keep
// is specialized for its arguments.
func opaque[T any]() T {
	return *(*T)(unsafe.Pointer(uintptr(volatile.LoadUint32(&sink))))
}

// setup is what the resident init routine did after the runtime starts.
//
//export libps_image_setup
func setup() {
	if volatile.LoadUint32(&sink) == 1 {
		for _, f := range keep {
			f()
		}
	}
	libps.Startup()
}

func main() {}

// strlen is for the calls LLVM makes of string loops: the Go target's C
// library has no strlen.
//
//export strlen
func strlen(s *byte) uintptr {
	n := uintptr(0)
	for *(*byte)(unsafe.Add(unsafe.Pointer(s), n)) != 0 {
		n++
	}
	return n
}

// fntPuts adds the n bytes at s, formatted already, to window id, or to
// FntPrint's default one when id is negative, for the image's C FntPrint:
// 0 for a window that is not open, as the original answered.
//
//export libps_fnt_puts
func fntPuts(id int32, s *byte, n int32) int32 {
	if id < 0 {
		id = libps.FntDump()
	}
	return libps.FntAddBytes(id, unsafe.Slice(s, n))
}

// kanjiPuts adds the n bytes at s, formatted already, to Kanji window id,
// or to the default one when id is negative, for the image's C
// KanjiFntPrint.
//
//export libps_kanji_puts
func kanjiPuts(id int32, s *byte, n int32) int32 {
	return libps.KanjiAddBytes(id, unsafe.Slice(s, n))
}
