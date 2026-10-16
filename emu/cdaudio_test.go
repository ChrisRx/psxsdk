package emu

import (
	"encoding/binary"
	"fmt"
	"testing"
	"testing/fstest"

	"go.chrisrx.dev/x/assert"
)

// cddaSampleAt is the sample a test disc's audio holds, left, in the n'th
// sector of track: a different value at every position. Right is its
// negation.
func cddaSampleAt(track, n, i int) int16 { return int16(track*10000 + n*600 + i) }

// openAudioDisc makes a disc of a 4-sector data track and two audio
// tracks of n sectors, each in a file of its own, so each starts where the
// last ends.
func openAudioDisc(t *testing.T, n int) *Disc {
	t.Helper()
	fsys := fstest.MapFS{}
	cue := ""
	for track := 1; track <= 3; track++ {
		name := fmt.Sprintf("t%d.bin", track)
		kind, sectors := "AUDIO", n
		if track == 1 {
			kind, sectors = "MODE2/2352", 4
		}
		bin := make([]byte, sectors*sectorSize)
		if track > 1 {
			for s := range sectors {
				for i := range cdSamples {
					v := cddaSampleAt(track, s, i)
					binary.LittleEndian.PutUint16(bin[s*sectorSize+4*i:], uint16(v))
					binary.LittleEndian.PutUint16(bin[s*sectorSize+4*i+2:], uint16(-v))
				}
			}
		}
		fsys[name] = &fstest.MapFile{Data: bin}
		cue += fmt.Sprintf("FILE %q BINARY\n  TRACK %02d %s\n    INDEX 01 00:00:00\n", name, track, kind)
	}
	fsys["disc.cue"] = &fstest.MapFile{Data: []byte(cue)}
	d, err := OpenDiscFS(fsys, "disc.cue")
	if !assert.NoError(t, err) {
		t.FailNow()
	}
	t.Cleanup(func() { d.Close() })
	return d
}

// playingDrive is a drive with CD audio on, playing from lba.
func playingDrive(t *testing.T, d *Disc, lba uint32, mode uint8) *cdrom {
	c := newCDROM(&ic{setIRQ: func(bool) {}}, new(uint64), new(bool), nil)
	c.insert(d)
	c.audioOn = true
	c.mode = mode
	c.ier = 0x1f
	c.lba = lba
	c.cddaPlaying = true
	c.state = cdPlaying
	c.playStart()
	return c
}

func TestCDDAPlay(t *testing.T) {
	d := openAudioDisc(t, 3)
	c := playingDrive(t, d, d.trackLBA(2), 0)

	// Two sectors' worth, in order, at unity volume.
	for s := range 2 {
		for i := range cdSamples {
			l, r := c.audioSample()
			want := cddaSampleAt(2, s, i)
			if l != want || r != -want {
				t.Fatalf("sector %d sample %d: got %d,%d, want %d,%d", s, i, l, r, want, -want)
			}
		}
	}
	assert.Equal(t, d.trackLBA(2)+2, c.lba, "two sectors played")

	// The volume matrix: left to both sides at half volume, right off.
	c.vol = [4]uint8{0x40, 0x40, 0, 0}
	l, r := c.audioSample()
	want := cddaSampleAt(2, 2, 0)
	assert.Equal(t, [2]int16{want / 2, want / 2}, [2]int16{l, r})

	c.mute = true
	l, r = c.audioSample()
	assert.Equal(t, [2]int16{0, 0}, [2]int16{l, r}, "muted")
}

func TestCDDADoubleSpeed(t *testing.T) {
	d := openAudioDisc(t, 3)
	c := playingDrive(t, d, d.trackLBA(2), cdModeSpeed)
	var got []int16
	for range cdSamples {
		l, _ := c.audioSample()
		got = append(got, l)
	}
	assert.Equal(t, cddaSampleAt(2, 0, 2), got[1], "every other sample")
	assert.Equal(t, cddaSampleAt(2, 1, 0), got[cdSamples/2], "two sectors a sector's time")
}

func TestCDDAReport(t *testing.T) {
	d := openAudioDisc(t, 30)
	// The track starts at LBA 154, so the play starts at frame 4: the
	// first report is on frame 10, 6 sectors in, relative to the track.
	c := playingDrive(t, d, d.trackLBA(2), cdModeReport)
	for range 7 * cdSamples {
		c.audioSample()
		if c.ifr != 0 {
			break
		}
	}
	assert.Equal(t, uint8(1), c.ifr)
	var resp []uint8
	for !c.response.empty() {
		resp = append(resp, c.response.pop())
	}
	peak := uint16(cddaSampleAt(2, 6, cdSamples-1))
	assert.Equal(t, []uint8{cdStatPlay | cdStatSpindle, 0x02, 0x01, 0x00, 0x80, 0x06, uint8(peak), uint8(peak >> 8)}, resp,
		"track 2, index 1, 00:00:06 into the track")
}

