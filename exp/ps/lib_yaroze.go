//go:build yaroze

package ps

import (
	"unsafe"

	"go.chrisrx.dev/psx/exp/cleanlib"
)

// The console side of the package: what the API calls on the library,
// exp/cleanlib, in a program built for the console. lib_stub.go is the
// same for every other build, where each call panics.

// A screenImpl is the library's side of a Screen: the two ordering
// tables and the two frames' packet space.
type screenImpl struct {
	ot      [2]cleanlib.GsOT
	overlay [2]cleanlib.GsOT
	packets [2][]uint32
}

// A frameTable is the ordering table a frame sorts into.
type frameTable = *cleanlib.GsOT

// The library's display modes: GsInitGraph's interlace and origin bits,
// and SetVideoMode's modes.
const (
	interlaced uint16 = 1
	offsetGPU  uint16 = 4
	modeNTSC   int32  = 0
	modePAL    int32  = 1
)

// The sequencer's play mode and its count for playing without end.
const (
	playMode    uint8 = 1
	playForever int16 = 0
)

// A Sprite and a Box are the library's structs under other names.
var (
	_ [unsafe.Sizeof(Sprite{}) - unsafe.Sizeof(cleanlib.GsSPRITE{})]byte
	_ [unsafe.Sizeof(cleanlib.GsSPRITE{}) - unsafe.Sizeof(Sprite{})]byte
	_ [unsafe.Sizeof(Box{}) - unsafe.Sizeof(cleanlib.GsBOXF{})]byte
	_ [unsafe.Sizeof(cleanlib.GsBOXF{}) - unsafe.Sizeof(Box{})]byte
)

func openScreen(s *Screen) {
	cleanlib.SetVideoMode(nativeVideoMode())
	dither := uint16(0)
	if s.Dither {
		dither = 1
	}
	w, h := uint16(s.Width), uint16(s.Height)
	if s.Height > 256 {
		cleanlib.GsInitGraph(w, h, interlaced|offsetGPU, dither, 0)
		cleanlib.GsDefDispBuff(0, 0, 0, 0)
	} else {
		cleanlib.GsInitGraph(w, h, offsetGPU, dither, 0)
		if s.SideBySide {
			cleanlib.GsDefDispBuff(0, 0, w, 0)
		} else {
			cleanlib.GsDefDispBuff(0, 0, 0, h)
		}
	}
	env := cleanlib.DispEnv()
	env.Screen.X += s.Offset.X
	env.Screen.Y += s.Offset.Y
	if s.Center {
		cleanlib.GsInit3D()
	}

	tags := make([]uint32, 2<<s.Depth)
	packets := make([]uint32, 2*s.Packets/4)
	for i := range s.impl.ot {
		s.impl.ot[i].Length = uint32(s.Depth)
		s.impl.ot[i].Org = &tags[i<<s.Depth]
		s.impl.packets[i] = packets[i*len(packets)/2 : (i+1)*len(packets)/2]
	}
	if s.OverlayDepth != 0 {
		tags := make([]uint32, 2<<s.OverlayDepth)
		for i := range s.impl.overlay {
			s.impl.overlay[i].Length = uint32(s.OverlayDepth)
			s.impl.overlay[i].Org = &tags[i<<s.OverlayDepth]
		}
	}
}

// nativeVideoMode is the console's video mode by its BIOS's region:
// PAL for a European console, NTSC otherwise.
func nativeVideoMode() int32 {
	if *(*byte)(unsafe.Pointer(uintptr(0xbfc7ff52))) == 'E' {
		return modePAL
	}
	return modeNTSC
}

func resetGraph() { cleanlib.ResetGraph(3) }

// beginFrame starts a frame: it takes the buffer the frame goes into,
// points the library at its packet space, and clears its ordering
// table.
func beginFrame(s *Screen) *Frame {
	s.active = int(cleanlib.GsGetActiveBuff())
	cleanlib.GsSetWorkBase(unsafe.Pointer(&s.impl.packets[s.active][0]))
	cleanlib.GsClearOt(0, 0, &s.impl.ot[s.active])
	s.cur.table = &s.impl.ot[s.active]
	if s.OverlayDepth != 0 {
		cleanlib.GsClearOt(0, 0, &s.impl.overlay[s.active])
		s.over.table = &s.impl.overlay[s.active]
	}
	return &s.cur
}

