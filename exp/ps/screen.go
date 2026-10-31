package ps

import "iter"

// A Screen is the double-buffered picture and the frames drawn into it.
// The fields are its settings, read by Open; from then on Frames gives
// out the frames, each of which draws what the loop's body puts on it:
//
//	var screen = ps.Screen{Background: ps.Color{R: 60, G: 120, B: 120}}
//
//	screen.Open()
//	for frame := range screen.Frames() {
//		ship.Draw(frame, 0)
//	}
//
// Behind each frame is an ordering table, which sorts what is drawn by
// depth, and the space the GPU's commands are built in; the screen has
// two of each, one a display buffer, and allocates them in Open.
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
	Offset Point

	// Dither says whether shaded colors are dithered, which smooths a
	// gouraud polygon's bands and leaves a sprite alone.
	Dither bool

	// SideBySide puts the two display buffers next to each other at the
	// top of video memory, at 0, 0 and Width, 0, rather than one above
	// the other, leaving the rows below Height whole for textures. It
	// is for a picture no wider than 512.
	SideBySide bool

	// Center puts the origin in the middle of the picture rather than its
	// top left corner, which is where 3D drawing wants it: a camera looks
	// through the middle. Sprites and text are then placed from the
	// middle too, or from the corner Bounds gives.
	Center bool

	// OverlayDepth, when it is not zero, gives each frame an overlay of
	// 1<<OverlayDepth depths, drawn after the frame: for what goes in
	// front of everything the frame sorted, a game's panels and text,
	// sorted among themselves. Frame.Overlay returns it.
	OverlayDepth int

	// impl is the library's side of the screen, its tables and packet
	// space, which only a console build has.
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
// at Width by Height with its origin at the top left, the two display
// buffers, and the ordering tables and packet space from the heap.
func (s *Screen) Open() {
	if s.Width == 0 {
		s.Width = 320
	}
	if s.Height == 0 {
		s.Height = 240
	}
	if s.Depth == 0 {
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

// Close puts the display back as the BIOS had it, for a program that is
// done drawing.
func (s *Screen) Close() { resetGraph() }

// Bounds is the picture as a frame's coordinates see it: its top left
// corner, which is 0, 0 unless Center moved the origin, and its size.
//
//	b := screen.Bounds()
//	box.MoveTo(b.X+24, b.Y+b.H-64)
func (s *Screen) Bounds() Rect {
	x, y := origin()
	return Rect{X: -x, Y: -y, W: s.Width, H: s.Height}
}

// Clamp returns x and y moved as little as they need to be for a w by h
// box at them to lie inside the rectangle.
func (r Rect) Clamp(x, y, w, h int) (int, int) {
	return min(max(x, r.X), r.X+r.W-w), min(max(y, r.Y), r.Y+r.H-h)
}

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