func TestCDDAAutoPause(t *testing.T) {
	d := openAudioDisc(t, 3)
	last := d.trackLBA(3) - 1
	c := playingDrive(t, d, last, cdModeAutoPause)
	for range cdSamples + 1 {
		c.audioSample()
	}
	assert.Equal(t, uint8(4), c.ifr, "INT4 at the end of the track")
	assert.Equal(t, []uint8{cdStatSpindle}, []uint8{c.response.pop()})
	assert.Equal(t, false, c.cddaPlaying)
	assert.Equal(t, last, c.lba, "it stays at the end of the old track")
}

func TestCDDAEndOfDisc(t *testing.T) {
	d := openAudioDisc(t, 3)
	c := playingDrive(t, d, d.trackLBA(0)-1, 0)
	for range cdSamples + 1 {
		c.audioSample()
	}
	assert.Equal(t, uint8(4), c.ifr, "INT4 at the end of the disc")
	assert.Equal(t, false, c.cddaPlaying)
}

func TestXADecodeBlock(t *testing.T) {
	var x xaDecoder
	var dst [28]int16
	src := make([]byte, 128)

	// Block 2 of a 4-bit portion: shift 0, filter 0, so each nibble times
	// 4096. Its nibbles are bits 8-11 of each word: 1, -1, 7, -8.
	src[4+2] = 0x00
	for j, n := range []uint32{1, 0xf, 7, 8} {
		binary.LittleEndian.PutUint32(src[16+j*4:], n<<8)
	}
	x.decodeBlock(src, 2, false, 0, &dst)
	assert.Equal(t, []int16{4096, -4096, 28672, -32768, 0}, dst[:5])

	// Filter 1 adds 60/64 of the last sample, rounded, so from 64 a zero
	// nibble decays it; shift 12 leaves a nibble of 1 as 1.
	x = xaDecoder{old: [2]int32{64, 0}}
	src = make([]byte, 128)
	src[4] = 0x1c
	binary.LittleEndian.PutUint32(src[16+3*4:], 1)
	x.decodeBlock(src, 0, false, 0, &dst)
	assert.Equal(t, []int16{60, 56, 53, 51}, dst[:4])
}

// TestXAZigZag puts an impulse through the resampler: the seven samples
// made after each six are the impulse weighted by each table, so the
// tables come out one per output, at the distance from the impulse.
func TestXAZigZag(t *testing.T) {
	var x xaDecoder
	x.sixStep = 6
	var out cdAudio
	x.output(0x7fff, 0, false, &out)
	for range 11 {
		x.output(0, 0, false, &out)
	}
	assert.Equal(t, 14, out.n, "seven samples for every six")
	for k := range 14 {
		l, _ := out.pop()
		group, tbl := k/7, k%7
		// The impulse was the first of the 12 samples; at the end of
		// the first group of six it is 6 back, at the end of the second 12.
		dist := 6 * (group + 1)
		want := int16(int32(0x7fff) * xaZigZag[tbl][dist-1] >> 15)
		assert.Equal(t, want, l, fmt.Sprintf("output %d", k))
	}
}

// xaSectorBytes makes a raw XA-ADPCM sector: Mode 2, an audio and real
// time submode, and the given file, channel and coding, with every block's
// header 0 and every nibble 1.
func xaSectorBytes(file, channel, coding uint8) []byte {
	buf := make([]byte, sectorSize)
	buf[0x0f] = 2
	buf[0x10], buf[0x11], buf[0x12], buf[0x13] = file, channel, 0x64, coding
	for p := range 18 {
		for j := range 28 {
			binary.LittleEndian.PutUint32(buf[24+p*128+16+j*4:], 0x11111111)
		}
	}
	return buf
}

func TestXASectorSamples(t *testing.T) {
	// 37.8kHz stereo makes 2016 pairs, mono 4032 samples, and half the
	// rate twice as many; each 6 become 7 at 44.1kHz.
	for _, tc := range []struct {
		coding uint8
		want   int
	}{
		{0x01, 2016 * 7 / 6},
		{0x00, 4032 * 7 / 6},
		{0x05, 2 * 2016 * 7 / 6},
		{0x04, 2 * 4032 * 7 / 6},
	} {
		var a cdAudio
		a.xa.sixStep = 6
		a.xa.decodeSector(xaSectorBytes(0, 0, tc.coding), &a)
		assert.Equal(t, tc.want, a.n, fmt.Sprintf("coding %#02x", tc.coding))
	}
}

