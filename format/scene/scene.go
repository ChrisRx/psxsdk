// Package scene reads and writes scene files, which place what the library
// draws: models from TMDs, each at a position, a rotation and a scale, a
// camera, up to three lights and the ambient light, sprites cut from TIMs,
// and up to three layers of 16 by 16 cells behind them, the tile maps of
// a 2D game. The playground's scene editor writes one as JSON, scene.json;
// mkscene cooks that into the binary file a program reads with package
// scene, SCENE.DAT, and this package is both forms.
//
// The binary file is little-endian words, so that a program reads it in
// place from the memory the disc package loaded it into, nothing
// allocated:
//
//	000h  ID, "SCN1" as a word, 0x314E4353
//	004h  counts: textures in bits 0-7, models in 8-15, objects in 16-23,
//	      sprites in 24-31
//	008h  the number of lights, 0 to 3
//	00Ch  the projection distance, for GsSetProjection
//	010h  the ambient light, R, G and B, 4096 as full, for GsSetAmbient
//	01Ch  the camera: viewpoint X, Y, Z, reference point X, Y, Z, and the
//	      twist, as GsRVIEW2 holds them
//	038h  three lights, each direction X, Y, Z and a color word R, G<<8,
//	      B<<16, as GsF_LIGHT holds them; those past the count are zero
//	068h  the textures, 32 bytes each: the TIM's path on the disc, as the
//	      program registered it, NUL-padded
//	      the models, 32 bytes each: the TMD's path, the same way
//	      the objects, 64 bytes each: a name of up to 15 bytes, NUL-padded
//	      to 16, the model's index, the object's number in the TMD, the
//	      position X, Y, Z, the rotation X, Y, Z and a pad as 16 bits each
//	      (4096 a whole turn), the scale X, Y, Z (4096 as 1.0), and two
//	      fields, numbers of the program's own, which a scene gives an
//	      object for its game to read
//	      the sprites, 64 bytes each: a name as above, the texture's index,
//	      then the sprite as GsSPRITE packs it: the attribute, X and Y, W
//	      and H, the texture page with U and V above it, CX and CY, R, G
//	      and B, MX and MY, the scales, the rotation, and two pad words
//	      the layers, when bit 8 of the word at 008h is set, as many as
//	      bits 10 and 11 say plus one, each: the texture's index (32
//	      bits), the cell size and the speed, the percent of the scene's
//	      scroll the layer moves (16 bits each), the map's width and height
//	      in cells, the scroll X and Y, the window's X, Y, W and H on the
//	      screen (16 bits each), R, G and B and a byte whose bit 0 says a
//	      name and flags follow the map, then the map, a 16-bit cell index
//	      per cell, a row at a time, 0xffff for an empty cell, padded to a
//	      word, then the name, NUL-padded to 16 bytes, the number of flag
//	      bytes and a pad (16 bits each), and the flags, a byte for each
//	      of the texture's cells in order, padded to a word
//	      the room, when bit 9 of the word at 008h is set: the map's
//	      columns and rows, the floor and wall texture indexes (16 bits
//	      each), the tint R, G, B and a flags byte (bit 0 rough, bit 1
//	      black, bit 2 heights given), the rock's height and lean, the
//	      camera's height and distance (16 bits each), the sky's top and
//	      bottom colors and the background color (R, G, B and a pad byte
//	      each), the numbers of links and of markers (16 bits each), then
//	      the cells, a byte each, a row at a time, padded to a word, the
//	      heights the same way when given, the level of each cell as a
//	      byte, the links, 36 bytes each: the door's digit as a word and
//	      the scene's path, as the program registered it, NUL-padded to
//	      32, and the markers, 28 bytes each: a name of up to 15 bytes,
//	      NUL-padded to 16, the column and row (16 bits each), and two
//	      fields, numbers of the program's own
//
// A layer's cells are cut from its texture in reading order, 16 by 16
// pixels each, as many across as the picture is wide, as GsSortFixBg16
// draws them. The layers are drawn back to front, the first farthest, and
// their flags say what each cell is to the program: solid, passed through
// from below, hurting, to be picked up, and four bits of its own. A file
// from before layers has one, with no name or flags, whose speed reads as
// 100.
//
// A room is a map of cells for package stage to draw, with the look
// round it: rows of runes in stage's legend, a level of height for each
// cell, the textures by index into the program's art, and the colors of
// the sky and the background. Its links say which scene each doorway
// digit leads to, where the doorway with the same digit is, and its
// markers name cells for the program's own things to stand on: a person,
// a chest, a creature, each with two fields for the program to read.
package scene

import (
	"encoding/binary"
	"errors"
	"fmt"
)

// ID is the first word of a scene file.
const ID = 0x314E4353

