package scene

import (
	"encoding/binary"
	"go.chrisrx.dev/psx/archive"
	farchive "go.chrisrx.dev/psx/format/archive"
	"testing"

	"go.chrisrx.dev/x/assert"

	"go.chrisrx.dev/psx/disc"
	format "go.chrisrx.dev/psx/format/scene"
	"go.chrisrx.dev/psx/stage"
	"go.chrisrx.dev/psx/yaroze"
)

func words(b []byte) []uint32 {
	out := make([]uint32, (len(b)+3)/4)
	for i := range out {
		var w [4]byte
		copy(w[:], b[4*i:])
		out[i] = binary.LittleEndian.Uint32(w[:])
	}
	return out
}

var (
	cubeData  [4]uint32
	stoneData [4]uint32
	cubeTMD   = disc.Register("ASSETS/CUBE.TMD", "assets/CUBE.TMD").Into(cubeData[:])
	stoneTIM  = disc.Register("ASSETS/STONE.TIM", "assets/STONE.TIM").Into(stoneData[:])
	emptyTMD  = disc.Register("ASSETS/EMPTY.TMD", "assets/EMPTY.TMD")
	files     = []*disc.File{&cubeTMD, &stoneTIM, &emptyTMD}
)

func sample() *format.Scene {
	return &format.Scene{
		Camera:     format.Camera{From: [3]int32{0, -200, -1000}, At: [3]int32{0, 0, 50}, Twist: 3},
		Projection: 320,
		Ambient:    [3]int32{1024, 512, 256},
		Lights: []format.Light{
			{Direction: [3]int32{1, 1, 1}, Color: [3]uint8{0xc0, 0xc0, 0xc0}},
			{Direction: [3]int32{0, -1, 0}, Color: [3]uint8{0x40, 0x40, 0x80}},
		},
		Textures: []string{"ASSETS/STONE.TIM"},
		Models:   []string{"ASSETS/CUBE.TMD"},
		Objects: []format.Object{
			{Name: "cube", Model: "ASSETS/CUBE.TMD", Position: [3]int32{10, -20, 300}, Rotation: [3]int16{0, 512, -4}, Scale: [3]int32{4096, 2048, 4096}},
			{Name: "second cube", Model: "ASSETS/CUBE.TMD", Index: 1, Position: [3]int32{-300, 28, 400}, Fields: [2]int32{7, -3}},
		},
		Sprites: []format.Sprite{
			{Name: "badge", Texture: "ASSETS/STONE.TIM", Attribute: 1 << 24, X: 10, Y: -20, W: 64, H: 32, TPage: 0xa, U: 3, V: 7, CX: 16, CY: 480, R: 128, G: 100, B: 90, MX: 2, MY: 3, ScaleX: 4096, ScaleY: 2048, Rotate: 90 * 4096},
		},
	}
}

