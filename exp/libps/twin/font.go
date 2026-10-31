//go:build yaroze

package main

import (
	"unsafe"

	"go.chrisrx.dev/psx/exp/libps"
	"go.chrisrx.dev/psx/yaroze"
)

// The font's script: the font loaded, a window opened and printed to,
// past its length and with a %% in the text, flushed, and a second
// window with a background and no width, which sizes itself. The
// windows' counts, sprites and the first flush's head are printed; the
// drawing is in the VRAM comparison.

//export FntLoad_orig
func FntLoad_orig(tx, ty int32)

//export FntOpen_orig
func FntOpen_orig(x, y, w, h, isbg, n int32) int32

//export FntPrint_orig
func FntPrint_orig(s *byte) int32

//export FntFlush_orig
func FntFlush_orig(id int32) *uint32

// The original's sprite packets, at their fixed address.
const origFontSprites = 0x8005016c

var fontBuf [80]byte

func cstr(s string) *byte {
	n := copy(fontBuf[:], s)
	fontBuf[n] = 0
	return &fontBuf[0]
}

var long = "0123456789012345678901234567890123456789012345678901234567890123456789"

func fontScript(mode int32) {
	var sprites *uint32
	if mode == 0 {
		FntLoad_orig(960, 256)
		print("font open ", FntOpen_orig(16, 16, 256, 64, 0, 64), "\n")
		print("font print ", FntPrint_orig(cstr("hello")), " ", FntPrint_orig(cstr("50%%")), " ", FntPrint_orig(cstr(long)), "\n")
		sprites = (*uint32)(unsafe.Pointer(uintptr(origFontSprites)))
		head := FntFlush_orig(0)
		printWords("font head", head, 3)
		printWords("font sprites", sprites, 4*8)
		print("font open ", FntOpen_orig(100, 100, 0, 0, 1, 8), "\n")
		print("font print ", FntPrint_orig(cstr("ab~c900c\n")), "\n")
		FntFlush_orig(1)
		DrawSync_orig(0)
	} else {
		yaroze.FntLoad(960, 256)
		print("font open ", yaroze.FntOpen(16, 16, 256, 64, 0, 64), "\n")
		print("font print ", yaroze.FntPrint(cstr("hello")), " ", yaroze.FntPrint(cstr("50%%")), " ", yaroze.FntPrint(cstr(long)), "\n")
		sprites = libps.FontSprites()
		head := yaroze.FntFlush(0)
		printWords("font head", head, 3)
		printWords("font sprites", sprites, 4*8)
		print("font open ", yaroze.FntOpen(100, 100, 0, 0, 1, 8), "\n")
		print("font print ", yaroze.FntPrint(cstr("ab~c900c\n")), "\n")
		yaroze.FntFlush(1)
		yaroze.DrawSync(0)
	}
}

// printWords prints n words from p, the tags' lengths only, since their
// links are addresses.
func printWords(name string, p *uint32, n int) {
	print(name)
	for i, w := range unsafe.Slice(p, n) {
		if i%4 == 0 {
			print(" ", hex32(w>>24))
		} else {
			print(" ", hex32(w))
		}
	}
	print("\n")
}