// The most of each a file holds, which are the sizes of package scene's
// arrays.
const (
	MaxTextures = 16
	MaxModels   = 16
	MaxObjects  = 64
	MaxSprites  = 128
	MaxLights   = 3
	MaxName     = 15 // bytes of an object's or a sprite's name
	MaxPath     = 31 // bytes of a disc path

	// A scene has at most MaxLayers layers. A layer's map holds at most
	// MaxMapCells cells, its texture at most MaxCells, and its cells are
	// CellSize square.
	MaxLayers   = 3
	MaxMapCells = 4096
	MaxCells    = 256
	CellSize    = 16
	EmptyCell   = 0xffff

	// A room is at most MaxRoomCells across and down, as package stage
	// has it, with at most MaxLinks links, one a digit, and MaxMarkers
	// markers.
	MaxRoomCells = 32
	MaxLinks     = 10
	MaxMarkers   = 64
)

// The flags the editor names for a layer's cells; the other four bits are
// the program's own.
const (
	Solid  = 1 << 0 // nothing passes through it
	OneWay = 1 << 1 // stood on from above, passed through from below
	Hurts  = 1 << 2 // hurts what touches it
	Pickup = 1 << 3 // taken by what touches it
)

// The sizes of the records, in bytes.
const (
	HeaderSize    = 0x68
	PathSize      = 32
	ObjectSize    = 64
	SpriteSize    = 64
	LayerSize     = 28 // before its map
	LayerNameSize = 20 // after its map: the name, the count of flags and a pad
	RoomSize      = 36 // before its cells
	LinkSize      = 36
	MarkerSize    = 28
	ObjectsAt     = HeaderSize // plus the paths
	nameSize      = 16
	lightSize     = 16
	lightsAt      = 0x38
	cameraAt      = 0x1C
	ambientAt     = 0x10
	projectionAt  = 0x0C
)

// A Camera is a viewpoint and the point it looks at, with a twist about
// the line between them, as GsRVIEW2 has them.
type Camera struct {
	From  [3]int32 `json:"from"`
	At    [3]int32 `json:"at"`
	Twist int32    `json:"twist,omitempty"`
}

// A Light is a parallel light: the direction it shines in and its color.
type Light struct {
	Direction [3]int32 `json:"direction"`
	Color     [3]uint8 `json:"color"`
}

// An Object is a model placed in the world.
type Object struct {
	Name string `json:"name"`

	// Model is the TMD's path on the disc, one of the scene's Models, and
	// Index the object's number in it.
	Model string `json:"model"`
	Index int32  `json:"index,omitempty"`

	Position [3]int32 `json:"position"`
	Rotation [3]int16 `json:"rotation"`

	// Scale is 4096 as 1.0 on each axis; all zero leaves the model its
	// size.
	Scale [3]int32 `json:"scale,omitempty"`

	// Fields are two numbers of the program's own, for what its game
	// wants to know about the object: a kind, a score, a door's key.
	Fields [2]int32 `json:"fields,omitempty"`
}

// A Sprite is a rectangle of a texture drawn on the screen, with the
// fields of GsSPRITE.
type Sprite struct {
	Name string `json:"name"`

	// Texture is the TIM's path on the disc, one of the scene's Textures,
	// which the program loads into video memory before the sprite shows.
	Texture string `json:"texture"`

	Attribute uint32 `json:"attribute,omitempty"`
	X         int16  `json:"x"`
	Y         int16  `json:"y"`
	W         uint16 `json:"w"`
	H         uint16 `json:"h"`
	TPage     uint16 `json:"tpage"`
	U         uint8  `json:"u"`
	V         uint8  `json:"v"`
	CX        int16  `json:"cx"`
	CY        int16  `json:"cy"`
	R         uint8  `json:"r"`
	G         uint8  `json:"g"`
	B         uint8  `json:"b"`
	MX        int16  `json:"mx,omitempty"`
	MY        int16  `json:"my,omitempty"`
	ScaleX    int16  `json:"scalex,omitempty"`
	ScaleY    int16  `json:"scaley,omitempty"`
	Rotate    int32  `json:"rotate,omitempty"`
}

// A Layer is a map of cells cut from a texture, drawn behind everything
// through a window on the screen, as GsSortFixBg16 draws it, scrolled by
// its own share of the scene's scroll, with a byte of flags for each of
// its texture's cells.
type Layer struct {
	// Name is what the program finds the layer by, up to 15 bytes.
	Name string `json:"name"`

	// Texture is the TIM's path on the disc, one of the scene's Textures,
	// whose 16 by 16 cells the map indexes in reading order.
	Texture string `json:"texture"`

	// Cell is the cells' size, which is 16.
	Cell int `json:"cell"`

	// Width and Height are the map's size in cells, at most MaxMapCells in
	// all, and Map its cells, a row at a time: an index into the
	// texture's cells, or EmptyCell.
	Width  int      `json:"width"`
	Height int      `json:"height"`
	Map    []uint16 `json:"map"`

	// Scroll is how far the map is scrolled within the window, in pixels,
	// where the scene's scroll at 0, 0 leaves it.
	Scroll [2]int16 `json:"scroll"`

	// Window is the window's X, Y, W and H on the screen, from the middle
	// as GsInit3D places the origin; all zero is the whole of a 320 by
	// 240 screen.
	Window [4]int16 `json:"window,omitempty"`

	// Brightness is R, G and B, 128 showing the cells as they are.
	Brightness [3]uint8 `json:"brightness"`

	// Speed is the percent of the scene's scroll the layer moves: 100
	// with it, less for one farther off, 0 not at all.
	Speed int `json:"speed"`

	// Flags are a byte for each of the texture's cells, in order, those
	// past the end none: Solid, OneWay, Hurts and Pickup, and four bits
	// of the program's own.
	Flags []int `json:"flags,omitempty"`
}

