package emu

import "encoding/binary"

// CD audio: CD-DA tracks played with Play, and XA-ADPCM sectors decoded as
// they are read. psxe pulls both from its host audio callback, so what they
// change, the drive's position, its report and pause interrupts, and the
// SPU's capture buffers, depends on the host; while tracing it leaves them
// out. Here they run in emulated time, a sample at a time with the SPU,
// and follow the hardware as psx-spx documents it rather than psxe.

// Mode bits for audio.
const (
	cdModeAutoPause = 0x02
	cdModeReport    = 0x04
	cdModeXAFilter  = 0x08
)

// cdSamples is the stereo samples in a CD-DA sector: 2352 bytes of 16-bit
// left and right pairs, 75 sectors a second at 44.1kHz.
const cdSamples = sectorSize / 4

// cdAudio is the drive's audio output, a sample at a time at 44.1kHz.
type cdAudio struct {
	// CD-DA: the sector being played, the next sample in it, and the
	// track that play started in, for AutoPause.
	cdda      [cdSamples][2]int16
	cddaPos   int
	cddaPeak  uint16
	playTrack int

	// XA-ADPCM: the decoder and the 44.1kHz samples it has made and not
	// yet played, a ring of them.
	xa     xaDecoder
	fifo   [1 << 14][2]int16
	r, n   int
	fromXA bool // whether what plays is XA, which the ADPCM mute bit mutes
}

// push adds a sample to the XA output, dropping the oldest if it is full.
func (a *cdAudio) push(l, r int16) {
	if a.n == len(a.fifo) {
		a.r = (a.r + 1) % len(a.fifo)
		a.n--
	}
	a.fifo[(a.r+a.n)%len(a.fifo)] = [2]int16{l, r}
	a.n++
}

// pop takes the next XA sample, or silence if there is none.
func (a *cdAudio) pop() (l, r int16) {
	if a.n == 0 {
		return 0, 0
	}
	s := a.fifo[a.r]
	a.r = (a.r + 1) % len(a.fifo)
	a.n--
	return s[0], s[1]
}

// audioSample is the drive's next sample for the SPU, as it leaves the
// drive: after Mute, the ADPCM mute bit for XA, and the ATV volume matrix,
// in which 80h is unity. CD-DA plays while Play is playing; otherwise the
// XA output does, which is silent when nothing is being decoded.
func (c *cdrom) audioSample() (l, r int16) {
	if c.cddaPlaying {
		l, r = c.cddaSample()
		c.audio.fromXA = false
	} else {
		l, r = c.audio.pop()
		c.audio.fromXA = true
	}
	if c.mute || c.audio.fromXA && c.xaMute {
		return 0, 0
	}
	ll, rr := int32(l), int32(r)
	v := c.vol
	return clamp16((ll*int32(v[0]) + rr*int32(v[3])) >> 7),
		clamp16((rr*int32(v[2]) + ll*int32(v[1])) >> 7)
}

// playStart starts CD-DA play at the current position.
func (c *cdrom) playStart() {
	c.audio.cddaPos = cdSamples
	c.audio.playTrack = c.disc.subqTrack(c.lba)
}

// cddaSample is the next CD-DA sample. A new sector is read each time one
// is used up, so play advances 75 sectors a second; at double speed every
// other sample is skipped, so it advances twice as fast, at twice the
// pitch. Data sectors play as silence.
func (c *cdrom) cddaSample() (l, r int16) {
	a := &c.audio
	if a.cddaPos >= cdSamples && !c.nextCDDASector() {
		return 0, 0
	}
	s := a.cdda[a.cddaPos]
	a.cddaPos++
	if c.mode&cdModeSpeed != 0 {
		a.cddaPos++
	}
	return s[0], s[1]
}

