//go:build yaroze

package cleanlib

import (
	"unsafe"

	"go.chrisrx.dev/psx/internal/volatile"
)

// The GPU and its DMA channel, and the primitives the rest of the library
// draws through: sending a packet list, setting a drawing or display
// environment, and loading an image into video memory.

// The GPU's ports: GP0 takes drawing commands and data, GP1 display
// control; reading GP1 gives the status register.
var (
	gp0 = (*volatile.Register32)(unsafe.Pointer(uintptr(0x1f801810)))
	gp1 = (*volatile.Register32)(unsafe.Pointer(uintptr(0x1f801814)))
)

// DMA channel 2, the GPU's, and the control register that enables the
// channels.
var (
	dmaGPUAddr    = (*volatile.Register32)(unsafe.Pointer(uintptr(0x1f8010a0)))
	dmaGPUBlock   = (*volatile.Register32)(unsafe.Pointer(uintptr(0x1f8010a4)))
	dmaGPUControl = (*volatile.Register32)(unsafe.Pointer(uintptr(0x1f8010a8)))
	dmaPriority   = (*volatile.Register32)(unsafe.Pointer(uintptr(0x1f8010f0)))
)

const (
	gpuStatReady = 1 << 26 // GPUSTAT: ready for a command
	gpuStatSend  = 1 << 27 // GPUSTAT: ready to send video memory

	dmaBusy       = 1 << 24
	dmaGPUEnable  = 1 << 11 // DPCR: channel 2 enabled
	dmaLinkedList = 0x01000401
	dmaToGPU      = 0x01000201 // in blocks, as the GPU asks for them
	dmaFromGPU    = 0x01000200 // in blocks, from GPUREAD to memory

	// gp1DMAToGPU points the GPU's DMA requests at GP0, gp1DMAFromGPU at
	// GPUREAD; gp1DMAOff turns them off, for words moved by the CPU.
	gp1DMAToGPU   = 0x04000002
	gp1DMAFromGPU = 0x04000003
	gp1DMAOff     = 0x04000000

	// gp1InfoDrawArea1 asks the GPU for the drawing area's top left
	// corner through GPUREAD; the next two ask for its bottom right
	// corner and the drawing offset.
	gp1InfoDrawArea1 = 0x10000003
)

// physical is p as the DMA controller and a packet's link take it, the low
// 24 bits of the address.
func physical(p unsafe.Pointer) uint32 { return uint32(uintptr(p)) & 0xffffff }

// at returns p as an address Go can reach whatever it holds. A C program's null
// pointer is address 0, which is RAM, the kernel's first words: the resident
// library reads and writes it as any other, and the program goes on (spec 20,
// "Address 0 is memory"). A Go nil stops with a panic, and the same word at
// 80000000h does not. Setting the top bit moves only addresses below it, each to
// its mirror, so no test is needed.
//
//go:inline
func at[T any](p *T) *T {
	return (*T)(unsafe.Pointer(uintptr(unsafe.Pointer(p)) | 0x80000000))
}

// endOfList is the link that ends a packet list.
const endOfList = 0xffffff

// gpuIdle reports whether the GPU has finished what it was given: its DMA
// channel stopped and the GPU ready for a command.
func gpuIdle() bool {
	return dmaGPUControl.Get()&dmaBusy == 0 && gp1.Get()&gpuStatReady != 0
}

// The drawing queue. Drawing requests, packet lists for the GPU's DMA
// channel, do not wait for the GPU: one made while the GPU is busy waits
// in the queue, and each starts when the one before it has gone out, from
// the interrupt that channel 2 raises when it finishes. DrawSync waits for
// the queue to empty. Only when words reach the GPU changes, never which
// words or their order.

// queueLen is how many requests the queue holds, the one running among
// them.
const queueLen = 32

// request is a packet list to send: from list, or, when list is 0, from
// env, a drawing environment's packet the queue keeps its own copy of,
// since the program may change its DRAWENV before the packet goes out.
// When restore is not 0, env[restore] to env[restore+2] are E3h, E4h and
// E5h words filled in as the request goes out, with the drawing area's
// corners and offset as the GPU then reports them (GP1 10000003h to
// 10000005h, answered through GPUREAD).
type request struct {
	list    uintptr
	env     [16]uint32
	restore uint32
}

