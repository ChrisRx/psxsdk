package tests

import (
	"os"
	"testing"

	"go.chrisrx.dev/psx/emu/emutest"
	"go.chrisrx.dev/x/assert"
)

// TestCompressionBench times package archive's inflate and the benchmark's
// LZ4 decoder on the console over internal/compressbench's files, and
// prints the speed of each: the measurements docs/architecture/compression.md
// rests on. It runs when task bench:compress has built the disc.
func TestCompressionBench(t *testing.T) {
	parallel(t)
	m := bootExample(t, "compressbench")
	names := []string{"CASTLE.TIM deflate", "CASTLE.TIM lz4", "DOG.DAT deflate", "DOG.DAT lz4", "MD0.TMD deflate", "MD0.TMD lz4"}
	raws := []string{"../examples/dino/assets/CASTLE.TIM", "../examples/dino/assets/CASTLE.TIM", "../examples/dogtale/assets/DOG.DAT", "../examples/dogtale/assets/DOG.DAT", "../examples/dino/assets/MD0.TMD", "../examples/dino/assets/MD0.TMD"}
	sums := make([]uint32, len(raws))
	for i, p := range raws {
		data, err := os.ReadFile(p)
		assert.NoError(t, err)
		for _, b := range data {
			sums[i] += uint32(b)
		}
	}
	const rounds = 3
	bytesAt := m.Sym("main.benchBytes")
	sumAt := m.Sym("main.benchSum")
	failAt := m.Sym("main.benchFail")
	// The first round of a file checks the bytes, and the program's first
	// includes the frame it waited for while loading, so each file's speed
	// is the mean of its later rounds.
	n := uint32(0)
	for i := range names {
		var cycles emutest.Cycles
		var bytes uint32
		for r := 0; r < rounds; r++ {
			n++
			start := m.Cycles()
			m.RunUntilCount("main.benchRound", n, 60*emutest.Second)
			if r == 0 {
				assert.Equal(t, sums[i], m.Read32(sumAt), names[i]+" sum")
				continue
			}
			cycles += m.Cycles() - start
			bytes += m.Read32(bytesAt)
		}
		assert.Equal(t, uint32(0), m.Read32(failAt), names[i]+" unpacked")
		kbs := float64(bytes) / 1024 / (float64(cycles) / float64(emutest.Second))
		t.Logf("%-20s %7.0f KB/s  %5.1f cycles/byte", names[i], kbs, float64(cycles)/float64(bytes))
	}
}