func TestLoad(t *testing.T) {
	b, err := sample().Bytes()
	if !assert.NoError(t, err) {
		t.FailNow()
	}
	var s Scene
	assert.NoError(t, s.Load(words(b), Files(files)))
	assert.Equal(t, true, s.Ready())

	got := struct {
		Camera     yaroze.GsRVIEW2
		Projection int32
		Ambient    [3]int32
		NLights    int
		Light0     yaroze.GsF_LIGHT
		Light1RGB  [3]uint8
		Counts     [4]int
		Cube       yaroze.VECTOR
		CubeRot    yaroze.SVECTOR
		CubeScale  yaroze.VECTOR
		Names      [2]string
		Second     *yaroze.Object
		None       *yaroze.Object
		Sprite     yaroze.GsSPRITE
		SpriteName string
		Texture0   uint32
		Model0     uint32
	}{
		Camera:     s.Camera,
		Projection: s.Projection,
		Ambient:    s.Ambient,
		NLights:    s.NLights,
		Light0:     s.Lights[0],
		Light1RGB:  [3]uint8{s.Lights[1].R, s.Lights[1].G, s.Lights[1].B},
		Counts:     [4]int{s.NTextures, s.NModels, s.NObjects, s.NSprites},
		Cube:       s.Object("cube").Position,
		CubeRot:    s.Object("cube").Rotation,
		CubeScale:  s.Object("cube").Scale,
		Names:      [2]string{s.ObjectName(0), s.ObjectName(1)},
		Second:     s.Object("second cube"),
		None:       s.Object("cub"),
		Sprite:     *s.Sprite("badge"),
		SpriteName: s.SpriteName(0),
		Texture0:   s.Textures[0][0],
		Model0:     s.Models[0][0],
	}
	cubeData[0], stoneData[0] = 0x41, 0x10
	want := got
	want.Camera = yaroze.GsRVIEW2{Vpx: 0, Vpy: -200, Vpz: -1000, Vrx: 0, Vry: 0, Vrz: 50, Rz: 3, Super: yaroze.WORLD}
	want.Projection = 320
	want.Ambient = [3]int32{1024, 512, 256}
	want.NLights = 2
	want.Light0 = yaroze.GsF_LIGHT{Vx: 1, Vy: 1, Vz: 1, R: 0xc0, G: 0xc0, B: 0xc0}
	want.Light1RGB = [3]uint8{0x40, 0x40, 0x80}
	want.Counts = [4]int{1, 1, 2, 1}
	want.Cube = yaroze.VECTOR{Vx: 10, Vy: -20, Vz: 300}
	want.CubeRot = yaroze.SVECTOR{Vx: 0, Vy: 512, Vz: -4}
	want.CubeScale = yaroze.VECTOR{Vx: 4096, Vy: 2048, Vz: 4096}
	want.Names = [2]string{"cube", "second cube"}
	want.Second = &s.Objects[1]
	want.None = nil
	want.Sprite = yaroze.GsSPRITE{Attribute: 1 << 24, X: 10, Y: -20, W: 64, H: 32, Tpage: 0xa, U: 3, V: 7, Cx: 16, Cy: 480, R: 128, G: 100, B: 90, Mx: 2, My: 3, Scalex: 4096, Scaley: 2048, Rotate: 90 * 4096}
	want.SpriteName = "badge"
	want.Texture0 = 0x10
	want.Model0 = 0x41
	got.Texture0, got.Model0 = s.Textures[0][0], s.Models[0][0]
	assert.Equal(t, want, got)
	model, index := s.ObjectModel(1)
	assert.Equal(t, [2]int32{0, 1}, [2]int32{int32(model), index})
	f0, f1 := s.ObjectFields(1)
	assert.Equal(t, [2]int32{7, -3}, [2]int32{f0, f1})
	f0, f1 = s.ObjectFields(0)
	assert.Equal(t, [2]int32{0, 0}, [2]int32{f0, f1})
}

// withLayers gives the sample a far layer and a ground layer with flags,
// as a 2D game's are.
func withLayers(sc *format.Scene) *format.Scene {
	sc.Layers = []format.Layer{
		{
			Name: "far", Texture: "ASSETS/STONE.TIM", Cell: 16, Width: 4, Height: 2,
			Map:        []uint16{0, 1, 2, 3, 4, format.EmptyCell, 4, 5},
			Scroll:     [2]int16{0, 8},
			Brightness: [3]uint8{128, 128, 128},
			Speed:      50,
		},
		{
			Name: "ground", Texture: "ASSETS/STONE.TIM", Cell: 16, Width: 3, Height: 2,
			Map:        []uint16{7, 7, 7, 2, format.EmptyCell, 2},
			Scroll:     [2]int16{4, -8},
			Brightness: [3]uint8{128, 100, 90},
			Speed:      100,
			Flags:      []int{0, 0, Solid | Hurts, 0, 0, 0, 0, Pickup | 1<<5},
		},
	}
	return sc
}