// endFrame finishes the frame: it waits for the GPU to finish the last
// one and for the display's vertical blank, swaps the buffers, puts the
// background behind everything, starts the GPU on the frame, and draws
// the frame's text after it.
func endFrame(s *Screen) {
	ot := &s.impl.ot[s.active]
	cleanlib.DrawSync(0)
	if s.Interval > 1 {
		cleanlib.VSync(int32(s.Interval))
	} else {
		cleanlib.VSync(0)
	}
	f := cleanlib.VSync(-1)
	s.fields = int(f - s.lastField)
	s.lastField = f
	cleanlib.GsSwapDispBuff()
	if s.backdrop != nil {
		// The backdrop goes into the buffer about to be drawn, which is
		// the other one from the frame's own index: below or beside the
		// first.
		r := cleanlib.RECT{
			W: s.backdropW,
			H: s.backdropH,
		}
		if s.SideBySide {
			r.X = int16(1-s.active) * int16(s.Width)
		} else {
			r.Y = int16(1-s.active) * int16(s.Height)
		}
		cleanlib.LoadImage(&r, s.backdrop)
		cleanlib.DrawSync(0)
	} else {
		cleanlib.GsSortClear(s.Background.R, s.Background.G, s.Background.B, ot)
	}
	cleanlib.GsDrawOt(ot)
	if s.OverlayDepth != 0 {
		cleanlib.GsDrawOt(&s.impl.overlay[s.active])
	}
	if s.text >= 0 {
		cleanlib.FntFlush(s.text)
	}
	for _, id := range s.kanji[:s.nkanji] {
		cleanlib.KanjiFntFlush(id)
	}
	s.frame++
}

func vsync(n int32) int32 { return cleanlib.VSync(n) }

func fontOpen(x, y, w, h, chars int) int32 {
	cleanlib.FntLoad(960, 256)
	return cleanlib.FntOpen(int32(x), int32(y), int32(w), int32(h), 0, int32(chars))
}

func fntWrite(id int32, s string)      { cleanlib.FntWrite(id, s) }
func fntWriteBytes(id int32, b []byte) { cleanlib.FntWriteBytes(id, b) }

func sortSprite(sp *Sprite, f *Frame, depth int) {
	cleanlib.GsSortSprite((*cleanlib.GsSPRITE)(unsafe.Pointer(sp)), f.table, uint16(depth))
}

func sortFastSprite(sp *Sprite, f *Frame, depth int) {
	cleanlib.GsSortFastSprite((*cleanlib.GsSPRITE)(unsafe.Pointer(sp)), f.table, uint16(depth))
}

func sortBox(b *Box, f *Frame, depth int) {
	cleanlib.GsSortBoxFill((*cleanlib.GsBOXF)(unsafe.Pointer(b)), f.table, uint16(depth))
}

// A Line, a GradientLine, a Cell, a tileMapImpl and a Background's first
// fields are the library's structs under other names.
var (
	_ [unsafe.Sizeof(Line{}) - unsafe.Sizeof(cleanlib.GsLINE{})]byte
	_ [unsafe.Sizeof(cleanlib.GsLINE{}) - unsafe.Sizeof(Line{})]byte
	_ [unsafe.Sizeof(GradientLine{}) - unsafe.Sizeof(cleanlib.GsGLINE{})]byte
	_ [unsafe.Sizeof(cleanlib.GsGLINE{}) - unsafe.Sizeof(GradientLine{})]byte
	_ [unsafe.Sizeof(Cell{}) - unsafe.Sizeof(cleanlib.GsCELL{})]byte
	_ [unsafe.Sizeof(cleanlib.GsCELL{}) - unsafe.Sizeof(Cell{})]byte
	_ [unsafe.Sizeof(tileMapImpl{}) - unsafe.Sizeof(cleanlib.GsMAP{})]byte
	_ [unsafe.Sizeof(cleanlib.GsMAP{}) - unsafe.Sizeof(tileMapImpl{})]byte
	_ [unsafe.Offsetof(Background{}.work) - unsafe.Sizeof(cleanlib.GsBG{})]byte
	_ [unsafe.Sizeof(cleanlib.GsBG{}) - unsafe.Offsetof(Background{}.work)]byte
)

func sortLine(l *Line, f *Frame, depth int) {
	cleanlib.GsSortLine((*cleanlib.GsLINE)(unsafe.Pointer(l)), f.table, uint16(depth))
}