var queue struct {
	reqs [queueLen]request

	// head is the request running, or next to run, and tail where the
	// next one goes; both only grow, and wrap into reqs. head is
	// advanced by the interrupt handler.
	head, tail uint32

	// running is 1 while the request at head is going out.
	running uint32
}

// DMA channel 2's interrupt enable and flag in DICR, and the master
// enable.
const (
	dicrGPUEnable = 1 << 18
	dicrGPUFlag   = 1 << 26
	dicrMaster    = 1 << 23
)

// libps_draw_check advances the drawing queue if channel 2 has finished,
// and returns 0. The target's libps_draw_irq jumps to it; the clean
// library no longer puts that into the kernel's interrupt chain, since the
// reference adds nothing to it, and the end of every interrupt runs
// drawDone instead.
//
//export libps_draw_check
func libps_draw_check() int32 {
	drawDone()
	return 0
}

// startQueue sets up channel 2's completion flag and the DMA interrupt.
// libStart calls it, with interrupts off.
func startQueue() {
	// The channel's completion does not interrupt the CPU, as with the
	// reference, which takes no interrupt for its drawing: the queue
	// moves on when the library next looks (pollQueue) and at every
	// interrupt, the vertical blank's included, at the end of each.
	dmaInterrupt.Set(dmaInterrupt.Get()&0x00ffffff&^dicrGPUEnable | dicrMaster | dicrGPUFlag)
	iStat.Set(^uint32(irqDMA))
	iMask.Set(iMask.Get() | irqDMA)
}

// queued returns the requests not yet finished, the running one included.
func queued() uint32 {
	return volatile.LoadUint32(&queue.tail) - volatile.LoadUint32(&queue.head)
}

// drawDone is called from the interrupt handler when channel 2 may have
// finished: it acknowledges the interrupt and, if the GPU is free, moves
// the queue on.
func drawDone() {
	if d := dmaInterrupt.Get(); d&dicrGPUFlag != 0 {
		dmaInterrupt.Set(d&0x00ffffff | dicrGPUFlag)
	}
	if queue.head != queue.tail && gpuIdle() {
		drawNext()
	}
}

// drawNext, with the GPU free, retires the request that was running and
// starts the next, if there is one. Interrupts must be off.
func drawNext() {
	if queue.running != 0 {
		queue.running = 0
		queue.head++
	}
	if queue.head != queue.tail {
		startHead()
	}
}

