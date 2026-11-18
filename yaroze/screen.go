package yaroze

import (
	"iter"
	"math/bits"

	"go.chrisrx.dev/psx/geom"
)

// A Screen is the double-buffered picture and the frames drawn into it.
// The fields are its settings, read by Open; from then on Frames gives
// out the frames, each of which draws what the loop's body puts on it:
//
//	var screen = yaroze.Screen{Background: yaroze.Color{R: 60, G: 120, B: 120}}
//
//	screen.Open()
//	for frame := range screen.Frames() {
//		ship.Draw(frame, 0)
//	}
//
// Behind each frame is an ordering table, which sorts what is drawn by
// depth, and the space the GPU's commands are built in; the screen has
// two of each, one a display buffer, and allocates them in Open unless
// Init gave it some first.
//
// A program that sorts with the library's own calls takes the frame's
// ordering table from Frame.OT, or drives the frame itself with Begin
// and End:
//
//	for frame := range screen.Frames() {
//		sprite.SortFast(frame.OT(), 0)
//	}
type Screen struct {
	// frame counts the frames finished. It is the first field so that a
	// test can read it at the screen's own address.
	frame uint32

	// Width and Height are the picture's size in pixels, 320 by 240 if
	// they are zero. A picture taller than 256 lines is interlaced.
	Width, Height int

	// Depth is how finely the frame sorts what it draws: 1<<Depth depths,
	// 8 if it is zero, 256 of them. A frame that draws at one depth, a
	// 2D game whose order of drawing is its order of depth, can have 1.
	Depth int

	// Packets is the space a frame's GPU commands have, in bytes, 16KB
	// if it is zero: a sprite takes about 24 bytes, a rotated or scaled
	// one 40.
	Packets int

	// Background is the color each frame starts from.
	Background Color

	// Interval is the vertical blanks a frame lasts: 1, or 0, for sixty
	// frames a second on an NTSC console, 2 for thirty.
	Interval int

	// Offset moves the picture on the television, for one that sits too
	// high or too far left.
	Offset geom.Point

	// NoDither turns dithering off. Shaded colors are dithered unless it
	// is set, which smooths a gouraud polygon's bands and leaves a sprite
	// alone.
	NoDither bool

	// Buffers is where the two display buffers lie in video memory:
	// one above the other, the default, or side by side.
	Buffers ScreenBuffers

	// Origin is where a frame's coordinates start on the picture: its
	// top left corner, the default, or its middle, which is where 3D
	// drawing wants it.
	Origin ScreenOrigin

	// OverlayDepth, when it is not zero, gives each frame an overlay of
	// 1<<OverlayDepth depths, drawn after the frame: for what goes in
	// front of everything the frame sorted, a game's panels and text,
	// sorted among themselves. Frame.Overlay returns it.
	OverlayDepth int

	// impl is the screen's tables and packet space.
	impl   screenImpl
	active int
	cur    Frame
	over   Frame

	// text is the window Font opened, flushed at the end of each frame,
	// or -1.
	text int32

	// backdrop is the picture SetBackdrop gave, copied under each frame,
	// or nil.
	backdrop             *uint32
	backdropW, backdropH int16

	// kanji is the Kanji windows KanjiFont opened, flushed at the end of
	// each frame after the font's window.
	kanji  [4]int32
	nkanji int

	fields    int
	lastField int32
}

// A screenImpl is the screen's storage: the two ordering tables, the
// overlay's two, and the two frames' packet space.
type screenImpl struct {
	ot      [2]GsOT
	overlay [2]GsOT
	packets [2][]uint32
}

// A frameTable is the ordering table a frame sorts into.
type frameTable = *GsOT

// A ScreenBuffers is where a screen's two display buffers lie in video
// memory.
type ScreenBuffers uint8

const (
	// ScreenBuffersStacked puts the buffers one above the other, at 0, 0
	// and 0, Height.
	ScreenBuffersStacked ScreenBuffers = iota

	// ScreenBuffersSideBySide puts them next to each other at the top of
	// video memory, at 0, 0 and Width, 0, leaving the rows below Height
	// whole for textures. It is for a picture no wider than 512.
	ScreenBuffersSideBySide
)

