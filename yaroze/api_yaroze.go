//go:build yaroze

package yaroze

import (
	"unsafe"

	"go.chrisrx.dev/psx/geom"
	"go.chrisrx.dev/psx/gpu"
)

// The console side of the Go API: what it calls on the library in a
// program built for the console. api_stub.go is the same for every other
// build, where each call panics.

// The library's display modes: GsInitGraph's interlace and origin bits.
const (
	interlaced uint16 = 1
	offsetGPU  uint16 = 4
)

// The sequencer's play mode and its count for playing without end.
const (
	playMode    uint8 = 1
	playForever int16 = 0
)

// A Sprite and a Box start with the library's structs under other names,
// up to the fields Draw packs into the attribute word.
var (
	_ [unsafe.Offsetof(Sprite{}.Blend) - unsafe.Sizeof(GsSPRITE{})]byte
	_ [unsafe.Sizeof(GsSPRITE{}) - unsafe.Offsetof(Sprite{}.Blend)]byte
	_ [unsafe.Offsetof(Box{}.Blend) - unsafe.Sizeof(GsBOXF{})]byte
	_ [unsafe.Sizeof(GsBOXF{}) - unsafe.Offsetof(Box{}.Blend)]byte
)

func openScreen(s *Screen) {
	SetVideoMode(int32(NativeVideoMode()))
	dither := uint16(1)
	if s.NoDither {
		dither = 0
	}
	w, h := uint16(s.Width), uint16(s.Height)
	if s.Height > 256 {
		GsInitGraph(w, h, interlaced|offsetGPU, dither, 0)
		GsDefDispBuff(0, 0, 0, 0)
	} else {
		GsInitGraph(w, h, offsetGPU, dither, 0)
		if s.Buffers == ScreenBuffersSideBySide {
			GsDefDispBuff(0, 0, w, 0)
		} else {
			GsDefDispBuff(0, 0, 0, h)
		}
	}
	env := GsDISPENV()
	env.Screen.X += s.Offset.X
	env.Screen.Y += s.Offset.Y
	if s.Origin == ScreenOriginMiddle {
		GsInit3D()
	}

	if s.impl.ot[0].Org == nil {
		s.Init(make([]GsOT_TAG, 2<<s.Depth), make([]uint32, 2*s.Packets/4))
	}
	if s.OverlayDepth != 0 && s.impl.overlay[0].Org == nil {
		tags := make([]GsOT_TAG, 2<<s.OverlayDepth)
		for i := range s.impl.overlay {
			s.impl.overlay[i].Length = uint32(s.OverlayDepth)
			s.impl.overlay[i].Org = &tags[i<<s.OverlayDepth]
		}
	}
}

func resetGraph() { ResetGraph(3) }

// beginFrame starts a frame: it takes the buffer the frame goes into,
// points the library at its packet space, and clears its ordering
// table.
func beginFrame(s *Screen) *Frame {
	s.active = int(GsGetActiveBuff())
	GsSetWorkBase(Packets(&s.impl.packets[s.active][0]))
	GsClearOt(0, 0, &s.impl.ot[s.active])
	s.cur.table = &s.impl.ot[s.active]
	if s.OverlayDepth != 0 {
		GsClearOt(0, 0, &s.impl.overlay[s.active])
		s.over.table = &s.impl.overlay[s.active]
	}
	return &s.cur
}