// The drawing calls time their waits and mask interrupts as Sony's
// library does (spec 08, "Calls through the drawing queue"). Each starts
// a watch, then saves I_MASK and writes it 0 while it touches the queue,
// reads D2_CHCR and GPUSTAT to see whether the GPU and its DMA are free,
// and puts I_MASK back at the end. Then each call ends in a pace of the
// cycles below, so that it takes as long as Sony's and a program's frames
// keep their place against the original's (spec 08, "Pacing to Sony's
// times"): the Sony column of the drawq table there, marker to marker on
// spec/probes/drawq, less the call's own time in the clean library,
// measured on the same probe. The paced calls are not inlined, so that
// each costs the same from every caller, as a call of Sony's does.
const (
	loadImagePad       = 182  // LoadImage, with the sizes' pads below
	storeImagePad      = 242  // StoreImage, with the sizes' pads below
	clearImagePad      = 50   // ClearImage, a rectangle (60h): 730
	clearFillPad       = 98   // ClearImage, a fill (02h): 586, measured the same way
	clearOtPad         = 158  // GsClearOt: 272 on average in the web archive (spec 22)
	sortClearPad       = 14   // GsSortClear: 162 in the same
	drawOtPad          = 26   // GsDrawOt: 390
	fntPrintPad        = 14   // FntPrint, with fntPrintSpace: 182 plus 50 a character
	fntPrintEmptyPad   = 38   // FntPrint of "", with fntPrintEmptySpace: 168
	fntPrintRefusedPad = 26   // FntPrint on a stream a print has cut
	putDrawEnvPad      = 1250 // PutDrawEnv: 1,946
	swapDispBuffPad    = 290  // GsSwapDispBuff, over its two PutDrawEnv: 4,611 in the same

	// MoveImage (460) takes 466 in the clean library by itself, so it has
	// no pad. FntFlush is paced by the character, below.

	// Image calls by size (spec 08, "StoreImage and LoadImage by size"), in
	// 2-cycle stores. Sony's spends 16 cycles on each word the CPU moves,
	// and from the pad at the call's end 684 in LoadImage and 694 in
	// StoreImage for whole blocks, 2 more with the words before them, and
	// 652 and 662 plus the words with no block at all. WordSpace is the
	// word's pad, BlockSpace an image with blocks', WholeSpace one of whole
	// blocks' on top of it and NoBlockSpace one of words alone's.
	loadImageWordSpace     = 3
	loadImageBlockSpace    = 1
	loadImageWholeSpace    = 2
	loadImageNoBlockSpace  = 3
	storeImageWordSpace    = 3
	storeImageBlockSpace   = 1
	storeImageWholeSpace   = 2
	storeImageNoBlockSpace = 4

	// The font's costs, in 2-cycle stores (spec 08, "FntPrint and FntFlush
	// by the character"). FntPrint spends fntPrintChar stores on each
	// character it copies (fntPrintPercentSpace on a "%%" pair, Sony's 10
	// more), then fntPrintSpace and its pace, or the empty string's: Sony's
	// 182 plus 50 a character, or 168; a print cut at the stream's limit
	// spends fntPrintCutSpace more (Sony's 20), and on a stream so cut every
	// print takes fntPrintRefusedPad, with fntPrintRefusedEmptySpace for the
	// empty string. FntFlush spends fntFlushEmptySpace stores with nothing
	// to draw (Sony 536), after the text fntGlyphsSpace once the first
	// sprite's link is set or fntNoGlyphSpace in its place, and by the
	// character the stores below, for Sony's 140 a glyph, 34 a space, 46 a
	// tab, 62 a newline, 28 more for the glyph, space or tab that fills a
	// line, and 52 for the newline and 18 for the wrap that reaches the
	// window's bottom and ends the flush. The isbg tile's 40 needs one
	// store, to a word of the font (font.pad). Each count is what put the
	// probe on Sony's number: a store costs 2 or 4 by where the compiler's
	// branches and delay slots fall around it, and a change to one path can
	// move another's, so every edit is measured.
	//
	// The four paths that end the flush at the window's bottom (the
	// newline and the wrap after a glyph, a space or a tab) each pad and
	// break to the same exit. When all four pads store the same value the
	// compiler sinks them into one block there and they share a count
	// (and the moves that feed it); the space's and the tab's store x to
	// the other word (spaceApart) while the newline's and the glyph's
	// store y, which leaves each pad in its own path with its own count.
	fntPrintChar              = 2
	fntPrintSpace             = 7
	fntPrintEmptySpace        = 12
	fntPrintPercentSpace      = 1 // "%%", in place of fntPrintChar: 10 more than a character
	fntPrintCutSpace          = 3 // the print cut at the limit: 20 more
	fntPrintRefusedEmptySpace = 8 // the empty string on a stream a print has cut: 16 less than an empty print
	fntFlushEmptySpace        = 4
	fntNulExitSpace           = 1 // the walk that ends at the NUL
	fntLimitExitSpace         = 1 // the walk that ends at the limit: 4 more than at the NUL
	fntGlyphsSpace            = 0
	fntNoGlyphSpace           = 6 // more than fntGlyphsSpace by what setting the first sprite's link costs
	fntGlyphSpace             = 35
	fntSpaceSpace             = 4
	fntTabSpace               = 5
	fntNewlineSpace           = 8
	fntWrapSpace              = 8  // the wrap, after the glyph that fills the line
	fntSpaceWrapSpace         = 8  // the wrap after a space that fills the line
	fntTabWrapSpace           = 8  // the wrap after a tab that fills the line
	fntNewlineEndSpace        = 10 // the newline that reaches the bottom, with spaceWith
	fntWrapEndSpace           = 10 // the glyph wrap that reaches the bottom, with spaceWith
	fntSpaceWrapEndSpace      = 10 // the space or tab wrap that reaches the bottom, with spaceApart
)

// drawWatch times the drawing calls' waits, as watch does the drive's.
var drawWatch cdWatch