func sortGradientLine(l *GradientLine, f *Frame, depth int) {
	cleanlib.GsSortGLine((*cleanlib.GsGLINE)(unsafe.Pointer(l)), f.table, uint16(depth))
}

func initBackground(bg *Background) {
	cleanlib.GsInitFixBg16((*cleanlib.GsBG)(unsafe.Pointer(bg)), &bg.work[0])
}

func sortBackground(bg *Background, f *Frame, depth int) {
	cleanlib.GsSortFixBg16((*cleanlib.GsBG)(unsafe.Pointer(bg)), &bg.work[0], f.table, uint16(depth))
}

// loadTIM reads the TIM's header, loads its pixels where the header says
// or at x, y, and its palette where the header says, waits for the
// transfer, and returns the image.
func loadTIM(words []uint32, at bool, x, y int) Image {
	var info cleanlib.GsIMAGE
	cleanlib.GsGetTimInfo(&words[1], &info)
	if at {
		info.Px, info.Py = int16(x), int16(y)
	}
	rect := cleanlib.RECT{
		X: info.Px,
		Y: info.Py,
		W: int16(info.Pw),
		H: int16(info.Ph),
	}
	cleanlib.LoadImage(&rect, info.Pixel)
	if info.Pmode&8 != 0 {
		rect = cleanlib.RECT{
			X: info.Cx,
			Y: info.Cy,
			W: int16(info.Cw),
			H: int16(info.Ch),
		}
		cleanlib.LoadImage(&rect, info.Clut)
	}
	cleanlib.DrawSync(0)

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
			Page: cleanlib.GetTPage(int32(depth), 0, int32(info.Px), int32(info.Py)),
			U:    uint8(uint16(info.Px&0x3f) * per),
			V:    uint8(info.Py & 0xff),
			CX:   info.Cx,
			CY:   info.Cy,
		},
	}
}

// The library reads both controllers every vertical blank into buffers
// of its own: a byte that is 0 while a controller is connected, a byte
// of its type and length, and then what it sent, two bytes of buttons
// for the standard controller, each bit clear while its button is held.
var padBuf [2]*uint8

func padButtons(port int) Buttons {
	_, _, _, held := padReading(port)
	return held
}

// padReading decodes a port's buffer: whether a controller is there,
// what it is, the four bytes after its buttons, and the buttons held.
func padReading(port int) (connected bool, kind Kind, axes [4]uint8, held Buttons) {
	if padBuf[0] == nil {
		cleanlib.GetPadBuf(&padBuf[0], &padBuf[1])
	}
	buf := unsafe.Slice(padBuf[port], 8)
	if buf[0] != 0 {
		return false, 0, axes, 0
	}
	copy(axes[:], buf[4:8])
	return true, Kind(buf[1] >> 4), axes, ^Buttons(uint16(buf[3]) | uint16(buf[2])<<8)
}

func mainVolume(left, right int) { cleanlib.SsSetMVol(int16(left), int16(right)) }

func vabTransfer(vh, vb []uint32) int16 {
	return int16(cleanlib.SsVabTransfer((*uint8)(unsafe.Pointer(&vh[0])), (*uint8)(unsafe.Pointer(&vb[0])), -1, 1))
}

func vabClose(id int16)                     { cleanlib.SsVabClose(id) }
func seqOpen(seq []uint32, vab int16) int16 { return int16(cleanlib.SsSeqOpen(&seq[0], vab)) }
func seqPlay(id int16, times int16)         { cleanlib.SsSeqPlay(id, playMode, times) }
func seqVolume(id int16, left, right int)   { cleanlib.SsSeqSetVol(id, int16(left), int16(right)) }
func seqPause(id int16)                     { cleanlib.SsSeqPause(id) }
func seqResume(id int16)                    { cleanlib.SsSeqReplay(id) }
func seqStop(id int16)                      { cleanlib.SsSeqStop(id) }
func seqPlaying(id int16) bool              { return cleanlib.SsIsEos(id, 0) != 0 }
func seqClose(id int16)                     { cleanlib.SsSeqClose(id) }

func randNext() int     { return int(cleanlib.Rand()) }
func randSeed(s uint32) { cleanlib.Srand(s) }

// origin is where the frame's origin is on the picture, from its top
// left corner.
func origin() (x, y int) {
	ox, oy := cleanlib.Origin()
	return int(ox), int(oy)
}