// A ScreenOrigin is where a frame's coordinates start on the picture.
type ScreenOrigin uint8

const (
	// ScreenOriginTopLeft puts 0, 0 at the picture's top left corner.
	ScreenOriginTopLeft ScreenOrigin = iota

	// ScreenOriginMiddle puts 0, 0 in the middle of the picture, which is
	// where 3D drawing wants it: a camera looks through the middle.
	// Sprites and text are then placed from the middle too, or from the
	// corner Bounds gives.
	ScreenOriginMiddle
)

// A VideoMode is the television standard the picture is sent as.
type VideoMode int32

const (
	NTSC VideoMode = 0 // sixty fields a second, 240 lines
	PAL  VideoMode = 1 // fifty, 256
)

// SetVideo switches the picture to mode, from the console's own that
// Open chose.
func (s *Screen) SetVideo(mode VideoMode) { setVideo(mode) }

// Shift moves the picture on the television by dx, dy from where it is,
// as Offset placed it.
func (s *Screen) Shift(dx, dy int) { shiftOffset(dx, dy) }

// SetBackdrop puts the picture in the TIM file tim, a 16-bit image of
// the screen's size, under every frame from now on: it is copied into
// the buffer the frame is drawn into before the frame is, in place of
// the clear to Background, so a frame starts from it. The file's words
// stay in use. A nil tim takes the backdrop away.
func (s *Screen) SetBackdrop(tim []uint32) {
	if tim == nil {
		s.backdrop = nil
		return
	}
	s.backdrop, s.backdropW, s.backdropH = backdropOf(tim)
}

// A Frame is one picture being drawn: what is drawn on it is sorted by
// depth, 0 in front, and drawn when the loop that gave it comes round.
type Frame struct {
	s     *Screen
	table frameTable
	depth int
}

// Number is which frame this is, counting from the first Frames gave.
func (f *Frame) Number() int { return int(f.s.frame) }

// Screen is the screen the frame is drawn on.
func (f *Frame) Screen() *Screen { return f.s }

// OT is the frame's ordering table, for sorting into it with the
// library's own calls.
func (f *Frame) OT() *GsOT { return f.table }

// Depths is how many depths the frame sorts into: 1<<Screen.Depth, or
// 1<<Screen.OverlayDepth for the overlay.
func (f *Frame) Depths() int { return 1 << f.depth }

// Overlay is the frame's overlay, drawn after the frame in front of
// everything on it, with its own depths, or nil when the screen has no
// OverlayDepth. It is drawn on as the frame is:
//
//	panel.Draw(frame.Overlay(), 0)
func (f *Frame) Overlay() *Frame {
	if f.s.OverlayDepth == 0 {
		return nil
	}
	return &f.s.over
}

// Open sets the display up and gives the screen its storage: the
// console's own video mode, sixty frames a second or fifty, the picture
// at Width by Height with its origin where Origin puts it, the two display
// buffers, and, unless Init gave it some, the ordering tables and packet
// space from the heap.
func (s *Screen) Open() {
	if s.Width == 0 {
		s.Width = 320
	}
	if s.Height == 0 {
		s.Height = 240
	}
	// A screen Init gave its tables keeps their depth, 0 included.
	if s.Depth == 0 && s.impl.ot[0].Org == nil {
		s.Depth = 8
	}
	if s.Packets == 0 {
		s.Packets = 16 << 10
	}
	s.text = -1
	s.cur = Frame{s: s, depth: s.Depth}
	s.over = Frame{s: s, depth: s.OverlayDepth}
	openScreen(s)
}