// endFrame finishes the frame: it waits for the GPU to finish the last
// one and for the display's vertical blank, swaps the buffers, puts the
// background behind everything, starts the GPU on the frame, and draws
// the frame's text after it.
func endFrame(s *Screen) int {
	ot := &s.impl.ot[s.active]
	DrawSync(0)
	var n int32
	if s.Interval > 1 {
		n = VSync(int32(s.Interval))
	} else {
		n = VSync(0)
	}
	f := VSync(-1)
	s.fields = int(f - s.lastField)
	s.lastField = f
	GsSwapDispBuff()
	if s.backdrop != nil {
		// The backdrop goes into the buffer about to be drawn, which
		// is the other one from the frame's own index: below or beside
		// the first.
		r := RECT{
			W: s.backdropW,
			H: s.backdropH,
		}
		if s.Buffers == ScreenBuffersSideBySide {
			r.X = int16(1-s.active) * int16(s.Width)
		} else {
			r.Y = int16(1-s.active) * int16(s.Height)
		}
		LoadImage(&r, s.backdrop)
		DrawSync(0)
	} else {
		GsSortClear(s.Background.R, s.Background.G, s.Background.B, ot)
	}
	GsDrawOt(ot)
	if s.OverlayDepth != 0 {
		GsDrawOt(&s.impl.overlay[s.active])
	}
	if s.text >= 0 {
		FntFlush(s.text)
	}
	for _, id := range s.kanji[:s.nkanji] {
		KanjiFntFlush(id)
	}
	s.frame++
	return int(n)
}

func vsync(n int32) int32 { return VSync(n) }

func fontOpen(x, y, w, h, chars int) int32 {
	FntLoad(960, 256)
	return FntOpen(int32(x), int32(y), int32(w), int32(h), 0, int32(chars))
}

func fntWrite(id int32, s string) {
	fntWriteBytes(id, unsafe.Slice(unsafe.StringData(s), len(s)))
}

// fntWriteBytes prints b on stream id through the library's own FntPrint,
// which takes a format, so a percent sign goes doubled, and a piece at a time.
func fntWriteBytes(id int32, b []byte) {
	var buf [128]byte
	for len(b) > 0 {
		n := 0
		for len(b) > 0 && n < len(buf)-2 {
			if b[0] == '%' {
				buf[n] = '%'
				n++
			}
			buf[n] = b[0]
			n++
			b = b[1:]
		}
		buf[n] = 0
		FntPrintStream(id, &buf[0])
	}
}

func sortSprite(sp *Sprite, f *Frame, depth int) {
	GsSortSprite((*GsSPRITE)(unsafe.Pointer(sp)), f.table, uint16(depth))
}

func sortFastSprite(sp *Sprite, f *Frame, depth int) {
	GsSortFastSprite((*GsSPRITE)(unsafe.Pointer(sp)), f.table, uint16(depth))
}

func sortBox(b *Box, f *Frame, depth int) {
	GsSortBoxFill((*GsBOXF)(unsafe.Pointer(b)), f.table, uint16(depth))
}

// A Line's and a GradientLine's first fields, a Cell, a tileMapImpl and a
// Background's first fields are the library's structs under other names.
var (
	_ [unsafe.Offsetof(Line{}.Blend) - unsafe.Sizeof(GsLINE{})]byte
	_ [unsafe.Sizeof(GsLINE{}) - unsafe.Offsetof(Line{}.Blend)]byte
	_ [unsafe.Offsetof(GradientLine{}.Blend) - unsafe.Sizeof(GsGLINE{})]byte
	_ [unsafe.Sizeof(GsGLINE{}) - unsafe.Offsetof(GradientLine{}.Blend)]byte
	_ [unsafe.Sizeof(Cell{}) - unsafe.Sizeof(GsCELL{})]byte
	_ [unsafe.Sizeof(GsCELL{}) - unsafe.Sizeof(Cell{})]byte
	_ [unsafe.Sizeof(tileMapImpl{}) - unsafe.Sizeof(GsMAP{})]byte
	_ [unsafe.Sizeof(GsMAP{}) - unsafe.Sizeof(tileMapImpl{})]byte
	_ [unsafe.Offsetof(Background{}.work) - unsafe.Sizeof(GsBG{})]byte
	_ [unsafe.Sizeof(GsBG{}) - unsafe.Offsetof(Background{}.work)]byte
)

func sortLine(l *Line, f *Frame, depth int) {
	GsSortLine((*GsLINE)(unsafe.Pointer(l)), f.table, uint16(depth))
}

func sortGradientLine(l *GradientLine, f *Frame, depth int) {
	GsSortGLine((*GsGLINE)(unsafe.Pointer(l)), f.table, uint16(depth))
}

