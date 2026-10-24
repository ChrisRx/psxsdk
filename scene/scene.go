// Package scene draws a scene file, SCENE.DAT, which the playground's
// scene editor makes and mkscene cooks: models from the program's TMDs,
// each where the file puts it, under its camera and lights, and sprites
// from its TIMs. The program reads the file off the disc as it reads any
// other, hands it to Load with its files, and draws the scene each frame,
// moving what it likes between frames:
//
//	var world scene.Scene
//
//	loader.Start(300, files...)
//	for ot := range screen.Frames() {
//		if !world.Ready() {
//			if done, failed := loader.Step(); done && !failed {
//				world.Load(sceneDAT.Data(), files)
//			}
//			continue
//		}
//		world.Object("cube").Rotation.Vy += 16
//		world.Draw(ot, screen.Overlay())
//	}
//
// A scene's layers are the tile maps of a 2D game, drawn behind everything
// else, the first farthest: Layer finds one by its name, Cell and Flags
// say what is at a cell of its map, the flags being what the scene's
// editor set for that cell of the texture (Solid, OneWay, Hurts, Pickup
// and four bits of the program's own), SetCell changes a cell, as when a
// coin is taken, and Scroll moves every layer at its own speed, a far one
// slower than a near one:
//
//	ground := world.Layer("ground")
//	if ground.FlagsAt(x, y+16)&scene.Solid != 0 {
//		// standing on something
//	}
//	world.Scroll(cameraX, cameraY)
//
// A scene may instead be a room of cells for package stage to draw, with
// doorways linked to other scenes and markers naming cells for the
// program's own people and things: the program loads the room's file,
// hands Room's map to its stage.Setting, places what its markers name,
// and on a doorway loads the scene Link names. A room's file leaves the
// camera, the projection and the lights as the program set them, since
// the program's own camera draws it.
//
// A Scene holds its objects and sprites in arrays of a fixed size, the
// most a file holds, so that nothing is allocated; its layout is
// format/scene's.
package scene

import (
	"errors"
	"unsafe"

	"go.chrisrx.dev/psx/archive"
	"go.chrisrx.dev/psx/disc"
	"go.chrisrx.dev/psx/internal/volatile"
	"go.chrisrx.dev/psx/stage"
	"go.chrisrx.dev/psx/yaroze"
)

// The most of each a scene holds, as format/scene's.
const (
	MaxTextures = 16
	MaxModels   = 16
	MaxObjects  = 64
	MaxSprites  = 128
	MaxLights   = 3

	// A scene has at most MaxLayers layers. A layer's map holds at most
	// MaxMapCells, cut from at most MaxCells cells of its texture, each
	// CellSize pixels square, and is drawn through a window of at most
	// 320 by 240.
	MaxLayers   = 3
	MaxMapCells = 4096
	MaxCells    = 256
	EmptyCell   = 0xffff
	CellSize    = 16

	// A room has at most MaxLinks links, one a doorway digit, and
	// MaxMarkers markers.
	MaxLinks   = 10
	MaxMarkers = 64
	workWords  = ((320/CellSize+1)*(240/CellSize+2)*6+4)*2 + 2
)

// The flags the scene's editor names for a layer's cells; the other four
// bits are the program's own.
const (
	Solid  = 1 << 0 // nothing passes through it
	OneWay = 1 << 1 // stood on from above, passed through from below
	Hurts  = 1 << 2 // hurts what touches it
	Pickup = 1 << 3 // taken by what touches it
)

const (
	id           = 0x314E4353
	headerWords  = 0x68 / 4
	pathWords    = 8
	objectWords  = 16
	spriteWords  = 16
	nameWords    = 4
	lightsAt     = 0x38 / 4
	cameraAt     = 0x1C / 4
	ambientAt    = 0x10 / 4
	projectionAt = 0x0C / 4
)

// Load's errors.
var (
	ErrFormat  = errors.New("scene: not a scene file, or one with more in it than a Scene holds")
	ErrMissing = errors.New("scene: a file the scene needs is not loaded; Missing names it")
)

