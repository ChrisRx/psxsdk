//go:build yaroze

package cleanlib

import (
	"unsafe"

	"go.chrisrx.dev/psx/internal/volatile"
)

// The Gs graphics system: the display and its two buffers, ordering
// tables and the packet area the sorts build in.

// MATRIX is a rotation in 4.12 fixed point and a translation.
type MATRIX struct {
	M [3][3]int16
	_ [2]byte
	T [3]int32
}

// GsOT is an ordering table: 1<<Length entries at Org, drawn from Tag.
type GsOT struct {
	Length uint32
	Org    *uint32
	Offset uint32
	Point  uint32
	Tag    *uint32
}

// GsPOSITION is the screen origin, added to the drawing offset in the
// GsOFSGPU mode.
type GsPOSITION struct {
	Offx, Offy int16
}

// The library's variables programs reach by name, whose storage is in the
// target's playstation-yaroze-libps-data.c. Each is declared on its own:
// TinyGo ignores go:extern on a variable in a var group, and makes it a Go
// variable programs never see.

//go:extern GsDISPENV
var gsDISPENV DISPENV

//go:extern GsDRAWENV
var gsDRAWENV DRAWENV

//go:extern GsIDMATRIX
var gsIDMATRIX MATRIX

//go:extern GsIDMATRIX2
var gsIDMATRIX2 MATRIX

//go:extern GsOUT_PACKET_P
var gsOUT_PACKET_P unsafe.Pointer

//go:extern POSITION
var position GsPOSITION

//go:extern PSDIDX
var psdIDX int32

//go:extern PSDCNT
var psdCNT uint32

//go:extern PSDOFSX
var psdOFSX [2]int16

//go:extern PSDOFSY
var psdOFSY [2]int16

// The display buffers are the library's variables, which a program may read
// and write and the next call then goes by (probe spec/probes/gspsd): PSDIDX
// is the buffer drawn into, and PSDOFSX and PSDOFSY hold each buffer's x and y.

// activeBuf returns the buffer drawn into.
func activeBuf() int { return int(psdIDX & 1) }

// bufCorner returns the top left of buffer i.
func bufCorner(i int) (x, y int16) { return psdOFSX[i&1], psdOFSY[i&1] }

// GsInitGraph's intmode bits.
const (
	gsInterlace = 1 << 0
	gsOfsGPU    = 1 << 2
)

// display is the state of the two display buffers.
type display struct {
	width, height int16

	// ofsGPU is whether the GPU's drawing offset carries POSITION
	// (GsOFSGPU) or the packets do (GsOFSGTE).
	ofsGPU bool

	// clip is the clipping area GsSetClip2D set, relative to a buffer's
	// corner, which each buffer drawn into takes.
	clip RECT
}

var gs display

// GsInitGraph resets the GPU and sets up a display w by h, interlaced if
// intmode has bit 0 or h is over 256 lines, with the drawing offset in the GPU (GsOFSGPU, bit 2)
// or the packets (GsOFSGTE). dith turns dithering on, and vrammode 1 is
// 24-bit color. The display stays off until the first GsSwapDispBuff. The GTE
// is usable from here, with a projection distance of 1000, whether or not
// GsInit3D follows: a turned sprite's corners come from it.
//
//export GsInitGraph
func GsInitGraph(w, h, intmode, dith, vrammode uint16) {
	resetGPU()
	gteEnable()
	setH(1000)
	// The environment starts over but for its clipping area, which goes
	// out as it was (probe spec/probes/gsclip) until draw sets it below.
	gsDRAWENV = DRAWENV{Clip: gsDRAWENV.Clip, Dtd: uint8(dith)}
	gsDRAWENV.put()
	gsDISPENV = DISPENV{
		Disp:    RECT{W: int16(w), H: int16(h)},
		Isinter: interlace(h, intmode),
		Isrgb24: uint8(vrammode),
	}
	if video.mode == modePAL {
		gsDISPENV.Screen.Y = 24
	}
	gsDISPENV.put()
	gs = display{
		width:  int16(w),
		height: int16(h),
		ofsGPU: intmode&gsOfsGPU != 0,
		clip:   RECT{W: int16(w), H: int16(h)},
	}
	position = GsPOSITION{}
	psdCNT = 1
	psdOFSX, psdOFSY = [2]int16{}, [2]int16{}
	gsIDMATRIX = identity()
	gsIDMATRIX2 = identity()
	// GsIDMATRIX2 scales y by the screen's aspect against 4:3, a half
	// for 640 by 240, so that GsSetRefView2's views look square.
	gsIDMATRIX2.M[1][1] = int16(4096 * 4 * int32(h) / (3 * int32(w)))
	// In the GsOFSGPU mode the GTE's screen offset goes back to 0, as
	// GsInit3D sets it (probe spec/probes/gsclip).
	if gs.ofsGPU {
		setOFX(0)
		setOFY(0)
	}
	gs.draw(0)
	if resident {
		spinCycles(residentInitGraphPad)
	}
}