// openXADisc makes a data track of sectors that are, in turn, data, XA
// file 1 channel 2, XA file 1 channel 3, and data, each data sector's
// first byte its index.
func openXADisc(t *testing.T) *Disc {
	t.Helper()
	var bin []byte
	for i, s := range [][]byte{nil, xaSectorBytes(1, 2, 1), xaSectorBytes(1, 3, 1), nil} {
		if s == nil {
			s = make([]byte, sectorSize)
			s[0x0f], s[0x12] = 2, 0x08
		}
		s[24] = byte(i)
		bin = append(bin, s...)
	}
	fsys := fstest.MapFS{
		"xa.cue": {Data: []byte("FILE \"xa.bin\" BINARY\n  TRACK 01 MODE2/2352\n    INDEX 01 00:00:00\n")},
		"xa.bin": {Data: bin},
	}
	d, err := OpenDiscFS(fsys, "xa.cue")
	if !assert.NoError(t, err) {
		t.FailNow()
	}
	t.Cleanup(func() { d.Close() })
	return d
}

// readSectors reads the XA disc from its start with mode set, and returns
// the first byte of each sector delivered to the CPU, and the samples the
// XA decoder made.
func readSectors(t *testing.T, cdAudioOn bool, mode uint8, filter ...uint8) ([]byte, int) {
	m := newTestMachine(t)
	m.SetCDAudio(cdAudioOn)
	m.InsertDisc(openXADisc(t))
	cdCommand(t, m, cdSetmode, mode)
	if len(filter) > 0 {
		cdCommand(t, m, cdSetfilter, filter...)
	}
	cdCommand(t, m, cdSetloc, 0x00, 0x02, 0x00)
	cdCommand(t, m, cdReadS)

	var got []byte
	const reg3 = 0x1f801803
	for range 400_000 {
		if !assert.NoError(t, m.Step()) {
			t.FailNow()
		}
		m.Bus.Write8(0x1f801800, 1)
		irq := m.Bus.Read8(reg3) & 7
		if irq == 0 {
			continue
		}
		if irq == 1 {
			got = append(got, m.cd.data.buf[24])
		}
		m.Bus.Write8(reg3, 0x1f)
		if m.cd.lba >= 154 {
			break
		}
	}
	return got, m.cd.audio.n
}

func TestXARouting(t *testing.T) {
	got, samples := readSectors(t, true, cdModeXAADPCM)
	assert.Equal(t, []byte{0, 3}, got, "XA on: the XA sectors go to the decoder")
	assert.Between(t, 1, 2*2352, samples, "both decoded, less what has played")

	got, samples = readSectors(t, true, cdModeXAADPCM|cdModeXAFilter, 1, 2)
	assert.Equal(t, []byte{0, 3}, got, "filter on: the other channel is dropped")
	assert.Between(t, 1, 2352, samples, "only channel 2 decoded")

	got, _ = readSectors(t, true, cdModeXAFilter, 1, 2)
	assert.Equal(t, []byte{0, 3}, got, "filter on without XA: audio sectors are still dropped")

	got, _ = readSectors(t, true, 0)
	assert.Equal(t, []byte{0, 1, 2, 3}, got, "XA off: every sector is data")

	got, samples = readSectors(t, false, cdModeXAADPCM)
	assert.Equal(t, []byte{0, 1, 2, 3}, got, "CD audio off, as psxe traces: every sector is data")
	assert.Equal(t, 0, samples)
}

func TestSPUCapture(t *testing.T) {
	s := newSPU(&ic{setIRQ: func(bool) {}})
	irqs := 0
	s.ic.setIRQ = func(bool) { irqs++ }
	s.ic.mask = irqSPU
	s.capture = true
	s.Write16(spuIRQAddr, 0x80>>3) // the left buffer's 40h'th sample
	s.Write16(spuRAMCtrl, 4)
	s.Write16(spuSPUCNT, 0x40)

	for i := range 0x110 {
		s.cdIn = [2]int16{int16(i), int16(-i)}
		s.sample()
	}
	assert.Equal(t, int16(0x40), int16(binary.LittleEndian.Uint16(s.ram[0x80:])))
	assert.Equal(t, int16(-0x40), int16(binary.LittleEndian.Uint16(s.ram[0x480:])))
	assert.Equal(t, 1, irqs, "a capture write at the IRQ address")
	assert.Equal(t, uint16(0x800), s.get16(spuSPUSTAT)&0x800, "writing the second half")
}

func TestSPUMixesCDThroughMute(t *testing.T) {
	s := newSPU(&ic{setIRQ: func(bool) {}})
	s.setAudio(true)
	s.Write16(spuCDVolL, 0x4000)
	s.Write16(spuCDVolR, 0x4000)
	s.Write16(spuMainVolL, 0x3fff)
	s.Write16(spuMainVolR, 0x3fff)
	s.Write16(spuSPUCNT, 0x0001) // CD input on, SPU muted
	s.cdIn = [2]int16{0x2000, -0x2000}
	s.sample()
	out := s.takeAudio(nil)
	assert.Equal(t, []int16{mainVolume(0x1000, 0x3fff), mainVolume(-0x1000, 0x3fff)}, out)

	s.Write16(spuSPUCNT, 0x0000) // CD input off
	s.sample()
	assert.Equal(t, []int16{0, 0}, s.takeAudio(nil))
}
