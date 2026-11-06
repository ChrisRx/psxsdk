//go:build yaroze

// Command stk shows what printf and sprintf leave below the stack.
package main

import (
	"device"
	"unsafe"

	"go.chrisrx.dev/psx/yaroze"
)

//export sprintf
func sprintfC(dst *byte, f *byte, a, b uint32) int32

//export printf
func printfC(f *byte, a, b, c uint32) int32

var (
	copyw [256]uint32
	buf   [64]byte
	spAt  uintptr
)

//go:noinline
func one(sp bool, f *byte) {
	s := uintptr(device.AsmFull(`move {}, $$sp`, nil))
	spAt = s
	for i := uintptr(1); i <= 256; i++ {
		*(*uint32)(yaroze.Pointer(s - i*4)) = 0xee000000 + uint32(i)
	}
	if sp {
		sprintfC(&buf[0], f, 123, 45)
	} else {
		printfC(f, 123, 45, 0)
	}
	for i := uintptr(1); i <= 256; i++ {
		copyw[i-1] = *(*uint32)(yaroze.Pointer(s - i*4))
	}
}

func main() {
	yaroze.ResetGraph(0)
	fs := []string{"x\x00", "%d\x00", "%d %x\x00", "\n\x00", "abc\n\x00"}
	for _, ff := range fs {
		b := []byte(ff)
		for _, sp := range []bool{true, false} {
			one(sp, (*byte)(unsafe.Pointer(&b[0])))
			println()
			print("C ", sp, " ", len(ff), " :")
			for i, v := range copyw {
				if v != 0xee000000+uint32(i+1) {
					print(" ", (i+1)*4, "=")
					for s := 28; s >= 0; s -= 4 {
						print(string(rune("0123456789abcdef"[v>>uint(s)&15])))
					}
				}
			}
			println()
		}
	}
	println("END")
	for {
		yaroze.VSync(0)
	}
}