// A Background is the one map a scene had before it had layers, as an
// older scene.json writes it: a layer named "background", moving with the
// scene's scroll, with no flags.
type Background struct {
	// Texture is the TIM's path on the disc, one of the scene's Textures,
	// whose 16 by 16 cells the map indexes in reading order.
	Texture string `json:"texture"`

	// Cell is the cells' size, which is 16.
	Cell int `json:"cell"`

	// Width and Height are the map's size in cells, and Map its cells, a
	// row at a time: an index into the texture's cells, or EmptyCell.
	Width  int      `json:"width"`
	Height int      `json:"height"`
	Map    []uint16 `json:"map"`

	// Scroll is how far the map is scrolled within the window, in pixels.
	Scroll [2]int16 `json:"scroll"`

	// Window is the window's X, Y, W and H on the screen, from the middle
	// as GsInit3D places the origin; all zero is the whole of a 320 by
	// 240 screen.
	Window [4]int16 `json:"window,omitempty"`

	// Brightness is R, G and B, 128 showing the cells as they are.
	Brightness [3]uint8 `json:"brightness"`
}

// A Room is a map of cells with heights, drawn by package stage, and the
// look round it.
type Room struct {
	// Floor and Wall are the ground's and the walls' textures, as
	// indexes into the program's art, and Tint the ground's color, 128
	// leaving the texture as it is.
	Floor int      `json:"floor,omitempty"`
	Wall  int      `json:"wall,omitempty"`
	Tint  [3]uint8 `json:"tint"`

	// Cells are the map's rows, a rune a cell in stage's legend, all the
	// same length, and Heights the ground's level at each, a digit a
	// cell, the same shape, or none for level ground.
	Cells   []string `json:"cells"`
	Heights []string `json:"heights,omitempty"`

	// Rough breaks the outlines of the cells, for a cave; RockTall is
	// how high rock stands, a tile when zero, RockLean how far a rock
	// face's top leans back, and Black leaves the rock beyond the
	// passages undrawn.
	Rough    bool `json:"rough,omitempty"`
	Black    bool `json:"black,omitempty"`
	RockTall int  `json:"rockTall,omitempty"`
	RockLean int  `json:"rockLean,omitempty"`

	// CamUp and CamBack put the program's camera that high above its
	// figure and that far behind, or leave its usual slant when zero.
	CamUp   int `json:"camUp,omitempty"`
	CamBack int `json:"camBack,omitempty"`

	// Sky is the color at the top of the screen and at the bottom,
	// shaded between, behind everything outdoors; all zero, there is
	// none and Background shows, the color the screen clears to, or
	// the program's own when that is zero too.
	Sky        [2][3]uint8 `json:"sky"`
	Background [3]uint8    `json:"background"`

	Links   []Link   `json:"links,omitempty"`
	Markers []Marker `json:"markers,omitempty"`
}

// A Link says where a doorway leads: the cells with the digit Door lead
// to the scene at the path Scene, in at its doorway with the same digit.
type Link struct {
	Door  int    `json:"door"`
	Scene string `json:"scene"`
}

// A Marker names a cell for the program's own thing to stand on, with
// two fields of the program's own: which way it faces, what it says.
type Marker struct {
	Name   string   `json:"name"`
	Cell   [2]int   `json:"cell"`
	Fields [2]int32 `json:"fields,omitempty"`
}

// A Scene is what a file holds, in the form scene.json has.
type Scene struct {
	Camera     Camera   `json:"camera"`
	Projection int32    `json:"projection"`
	Ambient    [3]int32 `json:"ambient"`
	Lights     []Light  `json:"lights"`

	// Textures and Models are the TIMs and TMDs the scene needs, by their
	// paths on the disc, as the program registered them.
	Textures []string `json:"textures"`
	Models   []string `json:"models"`

	Objects []Object `json:"objects"`
	Sprites []Sprite `json:"sprites"`

	// Layers are the maps drawn behind everything, back to front.
	Layers []Layer `json:"layers,omitempty"`

	// Background is an older scene.json's one map, written as the first
	// layer; Decode never sets it.
	Background *Background `json:"background,omitempty"`

	// Room is the room of cells the scene is, or nil.
	Room *Room `json:"room,omitempty"`
}

