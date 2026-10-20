package scene

import (
	"encoding/binary"
	"go.chrisrx.dev/psx/archive"
	farchive "go.chrisrx.dev/psx/format/archive"
	"testing"

	"go.chrisrx.dev/x/assert"

	"go.chrisrx.dev/psx/disc"
	format "go.chrisrx.dev/psx/format/scene"
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

func TestLoadBackground(t *testing.T) {
	sc := sample()
	sc.Background = &format.Background{
		Texture: "ASSETS/STONE.TIM", Cell: 16, Width: 3, Height: 2,
		Map:        []uint16{7, 7, 7, 2, format.EmptyCell, 2},
		Scroll:     [2]int16{4, -8},
		Brightness: [3]uint8{128, 100, 90},
	}
	b, err := sc.Bytes()
	if !assert.NoError(t, err) {
		t.FailNow()
	}
	var s Scene
	assert.NoError(t, s.Load(words(b), Files(files)))
	got := struct {
		Has      bool
		Scroll   [2]int16
		Window   [4]int16
		RGB      [3]uint8
		MapSize  [2]uint16
		Map      [6]uint16
		Texture  int
		Scale    [2]int16
		MapBound bool
	}{
		Has: s.HasBackground, Scroll: [2]int16{s.Background.Scrollx, s.Background.Scrolly},
		Window:   [4]int16{s.Background.X, s.Background.Y, s.Background.W, s.Background.H},
		RGB:      [3]uint8{s.Background.R, s.Background.G, s.Background.B},
		MapSize:  [2]uint16{s.tileMap.Ncellw, s.tileMap.Ncellh},
		Map:      [6]uint16(s.mapIndex[:6]),
		Texture:  s.backgroundTex,
		Scale:    [2]int16{s.Background.Scalex, s.Background.Scaley},
		MapBound: s.Background.Map == &s.tileMap && s.tileMap.Base == &s.cells[0] && s.tileMap.Index == &s.mapIndex[0],
	}
	want := got
	want.Has = true
	want.Scroll = [2]int16{4, -8}
	want.Window = [4]int16{-160, -120, 320, 240}
	want.RGB = [3]uint8{128, 100, 90}
	want.MapSize = [2]uint16{3, 2}
	want.Map = [6]uint16{7, 7, 7, 2, format.EmptyCell, 2}
	want.Texture = 0
	want.Scale = [2]int16{4096, 4096}
	want.MapBound = true
	assert.Equal(t, want, got)

	// Without one, a scene loaded into the same Scene has none.
	b, _ = sample().Bytes()
	assert.NoError(t, s.Load(words(b), Files(files)))
	assert.Equal(t, false, s.HasBackground)
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
