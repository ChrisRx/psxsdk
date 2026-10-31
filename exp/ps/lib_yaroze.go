//go:build yaroze

package ps

import (
	"unsafe"

	"go.chrisrx.dev/psx/exp/libps"
)

// The console side of the package: what the API calls on the library,
// exp/libps, in a program built for the console. lib_stub.go is the
// same for every other build, where each call panics.

// A screenImpl is the library's side of a Screen: the two ordering
// tables and the two frames' packet space.
type screenImpl struct {
	ot      [2]libps.GsOT
	overlay [2]libps.GsOT
	packets [2][]uint32
}

// A frameTable is the ordering table a frame sorts into.
type frameTable = *libps.GsOT

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
	_ [unsafe.Sizeof(Sprite{}) - unsafe.Sizeof(libps.GsSPRITE{})]byte
	_ [unsafe.Sizeof(libps.GsSPRITE{}) - unsafe.Sizeof(Sprite{})]byte
	_ [unsafe.Sizeof(Box{}) - unsafe.Sizeof(libps.GsBOXF{})]byte
	_ [unsafe.Sizeof(libps.GsBOXF{}) - unsafe.Sizeof(Box{})]byte
)

func openScreen(s *Screen) {
	libps.SetVideoMode(nativeVideoMode())
	dither := uint16(0)
	if s.Dither {
		dither = 1
	}
	w, h := uint16(s.Width), uint16(s.Height)
	if s.Height > 256 {
		libps.GsInitGraph(w, h, interlaced|offsetGPU, dither, 0)
		libps.GsDefDispBuff(0, 0, 0, 0)
	} else {
		libps.GsInitGraph(w, h, offsetGPU, dither, 0)
		if s.SideBySide {
			libps.GsDefDispBuff(0, 0, w, 0)
		} else {
			libps.GsDefDispBuff(0, 0, 0, h)
		}
	}
	env := libps.DispEnv()
	env.Screen.X += s.Offset.X
	env.Screen.Y += s.Offset.Y
	if s.Center {
		libps.GsInit3D()
	}

	tags := make([]libps.GsOT_TAG, 2<<s.Depth)
	packets := make([]uint32, 2*s.Packets/4)
	for i := range s.impl.ot {
		s.impl.ot[i].Length = uint32(s.Depth)
		s.impl.ot[i].Org = &tags[i<<s.Depth]
		s.impl.packets[i] = packets[i*len(packets)/2 : (i+1)*len(packets)/2]
	}
	if s.OverlayDepth != 0 {
		tags := make([]libps.GsOT_TAG, 2<<s.OverlayDepth)
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

func resetGraph() { libps.ResetGraph(3) }

// beginFrame starts a frame: it takes the buffer the frame goes into,
// points the library at its packet space, and clears its ordering
// table.
func beginFrame(s *Screen) *Frame {
	s.active = int(libps.GsGetActiveBuff())
	libps.GsSetWorkBase((*libps.PACKET)(unsafe.Pointer(&s.impl.packets[s.active][0])))
	libps.GsClearOt(0, 0, &s.impl.ot[s.active])
	s.cur.table = &s.impl.ot[s.active]
	if s.OverlayDepth != 0 {
		libps.GsClearOt(0, 0, &s.impl.overlay[s.active])
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
	libps.DrawSync(0)
	if s.Interval > 1 {
		libps.VSync(int32(s.Interval))
	} else {
		libps.VSync(0)
	}
	f := libps.VSync(-1)
	s.fields = int(f - s.lastField)
	s.lastField = f
	libps.GsSwapDispBuff()
	if s.backdrop != nil {
		// The backdrop goes into the buffer about to be drawn, which is
		// the other one from the frame's own index: below or beside the
		// first.
		r := libps.RECT{W: s.backdropW, H: s.backdropH}
		if s.SideBySide {
			r.X = int16(1-s.active) * int16(s.Width)
		} else {
			r.Y = int16(1-s.active) * int16(s.Height)
		}
		libps.LoadImage(&r, s.backdrop)
		libps.DrawSync(0)
	} else {
		libps.GsSortClear(s.Background.R, s.Background.G, s.Background.B, ot)
	}
	libps.GsDrawOt(ot)
	if s.OverlayDepth != 0 {
		libps.GsDrawOt(&s.impl.overlay[s.active])
	}
	if s.text >= 0 {
		libps.FntFlush(s.text)
	}
	for _, id := range s.kanji[:s.nkanji] {
		libps.KanjiFntFlush(id)
	}
	s.frame++
}

func vsync(n int32) int32 { return libps.VSync(n) }

func fontOpen(x, y, w, h, chars int) int32 {
	libps.FntLoad(960, 256)
	return libps.FntOpen(int32(x), int32(y), int32(w), int32(h), 0, int32(chars))
}

func fntWrite(id int32, s string)      { libps.FntWrite(id, s) }
func fntWriteBytes(id int32, b []byte) { libps.FntWriteBytes(id, b) }

func sortSprite(sp *Sprite, f *Frame, depth int) {
	libps.GsSortSprite((*libps.GsSPRITE)(unsafe.Pointer(sp)), f.table, uint16(depth))
}

func sortFastSprite(sp *Sprite, f *Frame, depth int) {
	libps.GsSortFastSprite((*libps.GsSPRITE)(unsafe.Pointer(sp)), f.table, uint16(depth))
}

func sortBox(b *Box, f *Frame, depth int) {
	libps.GsSortBoxFill((*libps.GsBOXF)(unsafe.Pointer(b)), f.table, uint16(depth))
}

// A Line, a GradientLine, a Cell, a tileMapImpl and a Background's first
// fields are the library's structs under other names.
var (
	_ [unsafe.Sizeof(Line{}) - unsafe.Sizeof(libps.GsLINE{})]byte
	_ [unsafe.Sizeof(libps.GsLINE{}) - unsafe.Sizeof(Line{})]byte
	_ [unsafe.Sizeof(GradientLine{}) - unsafe.Sizeof(libps.GsGLINE{})]byte
	_ [unsafe.Sizeof(libps.GsGLINE{}) - unsafe.Sizeof(GradientLine{})]byte
	_ [unsafe.Sizeof(Cell{}) - unsafe.Sizeof(libps.GsCELL{})]byte
	_ [unsafe.Sizeof(libps.GsCELL{}) - unsafe.Sizeof(Cell{})]byte
	_ [unsafe.Sizeof(tileMapImpl{}) - unsafe.Sizeof(libps.GsMAP{})]byte
	_ [unsafe.Sizeof(libps.GsMAP{}) - unsafe.Sizeof(tileMapImpl{})]byte
	_ [unsafe.Offsetof(Background{}.work) - unsafe.Sizeof(libps.GsBG{})]byte
	_ [unsafe.Sizeof(libps.GsBG{}) - unsafe.Offsetof(Background{}.work)]byte
)

func sortLine(l *Line, f *Frame, depth int) {
	libps.GsSortLine((*libps.GsLINE)(unsafe.Pointer(l)), f.table, uint16(depth))
}

func sortGradientLine(l *GradientLine, f *Frame, depth int) {
	libps.GsSortGLine((*libps.GsGLINE)(unsafe.Pointer(l)), f.table, uint16(depth))
}

func initBackground(bg *Background) {
	libps.GsInitFixBg16((*libps.GsBG)(unsafe.Pointer(bg)), &bg.work[0])
}

func sortBackground(bg *Background, f *Frame, depth int) {
	libps.GsSortFixBg16((*libps.GsBG)(unsafe.Pointer(bg)), &bg.work[0], f.table, uint16(depth))
}

// loadTIM reads the TIM's header, loads its pixels where the header says
// or at x, y, and its palette where the header says, waits for the
// transfer, and returns the image.
func loadTIM(words []uint32, at bool, x, y int) Image {
	var info libps.GsIMAGE
	libps.GsGetTimInfo(&words[1], &info)
	if at {
		info.Px, info.Py = int16(x), int16(y)
	}
	rect := libps.RECT{X: info.Px, Y: info.Py, W: int16(info.Pw), H: int16(info.Ph)}
	libps.LoadImage(&rect, info.Pixel)
	if info.Pmode&8 != 0 {
		rect = libps.RECT{X: info.Cx, Y: info.Cy, W: int16(info.Cw), H: int16(info.Ch)}
		libps.LoadImage(&rect, info.Clut)
	}
	libps.DrawSync(0)

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
			Page: libps.GetTPage(int32(depth), 0, int32(info.Px), int32(info.Py)),
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
		libps.GetPadBuf(&padBuf[0], &padBuf[1])
	}
	buf := unsafe.Slice(padBuf[port], 8)
	if buf[0] != 0 {
		return false, 0, axes, 0
	}
	copy(axes[:], buf[4:8])
	return true, Kind(buf[1] >> 4), axes, ^Buttons(uint16(buf[3]) | uint16(buf[2])<<8)
}

func mainVolume(left, right int) { libps.SsSetMVol(int16(left), int16(right)) }

func vabTransfer(vh, vb []uint32) int16 {
	return libps.SsVabTransfer((*uint8)(unsafe.Pointer(&vh[0])), (*uint8)(unsafe.Pointer(&vb[0])), -1, 1)
}

func vabClose(id int16)                     { libps.SsVabClose(id) }
func seqOpen(seq []uint32, vab int16) int16 { return libps.SsSeqOpen(&seq[0], vab) }
func seqPlay(id int16, times int16)         { libps.SsSeqPlay(id, playMode, times) }
func seqVolume(id int16, left, right int)   { libps.SsSeqSetVol(id, int16(left), int16(right)) }
func seqPause(id int16)                     { libps.SsSeqPause(id) }
func seqResume(id int16)                    { libps.SsSeqReplay(id) }
func seqStop(id int16)                      { libps.SsSeqStop(id) }
func seqPlaying(id int16) bool              { return libps.SsIsEos(id, 0) != 0 }
func seqClose(id int16)                     { libps.SsSeqClose(id) }

func randNext() int     { return int(libps.Rand()) }
func randSeed(s uint32) { libps.Srand(s) }

// origin is where the frame's origin is on the picture, from its top
// left corner.
func origin() (x, y int) {
	ox, oy := libps.Origin()
	return int(ox), int(oy)
}

// A Matrix, a coordImpl and an objectImpl are the library's MATRIX,
// GsCOORDINATE2 and GsDOBJ2 under other names.
var (
	_ [unsafe.Sizeof(Matrix{}) - unsafe.Sizeof(libps.MATRIX{})]byte
	_ [unsafe.Sizeof(libps.MATRIX{}) - unsafe.Sizeof(Matrix{})]byte
	_ [unsafe.Sizeof(coordImpl{}) - unsafe.Sizeof(libps.GsCOORDINATE2{})]byte
	_ [unsafe.Sizeof(libps.GsCOORDINATE2{}) - unsafe.Sizeof(coordImpl{})]byte
	_ [unsafe.Sizeof(objectImpl{}) - unsafe.Sizeof(libps.GsDOBJ2{})]byte
	_ [unsafe.Sizeof(libps.GsDOBJ2{}) - unsafe.Sizeof(objectImpl{})]byte
)

func mapModel(words []uint32) { libps.GsMapModelingData(&words[1]) }

func initCoord(c, super *coordImpl) {
	libps.GsInitCoordinate2((*libps.GsCOORDINATE2)(unsafe.Pointer(super)), (*libps.GsCOORDINATE2)(unsafe.Pointer(c)))
}

func linkObject(o *objectImpl, m Model, n int) {
	libps.GsLinkObject4(uint32(uintptr(unsafe.Pointer(&m[3]))), (*libps.GsDOBJ2)(unsafe.Pointer(o)), int32(n))
}

// objectMatrices makes the object's matrix from its position, rotation
// and scale, and works out its matrices to the world and the screen in
// one walk of its parents.
func objectMatrices(o *Object, world, screen *Matrix) {
	coord := (*libps.GsCOORDINATE2)(unsafe.Pointer(&o.coord))
	m := &coord.Coord
	r := svector(o.Rotation)
	libps.RotMatrix(&r, m)
	if o.Scale != (Vector{}) {
		s := lvector(o.Scale)
		libps.ScaleMatrix(m, &s)
	}
	m.T = [3]int32{int32(o.Position.X), int32(o.Position.Y), int32(o.Position.Z)}
	coord.Flg = 0
	libps.GsGetLws(coord, (*libps.MATRIX)(unsafe.Pointer(world)), (*libps.MATRIX)(unsafe.Pointer(screen)))
}

func setLightMatrix(m *Matrix)  { libps.GsSetLightMatrix((*libps.MATRIX)(unsafe.Pointer(m))) }
func setScreenMatrix(m *Matrix) { libps.GsSetLsMatrix((*libps.MATRIX)(unsafe.Pointer(m))) }

// sortObject sorts the model with the CPU's scratchpad as the
// subdivision's work space.
func sortObject(o *Object, f *Frame) {
	scratch := (*uint32)(unsafe.Pointer(uintptr(0x1f800000)))
	libps.GsSortObject4((*libps.GsDOBJ2)(unsafe.Pointer(&o.obj)), f.table, int32(14-f.s.Depth), scratch)
}

func setVideo(mode VideoMode) { libps.SetVideoMode(int32(mode)) }

func shiftOffset(dx, dy int) {
	env := libps.DispEnv()
	env.Screen.X += int16(dx)
	env.Screen.Y += int16(dy)
}

// backdropOf reads a 16-bit TIM's pixels and size, for the backdrop.
func backdropOf(tim []uint32) (pixels *uint32, w, h int16) {
	var info libps.GsIMAGE
	libps.GsGetTimInfo(&tim[1], &info)
	return info.Pixel, int16(info.Pw), int16(info.Ph)
}

func rect(r Rect) libps.RECT {
	return libps.RECT{
		X: int16(r.X),
		Y: int16(r.Y),
		W: int16(r.W),
		H: int16(r.H),
	}
}

func copyRect(src Rect, x, y int) {
	r := rect(src)
	libps.MoveImage(&r, int32(x), int32(y))
}

func clearRect(r Rect, c Color) {
	rr := rect(r)
	libps.ClearImage(&rr, c.R, c.G, c.B)
}

func loadRect(r Rect, words []uint32) {
	rr := rect(r)
	libps.LoadImage(&rr, &words[0])
}

func storeRect(r Rect, words []uint32) {
	rr := rect(r)
	libps.StoreImage(&rr, &words[0])
	libps.DrawSync(0)
}

func setReverb(kind Reverb, left, right int) {
	libps.SsUtSetReverbType(int16(kind))
	libps.SsUtReverbOn()
	libps.SsUtSetReverbDepth(int16(left), int16(right))
}

func reverbOff() { libps.SsUtReverbOff() }

// modelVectors returns the vectors whose address and count the model's
// object table holds at word i.
func modelVectors(m Model, i int) []Vertex {
	return unsafe.Slice((*Vertex)(unsafe.Pointer(uintptr(m[i]))), m[i+1])
}

var (
	_ [unsafe.Sizeof(Vertex{}) - unsafe.Sizeof(libps.SVECTOR{})]byte
	_ [unsafe.Sizeof(libps.SVECTOR{}) - unsafe.Sizeof(Vertex{})]byte
)

func blend(vertices, diffs *Vertex, n int, p Fixed) {
	libps.GteMIMefunc((*libps.SVECTOR)(unsafe.Pointer(vertices)), (*libps.SVECTOR)(unsafe.Pointer(diffs)), int32(n), int32(p))
}

func setFog(dqa int16, dqb int32, c Color) {
	fog := libps.GsFOGPARAM{Dqa: dqa, Dqb: dqb, Rfc: c.R, Gfc: c.G, Bfc: c.B}
	libps.GsSetFogParam(&fog)
}

func setLightMode(mode int32) { libps.GsSetLightMode(mode) }

// allocPacket takes n words of the frame's packet space, from where the
// library's own drawing has got to, and moves it on.
func allocPacket(n int) *uint32 {
	p := (*uint32)(unsafe.Pointer(libps.GsGetWorkBase()))
	libps.GsSetWorkBase((*libps.PACKET)(unsafe.Add(unsafe.Pointer(p), 4*n)))
	return p
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
	libps.GsSetProjection(int32(c.Distance))
	view := libps.GsRVIEW2{
		Vpx: int32(c.Eye.X),
		Vpy: int32(c.Eye.Y),
		Vpz: int32(c.Eye.Z),
		Vrx: int32(c.Target.X),
		Vry: int32(c.Target.Y),
		Vrz: int32(c.Target.Z),
		Rz:  int32(c.Roll),
	}
	libps.GsSetRefView2(&view)
}

func setLight(n int, l Light) {
	light := libps.GsF_LIGHT{
		Vx: int32(l.Direction.X),
		Vy: int32(l.Direction.Y),
		Vz: int32(l.Direction.Z),
		R:  l.Color.R,
		G:  l.Color.G,
		B:  l.Color.B,
	}
	libps.GsSetFlatLight(int32(n), &light)
}

func setAmbient(r, g, b Fixed) { libps.GsSetAmbient(int32(r), int32(g), int32(b)) }

func startCounter(c Counter)    { libps.StartRCnt(0xf2000000 | uint32(c)) }
func resetCounter(c Counter)    { libps.ResetRCnt(0xf2000000 | uint32(c)) }
func readCounter(c Counter) int { return int(libps.GetRCnt(0xf2000000 | uint32(c))) }
func fieldCount() int           { return int(libps.VSync(-1)) }
func linesSinceBlank() int      { return int(libps.VSync(1)) }
func videoMode() VideoMode      { return VideoMode(libps.GetVideoMode()) }

func testCard(slot int) CardState { return CardState(libps.TestCard(int32(slot))) }

func fileOpen(name *byte, mode int32) int32        { return libps.FileOpen(name, mode) }
func fileRead(fd int32, buf *byte, n int32) int32  { return libps.FileRead(fd, buf, n) }
func fileWrite(fd int32, buf *byte, n int32) int32 { return libps.FileWrite(fd, buf, n) }
func fileClose(fd int32)                           { libps.FileClose(fd) }
func fileFormat(name *byte) int32                  { return libps.FileFormat(name) }
func fileDelete(name *byte) int32                  { return libps.FileDelete(name) }

func fileFirst(name *byte, f *CardFile) bool {
	return libps.FileFirst(name, (*libps.DIRENTRY)(unsafe.Pointer(f))) != nil
}

func fileNext(f *CardFile) bool {
	return libps.FileNext((*libps.DIRENTRY)(unsafe.Pointer(f))) != nil
}

// The disc's audio is the sound processor's one serial source, mixed in
// rather than played in place of its own voices.
const ssCD, ssMix, ssOn = 0, 0, 1

func discVolume(left, right int) {
	libps.SsSetSerialAttr(ssCD, ssMix, ssOn)
	libps.SsSetSerialVol(ssCD, int16(left), int16(right))
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
	return int(libps.CdPlay(int32(mode), &cdTracks[0], int32(from)))
}

func kanjiOpen(x, y, w, h, texX, texY, clutX, clutY, chars int) int32 {
	return libps.KanjiFntOpen(int32(x), int32(y), int32(w), int32(h), int32(texX), int32(texY), int32(clutX), int32(clutY), 0, int32(chars))
}

// kanjiLine holds a string for the library, which wants it ended by a 0.
var kanjiLine [256]byte

func kanjiPrint(id int32, s string) {
	n := copy(kanjiLine[:len(kanjiLine)-1], s)
	kanjiLine[n] = 0
	libps.KanjiFntPrint(&kanjiLine[0])
}

func kanjiImage(s string, words []uint32, x, y, clutX, clutY int, fg, bg uint16) bool {
	n := copy(kanjiLine[:len(kanjiLine)-1], s)
	kanjiLine[n] = 0
	return libps.Krom2Tim(&kanjiLine[0], &words[0], int32(x), int32(y), int32(clutX), int32(clutY), uint32(fg), uint32(bg)) == 0
}

func useCamera() { libps.GsSetLsMatrix(libps.WSMatrix()) }

func project3(a, b, c *Vertex, sxy *[3]XY) bool {
	return libps.RotTransPers3((*libps.SVECTOR)(unsafe.Pointer(a)), (*libps.SVECTOR)(unsafe.Pointer(b)), (*libps.SVECTOR)(unsafe.Pointer(c)), (*[3]uint32)(unsafe.Pointer(sxy))) >= 0
}

func project(v *Vertex, sxy *XY) bool {
	return libps.RotTransPers((*libps.SVECTOR)(unsafe.Pointer(v)), (*uint32)(unsafe.Pointer(sxy))) >= 0
}

func clipArea() int      { return int(libps.NormalClip()) }
func averageDepth3() int { return int(libps.AverageZ3()) }
func averageDepth4() int { return int(libps.AverageZ4()) }

func rotateMatrix(m *Matrix, r Rotation) {
	v := svector(r)
	libps.RotMatrix(&v, (*libps.MATRIX)(unsafe.Pointer(m)))
}

func applyMatrix(m *Matrix, v Vector) Vector {
	in := lvector(v)
	var out libps.VECTOR
	libps.ApplyMatrixLV((*libps.MATRIX)(unsafe.Pointer(m)), &in, &out)
	return Vector{
		X: int(out.Vx),
		Y: int(out.Vy),
		Z: int(out.Vz),
	}
}

// svector and lvector are a rotation and a vector as the library takes
// them, 16 and 32 bits a component.
func svector(r Rotation) libps.SVECTOR {
	return libps.SVECTOR{
		Vx: int16(r.X),
		Vy: int16(r.Y),
		Vz: int16(r.Z),
	}
}

func lvector(v Vector) libps.VECTOR {
	return libps.VECTOR{
		Vx: int32(v.X),
		Vy: int32(v.Y),
		Vz: int32(v.Z),
	}
}