// live is the playground's way into a running program: where the scene
// last loaded has its file's words and how many there are, so that the
// editor can write a new scene over them, and a flag it then sets, which
// the next Draw or Reload sees and reads the file again. The editor
// finds it by its symbol, so the layout is fixed: the words' address,
// their count, the flag, and the Scene, each a word, then the name of
// the file the scene came from, 32 bytes, so that the editor knows which
// of a project's scenes the program is showing, then a cell of a room,
// its column and row in the low and high halves, a flag the editor sets
// to ask the program to put its figure there, which Teleport answers,
// and the disc path of the scene the cell is in, 32 bytes, empty for
// the one showing.
var live struct {
	data   *uint32
	words  uint32
	reload uint32
	scene  *Scene
	name   [32]byte
	cell   uint32
	place  uint32
	want   [32]byte
}

// setLive records a loaded scene in live, in a function of its own so that
// the compiler keeps live as a global the editor can find.
//
//go:noinline
func setLive(s *Scene, data []uint32, name string) {
	live.data = &data[0]
	live.words = uint32(len(data))
	live.scene = s
	live.name = [32]byte{}
	copy(live.name[:31], name)
	volatile.StoreUint32(&live.reload, 0)
	volatile.StoreUint32(&live.place, 0)
}

// A Scene is a scene file read in: what it places, ready to draw.
type Scene struct {
	Camera     yaroze.GsRVIEW2
	Projection int32
	Ambient    [3]int32
	Lights     [MaxLights]yaroze.GsF_LIGHT
	NLights    int

	// Textures and Models are the scene's TIMs and TMDs, in the order the
	// file lists them, each the memory its disc.File was loaded into.
	Textures  [MaxTextures]yaroze.TIM
	NTextures int
	Models    [MaxModels]yaroze.TMD
	NModels   int

	// Objects are the placed models, in the file's order, each linked to
	// its model and placed; the program moves them between frames.
	Objects  [MaxObjects]yaroze.Object
	NObjects int

	// Sprites are the file's sprites, in its order, ready to sort.
	Sprites  [MaxSprites]yaroze.GsSPRITE
	NSprites int

	// layers are the maps drawn behind everything, the first farthest.
	layers  [MaxLayers]Layer
	nLayers int

	// room is the room of cells the scene is, when hasRoom, with its
	// cells and heights as strings over the file's own bytes, its links
	// and its markers.
	room       Room
	hasRoom    bool
	cellRows   [stage.MaxCells]string
	heightRows [stage.MaxCells]string
	nLinks     int
	linkDoors  [MaxLinks]uint8
	linkScenes [MaxLinks]string
	nMarkers   int
	markers    [MaxMarkers]Marker
	markerName [MaxMarkers][16]byte

	name string // the disc file the scene came from, when it is one of files

	// Missing is the disc path Load could not find loaded among its files,
	// when it returns ErrMissing.
	Missing string

	objectNames [MaxObjects][16]byte
	spriteNames [MaxSprites][16]byte
	// Each object's model, an index into Models, and its number in the TMD.
	objectModel  [MaxObjects]uint8
	objectIndex  [MaxObjects]int32
	objectFields [MaxObjects][2]int32
	ready        bool

	// What Load was given, for reload.
	data []uint32
	src  Source
}

// A Layer is one of the scene's tile maps: the library's background, whose
// window and brightness the program may change, a map of cells cut from
// one texture, and a byte of flags for each of those cells, which says
// what the cell is to the program. The map is the scene's own copy, which
// SetCell changes and the file does not.
type Layer struct {
	// BG is the background GsSortFixBg16 draws. Scroll sets its Scrollx
	// and Scrolly, which the program may set itself instead.
	BG yaroze.GsBG

	// Speed is the percent of the scene's scroll the layer moves, as the
	// file says: 100 with it, less for one farther off.
	Speed int

	name    [16]byte
	tex     int
	scroll  [2]int16 // where the file scrolled it, where Scroll(0, 0) puts it
	tileMap yaroze.GsMAP
	cells   [MaxCells]yaroze.GsCELL
	index   [MaxMapCells]uint16
	flags   [MaxCells]uint8
	work    [workWords]uint32
}

