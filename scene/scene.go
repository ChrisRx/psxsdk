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
// A Scene holds its objects and sprites in arrays of a fixed size, the
// most a file holds, so that nothing is allocated; its layout is
// format/scene's.
package scene

import (
	"errors"

	"go.chrisrx.dev/psx/arc"

	"go.chrisrx.dev/psx/disc"
	"go.chrisrx.dev/psx/internal/volatile"
	"go.chrisrx.dev/psx/yaroze"
)

// The most of each a scene holds, as format/scene's.
const (
	MaxTextures = 16
	MaxModels   = 16
	MaxObjects  = 64
	MaxSprites  = 128
	MaxLights   = 3

	// A background's map holds at most MaxMapCells, cut from at most
	// MaxCells cells of its texture, each 16 pixels square, and is drawn
	// through a window of at most 320 by 240.
	MaxMapCells = 4096
	MaxCells    = 256
	EmptyCell   = 0xffff
	cellSize    = 16
	workWords   = ((320/cellSize+1)*(240/cellSize+2)*6+4)*2 + 2
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
// the next Draw sees and reads the file again. The editor finds it by its
// symbol, so the layout is fixed: the words' address, their count, the
// flag, and the Scene, each a word, then the name of the file the scene
// came from, 32 bytes, so that the editor knows which of a project's
// scenes the program is showing.
var live struct {
	data   *uint32
	words  uint32
	reload uint32
	scene  *Scene
	name   [32]byte
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

	// Background is the map drawn behind everything, when HasBackground,
	// with its cells, its map and the work area GsSortFixBg16 draws from.
	// The program scrolls it by its Scrollx and Scrolly.
	Background    yaroze.GsBG
	HasBackground bool
	backgroundTex int
	cells         [MaxCells]yaroze.GsCELL
	tileMap       yaroze.GsMAP
	mapIndex      [MaxMapCells]uint16
	work          [workWords]uint32

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
	s.setUp()
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

// reload reads the file again when the playground has written a new one
// over it: the editor's live view of the scene. A file it cannot read
// leaves the scene as it was.
func (s *Scene) reload() {
	if live.scene != s || volatile.LoadUint32(&live.reload) == 0 {
		return
	}
	volatile.StoreUint32(&live.reload, 0)
	if s.parse(s.data, s.src) != nil {
		return
	}
	s.setUp()
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
	hasBG := data[2]&(1<<8) != 0
	if nt > MaxTextures || nm > MaxModels || no > MaxObjects || ns > MaxSprites || nl > MaxLights {
		return ErrFormat
	}
	if len(data) < headerWords+pathWords*(nt+nm)+objectWords*no+spriteWords*ns {
		return ErrFormat
	}
	s.HasBackground = false
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
	if hasBG {
		if len(data) < at+backgroundWords {
			return ErrFormat
		}
		r := data[at:]
		t := int(r[0])
		w, h := int(r[2]&0xffff), int(r[2]>>16)
		n := w * h
		if t >= nt || uint16(r[1]) != cellSize || w == 0 || h == 0 || n > MaxMapCells || len(data) < at+backgroundWords+(n+1)/2 {
			return ErrFormat
		}
		s.backgroundTex = t
		s.tileMap = yaroze.GsMAP{Cellw: cellSize, Cellh: cellSize, Ncellw: uint16(w), Ncellh: uint16(h), Base: &s.cells[0], Index: &s.mapIndex[0]}
		bg := &s.Background
		*bg = yaroze.GsBG{
			Scrollx: int16(r[3]), Scrolly: int16(r[3] >> 16),
			X: int16(r[4]), Y: int16(r[4] >> 16), W: int16(r[5]), H: int16(r[5] >> 16),
			R: uint8(r[6]), G: uint8(r[6] >> 8), B: uint8(r[6] >> 16),
			Map:    &s.tileMap,
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
		words := r[backgroundWords:]
		for i := range n {
			s.mapIndex[i] = uint16(words[i/2] >> (16 * (i % 2)))
		}
		s.HasBackground = true
	}
	return nil
}

const backgroundWords = 28 / 4

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

// Draw sorts the background, if there is one, at the far end of ot, the
// objects into ot, and the sprites into sprites, which may be the screen's
// overlay table, so that they go in front of the models, or ot itself.
func (s *Scene) Draw(ot, sprites *yaroze.GsOT) {
	if !s.ready {
		return
	}
	s.reload()
	if s.HasBackground {
		yaroze.GsSortFixBg16(&s.Background, &s.work[0], ot, uint16(1<<ot.Length)-1)
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
	a     arc.Archive
	n     int
	names [MaxTextures + MaxModels]string
	words [MaxTextures + MaxModels][]uint32
}

// NewArchive makes an opened archive a Source.
func NewArchive(a arc.Archive) *Archive { return &Archive{a: a} }

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