// interlace is 1 for an interlaced display: one asked for, or one taller
// than a field, which can be shown no other way.
func interlace(h, intmode uint16) uint8 {
	if intmode&gsInterlace != 0 || h > 256 {
		return 1
	}
	return 0
}

// resetGPU waits for the drawing queue to empty, prints the reference's
// notice, resets the GPU and asks it for its version, which leaves the
// display off. The word the kernel writes first is one the reference
// library has it write too, which the GPU takes as a no-op.
func resetGPU() {
	drain()
	ttyPrintf(resetLine, 0x8004829c, 0x800482e4)
	libStart()
	libps_biosA(biosGPUCommand, 0x0004829c, 0, 0, 0)
	// As Sony's library does around the reset (spec 08): every interrupt
	// masked, channel 2 stopped in its linked-list mode and enabled.
	mask := iMask.Get()
	iMask.Set(0)
	dmaGPUControl.Set(0x0401)
	dmaPriority.Set(dmaPriority.Get() | dmaGPUEnable)
	gp1.Set(0x00000000)
	dispLast.valid = false
	// The reference clears its 6,400 bytes of state here a byte at a time
	// (spec 08). The clean library's is the drawing queue's requests,
	// none of them waiting after drain; the rest of the time is
	// resetRest.
	clearBytes(unsafe.Pointer(&queue.reqs), unsafe.Sizeof(queue.reqs))
	spin(resetRest)
	// The interrupts as they were, which libStart left with the vertical
	// blank's and the DMA's on, with any enabled since.
	iMask.Set(iMask.Get() | mask)
	// The GPU's version, which the answer to GP1 10h leaves in GPUREAD
	// until it is read: unread, it would be the first word a StoreImage
	// reads.
	gp1.Set(0x10000007)
	gp0.Get()
	startup()
}

// startup does the work Sony's resident library did before the program
// ran, right after the GPU reset of the program's first ResetGraph(0) or
// (3) (spec 07, "The library's start-up"), in this order: the kernel's
// controller driver, the drive's reset and the sound system with its tick
// at every vertical blank, and the memory card driver. Each start skips
// itself once done, so a system a program used before any ResetGraph is
// not started again, and a later ResetGraph starts nothing.
func startup() {
	padStart()
	soundStart()
	cardStart()
}

// resident says this library is laid out as the resident image, which Startup
// alone sets. The start of such a program is timed against Sony's routine (spec
// 21, "Start-up"), whereas a linked-in library's first ResetGraph does this work
// and has its time from the reference.
var resident bool

// What the resident image's start spends past its work, in cycles, so that the
// kernel calls come when they do on Sony's (spec 21, "Start-up"), which on some
// programs decides the vertical blank they land on. They are spent where the
// work is that they stand for: before the first kernel call, before the sound
// system's event, at the sound system's end and before the card driver.
// GsInitGraph's is the 30,000 by which Sony's call is longer, every call.
const (
	residentEntryPad     = 6000
	residentSoundPad     = 63000
	residentSoundEndPad  = 13000
	residentCardPad      = 1500
	residentInitGraphPad = 30000
)