// Name returns the layer's name.
func (l *Layer) Name() string { return nameString(&l.name) }

// Width and Height are the size of the layer's map in cells.
func (l *Layer) Width() int  { return int(l.tileMap.Ncellw) }
func (l *Layer) Height() int { return int(l.tileMap.Ncellh) }

// Cell returns the cell at column x and row y of the map, an index into
// the texture's cells, or EmptyCell for an empty one or one off the map.
func (l *Layer) Cell(x, y int) int {
	w, h := l.Width(), l.Height()
	if x < 0 || y < 0 || x >= w || y >= h {
		return EmptyCell
	}
	return int(l.index[y*w+x])
}

// SetCell puts cell c, an index into the texture's cells or EmptyCell, at
// column x and row y of the map, which shows from the next frame drawn.
// Off the map it does nothing.
func (l *Layer) SetCell(x, y, c int) {
	w, h := l.Width(), l.Height()
	if x < 0 || y < 0 || x >= w || y >= h {
		return
	}
	l.index[y*w+x] = uint16(c)
}

// Flags returns the flags of the cell at column x and row y, those the
// scene's editor set for that cell of the texture: Solid, OneWay, Hurts,
// Pickup and the program's own four bits. An empty cell, and one off the
// map, has none.
func (l *Layer) Flags(x, y int) uint8 {
	c := l.Cell(x, y)
	if c >= MaxCells {
		return 0
	}
	return l.flags[c]
}

// CellAt and FlagsAt are Cell and Flags at a point of the map, in pixels
// from its top left corner.
func (l *Layer) CellAt(px, py int) int    { return l.Cell(px>>4, py>>4) }
func (l *Layer) FlagsAt(px, py int) uint8 { return l.Flags(px>>4, py>>4) }

// Layer returns the layer the file names so, or nil.
func (s *Scene) Layer(name string) *Layer {
	for i := range s.nLayers {
		if nameIs(&s.layers[i].name, name) {
			return &s.layers[i]
		}
	}
	return nil
}

// NLayers is how many layers the scene has, and LayerAt layer i of them,
// the first the farthest.
func (s *Scene) NLayers() int         { return s.nLayers }
func (s *Scene) LayerAt(i int) *Layer { return &s.layers[i] }

// Scroll scrolls the layers to x, y, in the pixels of a layer that moves
// with the scroll: each moves its Speed's share of the way from where the
// file put it, so that a far layer of speed 50 moves half as far, and
// all of them line up again at 0, 0.
func (s *Scene) Scroll(x, y int) {
	for i := range s.nLayers {
		l := &s.layers[i]
		l.BG.Scrollx = int16(int(l.scroll[0]) + x*l.Speed/100)
		l.BG.Scrolly = int16(int(l.scroll[1]) + y*l.Speed/100)
	}
}

// A Room is a scene's room of cells, as package stage draws it, with the
// look round it: the sky's color at the top of the screen and at the
// bottom, or none when all zero, the color the screen clears to, or the
// program's own when zero, and how high above its figure and how far
// behind the program's camera sits, or its usual slant when zero.
type Room struct {
	stage.Map
	Sky            [2]stage.Color
	Background     stage.Color
	CamUp, CamBack int
}

// A Marker is a cell the file names for the program's own thing to
// stand on, with two fields of the program's own.
type Marker struct {
	I, J   int
	Fields [2]int32
}

// Room returns the scene's room, or nil when it has none. Its map's
// rows are strings over the file's own words, which the program keeps
// loaded while it draws the room.
func (s *Scene) Room() *Room {
	if !s.hasRoom {
		return nil
	}
	return &s.room
}

// Link returns the disc path of the scene the doorway with digit door
// leads to, or "" when the room has no link for it.
func (s *Scene) Link(door byte) string {
	for i := range s.nLinks {
		if s.linkDoors[i] == door {
			return s.linkScenes[i]
		}
	}
	return ""
}