func initBackground(bg *Background) {
	GsInitFixBg16((*GsBG)(unsafe.Pointer(bg)), &bg.work[0])
}

func sortBackground(bg *Background, f *Frame, depth int) {
	GsSortFixBg16((*GsBG)(unsafe.Pointer(bg)), &bg.work[0], f.table, uint16(depth))
}

// loadTIM reads the TIM's header, loads its pixels where the header says
// or at x, y, and its palette where the header says, waits for the
// transfer, and returns the image.
func loadTIM(words []uint32, at bool, x, y int) Image {
	var info GsIMAGE
	GsGetTimInfo(&words[1], &info)
	if at {
		info.Px, info.Py = int16(x), int16(y)
	}
	rect := RECT{
		X: info.Px,
		Y: info.Py,
		W: int16(info.Pw),
		H: int16(info.Ph),
	}
	LoadImage(&rect, info.Pixel)
	if info.Pmode&8 != 0 {
		rect = RECT{
			X: info.Cx,
			Y: info.Cy,
			W: int16(info.Cw),
			H: int16(info.Ch),
		}
		LoadImage(&rect, info.Clut)
	}
	DrawSync(0)

	// The file's length: its two header words, the palette block if it
	// has one, and the pixel block, each a three-word header and its
	// 16-bit units, worked out from the sizes rather than read from the
	// blocks' own length words, which some files leave short.
	n := 2 + 3 + int(info.Pw)*int(info.Ph)/2
	if info.Pmode&8 != 0 {
		n += 3 + int(info.Cw)*int(info.Ch)/2
	}

	// Pixels per 16-bit unit of video memory: four at 4 bits a pixel,
	// two at 8 and one at 16.
	depth := info.Pmode & 3
	per := uint16(4 >> depth)
	return Image{
		depth: depth,
		w:     info.Pw * per,
		h:     info.Ph,
		words: n,
		texture: Texture{
			Page: GetTPage(int32(depth), 0, int32(info.Px), int32(info.Py)),
			U:    uint8(uint16(info.Px&0x3f) * per),
			V:    uint8(info.Py & 0xff),
			CX:   info.Cx,
			CY:   info.Cy,
		},
	}
}

func mainVolume(left, right int) { SsSetMVol(int16(left), int16(right)) }

func vabTransfer(vh, vb []uint32) int16 {
	return int16(SsVabTransfer((*uint8)(unsafe.Pointer(&vh[0])), (*uint8)(unsafe.Pointer(&vb[0])), -1, 1))
}

func vabClose(id int16)                     { SsVabClose(id) }
func seqOpen(seq []uint32, vab int16) int16 { return int16(SsSeqOpen(&seq[0], vab)) }
func seqPlay(id int16, times int16)         { SsSeqPlay(id, playMode, times) }
func seqVolume(id int16, left, right int)   { SsSeqSetVol(id, int16(left), int16(right)) }
func seqPause(id int16)                     { SsSeqPause(id) }
func seqResume(id int16)                    { SsSeqReplay(id) }
func seqStop(id int16)                      { SsSeqStop(id) }
func seqPlaying(id int16) bool              { return SsIsEos(id, 0) != 0 }
func seqClose(id int16)                     { SsSeqClose(id) }

func randNext() int  { return int(Rand()) }
func randSeed(s int) { Srand(uint32(s)) }

// origin is where the frame's origin is on the picture, from its top
// left corner.
func origin() (x, y int) {
	origin := POSITION()
	ox, oy := origin.Offx, origin.Offy
	return int(ox), int(oy)
}

// A geom.Matrix, a coordImpl and an objectImpl are the library's MATRIX,
// GsCOORDINATE2 and GsDOBJ2 under other names.
var (
	_ [unsafe.Sizeof(geom.Matrix{}) - unsafe.Sizeof(MATRIX{})]byte
	_ [unsafe.Sizeof(MATRIX{}) - unsafe.Sizeof(geom.Matrix{})]byte
	_ [unsafe.Sizeof(coordImpl{}) - unsafe.Sizeof(GsCOORDINATE2{})]byte
	_ [unsafe.Sizeof(GsCOORDINATE2{}) - unsafe.Sizeof(coordImpl{})]byte
	_ [unsafe.Sizeof(objectImpl{}) - unsafe.Sizeof(GsDOBJ2{})]byte
	_ [unsafe.Sizeof(GsDOBJ2{}) - unsafe.Sizeof(objectImpl{})]byte
)

