// The compression benchmark: the program behind the measurements in
// docs/topics/compression.md, which chose deflate for the archives. It
// unpacks each packed file off its disc into memory a few times over,
// with package arc's inflate and with an LZ4 decoder kept for the
// comparison, counting rounds and bytes in globals that the emulator
// test, tests/compressbench_test.go, reads beside the cycle count.
//
// task bench:compress makes the data from three of the examples' files,
// builds the disc and runs the test, which prints each decoder's speed.
package main

//go:generate go run go.chrisrx.dev/psx/tools/discgen data

import (
	"unsafe"

	"go.chrisrx.dev/psx/arc"
	"go.chrisrx.dev/psx/disc"
	"go.chrisrx.dev/psx/internal/compressbench/lz4"
	"go.chrisrx.dev/psx/internal/volatile"
	"go.chrisrx.dev/psx/yaroze"
)

const rounds = 3

var (
	screen = yaroze.Screen{Background: yaroze.CVECTOR{R: 20, G: 20, B: 40}, Depth: 1}
	loader disc.Loader

	// The test reads these: the round finished so far, the bytes the last
	// round unpacked, a sum over them, and whether unpacking failed.
	benchRound uint32
	benchBytes uint32
	benchSum   uint32
	benchFail  uint32

	out [80000]uint32 // the largest raw file is 307,220 bytes
)

// The packed files, a deflate stream then an LZ4 block of each raw file.
var packed = []*disc.File{&castleDFL, &castleLZ4, &dogDFL, &dogLZ4, &md0DFL, &md0LZ4}

func main() {
	screen.Open()
	loader.Start(300, files...)
	done := false
	for range screen.Frames() {
		if done {
			yaroze.Print("done\n")
			continue
		}
		loaded, failed := loader.Step()
		if failed {
			yaroze.Print("could not read ")
			yaroze.Print(loader.Failed().Name)
			continue
		}
		if !loaded {
			continue
		}
		dst := unsafe.Slice((*byte)(unsafe.Pointer(&out[0])), len(out)*4)
		for i, f := range packed {
			words := f.Data()
			bytes := unsafe.Slice((*byte)(unsafe.Pointer(&words[0])), len(words)*4)
			raw := int(bytes[0]) | int(bytes[1])<<8 | int(bytes[2])<<16 | int(bytes[3])<<24
			n := int(bytes[4]) | int(bytes[5])<<8 | int(bytes[6])<<16 | int(bytes[7])<<24
			src := bytes[8 : 8+n]
			for r := 0; r < rounds; r++ {
				var got int
				if i&1 == 0 {
					got = arc.Inflate(dst[:raw], src)
				} else {
					got = lz4.Decode(dst[:raw], src)
				}
				if got != raw {
					volatile.StoreUint32(&benchFail, uint32(i+1))
				}
				if r == 0 {
					// Checked once a file; the later rounds are decoding alone.
					var sum uint32
					for _, b := range dst[:raw] {
						sum += uint32(b)
					}
					volatile.StoreUint32(&benchSum, sum)
				}
				volatile.StoreUint32(&benchBytes, uint32(raw))
				volatile.StoreUint32(&benchRound, volatile.LoadUint32(&benchRound)+1)
			}
		}
		done = true
	}
}
