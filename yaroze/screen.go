package yaroze

import "math/bits"

// A Screen is the double-buffered frame: two ordering tables, one per
// display buffer, and the packet space the library builds each frame's GPU
// commands in. Every frame has the same shape, which it carries out so that
// a program does not have to: Begin clears the table for the buffer being
// drawn and returns it for the frame's sorts, and End waits for the GPU and
// the display, swaps the buffers, and starts the GPU on the table:
//
//	for {
//		ot := screen.Begin()
//		sprite.SortFast(ot, 0)
//		screen.End(60, 120, 120)
//	}
//
// The storage is the program's, declared as package-level arrays and handed
// over once with Init, since nothing here allocates.
type Screen struct {
	ot      [2]GsOT
	packets [2][]uint32
	active  int
}

// Init gives the screen its storage: tags for both ordering tables, which
// it splits in half, so the array has 2<<n entries for tables of 1<<n
// depths; and packets for both frames' GPU commands, split in half the
// same way, each half large enough for everything one frame sorts. The
// packets are words rather than PACKET bytes so that they are aligned as
// the library's stores need; see Packets.
//
//	const otLength = 4
//	var otTags [2 << otLength]yaroze.GsOT_TAG
//	var packets [2 * 4096 / 4]uint32
//	screen.Init(otTags[:], packets[:])
func (s *Screen) Init(tags []GsOT_TAG, packets []uint32) {
	n := len(tags) / 2
	for i := range s.ot {
		s.ot[i].Length = uint32(bits.Len(uint(n)) - 1)
		s.ot[i].Org = &tags[i*n]
		s.packets[i] = packets[i*len(packets)/2 : (i+1)*len(packets)/2]
	}
}

// Begin starts a frame: it finds the buffer the frame is drawn into, points
// the library at the frame's packet space, clears the frame's ordering
// table, and returns it for the frame's sorts.
func (s *Screen) Begin() *GsOT {
	s.active = int(GsGetActiveBuff())
	GsSetWorkBase(Packets(&s.packets[s.active][0]))
	GsClearOt(0, 0, &s.ot[s.active])
	return &s.ot[s.active]
}

// End finishes the frame: it waits for the GPU to finish the last one and
// for the display to reach the vertical blank, swaps the buffers, sorts a
// clear of the screen to r, g, b in front of everything, and starts the
// GPU on the frame's table. It returns what VSync reported, the frame's
// length in horizontal lines. Text goes after it, since FntFlush draws into
// the buffer the swap has just made current.
func (s *Screen) End(r, g, b uint8) int32 {
	ot := &s.ot[s.active]
	DrawSync(0)
	n := VSync(0)
	GsSwapDispBuff()
	GsSortClear(r, g, b, ot)
	GsDrawOt(ot)
	return n
}

// OT returns the ordering table of the frame Begin started, for code that
// did not keep what Begin returned.
func (s *Screen) OT() *GsOT { return &s.ot[s.active] }