// drawNoLimit is the limit of a wait on the GPU: no spec gives one.
const drawNoLimit = 0xffff

// mark starts timing a wait as the drawing calls do: it reads the time
// and stores when the wait began and its limit.
//
//go:noinline
func (w *cdWatch) mark(limit uint32) {
	t := cdNow()
	volatile.StoreUint32(&w.start, t)
	volatile.StoreUint32(&w.limit, limit)
}

// queueEnter begins a drawing call: it starts the watch, masks every
// interrupt and returns I_MASK as it was, for queueLeave.
func queueEnter() uint32 {
	drawWatch.mark(drawNoLimit)
	m := iMask.Get()
	iMask.Set(0)
	return m
}

// queueFree reports whether the GPU is free for a request, reading
// D2_CHCR, then GPUSTAT, as Sony's library does once in every drawing
// call.
func queueFree() bool {
	busy := dmaGPUControl.Get()&dmaBusy != 0
	return gp1.Get()&gpuStatReady != 0 && !busy
}

// queueLeave ends a drawing call, putting I_MASK back as queueEnter found
// it.
func queueLeave(m uint32) {
	iMask.Set(iMask.Get() | m)
}

// submit adds the packet list at list, or the environment packet env when
// list is 0, to the queue, starting it if nothing is running and the GPU
// is free. It waits while the queue is full.
func submit(list uintptr, env []uint32) {
	enqueue(list, env, 0)
}

// enqueue is submit with the index of env's words to restore, or 0 (see
// request). It reports whether the request started at once, nothing being
// ahead of it, for the image calls that then clip the caller's rectangle.
func enqueue(list uintptr, env []uint32, restore uint32) bool {
	libStart()
	for queued() >= queueLen {
		pollQueue()
	}
	m := queueEnter()
	r := &queue.reqs[queue.tail%queueLen]
	r.list = list
	r.restore = restore
	for i := range env {
		r.env[i] = env[i]
	}
	queue.tail++
	// The request running may have finished before its interrupt came
	// in: then the next starts now, without waiting for it.
	if queueFree() {
		drawNext()
	}
	atOnce := queue.head+1 == queue.tail
	queueLeave(m)
	return atOnce
}

// startHead starts the GPU's DMA channel on the request at the queue's
// head. Interrupts must be off. The channel is enabled in DPCR from
// ResetGraph on.
func startHead() {
	r := &queue.reqs[queue.head%queueLen]
	p := r.list
	if p == 0 {
		p = uintptr(unsafe.Pointer(&r.env))
	}
	if r.restore != 0 {
		for k := range uint32(3) {
			gp1.Set(gp1InfoDrawArea1 + k)
			r.env[r.restore+k] = (0xe3+k)<<24 | gp0.Get()&0xffffff // GPUREAD
		}
	}
	queue.running = 1
	gp1.Set(gp1DMAToGPU)
	dmaGPUAddr.Set(uint32(p) & 0xffffff)
	dmaGPUBlock.Set(0)
	dmaGPUControl.Set(dmaLinkedList)
}

// pollQueue moves the queue on if the request running has finished.
func pollQueue() {
	sr := intOff()
	drawDone()
	intRestore(sr)
}

// drain waits until every queued request has gone out and the GPU is idle.
func drain() {
	for {
		sr := intOff()
		free := queueFree()
		if free {
			drawNext()
		}
		intRestore(sr)
		if free && queued() == 0 {
			return
		}
	}
}

// DrawSync with mode 0 waits until the queue is empty and the GPU idle,
// and returns 0. Any other mode returns at once the number of requests
// still queued or running, or 1 if only the GPU is still drawing.
//
//export DrawSync
//go:noinline
func DrawSync(mode int32) int32 {
	if mode == 0 {
		// Sony's starts a watch for the wait, then reads D2_CHCR and
		// GPUSTAT.
		drawWatch.mark(drawNoLimit)
		drain()
		return 0
	}
	pollQueue()
	if n := queued(); n != 0 {
		return int32(n)
	}
	if gpuIdle() {
		return 0
	}
	return 1
}

// sendList queues the packet list at p for the GPU, and returns without
// waiting for it.
func sendList(p unsafe.Pointer) {
	submit(uintptr(p), nil)
}