// A Matrix, a coordImpl and an objectImpl are the library's MATRIX,
// GsCOORDINATE2 and GsDOBJ2 under other names.
var (
	_ [unsafe.Sizeof(Matrix{}) - unsafe.Sizeof(cleanlib.MATRIX{})]byte
	_ [unsafe.Sizeof(cleanlib.MATRIX{}) - unsafe.Sizeof(Matrix{})]byte
	_ [unsafe.Sizeof(coordImpl{}) - unsafe.Sizeof(cleanlib.GsCOORDINATE2{})]byte
	_ [unsafe.Sizeof(cleanlib.GsCOORDINATE2{}) - unsafe.Sizeof(coordImpl{})]byte
	_ [unsafe.Sizeof(objectImpl{}) - unsafe.Sizeof(cleanlib.GsDOBJ2{})]byte
	_ [unsafe.Sizeof(cleanlib.GsDOBJ2{}) - unsafe.Sizeof(objectImpl{})]byte
)

func mapModel(words []uint32) { cleanlib.GsMapModelingData(&words[1]) }

func initCoord(c, super *coordImpl) {
	cleanlib.GsInitCoordinate2((*cleanlib.GsCOORDINATE2)(unsafe.Pointer(super)), (*cleanlib.GsCOORDINATE2)(unsafe.Pointer(c)))
}

func linkObject(o *objectImpl, m Model, n int) {
	cleanlib.GsLinkObject4(uint32(uintptr(unsafe.Pointer(&m[3]))), (*cleanlib.GsDOBJ2)(unsafe.Pointer(o)), int32(n))
}

// objectMatrices makes the object's matrix from its position, rotation
// and scale, and works out its matrices to the world and the screen in
// one walk of its parents.
func objectMatrices(o *Object, world, screen *Matrix) {
	coord := (*cleanlib.GsCOORDINATE2)(unsafe.Pointer(&o.coord))
	m := &coord.Coord
	r := svector(o.Rotation)
	cleanlib.RotMatrix(&r, m)
	if o.Scale != (Vector{}) {
		s := lvector(o.Scale)
		cleanlib.ScaleMatrix(m, &s)
	}
	m.T = [3]int32{int32(o.Position.X), int32(o.Position.Y), int32(o.Position.Z)}
	coord.Flg = 0
	cleanlib.GsGetLws(coord, (*cleanlib.MATRIX)(unsafe.Pointer(world)), (*cleanlib.MATRIX)(unsafe.Pointer(screen)))
}

func setLightMatrix(m *Matrix)  { cleanlib.GsSetLightMatrix((*cleanlib.MATRIX)(unsafe.Pointer(m))) }
func setScreenMatrix(m *Matrix) { cleanlib.GsSetLsMatrix((*cleanlib.MATRIX)(unsafe.Pointer(m))) }

// sortObject sorts the model with the CPU's scratchpad as the
// subdivision's work space.
func sortObject(o *Object, f *Frame) {
	scratch := (*uint32)(unsafe.Pointer(uintptr(0x1f800000)))
	cleanlib.GsSortObject4((*cleanlib.GsDOBJ2)(unsafe.Pointer(&o.obj)), f.table, int32(14-f.s.Depth), scratch)
}

func setVideo(mode VideoMode) { cleanlib.SetVideoMode(int32(mode)) }

func shiftOffset(dx, dy int) {
	env := cleanlib.DispEnv()
	env.Screen.X += int16(dx)
	env.Screen.Y += int16(dy)
}

// backdropOf reads a 16-bit TIM's pixels and size, for the backdrop.
func backdropOf(tim []uint32) (pixels *uint32, w, h int16) {
	var info cleanlib.GsIMAGE
	cleanlib.GsGetTimInfo(&tim[1], &info)
	return info.Pixel, int16(info.Pw), int16(info.Ph)
}

func rect(r Rect) cleanlib.RECT {
	return cleanlib.RECT{
		X: int16(r.X),
		Y: int16(r.Y),
		W: int16(r.W),
		H: int16(r.H),
	}
}

func copyRect(src Rect, x, y int) {
	r := rect(src)
	cleanlib.MoveImage(&r, int32(x), int32(y))
}

func clearRect(r Rect, c Color) {
	rr := rect(r)
	cleanlib.ClearImage(&rr, c.R, c.G, c.B)
}

func loadRect(r Rect, words []uint32) {
	rr := rect(r)
	cleanlib.LoadImage(&rr, &words[0])
}