// Marker returns the marker the file names so, or nil.
func (s *Scene) Marker(name string) *Marker {
	for i := range s.nMarkers {
		if nameIs(&s.markerName[i], name) {
			return &s.markers[i]
		}
	}
	return nil
}

// NMarkers is how many markers the room has, and MarkerName marker i's
// name.
func (s *Scene) NMarkers() int           { return s.nMarkers }
func (s *Scene) MarkerName(i int) string { return nameString(&s.markerName[i]) }

// Ready reports whether Load has read a file.
func (s *Scene) Ready() bool { return s.ready }

// Load reads the scene from data, the file's words as the disc package
// loaded them, binds its textures and models to the files src gives by
// their disc paths (Files for the disc's own, an Archive for the entries
// of one, Sources for both), sends the textures to video memory, maps the
// models, links and places the objects, and sets the camera, the
// projection and the lights. It returns ErrFormat for a file it cannot
// read and ErrMissing, with Missing set, for a texture or model src does
// not have or has not loaded.
func (s *Scene) Load(data []uint32, src Source) error {
	if err := s.parse(data, src); err != nil {
		return err
	}
	s.setup()
	s.ready = true
	s.data = data
	s.src = src
	s.name = ""
	if n, ok := src.(namer); ok {
		s.name = n.nameOf(data)
	}
	if len(data) > 0 {
		setLive(s, data, s.name)
	}
	return nil
}

// Name is the disc path of the file the scene was loaded from, when the
// data given to Load was one of its files, or "".
func (s *Scene) Name() string { return s.name }

// Reload reads the file again when the playground has written a new one
// over it, the editor's live view of the scene, and reports whether it
// did, so that a program that draws a room through its own stage.Setting
// can load the room again. Draw calls it; a program that does not Draw
// calls it itself each frame. A file it cannot read leaves the scene as
// it was.
func (s *Scene) Reload() bool {
	if live.scene != s || volatile.LoadUint32(&live.reload) == 0 {
		return false
	}
	volatile.StoreUint32(&live.reload, 0)
	if s.parse(s.data, s.src) != nil {
		return false
	}
	s.setup()
	return true
}

// Teleport returns the cell the playground asked the program to put its
// figure on, once, when the Room panel's Play from here was pressed, and
// the disc path of the scene the cell is in, or "" for the scene
// showing: the program loads that scene if it is another, as it would on
// a doorway, and moves its figure to the cell. The path is a string over
// the live record's own bytes, good until the next request.
func (s *Scene) Teleport() (scene string, i, j int, ok bool) {
	if live.scene != s || volatile.LoadUint32(&live.place) == 0 {
		return "", 0, 0, false
	}
	volatile.StoreUint32(&live.place, 0)
	cell := volatile.LoadUint32(&live.cell)
	n := 0
	for n < len(live.want) && live.want[n] != 0 {
		n++
	}
	if n > 0 {
		scene = unsafe.String(&live.want[0], n)
	}
	return scene, int(cell & 0xffff), int(cell >> 16), true
}