func mapModel(words []uint32) { GsMapModelingData(&words[1]) }

func initCoord(c, super *coordImpl) {
	GsInitCoordinate2((*GsCOORDINATE2)(unsafe.Pointer(super)), (*GsCOORDINATE2)(unsafe.Pointer(c)))
}

func linkObject(o *objectImpl, m Model, n int) {
	GsLinkObject4(&m[3], (*GsDOBJ2)(unsafe.Pointer(o)), int32(n))
}

// objectMatrices makes the object's matrix from its position, rotation
// and scale, and works out its matrices to the world and the screen in
// one walk of its parents.
func objectMatrices(o *Object, world, screen *geom.Matrix) {
	coord := (*GsCOORDINATE2)(unsafe.Pointer(&o.coord))
	m := &coord.Coord
	r := svector(o.Rotation)
	RotMatrix(&r, m)
	if o.Scale != (geom.Vector{}) {
		s := lvector(o.Scale)
		ScaleMatrix(m, &s)
	}
	m.T = [3]int32{int32(o.Position.X), int32(o.Position.Y), int32(o.Position.Z)}
	coord.Flg = 0
	GsGetLws(coord, (*MATRIX)(unsafe.Pointer(world)), (*MATRIX)(unsafe.Pointer(screen)))
}

func setLightMatrix(m *geom.Matrix)  { GsSetLightMatrix((*MATRIX)(unsafe.Pointer(m))) }
func setScreenMatrix(m *geom.Matrix) { GsSetLsMatrix((*MATRIX)(unsafe.Pointer(m))) }

// sortObject sorts the model with the CPU's scratchpad as the
// subdivision's work space.
func sortObject(o *Object, f *Frame) {
	scratch := (*uint32)(unsafe.Pointer(uintptr(0x1f800000)))
	GsSortObject4((*GsDOBJ2)(unsafe.Pointer(&o.obj)), f.table, int32(14-f.s.Depth), scratch)
}

func setVideo(mode VideoMode) { SetVideoMode(int32(mode)) }

func shiftOffset(dx, dy int) {
	env := GsDISPENV()
	env.Screen.X += int16(dx)
	env.Screen.Y += int16(dy)
}

// backdropOf reads a 16-bit TIM's pixels and size, for the backdrop.
func backdropOf(tim []uint32) (pixels *uint32, w, h int16) {
	var info GsIMAGE
	GsGetTimInfo(&tim[1], &info)
	return info.Pixel, int16(info.Pw), int16(info.Ph)
}

func copyRect(src geom.Rect, x, y int) {
	r := ToRECT(src)
	MoveImage(&r, int32(x), int32(y))
}

func clearRect(r geom.Rect, c Color) {
	rr := ToRECT(r)
	ClearImage(&rr, c.R, c.G, c.B)
}

func loadRect(r geom.Rect, words []uint32) {
	rr := ToRECT(r)
	LoadImage(&rr, &words[0])
}

func storeRect(r geom.Rect, words []uint32) {
	rr := ToRECT(r)
	StoreImage(&rr, &words[0])
	DrawSync(0)
}

func setReverb(kind Reverb, left, right int) {
	SsUtSetReverbType(int16(kind))
	SsUtReverbOn()
	SsUtSetReverbDepth(int16(left), int16(right))
}

func reverbOff() { SsUtReverbOff() }

// modelVectors returns the vectors whose address and count the model's
// object table holds at word i.
func modelVectors(m Model, i int) []Vertex {
	return unsafe.Slice((*Vertex)(unsafe.Pointer(uintptr(m[i]))), m[i+1])
}