// imageEnter begins LoadImage or StoreImage as enqueue begins a request:
// with the watch started and interrupts masked, once the queue is empty
// and the GPU free. It returns I_MASK for queueLeave. Sony's library
// would queue the image behind the drawing; the GPU in the emulator is
// always free by then, so the clean library waits for the queue instead,
// with interrupts on. idle reports that it had no need to, which is when
// Sony's call clips the caller's rectangle (spec 21).
func imageEnter() (m uint32, idle bool) {
	libStart()
	m = queueEnter()
	idle = true
	for !queueFree() || queued() > queue.running {
		idle = false
		iMask.Set(m)
		drain()
		m = queueEnter()
	}
	// What was running has gone out.
	drawNext()
	// The transfer's own wait: a second watch, then GPUSTAT read until
	// the GPU takes a command.
	drawWatch.mark(drawNoLimit)
	for gp1.Get()&gpuStatReady == 0 {
	}
	return m, idle
}

// loadImage copies the image at p, of 16-bit pixels, into the rectangle r
// of video memory. As the reference library does, the CPU writes the
// words that do not make a whole block of 16, from the start of the
// image, and DMA the rest, which goes on after it returns: the next call
// that uses the GPU waits for it. A size that comes to nothing sends nothing.
// It reports whether the queue was idle when it came.
func loadImage(r RECT, p unsafe.Pointer) bool {
	r.W, r.H = imageSize(r)
	m, idle := imageEnter()
	if r.W == 0 || r.H == 0 {
		queueLeave(m)
		pace(loadImagePad)
		return idle
	}
	gp1.Set(gp1DMAOff)
	gp0.Set(0x01000000) // clear the texture cache
	gp0.Set(0xa0000000)
	gp0.Set(pair(r.X, r.Y))
	gp0.Set(pair(r.W, r.H))
	words := (int(r.W)*int(r.H) + 1) / 2
	const block = 16
	head := words % block
	data := unsafe.Slice(at((*uint32)(p)), head)
	for i := range data {
		gp0.Set(data[i])
		space(loadImageWordSpace)
	}
	if n := words / block; n > 0 {
		gp1.Set(gp1DMAToGPU)
		// The pointer as given, as Sony's sends it: the channel uses its
		// low 24 bits.
		dmaGPUAddr.Set(uint32(uintptr(unsafe.Add(p, 4*head))))
		dmaGPUBlock.Set(uint32(n)<<16 | block)
		dmaGPUControl.Set(dmaToGPU)
		space(loadImageBlockSpace)
		if head == 0 {
			space(loadImageWholeSpace)
		}
	} else {
		space(loadImageNoBlockSpace)
	}
	queueLeave(m)
	pace(loadImagePad)
	return idle
}

// imageSize returns the size of an image kept within video memory: a negative
// width or height counts as 0 (spec 20, "LoadImage and StoreImage sizes").
func imageSize(r RECT) (w, h int16) {
	return int16(clamp(int(r.W), 1024)), int16(clamp(int(r.H), 512))
}

// pair packs two coordinates the way the GPU takes a point or a size: x in
// the low half, y in the high.
func pair(x, y int16) uint32 { return uint32(uint16(x)) | uint32(uint16(y))<<16 }

// RECT is a rectangle in video memory: its top left corner and its size.
type RECT struct {
	X, Y, W, H int16
}

// DR_ENV is the packet that sets a drawing environment: a link word and
// the GPU commands.
type DR_ENV struct {
	Tag  uint32
	Code [15]uint32
}

// DRAWENV is a drawing environment: where the GPU draws, with what offset,
// texture page and window, and whether it dithers.
type DRAWENV struct {
	Clip       RECT
	Ofs        [2]int16
	Tw         RECT
	Tpage      uint16
	Dtd        uint8
	Dfe        uint8
	Isbg       uint8
	R0, G0, B0 uint8
	Dr_env     DR_ENV
}