// parse reads the file into the Scene's fields, with no call on the
// library, so that it runs on a host too.
func (s *Scene) parse(data []uint32, src Source) error {
	if len(data) < headerWords || data[0] != id {
		return ErrFormat
	}
	counts := data[1]
	nt, nm, no, ns := int(counts&0xff), int(counts>>8&0xff), int(counts>>16&0xff), int(counts>>24)
	nl := int(data[2] & 0xff)
	nLayers := 0
	if data[2]&(1<<8) != 0 {
		nLayers = int(data[2]>>10&3) + 1
	}
	hasRoom := data[2]&(1<<9) != 0
	if nt > MaxTextures || nm > MaxModels || no > MaxObjects || ns > MaxSprites || nl > MaxLights || nLayers > MaxLayers {
		return ErrFormat
	}
	if len(data) < headerWords+pathWords*(nt+nm)+objectWords*no+spriteWords*ns {
		return ErrFormat
	}
	s.nLayers = 0
	s.hasRoom = false
	s.Projection = int32(data[projectionAt])
	for i := range s.Ambient {
		s.Ambient[i] = int32(data[ambientAt+i])
	}
	cam := data[cameraAt:]
	s.Camera = yaroze.GsRVIEW2{
		Vpx: int32(cam[0]), Vpy: int32(cam[1]), Vpz: int32(cam[2]),
		Vrx: int32(cam[3]), Vry: int32(cam[4]), Vrz: int32(cam[5]),
		Rz:    int32(cam[6]),
		Super: yaroze.WORLD,
	}
	s.NLights = nl
	for i := range s.Lights {
		l := data[lightsAt+4*i:]
		s.Lights[i] = yaroze.GsF_LIGHT{}
		if i < nl {
			s.Lights[i].Vx, s.Lights[i].Vy, s.Lights[i].Vz = int32(l[0]), int32(l[1]), int32(l[2])
			s.Lights[i].R, s.Lights[i].G, s.Lights[i].B = uint8(l[3]), uint8(l[3]>>8), uint8(l[3]>>16)
		}
	}
	at := headerWords
	s.NTextures = nt
	for i := range nt {
		w := src.Lookup(pathString(data[at : at+pathWords]))
		if w == nil {
			s.Missing = pathString(data[at : at+pathWords])
			return ErrMissing
		}
		s.Textures[i] = yaroze.TIM(w)
		at += pathWords
	}
	s.NModels = nm
	for i := range nm {
		w := src.Lookup(pathString(data[at : at+pathWords]))
		if w == nil {
			s.Missing = pathString(data[at : at+pathWords])
			return ErrMissing
		}
		s.Models[i] = yaroze.TMD(w)
		at += pathWords
	}
	s.NObjects = no
	for i := range no {
		r := data[at:]
		copyName(&s.objectNames[i], r[:nameWords])
		o := &s.Objects[i]
		o.Position = yaroze.VECTOR{Vx: int32(r[6]), Vy: int32(r[7]), Vz: int32(r[8])}
		o.Rotation = yaroze.SVECTOR{Vx: int16(r[9]), Vy: int16(r[9] >> 16), Vz: int16(r[10])}
		o.Scale = yaroze.VECTOR{Vx: int32(r[11]), Vy: int32(r[12]), Vz: int32(r[13])}
		if int(r[4]) >= nm {
			return ErrFormat
		}
		s.objectModel[i] = uint8(r[4])
		s.objectIndex[i] = int32(r[5])
		s.objectFields[i] = [2]int32{int32(r[14]), int32(r[15])}
		at += objectWords
	}
	s.NSprites = ns
	for i := range ns {
		r := data[at:]
		copyName(&s.spriteNames[i], r[:nameWords])
		if int(r[4]) >= nt {
			return ErrFormat
		}
		sp := &s.Sprites[i]
		sp.Attribute = r[5]
		sp.X, sp.Y = int16(r[6]), int16(r[6]>>16)
		sp.W, sp.H = uint16(r[7]), uint16(r[7]>>16)
		sp.Tpage = uint16(r[8])
		sp.U, sp.V = uint8(r[8]>>16), uint8(r[8]>>24)
		sp.Cx, sp.Cy = int16(r[9]), int16(r[9]>>16)
		sp.R, sp.G, sp.B = uint8(r[10]), uint8(r[10]>>8), uint8(r[10]>>16)
		sp.Mx, sp.My = int16(r[11]), int16(r[11]>>16)
		sp.Scalex, sp.Scaley = int16(r[12]), int16(r[12]>>16)
		sp.Rotate = int32(r[13])
		at += spriteWords
	}
	for k := range nLayers {
		next, err := s.layers[k].parse(data, at, nt)
		if err != nil {
			return err
		}
		s.nLayers = k + 1
		at = next
	}
	if hasRoom {
		if err := s.parseRoom(data, at); err != nil {
			return err
		}
	}
	return nil
}

