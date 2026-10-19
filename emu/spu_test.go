package emu

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"hash/fnv"
	"math/rand/v2"
	"strings"
	"testing"

	"go.chrisrx.dev/x/assert"
)

// runSPUScript runs a script of register writes, "w OFF VAL" in hex, and
// sample requests, "s N", returning the samples made as left and right
// pairs. The format is that of a harness around psxe's SPU, which made the
// expected output below.
func runSPUScript(t *testing.T, s *spu, script string) [][2]int16 {
	t.Helper()
	var out [][2]int16
	sc := bufio.NewScanner(strings.NewReader(script))
	for sc.Scan() {
		var off, v uint32
		var n int
		switch line := sc.Text(); {
		case line == "":
		case strings.HasPrefix(line, "w "):
			if _, err := fmt.Sscanf(line, "w %x %x", &off, &v); !assert.NoError(t, err) {
				t.FailNow()
			}
			s.Write16(off, uint16(v))
		case strings.HasPrefix(line, "s "):
			if _, err := fmt.Sscanf(line, "s %d", &n); !assert.NoError(t, err) {
				t.FailNow()
			}
			for range n {
				s.sample()
				a := s.takeAudio(nil)
				out = append(out, [2]int16{a[0], a[1]})
			}
		default:
			t.Fatalf("bad script line %q", line)
		}
	}
	return out
}

// spuScript uploads four ADPCM blocks at 1000h, of four filters and
// shifts, the first marking the loop start and the last ending it and
// looping back to the second; sets the main volume and turns the SPU on;
// and plays voice 0 over them, louder on the left, with an attack of a
// step a sample.
const spuScript = `
w 1a6 200
w 1a8 0414
w 1a8 2301
w 1a8 6745
w 1a8 ab89
w 1a8 efcd
w 1a8 1f0e
w 1a8 3e2d
w 1a8 7c4b
w 1a8 0022
w 1a8 9876
w 1a8 5432
w 1a8 1010
w 1a8 f0e1
w 1a8 d2c3
w 1a8 b4a5
w 1a8 9687
w 1a8 0033
w 1a8 77ff
w 1a8 0088
w 1a8 11ee
w 1a8 22dd
w 1a8 33cc
w 1a8 44bb
w 1a8 55aa
w 1a8 0349
w 1a8 4321
w 1a8 8765
w 1a8 cba9
w 1a8 0fed
w 1a8 f1e2
w 1a8 d3c4
w 1a8 b5a6
w 1aa 10
w 180 3fff
w 182 3000
w 1aa c000
w 0 3fff
w 2 1000
w 4 1800
w 6 200
w 8 0
w a 0
w e 210
w 188 1
s 112
`

// TestSPUAudioMatchesPsxe plays spuScript and checks the sound against
// what psxe's SPU makes from it, sample for sample: decoding, the loop,
// Gaussian interpolation, the envelope, and the voice and main volumes.
func TestSPUAudioMatchesPsxe(t *testing.T) {
	s := newSPU(&ic{setIRQ: func(bool) {}})
	s.setAudio(true)
	assert.Equal(t, spuScriptPsxe, runSPUScript(t, s, spuScript))
}

// TestSPUAudioOff checks that without audio on nothing is made, and that
// with the SPU's output muted, the samples are silent.
func TestSPUAudioOff(t *testing.T) {
	s := newSPU(&ic{setIRQ: func(bool) {}})
	runSPUScript(t, s, strings.Replace(spuScript, "s 112", "", 1))
	for range 100 {
		s.sample()
	}
	assert.Equal(t, 0, len(s.takeAudio(nil)), "nothing is made with audio off")

	s.setAudio(true)
	s.Write16(spuSPUCNT, 0x8000) // on, but muted
	for range 100 {
		s.sample()
	}
	assert.Equal(t, make([]int16, 200), s.takeAudio(nil))
}

// TestDecodeBlock decodes blocks whose samples can be worked out by hand.
func TestDecodeBlock(t *testing.T) {
	s := newSPU(nil)
	var snd voiceSound

	// Filter 0 and shift 0, which scales each nibble by 4096. The nibbles
	// are 1, -1, 7, -8, then zeros.
	copy(s.ram[0x100:], []byte{0x00, 0x00, 0xf1, 0x87})
	s.decodeBlock(0x100, &snd)
	assert.Equal(t, []int16{4096, -4096, 28672, -32768, 0, 0}, snd.buf[:6])

	// Filter 1 adds 60/64 of the last sample, rounded, to each: from 64,
	// a zero nibble decays it.
	snd = voiceSound{h: [2]int16{64, 0}}
	copy(s.ram[0x200:], []byte{0x1c, 0x00})
	s.decodeBlock(0x200, &snd)
	assert.Equal(t, []int16{60, 56, 53, 50}, snd.buf[:4])
}

// spuReverbRegs sets up the reverb: a work area from 60000h, a set of
// delays and volumes, and the output volume.
const spuReverbRegs = `
w 1a2 c000
w 1c0 28
w 1c2 bf
w 1c4 74be
w 1c6 1c96
w 1c8 d2ff
w 1ca d3ac
w 1cc ed68
w 1ce 7b08
w 1d0 a29f
w 1d2 30af
w 1d4 c42
w 1d6 c97
w 1d8 12c6
w 1da 1698
w 1dc 2322
w 1de d8e
w 1e0 bae
w 1e2 2b4c
w 1e4 2126
w 1e6 2a62
w 1e8 20e5
w 1ea ee4
w 1ec ad5
w 1ee 416
w 1f0 2ef
w 1f2 23b9
w 1f4 28d1
w 1f6 21f0
w 1f8 1162
w 1fa 262f
w 1fc 51c4
w 1fe fae5
w 184 2e1b
w 186 3876
`

