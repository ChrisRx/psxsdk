package yaroze

import (
	"iter"
	"math/bits"
)

// A Screen is the double-buffered frame: two ordering tables, one per
// display buffer, and the packet space the library builds each frame's GPU
// commands in. Every frame has the same shape, which it carries out so that
// a program does not have to: Begin clears the table for the buffer being
// drawn and returns it for the frame's sorts, and End waits for the GPU and
// the display, swaps the buffers, clears the frame to Background, and
// starts the GPU on the table. Frames puts the two around the body of a
// loop:
//
//	var screen = yaroze.Screen{Background: yaroze.CVECTOR{R: 60, G: 120, B: 120}}
//
//	screen.Open()
//	for ot := range screen.Frames() {
//		sprite.SortFast(ot, 0)
//	}
//
// The fields are the screen's settings, read by Open, and its storage is
// allocated there, once, unless Init gave it some first.
type Screen struct {
	// frame counts the frames End has finished. It is the first field so
	// that a test can read it at the screen's own address.
	frame uint32

	// Width and Height are the picture's size in pixels, 320 by 240 if
	// they are zero, on a European console too. A picture taller than 256
	// lines is interlaced. A European console can show 256 lines, but the
	// second buffer then reaches the bottom of video memory, rows 256 to
	// 511, where images often keep their palettes, at row 480 say, and the
	// clear each frame wipes them; a program that keeps its images clear of
	// there can ask for 256.
	Width, Height uint16

	// Depth is the ordering tables' depth as a power of two, 8 if it is
	// zero: each table holds 1<<Depth entries, the depths a frame's
	// primitives are sorted into.
	Depth int

	// Packets is the GPU commands a frame has room for, in bytes, 16KB if
	// it is zero. Each buffer has its own.
	Packets int

	// Background is the color End clears each frame to.
	Background CVECTOR

	// Interval is the vertical blanks a frame lasts: 1, or 0, for sixty
	// frames a second on an NTSC console, 2 for thirty.
	Interval int

	// SideBySide puts the two display buffers next to each other at the
	// top of video memory, at 0, 0 and Width, 0, rather than one above the
	// other, leaving the rows below Height whole for textures. It is for a
	// picture no wider than 512.
	SideBySide bool

	// OverlayDepth, when it is not zero, gives the screen a second ordering
	// table per buffer, of 1<<OverlayDepth depths, which End draws after
	// the frame's: for what goes in front of everything the frame sorted,
	// a game's panels and text, sorted among themselves. Overlay returns
	// it.
	OverlayDepth int

	ot      [2]GsOT
	overlay [2]GsOT
	packets [2][]uint32
	active  int

	// font is the stream Font opened, and text whether it has, so that End
	// flushes it.
	font int32
	text bool
}

// Open sets up the display and the screen. It puts the console in its own
// video mode, NativeVideoMode, sixty frames a second or fifty, and fills in
// the settings left zero; then
// GsInitGraph at Width by Height, interlaced for a picture taller than 256
// lines, with the GsOFSGPU origin, dithered and in 16-bit color; then
// GsDefDispBuff with the two buffers one above the other, or both at the
// top of video memory for an interlaced picture, which has no room for two;
// and then, unless Init gave it storage, it allocates the ordering tables
// and the packet space from the heap. A program that wants another
// arrangement calls those itself, and Init.
func (s *Screen) Open() {
	SetVideoMode(NativeVideoMode())
	if s.Width == 0 {
		s.Width = 320
	}
	if s.Height == 0 {
		s.Height = 240
	}
	if s.Packets == 0 {
		s.Packets = 16 << 10
	}
	if s.Height > 256 {
		GsInitGraph(s.Width, s.Height, GsINTER|GsOFSGPU, 1, 0)
		GsDefDispBuff(0, 0, 0, 0)
	} else {
		GsInitGraph(s.Width, s.Height, GsNONINTER|GsOFSGPU, 1, 0)
		if s.SideBySide {
			GsDefDispBuff(0, 0, s.Width, 0)
		} else {
			GsDefDispBuff(0, 0, 0, s.Height)
		}
	}
	if s.ot[0].Org == nil {
		if s.Depth == 0 {
			s.Depth = 8
		}
		s.Init(make([]GsOT_TAG, 2<<s.Depth), make([]uint32, 2*s.Packets/4))
	}
	if s.OverlayDepth != 0 && s.overlay[0].Org == nil {
		tags := make([]GsOT_TAG, 2<<s.OverlayDepth)
		for i := range s.overlay {
			s.overlay[i].Length = uint32(s.OverlayDepth)
			s.overlay[i].Org = &tags[i<<s.OverlayDepth]
		}
	}
}