// nextCDDASector reads the sector at the play position into the CD-DA
// buffer and moves on, reporting its position if reports are on. At the
// end of the disc, or with AutoPause at the end of the track, play stops
// with INT4. It reports whether there is a sector to play.
func (c *cdrom) nextCDDASector() bool {
	a := &c.audio
	kind := c.disc.query(c.lba)
	track := c.disc.subqTrack(c.lba)
	switch {
	case kind == sectorFar:
		// The motor stops at the end of the disc.
		c.cddaPlaying = false
		c.endPlay()
		return false
	case c.mode&cdModeAutoPause != 0 && track != a.playTrack:
		// The drive stays at the end of the old track.
		c.lba--
		c.cddaPlaying = false
		c.endPlay()
		return false
	}

	var buf [sectorSize]byte
	if c.disc.read(c.lba, buf[:]) != sectorAudio {
		clear(buf[:])
	}
	a.cddaPeak = 0
	for i := range a.cdda {
		l := int16(binary.LittleEndian.Uint16(buf[4*i:]))
		r := int16(binary.LittleEndian.Uint16(buf[4*i+2:]))
		a.cdda[i] = [2]int16{l, r}
		a.cddaPeak = max(a.cddaPeak, absPeak(l), absPeak(r))
	}
	a.cddaPos = 0
	if c.mode&cdModeReport != 0 {
		c.report(track)
	}
	c.lba++
	return true
}

func absPeak(v int16) uint16 {
	if v < 0 {
		return uint16(min(-int32(v), 0x7fff))
	}
	return uint16(v)
}

// report sends a play report, INT1, on the sectors psx-spx lists: at
// frames 0, 20, 40 and 60 of each second the absolute time, and at 10, 30,
// 50 and 70 the time within the track, with bit 7 of the seconds set. The
// last two bytes are the sector's peak level, without the left/right
// toggle the hardware's comes with. A report due while another interrupt
// is pending is dropped.
func (c *cdrom) report(track int) {
	frame := c.lba % 75
	if frame%10 != 0 || c.ifr&0x1f != 0 || !c.response.empty() {
		return
	}
	index := 1
	if c.disc.query(c.lba) == sectorPregap {
		index = 0
	}
	t, secOff := c.lba, uint8(0)
	if frame%20 != 0 {
		trackLBA := c.disc.trackLBA(track)
		if t >= trackLBA {
			t -= trackLBA
		} else {
			t = trackLBA - t
		}
		secOff = 0x80
	}
	peak := c.audio.cddaPeak
	c.respond(1, c.stat(), itob(track), itob(index),
		itob(int(t/4500)), itob(int(t/75%60))|secOff, itob(int(t%75)),
		uint8(peak), uint8(peak>>8))
	if c.ifr&c.ier != 0 {
		c.ic.raise(irqCDROM)
	}
}

// endPlay ends play with INT4(stat): DataEnd. A command in progress is
// left to finish, returning to idle rather than to play.
func (c *cdrom) endPlay() {
	if c.state == cdPlaying {
		c.state = cdIdle
	}
	if c.prevState == cdPlaying {
		c.prevState = cdIdle
	}
	if c.ifr&0x1f != 0 {
		return
	}
	c.respond(4, c.stat())
	if c.ifr&c.ier != 0 {
		c.ic.raise(irqCDROM)
	}
}

// xaSector routes a sector read with XA-ADPCM on, as psx-spx describes the
// controller doing: an audio sector in real time, if the filter is off or
// it matches the filter's file and channel, goes to the ADPCM decoder;
// with the filter on, an audio sector that does not match is dropped. It
// reports whether the sector went either way, rather than to the CPU.
func (c *cdrom) xaSector(buf []byte) bool {
	const (
		subFile, subChannel, subMode, subCoding = 0x10, 0x11, 0x12, 0x13
		submodeAudio, submodeRealTime           = 0x04, 0x40
	)
	if buf[0x0f] != 2 || buf[subMode]&(submodeAudio|submodeRealTime) != submodeAudio|submodeRealTime {
		return false
	}
	filter := c.mode&cdModeXAFilter != 0
	match := buf[subFile] == c.xaFile && buf[subChannel] == c.xaChannel
	if c.mode&cdModeXAADPCM != 0 && (!filter || match) {
		c.audio.xa.decodeSector(buf, &c.audio)
		return true
	}
	return filter
}