// AllLayers returns the scene's layers, an older file's Background first.
func (s *Scene) AllLayers() []Layer {
	bg := s.Background
	if bg == nil {
		return s.Layers
	}
	first := Layer{
		Name: "background", Texture: bg.Texture, Cell: bg.Cell, Width: bg.Width, Height: bg.Height, Map: bg.Map,
		Scroll: bg.Scroll, Window: bg.Window, Brightness: bg.Brightness, Speed: 100,
	}
	return append([]Layer{first}, s.Layers...)
}

// Size returns the file's size in bytes.
func (s *Scene) Size() int {
	n := HeaderSize + PathSize*(len(s.Textures)+len(s.Models)) + ObjectSize*len(s.Objects) + SpriteSize*len(s.Sprites)
	for _, l := range s.AllLayers() {
		n += LayerSize + mapBytes(l.Width*l.Height) + LayerNameSize + cellBytes(len(l.Flags))
	}
	if r := s.Room; r != nil {
		n += r.size()
	}
	return n
}

// mapBytes is the size of a map of n cells, padded to a word.
func mapBytes(n int) int { return (2*n + 3) &^ 3 }

// cellBytes is the size of a room's cells, a byte each, padded to a word.
func cellBytes(n int) int { return (n + 3) &^ 3 }

// cols and rows are the room's size in cells, 0 by 0 when it has none.
func (r *Room) cols() int {
	if len(r.Cells) == 0 {
		return 0
	}
	return len(r.Cells[0])
}

func (r *Room) rows() int { return len(r.Cells) }

// size is the room's size in the file.
func (r *Room) size() int {
	n := RoomSize + cellBytes(r.cols()*r.rows())
	if r.Heights != nil {
		n += cellBytes(r.cols() * r.rows())
	}
	return n + LinkSize*len(r.Links) + MarkerSize*len(r.Markers)
}

func index(list []string, path string) (int, bool) {
	for i, p := range list {
		if p == path {
			return i, true
		}
	}
	return 0, false
}

// Check reports what keeps the scene from being written.
func (s *Scene) Check() error {
	if len(s.Textures) > MaxTextures || len(s.Models) > MaxModels || len(s.Objects) > MaxObjects || len(s.Sprites) > MaxSprites || len(s.Lights) > MaxLights {
		return fmt.Errorf("scene: at most %d textures, %d models, %d objects, %d sprites and %d lights", MaxTextures, MaxModels, MaxObjects, MaxSprites, MaxLights)
	}
	for _, p := range append(append([]string{}, s.Textures...), s.Models...) {
		if p == "" || len(p) > MaxPath {
			return fmt.Errorf("scene: path %q is empty or longer than %d bytes", p, MaxPath)
		}
	}
	for i, o := range s.Objects {
		if len(o.Name) > MaxName {
			return fmt.Errorf("scene: object %d's name %q is longer than %d bytes", i, o.Name, MaxName)
		}
		if _, ok := index(s.Models, o.Model); !ok {
			return fmt.Errorf("scene: object %q names model %q, which is not in the scene's models", o.Name, o.Model)
		}
	}
	for i, sp := range s.Sprites {
		if len(sp.Name) > MaxName {
			return fmt.Errorf("scene: sprite %d's name %q is longer than %d bytes", i, sp.Name, MaxName)
		}
		if _, ok := index(s.Textures, sp.Texture); !ok {
			return fmt.Errorf("scene: sprite %q names texture %q, which is not in the scene's textures", sp.Name, sp.Texture)
		}
	}
	layers := s.AllLayers()
	if len(layers) > MaxLayers {
		return fmt.Errorf("scene: %d layers; a scene has at most %d", len(layers), MaxLayers)
	}
	for i, l := range layers {
		if err := l.check(s.Textures); err != nil {
			return fmt.Errorf("scene: layer %d, %q: %w", i, l.Name, err)
		}
		for _, m := range layers[:i] {
			if m.Name == l.Name {
				return fmt.Errorf("scene: two layers are named %q", l.Name)
			}
		}
	}
	if r := s.Room; r != nil {
		return r.check()
	}
	return nil
}

// check reports what keeps the layer from being written.
func (l *Layer) check(textures []string) error {
	if len(l.Name) > MaxName {
		return fmt.Errorf("the name is longer than %d bytes", MaxName)
	}
	if _, ok := index(textures, l.Texture); !ok {
		return fmt.Errorf("it names texture %q, which is not in the scene's textures", l.Texture)
	}
	if l.Cell != CellSize {
		return fmt.Errorf("its cells are %d pixels; GsSortFixBg16 draws cells of %d", l.Cell, CellSize)
	}
	if l.Width < 1 || l.Height < 1 || l.Width*l.Height > MaxMapCells {
		return fmt.Errorf("its map is %d by %d cells; it needs at least one and at most %d", l.Width, l.Height, MaxMapCells)
	}
	if len(l.Map) != l.Width*l.Height {
		return fmt.Errorf("its map has %d cells for %d by %d", len(l.Map), l.Width, l.Height)
	}
	for i, c := range l.Map {
		if c != EmptyCell && int(c) >= MaxCells {
			return fmt.Errorf("its cell %d is %d, past the %d a texture holds", i, c, MaxCells)
		}
	}
	if l.Speed < 0 || l.Speed > 0xffff {
		return fmt.Errorf("its speed is %d percent; it is 0 to 65535", l.Speed)
	}
	if len(l.Flags) > MaxCells {
		return fmt.Errorf("it has flags for %d cells; a texture holds %d", len(l.Flags), MaxCells)
	}
	for i, f := range l.Flags {
		if f < 0 || f > 0xff {
			return fmt.Errorf("the flags of its cell %d are %d, not a byte", i, f)
		}
	}
	return nil
}