// Init gives the screen storage of the program's own, in place of what Open
// would allocate: tags for both ordering tables, which it splits in half,
// so the array has 2<<n entries for tables of 1<<n depths; and packets for
// both frames' GPU commands, split in half the same way, each half large
// enough for everything one frame sorts. It sets Depth and Packets to
// match. The packets are words rather than PACKET bytes so that they are
// aligned as the library's stores need; see Packets.
//
//	const otLength = 4
//	var otTags [2 << otLength]yaroze.GsOT_TAG
//	var packets [2 * 4096 / 4]uint32
//	screen.Init(otTags[:], packets[:])
func (s *Screen) Init(tags []GsOT_TAG, packets []uint32) {
	n := len(tags) / 2
	s.Depth = bits.Len(uint(n)) - 1
	s.Packets = len(packets) / 2 * 4
	for i := range s.ot {
		s.ot[i].Length = uint32(s.Depth)
		s.ot[i].Org = &tags[i*n]
		s.packets[i] = packets[i*len(packets)/2 : (i+1)*len(packets)/2]
	}
}

// Font opens the on-screen font for the screen's frames: FntLoad puts the
// font into video memory at 960, 256, and FntOpen opens a window for text at
// x, y, w by h, holding up to chars characters, without a background. The
// window is placed from the origin Bounds gives. From then on End flushes
// what the frame printed, after the frame's table, which is when text has
// to be drawn. It returns the stream FntOpen opened.
func (s *Screen) Font(x, y, w, h, chars int32) int32 {
	FntLoad(960, 256)
	s.font = FntOpen(x, y, w, h, 0, chars)
	s.text = true
	return s.font
}

// Bounds is the screen as the frame's coordinates see it: the top left
// corner and the size. Where the corner is depends on the origin, which
// GsInitGraph puts at the top left, so that the corner is 0, 0, and
// GsInit3D in the middle, for 3D's sake, so that it is minus half the
// size.
//
//	b := screen.Bounds()
//	box.X, box.Y = b.Clamp(box.X+dx, box.Y+dy, box.W, box.H)
func (s *Screen) Bounds() RECT {
	o := POSITION()
	return RECT{X: -o.Offx, Y: -o.Offy, W: int16(s.Width), H: int16(s.Height)}
}

// Frame returns how many frames End has finished, for animation that keeps
// time by the frame.
func (s *Screen) Frame() int { return int(s.frame) }

// Begin starts a frame: it finds the buffer the frame is drawn into, points
// the library at the frame's packet space, clears the frame's ordering
// table, and returns it for the frame's sorts.
func (s *Screen) Begin() *GsOT {
	s.active = int(GsGetActiveBuff())
	GsSetWorkBase(Packets(&s.packets[s.active][0]))
	GsClearOt(0, 0, &s.ot[s.active])
	if s.OverlayDepth != 0 {
		GsClearOt(0, 0, &s.overlay[s.active])
	}
	return &s.ot[s.active]
}

// Overlay returns the frame's second ordering table, which End draws after
// the one Begin returned, or nil without OverlayDepth.
func (s *Screen) Overlay() *GsOT {
	if s.OverlayDepth == 0 {
		return nil
	}
	return &s.overlay[s.active]
}

// End finishes the frame: it waits for the GPU to finish the last one and
// for the display to reach the vertical blank, or the Interval'th, swaps the
// buffers, sorts a clear of the screen to Background in front of
// everything, starts the GPU on the frame's table, then on the overlay's
// if there is one, and then draws the frame's text if Font opened the
// font. It returns what VSync reported,
// the frame's length in horizontal lines. Without Font, text goes after
// it, since FntFlush draws into the buffer the swap has just made current.
func (s *Screen) End() int32 {
	ot := &s.ot[s.active]
	DrawSync(0)
	var n int32
	if s.Interval > 1 {
		n = VSync(int32(s.Interval))
	} else {
		n = VSync(0)
	}
	GsSwapDispBuff()
	GsSortClear(s.Background.R, s.Background.G, s.Background.B, ot)
	GsDrawOt(ot)
	if s.OverlayDepth != 0 {
		GsDrawOt(&s.overlay[s.active])
	}
	if s.text {
		FntFlush(s.font)
	}
	s.frame++
	return n
}

// Frames is the frame loop: each time round, Begin starts a frame, the
// loop's body sorts into the table it gives, and End finishes the frame. A
// break leaves before the frame it is in is drawn.
//
//	for ot := range screen.Frames() {
//		pad.Update()
//		if pad.Pressed(yaroze.PADselect) {
//			break
//		}
//		box.Sort(ot, 0)
//	}
func (s *Screen) Frames() iter.Seq[*GsOT] {
	return func(yield func(*GsOT) bool) {
		for yield(s.Begin()) {
			s.End()
		}
	}
}

// OT returns the ordering table of the frame Begin started, for code that
// did not keep what Begin returned.
func (s *Screen) OT() *GsOT { return &s.ot[s.active] }

// Clamp returns x and y moved as little as they need to be for a w by h box
// at them to lie inside the rectangle.
func (r RECT) Clamp(x, y int16, w, h uint16) (int16, int16) {
	return min(max(x, r.X), r.X+r.W-int16(w)), min(max(y, r.Y), r.Y+r.H-int16(h))
}