// Startup is the resident library's init routine for a library laid out as
// the image, which a program built for it calls before its own start-up
// (spec 15): spec 07's steps 1 to 6, in their order, and the tick mode the
// routine leaves. A program's own ResetGraph(0) after it starts nothing. The
// linked-in target does not call it, and starts these at the first ResetGraph.
func Startup() {
	resident = true
	spinCycles(residentEntryPad)
	libStart()
	SetDispMask(0)
	ResetGraph(0)
	SsSetTickMode(5)
}

// resetLine is the line ResetGraph prints, with the reference's own table
// addresses, 8004829Ch and 800482E4h, kept so that it is the same.
const resetLine = "ResetGraph:jtb=%08x,env=%08x\n"

// resetNotice prints the line the reference library prints at every
// ResetGraph(0) and (3), one character a call, and then has the kernel
// write the word 0004829Ch to GP0. The two numbers are the reference's
// own table addresses, kept so that the line is the same length.
func resetNotice() {
	ttyPrintf(resetLine, 0x8004829c, 0x800482e4)
	libps_biosA(biosGPUCommand, 0x0004829c, 0, 0, 0)
}

// resetRest is the part of the reference's 6,400-byte clear that has no
// state in the clean library to clear, 4,096 bytes at 8 cycles each, in
// spin's loops.
const resetRest = 4096 * 8 / 12

// spinCount is what spin counts on, so that the loop is not optimized
// away.
var spinCount uint32

// spinCycles busy-waits for about c CPU cycles, as the emulator counts
// them, longer if interrupts come in between.
func spinCycles(c uint32) { spin(c / spinTurn) }

// spinTurn is the cycles one turn of spin's loop takes in the emulator: a
// run of 512 turns spans 6,132 cycles in a trace of spec/probes/drawq
// (FntOpen's spin).
const spinTurn = 12

// spin busy-waits for about n of its loops, spinTurn cycles each.
//
//go:noinline
func spin(n uint32) {
	for range n {
		volatile.StoreUint32(&spinCount, volatile.LoadUint32(&spinCount)+1)
	}
}

// spinCall is what a call of spin costs beyond its turns, measured on
// spec/probes/drawq: a spin(1) at a call's end lengthens it by 26 cycles
// and a spin(2) by 38.
const spinCall = 14

// pace busy-waits so that the call takes about c cycles, the call itself
// included: whole turns of spin, so c less spinCall should be a multiple
// of spinTurn. c is a constant at every call and pace is inlined, so the
// turns are computed by the compiler. The library's calls end in one,
// sized so that each takes as long as Sony's (spec 08, "Pacing to Sony's
// times"); an interrupt in between lengthens it as it would Sony's.
//
//go:inline
func pace(c uint32) { spin((c - spinCall) / spinTurn) }

// spacer is the word the short spacers store to: a store to memory costs
// 2 cycles in the emulator, which spaces register accesses more finely
// than spin's turns.
var spacer uint32

// space spends 2 cycles a store on n stores to spacer. n is a constant at
// every call, so that the loop unrolls into plain stores.
func space(n int) {
	for range n {
		volatile.StoreUint32(&spacer, 0)
	}
}

// spaceWith is space storing v, a value live on its path: when two paths
// pad with stores of the same value the compiler merges their tails, and
// then a change to one path's count moves the other's time.
func spaceWith(n int, v uint32) {
	for range n {
		volatile.StoreUint32(&spacer, v)
	}
}

// spacer2 is a second word for a pad that ends where another path's pad
// does: the compiler sinks pads that store to the same word into one block
// after the paths join, feeding the stores with whichever value each path
// had, and then their counts move together. A pad that stores another
// value to another word stays where it is.
var spacer2 uint32

