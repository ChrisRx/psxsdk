// Package scene reads and writes scene files, which place what the library
// draws: models from TMDs, each at a position, a rotation and a scale, a
// camera, up to three lights and the ambient light, and sprites cut from
// TIMs. The playground's scene editor writes one as JSON, scene.json;
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
//	      the background, when bit 8 of the word at 008h is set: the
//	      texture's index, the cell size and a pad (16 bits each), the
//	      map's width and height in cells, the scroll X and Y, the window's
//	      X, Y, W and H on the screen (16 bits each), R, G, B and a pad
//	      byte, then the map, a 16-bit cell index per cell, a row at a
//	      time, 0xffff for an empty cell, padded to a word
//
// A background's cells are cut from its texture in reading order, 16 by
// 16 pixels each, as many across as the picture is wide, as GsSortFixBg16
// draws them.
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

	// A background's map holds at most MaxMapCells cells, its texture at
	// most MaxCells, and its cells are CellSize square.
	MaxMapCells = 4096
	MaxCells    = 256
	CellSize    = 16
	EmptyCell   = 0xffff
)

// The sizes of the records, in bytes.
const (
	HeaderSize     = 0x68
	PathSize       = 32
	ObjectSize     = 64
	SpriteSize     = 64
	BackgroundSize = 28         // before its map
	ObjectsAt      = HeaderSize // plus the paths
	nameSize       = 16
	lightSize      = 16
	lightsAt       = 0x38
	cameraAt       = 0x1C
	ambientAt      = 0x10
	projectionAt   = 0x0C
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

// A Background is a map of cells cut from a texture, drawn behind
// everything through a window on the screen, as GsSortFixBg16 draws it.
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

	// Background is the map drawn behind everything, or nil.
	Background *Background `json:"background,omitempty"`
}

// Size returns the file's size in bytes.
func (s *Scene) Size() int {
	n := HeaderSize + PathSize*(len(s.Textures)+len(s.Models)) + ObjectSize*len(s.Objects) + SpriteSize*len(s.Sprites)
	if s.Background != nil {
		n += BackgroundSize + mapBytes(s.Background.Width*s.Background.Height)
	}
	return n
}

// mapBytes is the size of a map of n cells, padded to a word.
func mapBytes(n int) int { return (2*n + 3) &^ 3 }

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
	if bg := s.Background; bg != nil {
		if _, ok := index(s.Textures, bg.Texture); !ok {
			return fmt.Errorf("scene: the background names texture %q, which is not in the scene's textures", bg.Texture)
		}
		if bg.Cell != CellSize {
			return fmt.Errorf("scene: the background's cells are %d pixels; GsSortFixBg16 draws cells of %d", bg.Cell, CellSize)
		}
		if bg.Width < 1 || bg.Height < 1 || bg.Width*bg.Height > MaxMapCells {
			return fmt.Errorf("scene: the background's map is %d by %d cells; it needs at least one and at most %d", bg.Width, bg.Height, MaxMapCells)
		}
		if len(bg.Map) != bg.Width*bg.Height {
			return fmt.Errorf("scene: the background's map has %d cells for %d by %d", len(bg.Map), bg.Width, bg.Height)
		}
		for i, c := range bg.Map {
			if c != EmptyCell && int(c) >= MaxCells {
				return fmt.Errorf("scene: the background's cell %d is %d, past the %d a texture holds", i, c, MaxCells)
			}
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
	if s.Background != nil {
		lights |= 1 << 8
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
	if bg := s.Background; bg != nil {
		t, _ := index(s.Textures, bg.Texture)
		b = le.AppendUint32(b, uint32(t))
		b = le.AppendUint16(b, uint16(bg.Cell))
		b = le.AppendUint16(b, 0)
		b = le.AppendUint16(b, uint16(bg.Width))
		b = le.AppendUint16(b, uint16(bg.Height))
		b = le.AppendUint16(b, uint16(bg.Scroll[0]))
		b = le.AppendUint16(b, uint16(bg.Scroll[1]))
		for _, c := range bg.Window {
			b = le.AppendUint16(b, uint16(c))
		}
		b = append(b, bg.Brightness[0], bg.Brightness[1], bg.Brightness[2], 0)
		for _, c := range bg.Map {
			b = le.AppendUint16(b, c)
		}
		for len(b)%4 != 0 {
			b = append(b, 0)
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
	hasBG := lights&(1<<8) != 0
	if nt > MaxTextures || nm > MaxModels || no > MaxObjects || ns > MaxSprites || nl > MaxLights {
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
	if hasBG {
		if len(b) < at+BackgroundSize {
			return nil, errors.New("scene: the background runs past the end of the file")
		}
		r := b[at:]
		bg := &Background{Cell: int(le.Uint16(r[4:])), Width: int(le.Uint16(r[8:])), Height: int(le.Uint16(r[10:]))}
		t := int(le.Uint32(r))
		if t >= nt {
			return nil, fmt.Errorf("scene: the background names texture %d of %d", t, nt)
		}
		bg.Texture = s.Textures[t]
		bg.Scroll = [2]int16{int16(le.Uint16(r[12:])), int16(le.Uint16(r[14:]))}
		for i := range bg.Window {
			bg.Window[i] = int16(le.Uint16(r[16+2*i:]))
		}
		bg.Brightness = [3]uint8{r[24], r[25], r[26]}
		n := bg.Width * bg.Height
		if n > MaxMapCells || len(b) < at+BackgroundSize+mapBytes(n) {
			return nil, errors.New("scene: the background's map runs past the end of the file")
		}
		bg.Map = make([]uint16, n)
		for i := range bg.Map {
			bg.Map[i] = le.Uint16(r[BackgroundSize+2*i:])
		}
		s.Background = bg
	}
	return s, nil
}

func (s *Scene) sizeFor(nt, nm, no, ns int) int {
	return HeaderSize + PathSize*(nt+nm) + ObjectSize*no + SpriteSize*ns
}