const (
	layerWords     = 28 / 4
	layerNameWords = 20 / 4
	roomWords      = 36 / 4
	linkWords      = 36 / 4
	markerWords    = 28 / 4
)

// parse reads the layer's record at word at of data, a file of nt
// textures, and returns the word after it.
func (l *Layer) parse(data []uint32, at, nt int) (int, error) {
	if len(data) < at+layerWords {
		return 0, ErrFormat
	}
	r := data[at:]
	t := int(r[0])
	w, h := int(r[2]&0xffff), int(r[2]>>16)
	n := w * h
	if t >= nt || uint16(r[1]) != CellSize || w == 0 || h == 0 || n > MaxMapCells || len(data) < at+layerWords+(n+1)/2 {
		return 0, ErrFormat
	}
	l.tex = t
	l.Speed = int(r[1] >> 16)
	l.scroll = [2]int16{int16(r[3]), int16(r[3] >> 16)}
	l.tileMap = yaroze.GsMAP{Cellw: CellSize, Cellh: CellSize, Ncellw: uint16(w), Ncellh: uint16(h), Base: &l.cells[0], Index: &l.index[0]}
	bg := &l.BG
	*bg = yaroze.GsBG{
		Scrollx: l.scroll[0], Scrolly: l.scroll[1],
		X: int16(r[4]), Y: int16(r[4] >> 16), W: int16(r[5]), H: int16(r[5] >> 16),
		R: uint8(r[6]), G: uint8(r[6] >> 8), B: uint8(r[6] >> 16),
		Map:    &l.tileMap,
		Scalex: yaroze.ONE, Scaley: yaroze.ONE,
	}
	if bg.W == 0 || bg.H == 0 {
		bg.X, bg.Y, bg.W, bg.H = -160, -120, 320, 240
	}
	if bg.W > 320 {
		bg.W = 320
	}
	if bg.H > 240 {
		bg.H = 240
	}
	words := r[layerWords:]
	for i := range n {
		l.index[i] = uint16(words[i/2] >> (16 * (i % 2)))
	}
	at += layerWords + (n+1)/2
	l.flags = [MaxCells]uint8{}
	if r[6]>>24&1 == 0 {
		// A file from before layers: its one map, moving with the scroll.
		l.name = [16]byte{}
		copy(l.name[:], "background")
		l.Speed = 100
		return at, nil
	}
	if len(data) < at+layerNameWords {
		return 0, ErrFormat
	}
	r = data[at:]
	copyName(&l.name, r[:nameWords])
	nf := int(r[4] & 0xffff)
	if nf > MaxCells || len(data) < at+layerNameWords+(nf+3)/4 {
		return 0, ErrFormat
	}
	for i := range nf {
		l.flags[i] = uint8(r[layerNameWords+i/4] >> (8 * (i % 4)))
	}
	return at + layerNameWords + (nf+3)/4, nil
}