var (
	_ [unsafe.Sizeof(Vertex{}) - unsafe.Sizeof(SVECTOR{})]byte
	_ [unsafe.Sizeof(SVECTOR{}) - unsafe.Sizeof(Vertex{})]byte
)

func blend(vertices, diffs *Vertex, n int, p geom.Fixed) {
	GteMIMefunc((*SVECTOR)(unsafe.Pointer(vertices)), (*SVECTOR)(unsafe.Pointer(diffs)), int32(n), int32(p))
}

func setFog(dqa int16, dqb int32, c Color) {
	fog := GsFOGPARAM{
		Dqa: dqa,
		Dqb: dqb,
		Rfc: c.R,
		Gfc: c.G,
		Bfc: c.B,
	}
	GsSetFogParam(&fog)
}

func setLightMode(mode int32) { GsSetLightMode(mode) }

// allocPacket takes n words of the frame's packet space, from where the
// library's own drawing has got to, and moves it on. It reads and writes
// the library's pointer itself, as PacketPointer and SetPacketPointer do,
// which saves the frame a call to GsSetWorkBase for every packet.
func allocPacket(n int) *uint32 {
	p := unsafe.Pointer(gsOUT_PACKET_P)
	gsOUT_PACKET_P = (*PACKET)(unsafe.Add(p, 4*n))
	return (*uint32)(p)
}

// linkPacket puts the packet whose tag is at tag at the head of the
// frame's entry for depth, keeping the tag's length and the entry's.
func linkPacket(f *Frame, depth int, tag *gpu.Tag) {
	entry := (*uint32)(unsafe.Add(unsafe.Pointer(f.table.Org), 4*depth))
	linkAfter((*gpu.Tag)(unsafe.Pointer(entry)), tag)
}

// linkAfter puts the packet whose tag is at tag after the tag prev, an
// entry's or another packet's.
func linkAfter(prev, tag *gpu.Tag) {
	*tag = *tag&0xff000000 | *prev&0xffffff
	*prev = *prev&0xff000000 | gpu.Tag(uintptr(unsafe.Pointer(tag)))&0xffffff
}

func setCamera(c *Camera) {
	projection = c.Distance
	GsSetProjection(int32(c.Distance))
	view := GsRVIEW2{
		Vpx: int32(c.Eye.X),
		Vpy: int32(c.Eye.Y),
		Vpz: int32(c.Eye.Z),
		Vrx: int32(c.Target.X),
		Vry: int32(c.Target.Y),
		Vrz: int32(c.Target.Z),
		Rz:  int32(c.Roll),
	}
	GsSetRefView2(&view)
}

func setLight(n int, l Light) {
	light := GsF_LIGHT{
		Vx: int32(l.Direction.X),
		Vy: int32(l.Direction.Y),
		Vz: int32(l.Direction.Z),
		R:  l.Color.R,
		G:  l.Color.G,
		B:  l.Color.B,
	}
	GsSetFlatLight(int32(n), &light)
}

func setAmbient(r, g, b geom.Fixed) { GsSetAmbient(int32(r), int32(g), int32(b)) }

func startCounter(c Counter)    { StartRCnt(0xf2000000 | uint32(c)) }
func resetCounter(c Counter)    { ResetRCnt(0xf2000000 | uint32(c)) }
func readCounter(c Counter) int { return int(GetRCnt(0xf2000000 | uint32(c))) }
func fieldCount() int           { return int(VSync(-1)) }
func linesSinceBlank() int      { return int(VSync(1)) }
func videoMode() VideoMode      { return VideoMode(GetVideoMode()) }

func testCard(slot int) CardState { return CardState(TestCard(int32(slot))) }

func fileOpen(name *byte, mode int32) int32        { return Open(name, mode) }
func fileRead(fd int32, buf *byte, n int32) int32  { return Read(fd, buf, n) }
func fileWrite(fd int32, buf *byte, n int32) int32 { return Write(fd, buf, n) }
func fileClose(fd int32)                           { Close(fd) }
func fileFormat(name *byte) int32                  { return Format(name) }
func fileDelete(name *byte) int32                  { return Delete(name) }