// spaceApart is spaceWith storing v to spacer2 (see spacer2).
func spaceApart(n int, v uint32) {
	for range n {
		volatile.StoreUint32(&spacer2, v)
	}
}

// clearBytes sets the n bytes at p to 0 one store at a time, about 8
// cycles a byte, as the clears the reference and Sony's library make in
// ResetGraph and FntLoad (spec 08).
//
//go:noinline
func clearBytes(p unsafe.Pointer, n uintptr) {
	b := unsafe.Slice((*uint8)(p), n)
	for i := range b {
		volatile.StoreUint8(&b[i], 0)
	}
}

func identity() MATRIX {
	return MATRIX{M: [3][3]int16{{4096, 0, 0}, {0, 4096, 0}, {0, 0, 4096}}}
}

// draw makes buffer i the one drawn into: it moves the clipping to it,
// installing the environment, and then the drawing offset.
func (d *display) draw(i int) {
	psdIDX = int32(i)
	d.clipTo()
	d.offset()
}

// clipTo sets the drawing area to the clipping area GsSetClip2D set, from
// the corner of the buffer drawn into, and installs the environment.
func (d *display) clipTo() {
	x, y := bufCorner(activeBuf())
	c := d.clip
	gsDRAWENV.Clip = RECT{X: x + c.X, Y: y + c.Y, W: c.W, H: c.H}
	gsDRAWENV.put()
}

// offset sets the drawing offset: in the GsOFSGPU mode the environment's,
// the corner of the buffer drawn into plus the screen origin, installing
// it; in the GsOFSGTE mode the GTE's screen offset, leaving the
// environment's as it is. The GTE's is the other buffer's corner plus the
// origin: what is sorted now is drawn after the next swap, into that
// buffer.
func (d *display) offset() {
	x, y := bufCorner(activeBuf())
	if !d.ofsGPU {
		if !gteOn {
			return
		}
		x, y = bufCorner(1 - activeBuf())
		setOFX((int32(x) + int32(position.Offx)) << 16)
		setOFY((int32(y) + int32(position.Offy)) << 16)
		return
	}
	x, y = x+position.Offx, y+position.Offy
	gsDRAWENV.Ofs = [2]int16{x, y}
	gsDRAWENV.put()
}

// GsDefDispBuff places buffer 0 at x0, y0 and buffer 1 at x1, y1 in video
// memory. The buffer drawn into stays the one it was (buffer 0 after
// GsInitGraph), its clipping and offset moved to its new place; a
// GsSortClear packet sorted before keeps the old place (probe
// spec/probes/sortclear).
//
//export GsDefDispBuff
func GsDefDispBuff(x0, y0, x1, y1 uint16) {
	psdOFSX = [2]int16{int16(x0), int16(x1)}
	psdOFSY = [2]int16{int16(y0), int16(y1)}
	gs.draw(activeBuf())
}

// GsGetActiveBuff returns the buffer being drawn into, 0 or 1.
//
//export GsGetActiveBuff
func GsGetActiveBuff() int32 { return int32(activeBuf()) }

// GsSwapDispBuff shows the buffer just drawn, turning the display on, and
// makes the other the one drawn into.
//
//export GsSwapDispBuff
//go:noinline
func GsSwapDispBuff() {
	shown := activeBuf()
	gsDISPENV.Disp.X, gsDISPENV.Disp.Y = bufCorner(shown)
	// As PutDispEnv puts it, writing the screen area and the mode only
	// when they changed, then the display turned on.
	gsDISPENV.put()
	SetDispMask(1)
	gs.draw(1 - shown)
	psdCNT++
	// Sony's copies the display environment a byte at a time before
	// GP1 03h (spec 08); the two PutDrawEnv of draw carry their own pace.
	pace(swapDispBuffPad)
}

// GsSetWorkBase sets where the sorts build their packets, each advancing
// it past what it wrote.
//
//export GsSetWorkBase
//go:noinline
func GsSetWorkBase(p unsafe.Pointer) {
	gsOUT_PACKET_P = p
}