// parseRoom reads the room record at word at: the map's rows are made
// strings over the file's bytes, nothing copied.
func (s *Scene) parseRoom(data []uint32, at int) error {
	if len(data) < at+roomWords {
		return ErrFormat
	}
	r := data[at:]
	cols, rows := int(r[0]&0xffff), int(r[0]>>16)
	flags := r[2] >> 24
	heights := flags&4 != 0
	nl, nm := int(r[8]&0xffff), int(r[8]>>16)
	n := cols * rows
	cellWords := (n + 3) / 4
	size := roomWords + cellWords + linkWords*nl + markerWords*nm
	if heights {
		size += cellWords
	}
	if cols < 1 || rows < 1 || cols > stage.MaxCells || rows > stage.MaxCells || nl > MaxLinks || nm > MaxMarkers || len(data) < at+size {
		return ErrFormat
	}
	room := &s.room
	*room = Room{
		Map: stage.Map{
			Floor: int(r[1] & 0xffff), Wall: int(r[1] >> 16),
			Tint:     stage.Color{uint8(r[2]), uint8(r[2] >> 8), uint8(r[2] >> 16)},
			Rough:    flags&1 != 0,
			Black:    flags&2 != 0,
			RockTall: int(r[3] & 0xffff), RockLean: int(r[3] >> 16),
		},
		CamUp: int(r[4] & 0xffff), CamBack: int(r[4] >> 16),
		Sky:        [2]stage.Color{{uint8(r[5]), uint8(r[5] >> 8), uint8(r[5] >> 16)}, {uint8(r[6]), uint8(r[6] >> 8), uint8(r[6] >> 16)}},
		Background: stage.Color{uint8(r[7]), uint8(r[7] >> 8), uint8(r[7] >> 16)},
	}
	// The cells and the heights as strings over the file's bytes, the
	// heights' levels made digits in place, which the file is loaded for.
	bytes := unsafe.Slice((*byte)(unsafe.Pointer(&r[roomWords])), 4*(size-roomWords))
	for j := range rows {
		s.cellRows[j] = unsafe.String(&bytes[j*cols], cols)
	}
	room.Cells = s.cellRows[:rows]
	room.Heights = nil
	at = 4 * cellWords
	if heights {
		for k := range n {
			if bytes[at+k] < '0' {
				bytes[at+k] += '0'
			}
		}
		for j := range rows {
			s.heightRows[j] = unsafe.String(&bytes[at+j*cols], cols)
		}
		room.Heights = s.heightRows[:rows]
		at += 4 * cellWords
	}
	s.nLinks = nl
	for i := range nl {
		s.linkDoors[i] = '0' + bytes[at]
		end := at + 4
		for end < at+4*linkWords && bytes[end] != 0 {
			end++
		}
		s.linkScenes[i] = unsafe.String(&bytes[at+4], end-at-4)
		at += 4 * linkWords
	}
	s.nMarkers = nm
	for i := range nm {
		w := r[roomWords+at/4:]
		copyName(&s.markerName[i], w[:nameWords])
		s.markers[i] = Marker{I: int(w[4] & 0xffff), J: int(w[4] >> 16), Fields: [2]int32{int32(w[5]), int32(w[6])}}
		at += 4 * markerWords
	}
	s.hasRoom = true
	return nil
}

// ObjectModel returns the index into Models of object i's model, and the
// object's number in that TMD.
func (s *Scene) ObjectModel(i int) (model int, index int32) {
	return int(s.objectModel[i]), s.objectIndex[i]
}

// ObjectFields returns the two numbers the file gives object i, for the
// program's own purposes: a kind, a score, a door's key, as the scene's
// editor set them.
func (s *Scene) ObjectFields(i int) (int32, int32) {
	return s.objectFields[i][0], s.objectFields[i][1]
}

// Draw sorts the layers at the far end of ot, the first farthest, the
// objects into ot, and the sprites into sprites, which may be the screen's
// overlay table, so that they go in front of the models, or ot itself.
func (s *Scene) Draw(ot, sprites *yaroze.GsOT) {
	if !s.ready {
		return
	}
	s.Reload()
	// Within one depth the last sorted is drawn first, so the nearest
	// layer goes in first and the farthest last, under the others.
	far := uint16(1<<ot.Length) - 1
	for i := s.nLayers - 1; i >= 0; i-- {
		yaroze.GsSortFixBg16(&s.layers[i].BG, &s.layers[i].work[0], ot, far)
	}
	for i := range s.NObjects {
		s.Objects[i].Draw(ot)
	}
	for i := range s.NSprites {
		s.Sprites[i].SortFast(sprites, 0)
	}
}

// Object returns the object the file names so, or nil.
func (s *Scene) Object(name string) *yaroze.Object {
	for i := range s.NObjects {
		if nameIs(&s.objectNames[i], name) {
			return &s.Objects[i]
		}
	}
	return nil
}

// Sprite returns the sprite the file names so, or nil.
func (s *Scene) Sprite(name string) *yaroze.GsSPRITE {
	for i := range s.NSprites {
		if nameIs(&s.spriteNames[i], name) {
			return &s.Sprites[i]
		}
	}
	return nil
}