// check reports what keeps the room from being written.
func (r *Room) check() error {
	cols, rows := r.cols(), r.rows()
	if cols < 1 || rows < 1 || cols > MaxRoomCells || rows > MaxRoomCells {
		return fmt.Errorf("scene: the room is %d by %d cells; it needs at least one each way and at most %d", cols, rows, MaxRoomCells)
	}
	for j, row := range r.Cells {
		if len(row) != cols {
			return fmt.Errorf("scene: the room's row %d has %d cells, not %d", j, len(row), cols)
		}
	}
	if r.Heights != nil {
		if len(r.Heights) != rows {
			return fmt.Errorf("scene: the room's heights have %d rows, not %d", len(r.Heights), rows)
		}
		for j, row := range r.Heights {
			if len(row) != cols {
				return fmt.Errorf("scene: the room's heights row %d has %d cells, not %d", j, len(row), cols)
			}
			for i := 0; i < len(row); i++ {
				if row[i] < '0' || row[i] > '9' {
					return fmt.Errorf("scene: the room's height at %d, %d is %q, not a digit", i, j, row[i])
				}
			}
		}
	}
	if r.Floor < 0 || r.Wall < 0 || r.Floor > 0xffff || r.Wall > 0xffff {
		return errors.New("scene: the room's floor and wall are indexes into the program's textures")
	}
	for _, v := range [4]int{r.RockTall, r.RockLean, r.CamUp, r.CamBack} {
		if v < 0 || v > 0x7fff {
			return errors.New("scene: the room's rock height and lean and camera height and distance are 0 to 32767")
		}
	}
	if len(r.Links) > MaxLinks || len(r.Markers) > MaxMarkers {
		return fmt.Errorf("scene: a room has at most %d links and %d markers", MaxLinks, MaxMarkers)
	}
	for _, l := range r.Links {
		if l.Door < 0 || l.Door > 9 {
			return fmt.Errorf("scene: link to %q is from door %d; doors are digits", l.Scene, l.Door)
		}
		if l.Scene == "" || len(l.Scene) > MaxPath {
			return fmt.Errorf("scene: link from door %d names scene %q, which is empty or longer than %d bytes", l.Door, l.Scene, MaxPath)
		}
	}
	for _, m := range r.Markers {
		if len(m.Name) > MaxName {
			return fmt.Errorf("scene: marker %q's name is longer than %d bytes", m.Name, MaxName)
		}
		if m.Cell[0] < 0 || m.Cell[0] >= cols || m.Cell[1] < 0 || m.Cell[1] >= rows {
			return fmt.Errorf("scene: marker %q is on cell %d, %d, outside the room", m.Name, m.Cell[0], m.Cell[1])
		}
	}
	return nil
}