func storeRect(r Rect, words []uint32) {
	rr := rect(r)
	cleanlib.StoreImage(&rr, &words[0])
	cleanlib.DrawSync(0)
}

func setReverb(kind Reverb, left, right int) {
	cleanlib.SsUtSetReverbType(int16(kind))
	cleanlib.SsUtReverbOn()
	cleanlib.SsUtSetReverbDepth(int16(left), int16(right))
}

func reverbOff() { cleanlib.SsUtReverbOff() }

// modelVectors returns the vectors whose address and count the model's
// object table holds at word i.
func modelVectors(m Model, i int) []Vertex {
	return unsafe.Slice((*Vertex)(unsafe.Pointer(uintptr(m[i]))), m[i+1])
}

var (
	_ [unsafe.Sizeof(Vertex{}) - unsafe.Sizeof(cleanlib.SVECTOR{})]byte
	_ [unsafe.Sizeof(cleanlib.SVECTOR{}) - unsafe.Sizeof(Vertex{})]byte
)

func blend(vertices, diffs *Vertex, n int, p Fixed) {
	cleanlib.GteMIMefunc((*cleanlib.SVECTOR)(unsafe.Pointer(vertices)), (*cleanlib.SVECTOR)(unsafe.Pointer(diffs)), int32(n), int32(p))
}

func setFog(dqa int16, dqb int32, c Color) {
	fog := cleanlib.GsFOGPARAM{
		Dqa: dqa,
		Dqb: dqb,
		Rfc: c.R,
		Gfc: c.G,
		Bfc: c.B,
	}
	cleanlib.GsSetFogParam(&fog)
}

func setLightMode(mode int32) { cleanlib.GsSetLightMode(mode) }

// allocPacket takes n words of the frame's packet space, from where the
// library's own drawing has got to, and moves it on.
func allocPacket(n int) *uint32 {
	p := cleanlib.GsGetWorkBase()
	cleanlib.GsSetWorkBase(unsafe.Add(p, 4*n))
	return (*uint32)(p)
}

// linkPacket puts the packet whose tag is at tag at the head of the
// frame's entry for depth, keeping the tag's length and the entry's.
func linkPacket(f *Frame, depth int, tag *Tag) {
	entry := (*uint32)(unsafe.Add(unsafe.Pointer(f.table.Org), 4*depth))
	linkAfter((*Tag)(unsafe.Pointer(entry)), tag)
}

// linkAfter puts the packet whose tag is at tag after the tag prev, an
// entry's or another packet's.
func linkAfter(prev, tag *Tag) {
	*tag = *tag&0xff000000 | *prev&0xffffff
	*prev = *prev&0xff000000 | Tag(uintptr(unsafe.Pointer(tag)))&0xffffff
}

func setCamera(c *Camera) {
	projection = c.Distance
	cleanlib.GsSetProjection(int32(c.Distance))
	view := cleanlib.GsRVIEW2{
		Vpx: int32(c.Eye.X),
		Vpy: int32(c.Eye.Y),
		Vpz: int32(c.Eye.Z),
		Vrx: int32(c.Target.X),
		Vry: int32(c.Target.Y),
		Vrz: int32(c.Target.Z),
		Rz:  int32(c.Roll),
	}
	cleanlib.GsSetRefView2(&view)
}

func setLight(n int, l Light) {
	light := cleanlib.GsF_LIGHT{
		Vx: int32(l.Direction.X),
		Vy: int32(l.Direction.Y),
		Vz: int32(l.Direction.Z),
		R:  l.Color.R,
		G:  l.Color.G,
		B:  l.Color.B,
	}
	cleanlib.GsSetFlatLight(int32(n), &light)
}

func setAmbient(r, g, b Fixed) { cleanlib.GsSetAmbient(int32(r), int32(g), int32(b)) }

func startCounter(c Counter)    { cleanlib.StartRCnt(0xf2000000 | uint32(c)) }
func resetCounter(c Counter)    { cleanlib.ResetRCnt(0xf2000000 | uint32(c)) }
func readCounter(c Counter) int { return int(cleanlib.GetRCnt(0xf2000000 | uint32(c))) }
func fieldCount() int           { return int(cleanlib.VSync(-1)) }
func linesSinceBlank() int      { return int(cleanlib.VSync(1)) }
func videoMode() VideoMode      { return VideoMode(cleanlib.GetVideoMode()) }

func testCard(slot int) CardState { return CardState(cleanlib.TestCard(int32(slot))) }