// xaDecoder decodes XA-ADPCM sectors to 44.1kHz, as psx-spx describes:
// each sector's 18 portions of 128 bytes hold 4-bit (or 8-bit) samples in
// blocks of 28, each block with a shift and one of four filters over the
// two samples before it. The 37.8kHz result goes through a ring of 32
// samples per channel, and after every six samples, seven are made from it
// with the zigzag tables; at 18.9kHz each sample goes in twice.
type xaDecoder struct {
	old, older [2]int32 // per channel; mono uses the first
	ring       [2][32]int16
	p          int
	sixStep    int
}

// XA-ADPCM filters' coefficients, in 64ths: only the first four of SPU
// ADPCM's.
var (
	xaPos = [4]int32{0, 60, 115, 98}
	xaNeg = [4]int32{0, 0, -52, -55}
)

// decodeSector decodes a whole sector, as raw 2352 bytes, into out.
func (x *xaDecoder) decodeSector(buf []byte, out *cdAudio) {
	coding := buf[0x13]
	stereo := coding&3 == 1
	half := coding>>2&3 == 1
	eightBit := coding>>4&3 == 1

	blocks := 8 // per portion: 4-bit nibbles of a 32-bit word
	if eightBit {
		blocks = 4
	}
	var samples [2][28]int16
	for portion := range 18 {
		src := buf[24+portion*128 : 24+portion*128+128]
		for blk := 0; blk < blocks; blk++ {
			ch := 0
			if stereo {
				ch = blk & 1
			}
			x.decodeBlock(src, blk, eightBit, ch, &samples[ch])
			// A stereo pair of blocks is output once its right block is
			// decoded; a mono block straight away.
			switch {
			case !stereo:
				for _, s := range samples[0] {
					x.output(s, s, half, out)
				}
			case ch == 1:
				for i := range 28 {
					x.output(samples[0][i], samples[1][i], half, out)
				}
			}
		}
	}
}

// decodeBlock decodes block blk of a portion into dst, with channel ch's
// history: each sample, sign-extended, shifted up to 16 bits and down by
// the header's shift, plus the filter's share of the two before it,
// clamped to 16 bits. Shifts 13 to 15 act as 9.
func (x *xaDecoder) decodeBlock(src []byte, blk int, eightBit bool, ch int, dst *[28]int16) {
	hdr := src[4+blk]
	shift := uint(hdr & 0xf)
	if shift > 12 {
		shift = 9
	}
	filter := hdr >> 4 & 3
	f0, f1 := xaPos[filter], xaNeg[filter]
	old, older := x.old[ch], x.older[ch]
	for j := range 28 {
		word := binary.LittleEndian.Uint32(src[16+j*4:])
		var t int32
		if eightBit {
			t = int32(int8(word>>(8*blk))) << 8
		} else {
			t = int32(int8(word>>(4*blk))<<4) << 8
		}
		s := t>>shift + (old*f0+older*f1+32)>>6
		s = int32(clamp16(s))
		dst[j] = int16(s)
		older, old = old, s
	}
	x.old[ch], x.older[ch] = old, older
}

// output takes a 37.8kHz sample pair, or an 18.9kHz one twice, into the
// zigzag ring, and after every sixth, makes seven 44.1kHz samples.
func (x *xaDecoder) output(l, r int16, half bool, out *cdAudio) {
	n := 1
	if half {
		n = 2
	}
	for range n {
		x.ring[0][x.p&31] = l
		x.ring[1][x.p&31] = r
		x.p++
		if x.sixStep--; x.sixStep > 0 {
			continue
		}
		x.sixStep = 6
		for t := range xaZigZag {
			out.push(x.zigzag(0, t), x.zigzag(1, t))
		}
	}
}