// Bytes returns the scene as a file.
func (s *Scene) Bytes() ([]byte, error) {
	if err := s.Check(); err != nil {
		return nil, err
	}
	le := binary.LittleEndian
	b := make([]byte, 0, s.Size())
	b = le.AppendUint32(b, ID)
	b = le.AppendUint32(b, uint32(len(s.Textures))|uint32(len(s.Models))<<8|uint32(len(s.Objects))<<16|uint32(len(s.Sprites))<<24)
	lights := uint32(len(s.Lights))
	layers := s.AllLayers()
	if len(layers) > 0 {
		lights |= 1<<8 | uint32(len(layers)-1)<<10
	}
	if s.Room != nil {
		lights |= 1 << 9
	}
	b = le.AppendUint32(b, lights)
	b = le.AppendUint32(b, uint32(s.Projection))
	for _, c := range s.Ambient {
		b = le.AppendUint32(b, uint32(c))
	}
	for _, c := range [7]int32{s.Camera.From[0], s.Camera.From[1], s.Camera.From[2], s.Camera.At[0], s.Camera.At[1], s.Camera.At[2], s.Camera.Twist} {
		b = le.AppendUint32(b, uint32(c))
	}
	for i := 0; i < MaxLights; i++ {
		var l Light
		if i < len(s.Lights) {
			l = s.Lights[i]
		}
		for _, c := range l.Direction {
			b = le.AppendUint32(b, uint32(c))
		}
		b = le.AppendUint32(b, uint32(l.Color[0])|uint32(l.Color[1])<<8|uint32(l.Color[2])<<16)
	}
	pad := func(s string, n int) []byte {
		out := make([]byte, n)
		copy(out, s)
		return out
	}
	for _, p := range s.Textures {
		b = append(b, pad(p, PathSize)...)
	}
	for _, p := range s.Models {
		b = append(b, pad(p, PathSize)...)
	}
	for _, o := range s.Objects {
		b = append(b, pad(o.Name, nameSize)...)
		m, _ := index(s.Models, o.Model)
		b = le.AppendUint32(b, uint32(m))
		b = le.AppendUint32(b, uint32(o.Index))
		for _, c := range o.Position {
			b = le.AppendUint32(b, uint32(c))
		}
		for _, c := range [4]int16{o.Rotation[0], o.Rotation[1], o.Rotation[2], 0} {
			b = le.AppendUint16(b, uint16(c))
		}
		for _, c := range o.Scale {
			b = le.AppendUint32(b, uint32(c))
		}
		for _, c := range o.Fields {
			b = le.AppendUint32(b, uint32(c))
		}
	}
	for _, sp := range s.Sprites {
		b = append(b, pad(sp.Name, nameSize)...)
		t, _ := index(s.Textures, sp.Texture)
		b = le.AppendUint32(b, uint32(t))
		b = le.AppendUint32(b, sp.Attribute)
		b = le.AppendUint16(b, uint16(sp.X))
		b = le.AppendUint16(b, uint16(sp.Y))
		b = le.AppendUint16(b, sp.W)
		b = le.AppendUint16(b, sp.H)
		b = le.AppendUint16(b, sp.TPage)
		b = append(b, sp.U, sp.V)
		b = le.AppendUint16(b, uint16(sp.CX))
		b = le.AppendUint16(b, uint16(sp.CY))
		b = append(b, sp.R, sp.G, sp.B, 0)
		b = le.AppendUint16(b, uint16(sp.MX))
		b = le.AppendUint16(b, uint16(sp.MY))
		b = le.AppendUint16(b, uint16(sp.ScaleX))
		b = le.AppendUint16(b, uint16(sp.ScaleY))
		b = le.AppendUint32(b, uint32(sp.Rotate))
		b = append(b, 0, 0, 0, 0, 0, 0, 0, 0)
	}
	for _, l := range layers {
		t, _ := index(s.Textures, l.Texture)
		b = le.AppendUint32(b, uint32(t))
		b = le.AppendUint16(b, uint16(l.Cell))
		b = le.AppendUint16(b, uint16(l.Speed))
		b = le.AppendUint16(b, uint16(l.Width))
		b = le.AppendUint16(b, uint16(l.Height))
		b = le.AppendUint16(b, uint16(l.Scroll[0]))
		b = le.AppendUint16(b, uint16(l.Scroll[1]))
		for _, c := range l.Window {
			b = le.AppendUint16(b, uint16(c))
		}
		b = append(b, l.Brightness[0], l.Brightness[1], l.Brightness[2], 1)
		for _, c := range l.Map {
			b = le.AppendUint16(b, c)
		}
		for len(b)%4 != 0 {
			b = append(b, 0)
		}
		b = append(b, pad(l.Name, nameSize)...)
		b = le.AppendUint16(b, uint16(len(l.Flags)))
		b = le.AppendUint16(b, 0)
		for _, f := range l.Flags {
			b = append(b, uint8(f))
		}
		for len(b)%4 != 0 {
			b = append(b, 0)
		}
	}
	if r := s.Room; r != nil {
		b = le.AppendUint16(b, uint16(r.cols()))
		b = le.AppendUint16(b, uint16(r.rows()))
		b = le.AppendUint16(b, uint16(r.Floor))
		b = le.AppendUint16(b, uint16(r.Wall))
		var flags uint8
		if r.Rough {
			flags |= 1
		}
		if r.Black {
			flags |= 2
		}
		if r.Heights != nil {
			flags |= 4
		}
		b = append(b, r.Tint[0], r.Tint[1], r.Tint[2], flags)
		b = le.AppendUint16(b, uint16(r.RockTall))
		b = le.AppendUint16(b, uint16(r.RockLean))
		b = le.AppendUint16(b, uint16(r.CamUp))
		b = le.AppendUint16(b, uint16(r.CamBack))
		b = append(b, r.Sky[0][0], r.Sky[0][1], r.Sky[0][2], 0)
		b = append(b, r.Sky[1][0], r.Sky[1][1], r.Sky[1][2], 0)
		b = append(b, r.Background[0], r.Background[1], r.Background[2], 0)
		b = le.AppendUint16(b, uint16(len(r.Links)))
		b = le.AppendUint16(b, uint16(len(r.Markers)))
		for _, row := range r.Cells {
			b = append(b, row...)
		}
		for len(b)%4 != 0 {
			b = append(b, 0)
		}
		for _, row := range r.Heights {
			for i := 0; i < len(row); i++ {
				b = append(b, row[i]-'0')
			}
		}
		for len(b)%4 != 0 {
			b = append(b, 0)
		}
		for _, l := range r.Links {
			b = le.AppendUint32(b, uint32(l.Door))
			b = append(b, pad(l.Scene, PathSize)...)
		}
		for _, m := range r.Markers {
			b = append(b, pad(m.Name, nameSize)...)
			b = le.AppendUint16(b, uint16(m.Cell[0]))
			b = le.AppendUint16(b, uint16(m.Cell[1]))
			b = le.AppendUint32(b, uint32(m.Fields[0]))
			b = le.AppendUint32(b, uint32(m.Fields[1]))
		}
	}
	return b, nil
}