// put installs the environment: it builds the packet in Dr_env and queues
// it for the GPU.
func (e *DRAWENV) put() {
	c := &e.Dr_env.Code
	x0, y0 := clamp(int(e.Clip.X), 1023), clamp(int(e.Clip.Y), 511)
	x1 := clamp(int(e.Clip.X)+int(e.Clip.W)-1, 1023)
	y1 := clamp(int(e.Clip.Y)+int(e.Clip.H)-1, 511)
	c[0] = 0xe3000000 | uint32(x0) | uint32(y0)<<10
	c[1] = 0xe4000000 | uint32(x1) | uint32(y1)<<10
	c[2] = 0xe5000000 | uint32(e.Ofs[0])&0x7ff | (uint32(e.Ofs[1])&0x7ff)<<11
	c[3] = 0xe1000000 | uint32(e.Tpage)&0x1ff | uint32(e.Dtd&1)<<9 | uint32(e.Dfe&1)<<10
	c[4] = 0xe2000000 | textureWindow(e.Tw)
	c[5] = 0xe6000000
	n := uint32(6)
	if e.Isbg != 0 {
		// By ClearImage's rule: the size kept to what the GPU takes, and
		// a fill where x and the width allow it, a rectangle elsewhere.
		w, h := int16(clamp(int(e.Clip.W), 1023)), int16(clamp(int(e.Clip.H), 511))
		code := uint32(0x60000000)
		if e.Clip.X%64 == 0 && w%64 == 0 {
			code = 0x02000000
		}
		c[6] = code | uint32(e.B0)<<16 | uint32(e.G0)<<8 | uint32(e.R0)
		c[7] = pair(e.Clip.X, e.Clip.Y)
		c[8] = pair(w, h)
		n = 9
	}
	e.Dr_env.Tag = n<<24 | endOfList
	submit(0, (*[16]uint32)(unsafe.Pointer(&e.Dr_env))[:n+1])
	// Sony's copies the environment into its own state after queueing
	// it, a byte at a time (spec 08).
	pace(putDrawEnvPad)
}

// textureWindow is the E2h command's bits for the texture window w: its
// size as a mask in steps of 8 pixels, 0 for the whole page, and its
// position in the same steps.
func textureWindow(w RECT) uint32 {
	return uint32(-w.W)&0xff>>3 | (uint32(-w.H)&0xff>>3)<<5 |
		(uint32(w.X)>>3&0x1f)<<10 | (uint32(w.Y)>>3&0x1f)<<15
}

func clamp(v, hi int) int { return min(max(v, 0), hi) }

// DISPENV is a display environment: the area of video memory shown and
// where on the monitor it goes.
type DISPENV struct {
	Disp    RECT
	Screen  RECT
	Isinter uint8
	Isrgb24 uint8
	Pad0    uint8
	Pad1    uint8
}

// mode is the GP1 08h command for the display: its width and height, the
// video mode, the color depth and interlacing.
func (d *DISPENV) mode() uint32 {
	// Each width takes the nearest of the GPU's, the boundaries halfway
	// between (probe spec/probes/dispsys); a height past a field, 256
	// lines on NTSC and 288 on PAL, is interlaced whatever Isinter says.
	var m uint32
	switch w := d.Disp.W; {
	case w <= 280:
	case w <= 352:
		m = 1
	case w <= 400:
		m = 0x40
	case w <= 560:
		m = 2
	default:
		m = 3
	}
	field := int16(256)
	if video.mode == modePAL {
		m |= 0x08
		field = 288
	}
	if d.Disp.H > field {
		m |= 0x24
	}
	if d.Isrgb24 != 0 {
		m |= 0x10
	}
	if d.Isinter != 0 {
		m |= 0x20
	}
	return 0x08000000 | m
}

// start is the GP1 05h command that shows Disp.
func (d *DISPENV) start() uint32 {
	return 0x05000000 | uint32(d.Disp.X)&0x3ff | (uint32(d.Disp.Y)&0x3ff)<<10
}

// dispLast is the display environment last put, for the next put to
// write only what changed. valid is cleared by ResetGraph(0), so that the
// next put writes everything.
var dispLast struct {
	valid bool
	env   DISPENV
}