func TestLoadLayers(t *testing.T) {
	b, err := withLayers(sample()).Bytes()
	if !assert.NoError(t, err) {
		t.FailNow()
	}
	var s Scene
	assert.NoError(t, s.Load(words(b), Files(files)))
	assert.Equal(t, 2, s.NLayers())
	far, ground := s.Layer("far"), s.Layer("ground")
	if far == nil || ground == nil {
		t.Fatal("both layers should be found by name")
	}
	assert.Equal(t, far, s.LayerAt(0), "the far layer is the first")
	assert.Equal(t, (*Layer)(nil), s.Layer("sky"))
	got := struct {
		Names    [2]string
		Speeds   [2]int
		Size     [2]int
		Scroll   [2]int16
		Window   [4]int16
		RGB      [3]uint8
		Scale    [2]int16
		MapBound bool
	}{
		Names:  [2]string{far.Name(), ground.Name()},
		Speeds: [2]int{far.Speed, ground.Speed},
		Size:   [2]int{ground.Width(), ground.Height()},
		Scroll: [2]int16{ground.BG.Scrollx, ground.BG.Scrolly},
		Window: [4]int16{ground.BG.X, ground.BG.Y, ground.BG.W, ground.BG.H},
		RGB:    [3]uint8{ground.BG.R, ground.BG.G, ground.BG.B},
		Scale:  [2]int16{ground.BG.Scalex, ground.BG.Scaley},
		MapBound: ground.BG.Map == &ground.tileMap && ground.tileMap.Base == &ground.cells[0] && ground.tileMap.Index == &ground.index[0] &&
			far.BG.Map == &far.tileMap && far.tileMap.Index == &far.index[0],
	}
	assert.Equal(t, struct {
		Names    [2]string
		Speeds   [2]int
		Size     [2]int
		Scroll   [2]int16
		Window   [4]int16
		RGB      [3]uint8
		Scale    [2]int16
		MapBound bool
	}{[2]string{"far", "ground"}, [2]int{50, 100}, [2]int{3, 2}, [2]int16{4, -8}, [4]int16{-160, -120, 320, 240}, [3]uint8{128, 100, 90}, [2]int16{4096, 4096}, true}, got)

	// Cells and their flags, by cell and by pixel, off the map none.
	assert.Equal(t, 2, ground.Cell(0, 1))
	assert.Equal(t, EmptyCell, ground.Cell(1, 1))
	assert.Equal(t, EmptyCell, ground.Cell(3, 0))
	assert.Equal(t, EmptyCell, ground.Cell(-1, 0))
	assert.Equal(t, uint8(Solid|Hurts), ground.Flags(2, 1))
	assert.Equal(t, uint8(0), ground.Flags(1, 1), "an empty cell has no flags")
	assert.Equal(t, uint8(Pickup|1<<5), ground.FlagsAt(20, 3))
	assert.Equal(t, uint8(Solid|Hurts), ground.FlagsAt(47, 31))
	assert.Equal(t, uint8(0), ground.FlagsAt(-1, 3), "a point left of the map is off it")
	assert.Equal(t, uint8(0), far.Flags(0, 0), "a layer without flags has none")

	// SetCell changes the scene's map, not the file's.
	ground.SetCell(1, 0, EmptyCell)
	ground.SetCell(9, 9, 3)
	assert.Equal(t, EmptyCell, ground.Cell(1, 0))
	assert.Equal(t, uint8(0), ground.Flags(1, 0))

	// Scroll moves each layer by its speed's share, from where the file
	// put it.
	s.Scroll(100, -20)
	assert.Equal(t, [4]int16{50, -2, 104, -28}, [4]int16{far.BG.Scrollx, far.BG.Scrolly, ground.BG.Scrollx, ground.BG.Scrolly})
	s.Scroll(0, 0)
	assert.Equal(t, [4]int16{0, 8, 4, -8}, [4]int16{far.BG.Scrollx, far.BG.Scrolly, ground.BG.Scrollx, ground.BG.Scrolly})

	// Without layers, a scene loaded into the same Scene has none.
	b, _ = sample().Bytes()
	assert.NoError(t, s.Load(words(b), Files(files)))
	assert.Equal(t, 0, s.NLayers())
	assert.Equal(t, (*Layer)(nil), s.Layer("ground"))
}