func str(b []byte) string {
	for i, c := range b {
		if c == 0 {
			return string(b[:i])
		}
	}
	return string(b)
}

// Decode reads a scene file.
func Decode(b []byte) (*Scene, error) {
	le := binary.LittleEndian
	if len(b) < HeaderSize || le.Uint32(b) != ID {
		return nil, errors.New("scene: not a scene file")
	}
	counts := le.Uint32(b[4:])
	nt, nm, no, ns := int(counts&0xff), int(counts>>8&0xff), int(counts>>16&0xff), int(counts>>24)
	lights := le.Uint32(b[8:])
	nl := int(lights & 0xff)
	nLayers := 0
	if lights&(1<<8) != 0 {
		nLayers = int(lights>>10&3) + 1
	}
	hasRoom := lights&(1<<9) != 0
	if nt > MaxTextures || nm > MaxModels || no > MaxObjects || ns > MaxSprites || nl > MaxLights || nLayers > MaxLayers {
		return nil, errors.New("scene: the counts are past what a scene holds")
	}
	s := &Scene{Projection: int32(le.Uint32(b[projectionAt:]))}
	for i := range s.Ambient {
		s.Ambient[i] = int32(le.Uint32(b[ambientAt+4*i:]))
	}
	cam := b[cameraAt:]
	for i := range 3 {
		s.Camera.From[i] = int32(le.Uint32(cam[4*i:]))
		s.Camera.At[i] = int32(le.Uint32(cam[12+4*i:]))
	}
	s.Camera.Twist = int32(le.Uint32(cam[24:]))
	for i := range nl {
		l := b[lightsAt+lightSize*i:]
		var light Light
		for k := range 3 {
			light.Direction[k] = int32(le.Uint32(l[4*k:]))
		}
		c := le.Uint32(l[12:])
		light.Color = [3]uint8{uint8(c), uint8(c >> 8), uint8(c >> 16)}
		s.Lights = append(s.Lights, light)
	}
	if len(b) < s.sizeFor(nt, nm, no, ns) {
		return nil, errors.New("scene: the records run past the end of the file")
	}
	at := HeaderSize
	for range nt {
		s.Textures = append(s.Textures, str(b[at:at+PathSize]))
		at += PathSize
	}
	for range nm {
		s.Models = append(s.Models, str(b[at:at+PathSize]))
		at += PathSize
	}
	for range no {
		r := b[at:]
		o := Object{Name: str(r[:nameSize]), Index: int32(le.Uint32(r[20:]))}
		m := int(le.Uint32(r[16:]))
		if m >= nm {
			return nil, fmt.Errorf("scene: object %q names model %d of %d", o.Name, m, nm)
		}
		o.Model = s.Models[m]
		for i := range 3 {
			o.Position[i] = int32(le.Uint32(r[24+4*i:]))
			o.Rotation[i] = int16(le.Uint16(r[36+2*i:]))
			o.Scale[i] = int32(le.Uint32(r[44+4*i:]))
		}
		o.Fields = [2]int32{int32(le.Uint32(r[56:])), int32(le.Uint32(r[60:]))}
		s.Objects = append(s.Objects, o)
		at += ObjectSize
	}
	for range ns {
		r := b[at:]
		sp := Sprite{Name: str(r[:nameSize])}
		t := int(le.Uint32(r[16:]))
		if t >= nt {
			return nil, fmt.Errorf("scene: sprite %q names texture %d of %d", sp.Name, t, nt)
		}
		sp.Texture = s.Textures[t]
		sp.Attribute = le.Uint32(r[20:])
		sp.X, sp.Y = int16(le.Uint16(r[24:])), int16(le.Uint16(r[26:]))
		sp.W, sp.H = le.Uint16(r[28:]), le.Uint16(r[30:])
		sp.TPage = le.Uint16(r[32:])
		sp.U, sp.V = r[34], r[35]
		sp.CX, sp.CY = int16(le.Uint16(r[36:])), int16(le.Uint16(r[38:]))
		sp.R, sp.G, sp.B = r[40], r[41], r[42]
		sp.MX, sp.MY = int16(le.Uint16(r[44:])), int16(le.Uint16(r[46:]))
		sp.ScaleX, sp.ScaleY = int16(le.Uint16(r[48:])), int16(le.Uint16(r[50:]))
		sp.Rotate = int32(le.Uint32(r[52:]))
		s.Sprites = append(s.Sprites, sp)
		at += SpriteSize
	}
	for k := range nLayers {
		if len(b) < at+LayerSize {
			return nil, fmt.Errorf("scene: layer %d runs past the end of the file", k)
		}
		r := b[at:]
		l := Layer{Cell: int(le.Uint16(r[4:])), Speed: int(le.Uint16(r[6:])), Width: int(le.Uint16(r[8:])), Height: int(le.Uint16(r[10:]))}
		t := int(le.Uint32(r))
		if t >= nt {
			return nil, fmt.Errorf("scene: layer %d names texture %d of %d", k, t, nt)
		}
		l.Texture = s.Textures[t]
		l.Scroll = [2]int16{int16(le.Uint16(r[12:])), int16(le.Uint16(r[14:]))}
		for i := range l.Window {
			l.Window[i] = int16(le.Uint16(r[16+2*i:]))
		}
		l.Brightness = [3]uint8{r[24], r[25], r[26]}
		named := r[27]&1 != 0
		n := l.Width * l.Height
		if n > MaxMapCells || len(b) < at+LayerSize+mapBytes(n) {
			return nil, fmt.Errorf("scene: layer %d's map runs past the end of the file", k)
		}
		l.Map = make([]uint16, n)
		for i := range l.Map {
			l.Map[i] = le.Uint16(r[LayerSize+2*i:])
		}
		at += LayerSize + mapBytes(n)
		if !named {
			// A file from before layers: its one map, moving with the
			// scroll.
			l.Name, l.Speed = "background", 100
			s.Layers = append(s.Layers, l)
			continue
		}
		if len(b) < at+LayerNameSize {
			return nil, fmt.Errorf("scene: layer %d's name runs past the end of the file", k)
		}
		r = b[at:]
		l.Name = str(r[:nameSize])
		nf := int(le.Uint16(r[nameSize:]))
		if nf > MaxCells || len(b) < at+LayerNameSize+cellBytes(nf) {
			return nil, fmt.Errorf("scene: layer %q's flags run past the end of the file", l.Name)
		}
		for _, f := range r[LayerNameSize : LayerNameSize+nf] {
			l.Flags = append(l.Flags, int(f))
		}
		at += LayerNameSize + cellBytes(nf)
		s.Layers = append(s.Layers, l)
	}
	if hasRoom {
		if len(b) < at+RoomSize {
			return nil, errors.New("scene: the room runs past the end of the file")
		}
		r := b[at:]
		room := &Room{Floor: int(le.Uint16(r[4:])), Wall: int(le.Uint16(r[6:]))}
		cols, rows := int(le.Uint16(r)), int(le.Uint16(r[2:]))
		room.Tint = [3]uint8{r[8], r[9], r[10]}
		flags := r[11]
		room.Rough, room.Black = flags&1 != 0, flags&2 != 0
		room.RockTall, room.RockLean = int(le.Uint16(r[12:])), int(le.Uint16(r[14:]))
		room.CamUp, room.CamBack = int(le.Uint16(r[16:])), int(le.Uint16(r[18:]))
		room.Sky = [2][3]uint8{{r[20], r[21], r[22]}, {r[24], r[25], r[26]}}
		room.Background = [3]uint8{r[28], r[29], r[30]}
		nl, nmk := int(le.Uint16(r[32:])), int(le.Uint16(r[34:]))
		n := cols * rows
		size := RoomSize + cellBytes(n) + LinkSize*nl + MarkerSize*nmk
		if flags&4 != 0 {
			size += cellBytes(n)
		}
		if cols < 1 || rows < 1 || cols > MaxRoomCells || rows > MaxRoomCells || nl > MaxLinks || nmk > MaxMarkers || len(b) < at+size {
			return nil, errors.New("scene: the room's cells run past the end of the file")
		}
		r = r[RoomSize:]
		for range rows {
			room.Cells = append(room.Cells, string(r[:cols]))
			r = r[cols:]
		}
		r = r[cellBytes(n)-n:]
		if flags&4 != 0 {
			for range rows {
				row := make([]byte, cols)
				for i := range row {
					row[i] = '0' + r[i]
				}
				room.Heights = append(room.Heights, string(row))
				r = r[cols:]
			}
			r = r[cellBytes(n)-n:]
		}
		for range nl {
			room.Links = append(room.Links, Link{Door: int(le.Uint32(r)), Scene: str(r[4 : 4+PathSize])})
			r = r[LinkSize:]
		}
		for range nmk {
			m := Marker{Name: str(r[:nameSize]), Cell: [2]int{int(le.Uint16(r[16:])), int(le.Uint16(r[18:]))}}
			m.Fields = [2]int32{int32(le.Uint32(r[20:])), int32(le.Uint32(r[24:]))}
			room.Markers = append(room.Markers, m)
			r = r[MarkerSize:]
		}
		s.Room = room
	}
	return s, nil
}

func (s *Scene) sizeFor(nt, nm, no, ns int) int {
	return HeaderSize + PathSize*(nt+nm) + ObjectSize*no + SpriteSize*ns
}