// The library reads a Cell's first word with one load, so cells are
// aligned as words are.
var _ [unsafe.Alignof(Cell{}) - 4]byte

// A CardFile is the kernel's DIRENTRY under another name, which Firstfile
// and Nextfile write whole, so the two stay the same size.
var (
	_ [unsafe.Sizeof(CardFile{}) - unsafe.Sizeof(DIRENTRY{})]byte
	_ [unsafe.Sizeof(DIRENTRY{}) - unsafe.Sizeof(CardFile{})]byte
)

func fileFirst(name *byte, f *CardFile) bool {
	return Firstfile(name, (*DIRENTRY)(unsafe.Pointer(f))) != nil
}

func fileNext(f *CardFile) bool {
	return Nextfile((*DIRENTRY)(unsafe.Pointer(f))) != nil
}

// The disc's audio is the sound processor's one serial source, mixed in
// rather than played in place of its own voices.
const ssCD, ssMix, ssOn = 0, 0, 1

func discVolume(left, right int) {
	SsSetSerialAttr(ssCD, ssMix, ssOn)
	SsSetSerialVol(ssCD, int16(left), int16(right))
}

// cdTracks is the list CdPlay is given, the track numbers ended by a 0,
// kept so that a program's own list need not be.
var cdTracks [16]int32

func cdPlay(mode int, tracks []int, from int) int {
	if mode == 1 || mode == 2 {
		n := min(len(tracks), len(cdTracks)-1)
		for i := range n {
			cdTracks[i] = int32(tracks[i])
		}
		cdTracks[n] = 0
	}
	return int(CdPlay(int32(mode), &cdTracks[0], int32(from)))
}

func kanjiOpen(x, y, w, h, texX, texY, clutX, clutY, chars int) int32 {
	return KanjiFntOpen(int32(x), int32(y), int32(w), int32(h), int32(texX), int32(texY), int32(clutX), int32(clutY), 0, int32(chars))
}

// kanjiLine holds a string for the library, which wants it ended by a 0.
var kanjiLine [256]byte

func kanjiPrint(id int32, s string) {
	n := copy(kanjiLine[:len(kanjiLine)-1], s)
	kanjiLine[n] = 0
	KanjiFntPrint(&kanjiLine[0])
}

func kanjiImage(s string, words []uint32, x, y, clutX, clutY int, fg, bg uint16) bool {
	n := copy(kanjiLine[:len(kanjiLine)-1], s)
	kanjiLine[n] = 0
	return Krom2Tim(&kanjiLine[0], &words[0], int32(x), int32(y), int32(clutX), int32(clutY), uint32(fg), uint32(bg)) == 0
}

func useCamera() { GsSetLsMatrix(GsWSMATRIX()) }

func project3(a, b, c *Vertex, sxy *[3]gpu.XY) bool {
	return RotTransPers3((*SVECTOR)(unsafe.Pointer(a)), (*SVECTOR)(unsafe.Pointer(b)), (*SVECTOR)(unsafe.Pointer(c)), (*[3]uint32)(unsafe.Pointer(sxy))) >= 0
}

func project(v *Vertex, sxy *gpu.XY) bool {
	return RotTransPers((*SVECTOR)(unsafe.Pointer(v)), (*uint32)(unsafe.Pointer(sxy))) >= 0
}

func clipArea() int      { return int(NormalClip()) }
func averageDepth3() int { return int(AverageZ3()) }
func averageDepth4() int { return int(AverageZ4()) }

// svector and lvector are a rotation and a vector as the library takes
// them, 16 and 32 bits a component.
func svector(r geom.Rotation) SVECTOR {
	return SVECTOR{
		Vx: int16(r.X),
		Vy: int16(r.Y),
		Vz: int16(r.Z),
	}
}

func lvector(v geom.Vector) VECTOR {
	return VECTOR{
		Vx: int32(v.X),
		Vy: int32(v.Y),
		Vz: int32(v.Z),
	}
}