// ObjectName returns object i's name.
func (s *Scene) ObjectName(i int) string { return nameString(&s.objectNames[i]) }

// SpriteName returns sprite i's name.
func (s *Scene) SpriteName(i int) string { return nameString(&s.spriteNames[i]) }

func copyName(dst *[16]byte, words []uint32) {
	for i, w := range words[:4] {
		dst[4*i], dst[4*i+1], dst[4*i+2], dst[4*i+3] = byte(w), byte(w>>8), byte(w>>16), byte(w>>24)
	}
}

func nameIs(name *[16]byte, s string) bool {
	if len(s) > 15 {
		return false
	}
	for i := 0; i < len(s); i++ {
		if name[i] != s[i] {
			return false
		}
	}
	return name[len(s)] == 0
}

func nameString(name *[16]byte) string {
	n := 0
	for n < len(name) && name[n] != 0 {
		n++
	}
	return string(name[:n])
}

// pathString is a disc path from its padded words; it allocates, and is
// only for Missing.
func pathString(words []uint32) string {
	var b [32]byte
	for i, w := range words {
		b[4*i], b[4*i+1], b[4*i+2], b[4*i+3] = byte(w), byte(w>>8), byte(w>>16), byte(w>>24)
	}
	n := 0
	for n < len(b) && b[n] != 0 {
		n++
	}
	return string(b[:n])
}

// A Source gives a scene its textures and models: the words a file has
// been loaded into, by the file's disc path, or nil for one it does not
// have or has not loaded.
type Source interface {
	Lookup(path string) []uint32
}

// namer is a Source that knows which of its files data came from.
type namer interface {
	nameOf(data []uint32) string
}

// Files is the files registered with package disc, as a Source: each by
// its disc path, once Load has read it.
type Files []*disc.File

func (f Files) Lookup(path string) []uint32 {
	for _, x := range f {
		if x.Name == path {
			return x.Data()
		}
	}
	return nil
}

func (f Files) nameOf(data []uint32) string {
	for _, x := range f {
		if d := x.Data(); len(d) > 0 && len(data) > 0 && &d[0] == &data[0] {
			return x.Name
		}
	}
	return ""
}

// An Archive is an opened archive as a Source. A path finds the entry
// named by its last part, DATA/CUBE.TIM finding CUBE.TIM: a stored entry
// where it lies in the archive, a packed one unpacked into new memory the
// first time it is asked for and kept for the times after, so that a
// reload or a second scene over the same archive unpacks nothing twice.
type Archive struct {
	a     archive.Archive
	n     int
	names [MaxTextures + MaxModels]string
	words [MaxTextures + MaxModels][]uint32
}

// NewArchive makes an opened archive a Source.
func NewArchive(a archive.Archive) *Archive { return &Archive{a: a} }

func (ar *Archive) Lookup(path string) []uint32 {
	name := path
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' || path[i] == '\\' {
			name = path[i+1:]
			break
		}
	}
	for i := 0; i < ar.n; i++ {
		if ar.names[i] == name {
			return ar.words[i]
		}
	}
	e, ok := ar.a.Lookup(name)
	if !ok {
		return nil
	}
	w := e.Data()
	if w == nil {
		var err error
		if w, err = e.Load(); err != nil {
			return nil
		}
	}
	if ar.n < len(ar.names) {
		ar.names[ar.n] = name
		ar.words[ar.n] = w
		ar.n++
	}
	return w
}

// Sources looks in each of its sources in turn.
type Sources []Source

func (s Sources) Lookup(path string) []uint32 {
	for _, src := range s {
		if w := src.Lookup(path); w != nil {
			return w
		}
	}
	return nil
}

func (s Sources) nameOf(data []uint32) string {
	for _, src := range s {
		if n, ok := src.(namer); ok {
			if name := n.nameOf(data); name != "" {
				return name
			}
		}
	}
	return ""
}