// spuReverbScript adds reverb to spuScript's voice: spuReverbRegs, the
// voice sent to the reverb, and the reverb turned on.
var spuReverbScript = strings.NewReplacer("w 1aa c000\n", "", "w 188 1\n", "", "s 112\n", "").Replace(spuScript) + spuReverbRegs + `
w 198 1
w 1aa c080
w 188 1
s 3000
`

// TestSPUReverbMatchesMednafen runs the reverb alone, set up by
// spuReverbRegs, over a work area of noise, on a second of noise, loud and
// quiet in turn, with the reverb turned off for a while along the way. Its
// output and what it leaves in SPU RAM are Mednafen's from the same input,
// which follows the hardware's arithmetic and resampling.
func TestSPUReverbMatchesMednafen(t *testing.T) {
	s := newSPU(&ic{setIRQ: func(bool) {}})
	runSPUScript(t, s, spuReverbRegs)
	rng := rand.New(rand.NewPCG(1, 2))
	for i := range s.ram {
		s.ram[i] = byte(rng.Uint32())
	}

	out := fnv.New64a()
	for i := range 44100 {
		cnt := uint16(0xc080)
		if i/10000 == 2 {
			cnt = 0xc000
		}
		s.put16(spuSPUCNT, cnt)
		amp := [2]int32{0x8000, 1000}[i/2000%2]
		l, r := s.reverb(int16(rng.Int32N(2*amp)-amp), int16(rng.Int32N(2*amp)-amp))
		out.Write(binary.LittleEndian.AppendUint16(binary.LittleEndian.AppendUint16(nil, uint16(l)), uint16(r)))
	}
	ram := fnv.New64a()
	ram.Write(s.ram[:])
	assert.Equal(t, uint64(0x2aad3c15844ad642), out.Sum64(), "the output")
	assert.Equal(t, uint64(0xa835d310410016b3), ram.Sum64(), "SPU RAM")
}

// TestSPUReverbOff checks that the reverb leaves SPU RAM alone when it is
// off, with a voice sent to it.
func TestSPUReverbOff(t *testing.T) {
	s := newSPU(&ic{setIRQ: func(bool) {}})
	s.setAudio(true)
	runSPUScript(t, s, strings.Replace(spuReverbScript, "w 1aa c080", "w 1aa c000", 1))
	var area [spuRAMSize - 0x60000]byte
	assert.Equal(t, area, [spuRAMSize - 0x60000]byte(s.ram[0x60000:]))
}

// spuScriptPsxe is psxe's output from spuScript.
var spuScriptPsxe = [][2]int16{
	{0, 0}, {27, 4}, {46, 8}, {373, 70}, {627, 117}, {1917, 359},
	{2433, 456}, {995, 186}, {559, 104}, {-89, -16}, {-205, -38}, {-183, -34},
	{-196, -36}, {-105, -19}, {-101, -19}, {-696, -130}, {-694, -130}, {-1462, -274},
	{-1417, -265}, {-693, -130}, {27, 5}, {7239, 1357}, {9657, 1810}, {7955, 1491},
	{7835, 1469}, {-3079, -577}, {-10652, -1997}, {932, 174}, {6979, 1308}, {4227, 792},
	{-1692, -317}, {6899, 1293}, {11378, 2133}, {5619, 1053}, {-1104, -207}, {4465, 837},
	{8291, 1554}, {-736, -138}, {-10384, -1947}, {-2956, -554}, {3171, 594}, {10799, 2025},
	{9766, 1831}, {-4328, -811}, {-9350, -1753}, {-8282, -1552}, {-4567, -856}, {4512, 846},
	{6135, 1150}, {2465, 462}, {-480, -90}, {-4873, -913}, {-4516, -846}, {-963, -180},
	{816, 153}, {2242, 420}, {947, 177}, {-513, -96}, {-316, -59}, {232, 43},
	{446, 83}, {940, 176}, {1113, 208}, {1301, 244}, {1303, 244}, {1094, 205},
	{962, 180}, {543, 102}, {364, 68}, {-59, -11}, {-218, -40}, {-537, -100},
	{-634, -118}, {-767, -144}, {-792, -148}, {-762, -142}, {-709, -133}, {-323, -60},
	{-127, -24}, {713, 133}, {1209, 226}, {1824, 342}, {1533, 287}, {-188, -35},
	{-747, -140}, {-1459, -273}, {-1644, -308}, {-1843, -345}, {-1852, -347}, {-1877, -352},
	{-1839, -345}, {-1373, -257}, {-1329, -249}, {-1648, -309}, {-1456, -273}, {4703, 882},
	{8578, 1608}, {9039, 1695}, {7636, 1431}, {10079, 1890}, {8297, 1555}, {-6726, -1261},
	{-116, -21}, {-3362, -630}, {-10148, -1902}, {1450, 271}, {7121, 1335}, {-1259, -236},
	{-9481, -1777}, {594, 111}, {5038, 944}, {11079, 2077},
}