// A file cooked before layers had one, with no name or flags after its map:
// it loads as a layer named "background", moving with the scroll.
func TestLoadBeforeLayers(t *testing.T) {
	sc := sample()
	sc.Background = &format.Background{
		Texture: "ASSETS/STONE.TIM", Cell: 16, Width: 3, Height: 2,
		Map:        []uint16{7, 7, 7, 2, format.EmptyCell, 2},
		Brightness: [3]uint8{128, 128, 128},
	}
	b, err := sc.Bytes()
	if !assert.NoError(t, err) {
		t.FailNow()
	}
	at := format.HeaderSize + 2*format.PathSize + 2*format.ObjectSize + format.SpriteSize
	old := append([]byte{}, b[:at+format.LayerSize+12]...)
	old[at+6], old[at+7], old[at+27] = 0, 0, 0
	var s Scene
	assert.NoError(t, s.Load(words(old), Files(files)))
	assert.Equal(t, 1, s.NLayers())
	l := s.Layer("background")
	if l == nil {
		t.Fatal("the old file's map should load as the layer named background")
	}
	assert.Equal(t, 100, l.Speed)
	assert.Equal(t, 2, l.Cell(2, 1))
}

func TestLoadRoom(t *testing.T) {
	sc := &format.Scene{Room: &format.Room{
		Floor: 1, Wall: 2, Tint: [3]uint8{128, 100, 90},
		Cells:   []string{"#1##", "#..#", "#2##"},
		Heights: []string{"0000", "0120", "0000"},
		Rough:   true, RockTall: 128, RockLean: 36, CamUp: 560, CamBack: 300,
		Sky:        [2][3]uint8{{30, 70, 190}, {170, 210, 245}},
		Background: [3]uint8{1, 1, 1},
		Links:      []format.Link{{Door: 1, Scene: "ASSETS/HALL.DAT"}, {Door: 2, Scene: "ASSETS/CAVE.DAT"}},
		Markers:    []format.Marker{{Name: "chest", Cell: [2]int{1, 1}, Fields: [2]int32{4, -1}}, {Name: "host", Cell: [2]int{2, 1}}},
	}}
	b, err := sc.Bytes()
	if !assert.NoError(t, err) {
		t.FailNow()
	}
	var s Scene
	// A room's file needs no source: it names no texture or model.
	assert.NoError(t, s.Load(words(b), nil))
	r := s.Room()
	if r == nil {
		t.Fatal("the scene has no room")
	}
	assert.Equal(t, []string{"#1##", "#..#", "#2##"}, r.Cells)
	assert.Equal(t, []string{"0000", "0120", "0000"}, r.Heights, "the levels come back as digits")
	assert.Equal(t, stage.Map{Floor: 1, Wall: 2, Tint: stage.Color{128, 100, 90}, Cells: r.Cells, Heights: r.Heights, Rough: true, RockTall: 128, RockLean: 36}, r.Map)
	assert.Equal(t, [2]stage.Color{{30, 70, 190}, {170, 210, 245}}, r.Sky)
	assert.Equal(t, stage.Color{1, 1, 1}, r.Background)
	assert.Equal(t, 560, r.CamUp)
	assert.Equal(t, 300, r.CamBack)
	assert.Equal(t, "ASSETS/CAVE.DAT", s.Link('2'))
	assert.Equal(t, "", s.Link('3'))
	assert.Equal(t, &Marker{I: 1, J: 1, Fields: [2]int32{4, -1}}, s.Marker("chest"))
	assert.Equal(t, &Marker{I: 2, J: 1}, s.Marker("host"))
	assert.Equal(t, (*Marker)(nil), s.Marker("nobody"))
	assert.Equal(t, 2, s.NMarkers())
	assert.Equal(t, "host", s.MarkerName(1))
	assert.Equal(t, 2, r.Level(2, 1))
	assert.Equal(t, byte('1'), r.Cell(1, 0))

	// A file with no room has none, and a room's rows are read again
	// on a second load without harm.
	assert.NoError(t, s.Load(words(b), nil))
	assert.Equal(t, []string{"0000", "0120", "0000"}, s.Room().Heights)
	plain, _ := sample().Bytes()
	assert.NoError(t, s.Load(words(plain), Files(files)))
	assert.Equal(t, (*Room)(nil), s.Room())
}