// packet takes n words from the packet area for a packet, its link word
// and n-1 words of commands, and advances the area past them.
func packet(n int) []uint32 {
	p := at((*uint32)(gsOUT_PACKET_P))
	gsOUT_PACKET_P = unsafe.Add(unsafe.Pointer(p), 4*n)
	return unsafe.Slice(p, n)
}

// otEnd is the packet every cleared ordering table ends in, after its
// first entry: four words the GPU takes as no-ops, as the reference
// library's tables end.
var otEnd = [5]uint32{4<<24 | endOfList}

// GsClearOt empties ot, whose Length and Org the program set: it links the
// 1<<Length entries from the last to the first, which leads to otEnd and
// the end of the list, and points Tag at the last. It keeps offset and
// point in the table. An Org of null is a table at address 0 like any other,
// and with a Length of 0 Tag is null (spec 20).
//
//export GsClearOt
//go:noinline
func GsClearOt(offset, point uint16, ot *GsOT) {
	ot = at(ot)
	n := uint32(1) << (ot.Length & 31)
	last := unsafe.Pointer(uintptr(unsafe.Pointer(ot.Org)) + 4*uintptr(n) - 4)
	// DMA channel 6 links the table, each entry to the one before, as
	// the reference does it; then the first goes to otEnd.
	dmaPriority.Set(dmaPriority.Get() | dmaOTCEnable)
	dmaOTCAddr.Set(physical(last))
	dmaOTCBlock.Set(n)
	dmaOTCControl.Set(dmaOTCStart)
	for dmaOTCControl.Get()&dmaBusy != 0 {
	}
	*at(ot.Org) = physical(unsafe.Pointer(&otEnd))
	ot.Offset, ot.Point = uint32(offset), uint32(point)
	ot.Tag = (*uint32)(last)
	// Sony's times its wait for the channel, with a time read (spec 08).
	pace(clearOtPad)
}

// DMA channel 6, which clears ordering tables.
var (
	dmaOTCAddr    = (*volatile.Register32)(unsafe.Pointer(uintptr(0x1f8010e0)))
	dmaOTCBlock   = (*volatile.Register32)(unsafe.Pointer(uintptr(0x1f8010e4)))
	dmaOTCControl = (*volatile.Register32)(unsafe.Pointer(uintptr(0x1f8010e8)))
)

const (
	dmaOTCEnable = 1 << 27
	dmaOTCStart  = 0x11000002 // start, at once, backwards
)

// clearPackets are GsSortClear's packets, one for each buffer, which the
// library keeps itself rather than taking them from the packet area: a
// second GsSortClear in the same buffer relinks the same packet (probe
// spec/probes/gsclip).
var clearPackets [2][4]uint32

// GsSortClear sorts a fill of the buffer drawn into with the color r, g, b
// into the entry Tag points to, the one drawn first, before whatever that
// entry held: that buffer's own packet, the 02h fill alone. Its size is the
// screen's as GsInitGraph was given it, half as wide again in 24-bit color,
// whatever the clip says (spec 20). Tag is left as it is, and a primitive
// sorted into that entry after is drawn before the fill. A null ot is the GsOT
// at address 0, and a null Tag the word there.
//
//export GsSortClear
//go:noinline
func GsSortClear(r, g, b uint8, ot *GsOT) {
	ot = at(ot)
	x, y := bufCorner(activeBuf())
	w := gs.width
	if gsDISPENV.Isrgb24 != 0 {
		w = w * 3 / 2
	}
	p := clearPackets[activeBuf()][:]
	p[1] = 0x02000000 | uint32(b)<<16 | uint32(g)<<8 | uint32(r)
	p[2] = pair(x, y)
	p[3] = pair(w, gs.height)
	link(at(ot.Tag), p)
	pace(sortClearPad)
}