// zigzag interpolates channel ch's ring with zigzag table t: a sum of the
// last 29 samples, each weighted in 32768ths, clamped to 16 bits.
func (x *xaDecoder) zigzag(ch, t int) int16 {
	var sum int32
	for i := 1; i <= 29; i++ {
		sum += int32(x.ring[ch][(x.p-i)&31]) * xaZigZag[t][i-1] >> 15
	}
	return clamp16(sum)
}

// xaZigZag is the seven zigzag interpolation tables, as psx-spx gives
// them, each of 29 weights for the samples before the ring's position,
// nearest first. psx-spx notes they give nearly, but not exactly, the
// hardware's results. Each sums to about 7400h, a gain of 0.91.
var xaZigZag = [7][29]int32{
	{0x0000, 0x0000, 0x0000, 0x0000, 0x0000, -0x0002, 0x000a, -0x0022, 0x0041, -0x0054, 0x0034, 0x0009, -0x010a, 0x0400, -0x0a78, 0x234c, 0x6794, -0x1780, 0x0bcd, -0x0623, 0x0350, -0x016d, 0x006b, 0x000a, -0x0010, 0x0011, -0x0008, 0x0003, -0x0001},
	{0x0000, 0x0000, 0x0000, -0x0002, 0x0000, 0x0003, -0x0013, 0x003c, -0x004b, 0x00a2, -0x00e3, 0x0132, -0x0043, -0x0267, 0x0c9d, 0x74bb, -0x11b4, 0x09b8, -0x05bf, 0x0372, -0x01a8, 0x00a6, -0x001b, 0x0005, 0x0006, -0x0008, 0x0003, -0x0001, 0x0000},
	{0x0000, 0x0000, -0x0001, 0x0003, -0x0002, -0x0005, 0x001f, -0x004a, 0x00b3, -0x0192, 0x02b1, -0x039e, 0x04f8, -0x05a6, 0x7939, -0x05a6, 0x04f8, -0x039e, 0x02b1, -0x0192, 0x00b3, -0x004a, 0x001f, -0x0005, -0x0002, 0x0003, -0x0001, 0x0000, 0x0000},
	{0x0000, -0x0001, 0x0003, -0x0008, 0x0006, 0x0005, -0x001b, 0x00a6, -0x01a8, 0x0372, -0x05bf, 0x09b8, -0x11b4, 0x74bb, 0x0c9d, -0x0267, -0x0043, 0x0132, -0x00e3, 0x00a2, -0x004b, 0x003c, -0x0013, 0x0003, 0x0000, -0x0002, 0x0000, 0x0000, 0x0000},
	{-0x0001, 0x0003, -0x0008, 0x0011, -0x0010, 0x000a, 0x006b, -0x016d, 0x0350, -0x0623, 0x0bcd, -0x1780, 0x6794, 0x234c, -0x0a78, 0x0400, -0x010a, 0x0009, 0x0034, -0x0054, 0x0041, -0x0022, 0x000a, -0x0001, 0x0000, 0x0001, 0x0000, 0x0000, 0x0000},
	{0x0002, -0x0008, 0x0010, -0x0023, 0x002b, 0x001a, -0x00eb, 0x027b, -0x0548, 0x0afa, -0x16fa, 0x53e0, 0x3c07, -0x1249, 0x080e, -0x0347, 0x015b, -0x0044, -0x0017, 0x0046, -0x0023, 0x0011, -0x0005, 0x0000, 0x0000, 0x0000, 0x0000, 0x0000, 0x0000},
	{-0x0005, 0x0011, -0x0023, 0x0046, -0x0017, -0x0044, 0x015b, -0x0347, 0x080e, -0x1249, 0x3c07, 0x53e0, -0x16fa, 0x0afa, -0x0548, 0x027b, -0x00eb, 0x001a, 0x002b, -0x0023, 0x0010, -0x0008, 0x0002, 0x0000, 0x0000, 0x0000, 0x0000, 0x0000, 0x0000},
}