// Init gives the screen storage of the program's own, in place of what
// Open would allocate: tags for both ordering tables, which it splits in
// half, so the array has 2<<n entries for tables of 1<<n depths, and
// panics given any other count; and packets for both frames' GPU
// commands, split in half the same way, each half large enough for
// everything one frame sorts. It sets Depth and Packets to match. The
// packets are words rather than PACKET bytes so that they are aligned as
// the library's stores need; see Packets.
//
//	const otLength = 4
//	var otTags [2 << otLength]yaroze.GsOT_TAG
//	var packets [2 * 4096 / 4]uint32
//	screen.Init(otTags[:], packets[:])
//
// A program that sets the display up itself, with GsInitGraph and
// GsDefDispBuff, calls Init and not Open, and drives each frame with
// Begin and End.
func (s *Screen) Init(tags []GsOT_TAG, packets []uint32) {
	n := len(tags) / 2
	if n == 0 || len(tags) != 2*n || n&(n-1) != 0 {
		panic("yaroze: Screen.Init takes 2<<n tags, two tables of 1<<n")
	}
	s.Depth = bits.Len(uint(n)) - 1
	s.Packets = len(packets) / 2 * 4
	for i := range s.impl.ot {
		s.impl.ot[i].Length = uint32(s.Depth)
		s.impl.ot[i].Org = &tags[i*n]
		s.impl.packets[i] = packets[i*len(packets)/2 : (i+1)*len(packets)/2]
	}
	s.text = -1
	s.cur = Frame{s: s, depth: s.Depth}
	s.over = Frame{s: s, depth: s.OverlayDepth}
}

// Close puts the display back as the BIOS had it, for a program that is
// done drawing.
func (s *Screen) Close() { resetGraph() }

// Bounds is the picture as a frame's coordinates see it: its top left
// corner, which is 0, 0 unless Origin moved the origin, and its size.
//
//	b := screen.Bounds()
//	box.MoveTo(b.X+24, b.Y+b.H-64)
func (s *Screen) Bounds() geom.Rect {
	x, y := origin()
	return geom.Rect{
		X: -x,
		Y: -y,
		W: s.Width,
		H: s.Height,
	}
}

// Frame returns how many frames have been finished, for animation that
// keeps time by the frame.
func (s *Screen) Frame() int { return int(s.frame) }

// Begin starts a frame: it finds the buffer the frame is drawn into,
// points the library at the frame's packet space, clears the frame's
// ordering table, and returns it for the frame's sorts. Frames calls it
// each time round; a program that keeps its own loop calls it and End.
func (s *Screen) Begin() *GsOT { return beginFrame(s).table }

// OT returns the ordering table of the frame Begin started, for code
// that did not keep what Begin returned.
func (s *Screen) OT() *GsOT { return &s.impl.ot[s.active] }

// Overlay returns the overlay's ordering table for the frame Begin
// started, which End draws after the frame's, or nil without
// OverlayDepth.
func (s *Screen) Overlay() *GsOT {
	if s.OverlayDepth == 0 {
		return nil
	}
	return &s.impl.overlay[s.active]
}

// End finishes the frame: it waits for the GPU to finish the last one
// and for the display to reach the vertical blank, or the Interval'th,
// swaps the buffers, puts the background behind everything, starts the
// GPU on the frame, then on the overlay, and then draws the text of the
// windows Font and KanjiFont opened. It returns what VSync reported, the
// frame's length in horizontal lines.
func (s *Screen) End() int32 { return endFrame(s) }

// Frames is the frame loop: each time round it starts a frame, the body
// draws on it, and the frame is finished and shown when the body is
// done. A break leaves before the frame it is in is drawn.
func (s *Screen) Frames() iter.Seq[*Frame] {
	return func(yield func(*Frame) bool) {
		for yield(beginFrame(s)) {
			endFrame(s)
		}
	}
}

// Fields is how many vertical blanks the last frame took: 1 when it was
// drawn within one, 2 when its work ran past the blank and the frame
// waited for the next, which halves the frame rate while it lasts. A
// game that moves things a fixed amount a frame can move them that many
// times, so that its speed holds whatever is on the screen.
func (s *Screen) Fields() int { return max(s.fields, 1) }

// Wait lets n vertical blanks go by without drawing, for a pause.
func (s *Screen) Wait(n int) {
	for range n {
		vsync(0)
	}
}

// Font opens a window for text at x, y, w by h, holding up to chars
// characters at a time, in the console's own font, and returns it. What
// is printed to it during a frame is drawn at the frame's end, in front
// of everything else, and the window empties for the next.
func (s *Screen) Font(x, y, w, h, chars int) Text {
	id := fontOpen(x, y, w, h, chars)
	if s.text < 0 {
		s.text = id
	}
	return Text{id}
}