func fileOpen(name *byte, mode int32) int32        { return cleanlib.FileOpen(name, mode) }
func fileRead(fd int32, buf *byte, n int32) int32  { return cleanlib.FileRead(fd, buf, n) }
func fileWrite(fd int32, buf *byte, n int32) int32 { return cleanlib.FileWrite(fd, buf, n) }
func fileClose(fd int32)                           { cleanlib.FileClose(fd) }
func fileFormat(name *byte) int32                  { return cleanlib.FileFormat(name) }
func fileDelete(name *byte) int32                  { return cleanlib.FileDelete(name) }

func fileFirst(name *byte, f *CardFile) bool {
	return cleanlib.FileFirst(name, unsafe.Pointer(f)) != nil
}

func fileNext(f *CardFile) bool {
	return cleanlib.FileNext(unsafe.Pointer(f)) != nil
}

// The disc's audio is the sound processor's one serial source, mixed in
// rather than played in place of its own voices.
const ssCD, ssMix, ssOn = 0, 0, 1

func discVolume(left, right int) {
	cleanlib.SsSetSerialAttr(ssCD, ssMix, ssOn)
	cleanlib.SsSetSerialVol(ssCD, int16(left), int16(right))
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
	return int(cleanlib.CdPlay(int32(mode), &cdTracks[0], int32(from)))
}

func kanjiOpen(x, y, w, h, texX, texY, clutX, clutY, chars int) int32 {
	return cleanlib.KanjiFntOpen(int32(x), int32(y), int32(w), int32(h), int32(texX), int32(texY), int32(clutX), int32(clutY), 0, int32(chars))
}

// kanjiLine holds a string for the library, which wants it ended by a 0.
var kanjiLine [256]byte

func kanjiPrint(id int32, s string) {
	n := copy(kanjiLine[:len(kanjiLine)-1], s)
	kanjiLine[n] = 0
	cleanlib.KanjiFntPrint(&kanjiLine[0])
}

func kanjiImage(s string, words []uint32, x, y, clutX, clutY int, fg, bg uint16) bool {
	n := copy(kanjiLine[:len(kanjiLine)-1], s)
	kanjiLine[n] = 0
	return cleanlib.Krom2Tim(&kanjiLine[0], &words[0], int32(x), int32(y), int32(clutX), int32(clutY), uint32(fg), uint32(bg)) == 0
}

func useCamera() { cleanlib.GsSetLsMatrix(cleanlib.WSMatrix()) }

func project3(a, b, c *Vertex, sxy *[3]XY) bool {
	return cleanlib.RotTransPers3((*cleanlib.SVECTOR)(unsafe.Pointer(a)), (*cleanlib.SVECTOR)(unsafe.Pointer(b)), (*cleanlib.SVECTOR)(unsafe.Pointer(c)), (*[3]uint32)(unsafe.Pointer(sxy))) >= 0
}

func project(v *Vertex, sxy *XY) bool {
	return cleanlib.RotTransPers((*cleanlib.SVECTOR)(unsafe.Pointer(v)), (*uint32)(unsafe.Pointer(sxy))) >= 0
}

func clipArea() int      { return int(cleanlib.NormalClip()) }
func averageDepth3() int { return int(cleanlib.AverageZ3()) }
func averageDepth4() int { return int(cleanlib.AverageZ4()) }

func rotateMatrix(m *Matrix, r Rotation) {
	v := svector(r)
	cleanlib.RotMatrix(&v, (*cleanlib.MATRIX)(unsafe.Pointer(m)))
}

func applyMatrix(m *Matrix, v Vector) Vector {
	in := lvector(v)
	var out cleanlib.VECTOR
	cleanlib.ApplyMatrixLV((*cleanlib.MATRIX)(unsafe.Pointer(m)), &in, &out)
	return Vector{
		X: int(out.Vx),
		Y: int(out.Vy),
		Z: int(out.Vz),
	}
}

// svector and lvector are a rotation and a vector as the library takes
// them, 16 and 32 bits a component.
func svector(r Rotation) cleanlib.SVECTOR {
	return cleanlib.SVECTOR{
		Vx: int16(r.X),
		Vy: int16(r.Y),
		Vz: int16(r.Z),
	}
}

func lvector(v Vector) cleanlib.VECTOR {
	return cleanlib.VECTOR{
		Vx: int32(v.X),
		Vy: int32(v.Y),
		Vz: int32(v.Z),
	}
}