func TestLoadMissing(t *testing.T) {
	sc := sample()
	sc.Models = []string{"ASSETS/EMPTY.TMD"}
	for i := range sc.Objects {
		sc.Objects[i].Model = "ASSETS/EMPTY.TMD"
	}
	b, err := sc.Bytes()
	if !assert.NoError(t, err) {
		t.FailNow()
	}
	var s Scene
	assert.Equal(t, ErrMissing, s.Load(words(b), Files(files)))
	assert.Equal(t, "ASSETS/EMPTY.TMD", s.Missing)
	assert.Equal(t, false, s.Ready())

	sc = sample()
	sc.Textures = []string{"ASSETS/NONE.TIM"}
	sc.Sprites[0].Texture = "ASSETS/NONE.TIM"
	b, _ = sc.Bytes()
	assert.Equal(t, ErrMissing, s.Load(words(b), Files(files)))
	assert.Equal(t, "ASSETS/NONE.TIM", s.Missing)
}

func TestLoadFormat(t *testing.T) {
	var s Scene
	assert.Equal(t, ErrFormat, s.Load([]uint32{1, 2, 3}, Files(files)))
	b, _ := sample().Bytes()
	w := words(b)
	assert.Equal(t, ErrFormat, s.Load(w[:len(w)-1], Files(files)))
	w[1] |= 200 << 16 // more objects than a scene holds
	assert.Equal(t, ErrFormat, s.Load(w, Files(files)))
}

// TestArchive loads a scene whose texture and model are entries of an
// archive, the texture stored and used where it lies, the model packed and
// unpacked once for both loads.
func TestArchive(t *testing.T) {
	stone := make([]byte, 16)
	for i := range stone {
		stone[i] = byte(i)
	}
	cube := make([]byte, 4096)
	for i := range cube {
		cube[i] = byte(i / 64)
	}
	packed, err := farchive.Pack([]farchive.Entry{
		{Name: "STONE.TIM", Data: stone, Store: true},
		{Name: "CUBE.TMD", Data: cube},
	})
	assert.NoError(t, err)
	a, err := archive.Open(words(packed))
	assert.NoError(t, err)
	src := NewArchive(a)

	b, err := sample().Bytes()
	assert.NoError(t, err)
	var s Scene
	assert.NoError(t, s.Load(words(b), Sources{Files(files[2:]), src}))
	assert.Equal(t, 2, src.n, "both entries found")
	assert.Equal(t, "STONE.TIM", src.names[0])
	assert.Equal(t, "", s.Name(), "a scene not from a file has no name")
	model := src.Lookup("ASSETS/CUBE.TMD")
	assert.Equal(t, 1024, len(model), "the model unpacked")
	assert.Equal(t, uint32(0x01010101), model[16], "to its bytes")
	assert.Equal(t, true, &model[0] == &src.Lookup("CUBE.TMD")[0], "and kept")
	assert.Equal(t, 0, len(src.Lookup("ASSETS/NOWHERE.TIM")))

	sc := sample()
	sc.Textures = append(sc.Textures, "ASSETS/ELSEWHERE.TIM")
	b, err = sc.Bytes()
	assert.NoError(t, err)
	assert.Equal(t, ErrMissing, s.Load(words(b), src))
	assert.Equal(t, "ASSETS/ELSEWHERE.TIM", s.Missing)
}
