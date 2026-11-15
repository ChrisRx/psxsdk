package yaroze

// A Cell is a kind of tile: a 16 by 16 square of an image in video
// memory, with its palette. Image.Cell cuts one.
type Cell struct {
	U, V uint8
	Clut uint16
	_    uint16
	Page uint16
}

// Cell returns the cell of the image whose top left corner is x, y
// pixels from the image's own.
func (im Image) Cell(x, y int) Cell {
	t := im.texture
	return Cell{
		U:    t.U + uint8(x),
		V:    t.V + uint8(y),
		Clut: uint16(t.CY)<<6 | uint16(t.CX)>>4,
		Page: t.Page,
	}
}

// A TileMap is a grid of cells: Cells are the kinds, and Layout says
// which kind is at each place, a row at a time, Width places across and
// Height down. The map can be any size; a Background shows a window of
// it.
type TileMap struct {
	Width, Height int
	Cells         []Cell
	Layout        []uint16

	// impl is the map as the library reads it, which Background.Init
	// fills from the fields above.
	impl tileMapImpl
}

// A tileMapImpl is the library's map: the cell size, the grid's size, and
// where the cells and the layout are.
type tileMapImpl struct {
	cellW, cellH  uint8
	width, height uint16
	_             [2]byte
	cells         *Cell
	layout        *uint16
}

// A Background is a window onto a tile map, drawn behind a frame's
// sprites: the window at X, Y, W by H on the screen, showing the map
// from ScrollX, ScrollY, a pixel at a time. Init prepares it for a
// screen, once, and Draw puts it on a frame.
//
//	var bg = yaroze.Background{W: 320, H: 240, Color: yaroze.Color{128, 128, 128}}
//
//	bg.Init(&screen, &tiles)
//	for frame := range screen.Frames() {
//		bg.ScrollX++
//		bg.Draw(frame, 1)
//	}
//
// Mode is read by Init, for Translucent; the rest can change between
// frames. A Background is the library's own layout, with its unused
// fields hidden, so drawing one copies nothing.
type Background struct {
	Mode             Mode
	X, Y             int16
	W, H             int16
	ScrollX, ScrollY int16

	// Color tints the cells: 128 a component leaves them as they are.
	Color Color
	_     uint8

	m *tileMapImpl
	_ [12]byte

	// work is where the library builds the window's packets, one a cell
	// position on the screen, for each display buffer.
	work []uint32
}

// Init points the background at the map, builds the map as the library
// reads it, and prepares the packets for a window on screen s, which
// must be open. It allocates the packets, once; InitBuffer takes them
// from the program instead.
func (bg *Background) Init(s *Screen, m *TileMap) {
	bg.InitBuffer(s, m, make([]uint32, WorkWords(s.Width, s.Height)))
}

// WorkWords is the words of packet space a background's window on a
// screen w by h takes: a list for each buffer of an offset packet, a
// cell packet of five words for every 16 pixels of the screen and one
// more each way, and an offset packet to end on, behind the two words
// that say where each list starts.
func WorkWords(w, h int) int {
	cols, rows := w/16+1, h/16+1
	return 2 + 2*(4+cols*rows*5)
}

// InitBuffer is Init with the packet space given, at least WorkWords
// of the screen's size, for a program that keeps its own.
func (bg *Background) InitBuffer(s *Screen, m *TileMap, work []uint32) {
	m.impl = tileMapImpl{
		cellW: 16, cellH: 16,
		width: uint16(m.Width), height: uint16(m.Height),
		cells: &m.Cells[0], layout: &m.Layout[0],
	}
	bg.m = &m.impl
	// The cells' color depth goes into the mode, where the library reads
	// it, from the first cell's page.
	bg.Mode = bg.Mode&^(3<<24) | Mode(m.Cells[0].Page>>7&3)<<24
	bg.work = work
	initBackground(bg)
}

// Draw puts the background on the frame at depth.
func (bg *Background) Draw(f *Frame, depth int) { sortBackground(bg, f, depth) }

// MoveTo puts the window's top left corner on the screen at x, y.
func (bg *Background) MoveTo(x, y int) { bg.X, bg.Y = int16(x), int16(y) }

// Size returns the window's width and height on the screen, in pixels.
func (bg *Background) Size() (w, h int) { return int(bg.W), int(bg.H) }

// Scroll returns how far the window is scrolled into the map, in pixels.
func (bg *Background) Scroll() (x, y int) { return int(bg.ScrollX), int(bg.ScrollY) }

// ScrollTo scrolls the window to x, y pixels into the map.
func (bg *Background) ScrollTo(x, y int) { bg.ScrollX, bg.ScrollY = int16(x), int16(y) }