// put sets the display, as Sony's library does (probe
// spec/probes/dispsys and dispswap). It always writes the area shown (GP1
// 05h). If env is the same as the last put's, byte for byte, that is all.
// Otherwise it sets env's Pad0 to the video mode, 1 for PAL, and then:
// with a Screen other than the last put's, it writes the picture's place
// on the monitor (06h, 07h), and the mode (08h) only if the rest (Disp,
// Isinter, Isrgb24 and the pad bytes, Pad0 as now set) differs too; with
// the same Screen, it writes the mode. So a SetVideoMode since shows at
// the next put only through Pad0: a put of the same env, unchanged since,
// writes no mode.
func (d *DISPENV) put() {
	last := &dispLast
	gp1.Set(d.start())
	if last.valid && *d == last.env {
		return
	}
	d.Pad0 = uint8(video.mode & 1)
	if !last.valid || d.Screen != last.env.Screen {
		d.screenRange()
		if !last.valid || d.Disp != last.env.Disp || d.Isinter != last.env.Isinter ||
			d.Isrgb24 != last.env.Isrgb24 || d.Pad0 != last.env.Pad0 || d.Pad1 != last.env.Pad1 {
			gp1.Set(d.mode())
		}
	} else {
		gp1.Set(d.mode())
	}
	last.env, last.valid = *d, true
}

// screenRange writes the part of the monitor's picture the display
// covers, GP1 06h and 07h, from Screen, in a frame 256 units wide and 240
// lines high, a zero width or height being the whole of it (probe
// spec/probes/dispsys).
func (d *DISPENV) screenRange() {
	// Horizontally a unit is ten GPU clocks, from 608. The start is kept
	// from 500 to 3290 and the end from 580 to 3290; then a width under 8
	// is 8 from the start, past 3290 if need be.
	w := int(d.Screen.W)
	if w == 0 {
		w = 256
	}
	x0 := 608 + int(d.Screen.X)*10
	x1 := x0 + w*10
	x0 = min(max(x0, 500), 3290)
	x1 = min(max(x1, 580), 3290)
	if w < 8 {
		x1 = x0 + 80
	}
	gp1.Set(0x06000000 | uint32(x0)&0xfff | (uint32(x1)&0xfff)<<12)
	// Vertically a unit is a line, from line 16 on NTSC and 19 on PAL
	// (where GsInitGraph puts the screen 24 lines down). The start is
	// kept from 16 to two lines before the last, 258 on NTSC and 312 on
	// PAL, and the end from 18 to the last; then a height under 2 is 2.
	y0, last := 16, 258
	if video.mode == modePAL {
		y0, last = 19, 312
	}
	h := int(d.Screen.H)
	if h == 0 {
		h = 240
	}
	y0 += int(d.Screen.Y)
	y1 := y0 + h
	y0 = min(max(y0, 16), last-2)
	y1 = min(max(y1, 18), last)
	if h < 2 {
		y1 = y0 + 2
	}
	gp1.Set(0x07000000 | uint32(y0)&0x3ff | (uint32(y1)&0x3ff)<<10)
}

// PutDispEnv installs the display environment env at once, as put writes
// it, and returns env. GsDISPENV is left alone.
//
//export PutDispEnv
func PutDispEnv(env *DISPENV) *DISPENV {
	env.put()
	return env
}

// SetDispMask turns the display on (GP1 03000000h) with any mask but 0,
// and off (03000001h) with 0.
//
//export SetDispMask
//go:noinline
func SetDispMask(mask int32) {
	if mask == 0 {
		gp1.Set(0x03000001)
	} else {
		gp1.Set(0x03000000)
	}
	pace(setDispMaskPad)
}

// setDispMaskPad is the rest of SetDispMask's 82 cycles (spec 21).
const setDispMaskPad = 62

// PutDrawEnv installs the drawing environment env, queued with the
// drawing, until the next PutDrawEnv or GsSwapDispBuff, and returns env.
//
//export PutDrawEnv
//go:noinline
func PutDrawEnv(env *DRAWENV) *DRAWENV {
	env.put()
	return env
}

// MoveImage queues a copy of the rectangle r of video memory to x, y,
// GP0 80h, and returns 0. DrawSync waits for it.
//
//export MoveImage
//go:noinline
func MoveImage(r *RECT, x, y int32) int32 {
	p := [5]uint32{
		4<<24 | endOfList,
		0x80000000,
		pair(r.X, r.Y),
		pair(int16(x), int16(y)),
		pair(r.W, r.H),
	}
	submit(0, p[:])
	return 0
}