// GsDrawOt starts the GPU drawing ot, from Tag, and returns without
// waiting for it.
//
//export GsDrawOt
//go:noinline
func GsDrawOt(ot *GsOT) {
	// Through at, since a request of 0 is an environment's (request).
	sendList(unsafe.Pointer(at(at(ot).Tag)))
	pace(drawOtPad)
}

// ResetGraph resets the GPU. Mode 0 resets it completely, 1 cancels the
// drawing in progress and empties the command queue, and 3 resets it but
// keeps the drawing and display environments. Mode 1 empties the drawing
// queue too. It returns 0.
//
//export ResetGraph
func ResetGraph(mode int32) int32 {
	switch mode {
	case 0:
		resetGPU()
	case 1, 3:
		// Mode 3 is mode 1 after the notice mode 0 prints (Sony's
		// library, probe spec/probes/resetwait).
		if mode == 3 {
			resetNotice()
		}
		sr := intOff()
		dmaGPUControl.Set(0)
		queue.head, queue.running = queue.tail, 0
		intRestore(sr)
		// As the reference does: acknowledge the GPU's interrupt, then
		// empty its command queue.
		gp1.Set(0x02000000)
		gp1.Set(0x01000000)
		if mode == 3 {
			startup()
		}
	}
	return 0
}

// GsSetOffset sets the drawing offset of the buffer drawn into to x, y
// from its corner, until the next swap, which puts back the screen origin.
// In the GsOFSGTE mode it sets the GTE's screen offset instead, from the
// other buffer's corner, as offset does (probe spec/probes/gsclip).
//
//export GsSetOffset
func GsSetOffset(x, y int32) {
	if !gs.ofsGPU {
		if gteOn {
			bx, by := bufCorner(1 - activeBuf())
			setOFX((int32(bx) + x) << 16)
			setOFY((int32(by) + y) << 16)
		}
		return
	}
	bx, by := bufCorner(activeBuf())
	gsDRAWENV.Ofs = [2]int16{bx + int16(x), by + int16(y)}
	gsDRAWENV.put()
}

// GsSetClip sets the drawing area of the buffer drawn into to the
// rectangle clip, relative to the buffer's corner, until the next swap.
//
//export GsSetClip
func GsSetClip(clip *RECT) {
	bx, by := bufCorner(activeBuf())
	gsDRAWENV.Clip = RECT{X: bx + clip.X, Y: by + clip.Y, W: clip.W, H: clip.H}
	gsDRAWENV.put()
}

// GsSetClip2D sets the clipping area, relative to a buffer's corner, that
// each buffer drawn into takes from the next GsSwapDispBuff,
// GsSetDrawBuffClip or GsDefDispBuff on. It keeps a copy of clip and
// installs nothing. GsInitGraph sets it to the whole buffer.
//
//export GsSetClip2D
func GsSetClip2D(clip *RECT) {
	gs.clip = *clip
}

// GsSetDrawBuffClip sets the drawing area of the buffer drawn into to the
// clipping area GsSetClip2D set, and installs the environment.
//
//export GsSetDrawBuffClip
func GsSetDrawBuffClip() {
	gs.clipTo()
}

// GsSetDrawBuffOffset sets the drawing offset to the screen origin from
// the buffer drawn into, as a swap does: in the GsOFSGPU mode the
// environment's, installed, and in the GsOFSGTE mode the GTE's, from the
// other buffer's corner.
//
//export GsSetDrawBuffOffset
func GsSetDrawBuffOffset() {
	gs.offset()
}

// GsSetOrign sets the screen origin, POSITION, cut to 16 bits, for the
// next GsSwapDispBuff or GsSetDrawBuffOffset to apply. It installs
// nothing.
//
//export GsSetOrign
func GsSetOrign(x, y int32) {
	position = GsPOSITION{Offx: int16(x), Offy: int16(y)}
}

// GsGetWorkBase returns where the next packet goes in the packet area.
//
//export GsGetWorkBase
func GsGetWorkBase() unsafe.Pointer {
	return gsOUT_PACKET_P
}
