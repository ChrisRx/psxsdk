//go:build yaroze

package cleanlib

import (
	"device"
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
// the interrupt that channel 2 raises when it finishes. One made with nothing
// ahead of it and channel 2 finished, the GPU still drawing, waits in the call
// until the GPU is ready and starts at once, since no interrupt is coming
// (drawWait, spec 40). DrawSync waits for
// the queue to empty. Only when words reach the GPU changes, never which
// words or their order.

// queueLen is how many requests the queue holds, the one running among
// them: behind a drawing, Sony's takes 63 images and the 64th waits (spec
// 41).
const queueLen = 64

// request is a packet list to send: from list, or, when list is 0, from
// env, a drawing environment's packet the queue keeps its own copy of,
// since the program may change its DRAWENV before the packet goes out.
// When restore is not 0, env[restore] to env[restore+2] are E3h, E4h and
// E5h words filled in as the request goes out, with the drawing area's
// corners and offset as the GPU then reports them (GP1 10000003h to
// 10000005h, answered through GPUREAD).
//
// A restore of imageLoad or imageStore makes the request a LoadImage or a
// StoreImage that had to wait. Its env holds an empty packet list, the
// rectangle's corner and size, packed as pair packs them, and the caller's
// pixels: startHead sends the image (startImage), then starts the list.
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

	// lastImage is where the last image call queued went: no image waits
	// behind the request there. One the queue dropped (ResetGraph) is
	// behind head, which only grows.
	lastImage uint32
}

// DMA channel 2's interrupt enable and flag in DICR, and the master
// enable.
const (
	dicrGPUEnable = 1 << 18
	dicrGPUFlag   = 1 << 26
	dicrMaster    = 1 << 23
)

// The restore values that mark an image request, beyond any index into env.
const (
	imageLoad  = 0x100
	imageStore = 0x200
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
	// The channel's completion does not interrupt the CPU while nothing
	// waits behind the request running, as with the reference: the queue
	// moves on when the library next looks (pollQueue) and at every
	// interrupt, the vertical blank's included, at the end of each. A
	// request queued behind one still going out turns it on (drawWait,
	// queueImage, spec 43).
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
// the queue on. With the GPU still drawing, an image waiting next goes
// once it has finished (imageTurn).
//
//go:inline
func drawDone() {
	if d := dmaInterrupt.Get(); d&dicrGPUFlag != 0 {
		dmaInterrupt.Set(d&0x00ffffff | dicrGPUFlag)
	}
	if queue.head != queue.tail {
		if gpuIdle() || imageTurn() {
			drawNext()
		}
		return
	}
	// Sony's handler takes its time here whether anything is queued or not.
	nops(24)
}

// handlerDrawDone is drawDone for the interrupt handler's own pass. A
// request that has gone out with nothing waiting behind it is left as it
// is, for the next call that looks at the queue to retire (enqueue, drain,
// pollQueue): Sony's vertical blank interrupt takes the same time with the
// last request finished and not yet looked at as with the queue empty, and
// its next drawing call takes the same time after a blank as before one
// (scee-topten-cardem, spec 47). Retiring it here made the interrupt 10
// cycles longer in that state, where most of a program's blanks find the
// queue, and the next GsDrawOt 6 shorter. With a request waiting it does
// what drawDone does.
//
//go:inline
func handlerDrawDone() {
	if d := dmaInterrupt.Get(); d&dicrGPUFlag != 0 {
		dmaInterrupt.Set(d&0x00ffffff | dicrGPUFlag)
	}
	if requestNext() {
		if gpuIdle() || imageTurn() {
			drawNext()
		}
		return
	}
	nops(handlerDrawDoneNops)
}

// handlerDrawDoneNops is drawDone's 24 less what requestNext takes over
// its test of the queue for empty.
const handlerDrawDoneNops = 21

// imageTurn is drawDone's work when the GPU is not yet free, kept out of it
// so that the handler's usual path takes the time it did before spec 39. If
// the channel has sent all of what is running and a request is next, it
// waits for the GPU to finish drawing and sends it, as Sony's handler does:
// an image (spec 39), with each image queued right behind it as soon as the
// one before has gone out (spec 41), and a packet list, after the images or
// alone, which then goes out on its own (spec 43). In the handler, it then
// handles what was raised meanwhile, a vertical blank that came during the
// drawing among it, before the handler returns (lateInterrupts). It reports
// whether the GPU is free, for drawDone to move the queue on. Waiting for
// each image's own interrupt, as before spec 41, let the kernel's way out
// and in again, and its handlers for that vertical blank, come between them.
// Leaving a packet list for a later interrupt, as before spec 43, left it to
// the next vertical blank, after the frame, where the program's
// ResetGraph(1) could stop it partway through. Interrupts must be off.
//
//go:noinline
func imageTurn() bool {
	if dmaGPUControl.Get()&dmaBusy != 0 || !requestNext() {
		return false
	}
	for gp1.Get()&gpuStatReady == 0 {
	}
	return sendImages()
}

// sendImages is imageTurn's work once the GPU is ready, apart so that
// imageTurn's way out when nothing is next costs what it did before spec
// 41: it sends the images next and reports whether the GPU is then free.
// Each goes as startHead sends it, with Sony's pace around it
// (sendImagesMarkSpace, sendImagesSpace), and its empty packet list, which
// moves the queue on, is waited for here. A packet list next, before the
// images or after them, is started once the GPU is ready, and goes out on
// its own.
//
//go:noinline
func sendImages() bool {
	for requestNext() {
		if !imageNext() {
			listTurn()
			break
		}
		drawWatch.mark(drawNoLimit)
		space(sendImagesMarkSpace)
		drawNext()
		space(sendImagesSpace)
		for dmaGPUControl.Get()&dmaBusy != 0 {
		}
		imageBlank()
	}
	lateInterrupts()
	return gpuIdle()
}

// listTurn starts the packet list waiting next, for sendImages, once the
// GPU is ready for it. With nothing waiting behind it, channel 2's
// completion stops interrupting the CPU first, as before the last image
// (startImage): Sony's is seen to leave the interrupt off once the list
// queued behind a drawing has gone, and on when ResetGraph(1) drops it
// (spec 43).
//
//go:noinline
func listTurn() {
	if queue.head+queue.running+1 == queue.tail {
		dmaInterrupt.Set(dmaInterrupt.Get()&0x00ffffff&^dicrGPUEnable | dicrGPUFlag)
	}
	for gp1.Get()&gpuStatReady == 0 {
	}
	drawNext()
}

// Sony's handler, sending the images queued behind a drawing one after
// another, starts a watch once the GPU is ready for each, reads GPUSTAT
// again about 140 cycles later and then sends the image: 508 cycles from
// one image's blocks to the next's, 16 by 16, in the emulator (spec 41),
// and 966 since the emulator's image DMA takes time (spec 45), of which
// imageBlocksWait takes 528. sendImages spends these 2-cycle stores to
// match: after starting the image, and after starting its empty list. The
// watch comes 112 cycles before the GPUSTAT read, where Sony's comes 140
// before it: the clean library takes 26 cycles more from that read to the
// image's blocks.
const (
	sendImagesMarkSpace = 5
	sendImagesSpace     = 38
)

// lateInterrupts, in the interrupt handler, runs the handler again, for the
// interrupts raised while imageTurn waited for the GPU and sent the images,
// as Sony's handler takes them before it returns (spec 41): the kernel's
// handlers do not see them, and the kernel's way out and in again is not
// taken. Outside the handler, on the program's stack, it does nothing, and
// with the drive's interrupt raised it leaves everything to the handler's
// own pass, which takes the drive's after drawDone and would take it twice.
func lateInterrupts() {
	if inHandler() && iStat.Get()&iMask.Get()&irqCD == 0 {
		libps_interrupt()
	}
}

// inHandler reports whether the code is running in the interrupt handler,
// on the library's own stack.
//
//go:inline
func inHandler() bool {
	sp := device.AsmFull("move {}, $$sp", nil)
	base := uintptr(unsafe.Pointer(&intr.stack))
	return sp >= base && sp < base+unsafe.Sizeof(intr.stack)
}

// imageNext reports whether the request after the one running is an image.
func imageNext() bool {
	i := queue.head + queue.running
	return i != queue.tail && queue.reqs[i%queueLen].restore >= imageLoad
}

// requestNext reports whether a request waits after the one running.
func requestNext() bool {
	return queue.head+queue.running != queue.tail
}

// drawNext, with the GPU free, retires the request that was running and
// starts the next, if there is one. Interrupts must be off.
//
// Sony's calls through the queue take the same time whether the request
// before them has been looked at since it went out or not (probe
// spec/probes/qstate, spec 47), where GsDrawOt, DrawSync(0), FntFlush,
// PutDrawEnv, LoadImage, StoreImage, MoveImage and ClearImage each took the
// clean library 6 cycles more after a PutDrawEnv than after a DrawSync(0):
// drawNext retired the request that was running behind a branch. It retires
// without one, and stores to queue.running bring it to the time the
// retiring took, which the calls' pads were set with: three, the compiler
// taking the plain one before them as dead.
func drawNext() {
	// No branch on whether a request was running: both ways take the same
	// time, as Sony's calls do.
	running := queue.running
	queue.running = 0
	spaceAt(&queue.running)
	spaceAt(&queue.running)
	spaceAt(&queue.running)
	queue.head += running
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
	clearOtPad         = 158 // GsClearOt: 272 on average in the web archive (spec 22)
	sortClearPad       = 14  // GsSortClear: 162 in the same
	drawOtPad          = 26  // GsDrawOt: 390
	drawOtEnterNops    = 6   // before its watch, Sony's 98 cycles into the call
	fntPrintPad        = 14  // FntPrint, with fntPrintSpace: 182 plus 50 a character
	fntPrintEmptyPad   = 38  // FntPrint of "", with fntPrintEmptySpace: 168
	fntPrintRefusedPad = 26  // FntPrint on a stream a print has cut
	swapDispBuffPad    = 290 // GsSwapDispBuff, over its two PutDrawEnv: 4,611 in the same

	// PutDrawEnv (1,952 on spec/probes/qstate): turns of spin and stores
	// before its request is queued, so that the channel starts 732 cycles
	// into the call, as Sony's does (spec 49), and turns and nops after,
	// where Sony's copies the environment. GsSwapDispBuff, over its two
	// PutDrawEnv, has its pace and nops (spec 47).
	putDrawEnvEnterTurns = 7
	putDrawEnvEnterSpace = 7
	putDrawEnvTurns      = 92
	putDrawEnvNops       = 8
	swapDispBuffNops     = 3

	// FntFlush is paced by the character, below.

	// LoadImage and StoreImage spend their time between their accesses,
	// where Sony's does, and have no pad at the end (spec 49): the pads
	// are in imageEnter, sendImage and readImage, and these, in turns of
	// spin, 2-cycle stores, nops, and since spec 50 loads and multiplies,
	// which take the emulator the same time and a console more (see
	// imageEnter's pads). Enter comes before the call's first
	// watch, Ready is the wait before the read of GPUSTAT that the
	// transfer starts on and NoSize an image of no size's in its place.
	// From the transfer's last access to I_MASK, Block is an image of
	// whole blocks', NoBlock one of the CPU's words alone's and Mixed one
	// of both's. Leave is from I_MASK to the return. Word is what comes
	// between two of the CPU's words, Sony's 16 cycles apart. Queued and
	// QueuedLeave are an image's that goes into the queue, before I_MASK
	// is put back and after the read of D2_CHCR that follows it. Each
	// count is what put probe spec/probes/imgsync on Sony's cycle: the
	// first store of a run costs 4, forming the address, and a change to
	// one path can move another's code.
	loadImageEnterSpace       = 1
	loadImageReadyTurns       = 0
	loadImageNoSizeTurns      = 7
	loadImageNoSizeSpace      = 7
	loadImageBlockNops        = 2
	loadImageNoBlockNops      = 2
	loadImageMixedNops        = 1
	loadImageLeaveNops        = 1
	loadImageWordSpace        = 2
	loadImageQueuedTurns      = 0
	loadImageQueuedSpace      = 6
	loadImageQueuedLeaveSpace = 7

	storeImageEnterSpace       = 1
	storeImageReadyTurns       = 0
	storeImageReadyLoads       = 18
	storeImageReadySpace       = 0
	storeImageNoSizeTurns      = 7
	storeImageNoSizeLoads      = 0
	storeImageNoSizeSpace      = 5
	storeImageNoBlockLoads     = 18
	storeImageNoBlockSpace     = 0
	storeImageMixedSpace       = 8
	storeImageLeaveNops        = 1
	storeImageBlockNops        = 0
	storeImageNoBlockNops      = 0
	storeImageMixedNops        = 9
	storeImageWordNops         = 0
	storeImageWordSpace        = 2
	storeImageQueuedTurns      = 0
	storeImageQueuedSpace      = 6
	storeImageQueuedLeaveSpace = 7

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
	// fntFlushLeaveNops is what a flush, the kanji's too, spends after its
	// list is queued: 12 cycles that enqueue took before spec 49.
	fntFlushLeaveNops         = 6
	fntPrintChar              = 2
	fntPrintSpace             = 7
	fntPrintEmptySpace        = 12
	fntPrintPercentSpace      = 1 // "%%", in place of fntPrintChar: 10 more than a character
	fntPrintCutSpace          = 3 // the print cut at the limit: 20 more
	fntPrintRefusedEmptySpace = 8 // the empty string on a stream a print has cut: 16 less than an empty print
	fntFlushEmptySpace        = 0
	fntNulExitSpace           = 1 // the walk that ends at the NUL
	fntLimitExitSpace         = 1 // the walk that ends at the limit: 4 more than at the NUL
	fntGlyphsSpace            = 0
	fntNoGlyphSpace           = 4 // more than fntGlyphsSpace by what setting the first sprite's link costs
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

// drawNow is cdNow for the drawing calls' watches, with the two reads in
// consecutive instructions, 2 cycles apart, as Sony's are in every watch
// (spec 08, spec 49): as Go they come 4 apart, with the second register's
// address formed between them. The nops keep the read of GPUSTAT and the
// end where the Go has them. The drive's watches keep cdNow: in their
// loops the Go's instructions differ, and their passes would change length.
//
//go:inline
func drawNow() uint32 {
	t := device.AsmFull(`.set push
	.set noat
	nop
	lui {}, 0x1f80
	lw $$0, 0x1814({})
	lw {}, 0x1110({})
	nop
	nop
	.set pop`, nil)
	return uint32(t) & 0xffff
}

// mark starts timing a wait as the drawing calls do: it reads the time
// and stores when the wait began and its limit.
//
// It lies in DrawSync's section with the calls that use it (see drawPoll).
//
//go:section .text.DrawSync
//go:noinline
func (w *cdWatch) mark(limit uint32) {
	t := drawNow()
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
// it, and returns what it wrote.
func queueLeave(m uint32) uint32 {
	m |= iMask.Get()
	iMask.Set(m)
	return m
}

// imageLeave is queueLeave for LoadImage and StoreImage: I_MASK read, and
// written with the mask imageEnter found in the next instruction, as
// Sony's is seen to (spec 08, "The exit, every queued call"). Nothing
// changes I_MASK while it is 0.
//
//go:inline
func imageLeave(m uint32) {
	iMask.Get()
	iMask.Set(m)
}

// submit adds the packet list at list, or the environment packet env when
// list is 0, to the queue, starting it if nothing is running and the GPU
// is free. It waits while the queue is full.
//
//go:inline
func submit(list uintptr, env []uint32) {
	enqueue(list, env)
}

// enqueue is submit's work, a call of its own so that each caller pays the
// same for it. It starts its watch, masks the interrupts and looks at the
// queue as queueEnter and queueFree do, with Sony's time from the watch to
// the channel's start, 206 cycles with the GPU free (spec 49): until then
// it started the channel 32 cycles sooner before its return than Sony's
// does, and a DrawSync(1) straight after could find a short list already
// gone. Its accesses between the two are not at Sony's cycles: I_MASK is
// written 60 cycles after the watch where Sony's is 86 after, and D2_CHCR
// and GPUSTAT are read 32 and 36 after that where Sony's are 46 and 72.
//
//go:noinline
func enqueue(list uintptr, env []uint32) {
	libStart()
	for queued() >= queueLen {
		pollQueue()
	}
	drawWatch.mark(drawNoLimit)
	space(enqueueMaskSpace)
	m := iMask.Get()
	iMask.Set(0)
	r := &queue.reqs[queue.tail%queueLen]
	r.list = list
	r.restore = 0
	for i := range env {
		r.env[i] = env[i]
	}
	queue.tail++
	// The request running may have finished before its interrupt came
	// in: then the next starts now, without waiting for it.
	if queueFree() || drawWait() {
		drawNext()
	}
	nops(enqueueLeaveNops)
	queueLeave(m)
}

// enqueue's pads: 2-cycle stores between the watch and I_MASK, and nops
// before I_MASK is put back. Its callers pad before and after the call, to
// Sony's time for each (spec 08, spec 49).
const (
	enqueueMaskSpace = 15
	enqueueLeaveNops = 5
)

// drawWait is enqueue's work when the GPU is not free, kept out of it so
// that the free path takes the time it did before spec 40. If channel 2 has
// finished and the new request is the only one waiting, no completion is
// coming to start it: as Sony's library does, it waits in the call for the
// GPU to be ready for a command and reports true, for enqueue to start the
// request (spec 40). Otherwise the request stays queued for the interrupt.
// With channel 2 still sending, its completion is made to interrupt the
// CPU, as Sony's library has it with a request waiting (spec 43), and
// drawDone starts the request from there; should the channel have finished
// meanwhile, no completion is coming, and the request is taken as above.
// Interrupts must be off.
//
//go:noinline
func drawWait() bool {
	if dmaGPUControl.Get()&dmaBusy != 0 {
		dmaInterrupt.Set(dmaInterrupt.Get()&0x00ffffff | dicrGPUEnable)
		if dmaGPUControl.Get()&dmaBusy != 0 {
			return false
		}
	}
	if queue.head+queue.running+1 != queue.tail {
		return false
	}
	for gp1.Get()&gpuStatReady == 0 {
	}
	return true
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
		if r.env[0] == endOfList {
			startImage(r)
		} else {
			for k := range uint32(3) {
				gp1.Set(gp1InfoDrawArea1 + k)
				r.env[r.restore+k] = (0xe3+k)<<24 | gp0.Get()&0xffffff // GPUREAD
			}
		}
	}
	queue.running = 1
	gp1.Set(gp1DMAToGPU)
	// The address as it is, as Sony's sends it: the channel uses its low
	// 24 bits.
	dmaGPUAddr.Set(uint32(p))
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

// drainOnce is drain's first look at the queue: it reports whether every
// request has gone out and the GPU is idle.
//
//go:inline
func drainOnce() bool {
	sr := intOff()
	free := queueFree()
	if free {
		drawNext()
	}
	intRestore(sr)
	return free && queued() == 0
}

// drainRest is drain, for DrawSync(0) when its first look found the GPU
// busy.
//
//go:noinline
func drainRest() { drain() }

// DrawSync with mode 0 waits until the queue is empty and the GPU idle,
// and returns 0. Any other mode returns at once the number of requests
// waiting behind the one running, or, with none waiting, 1 if the channel
// or the GPU is still busy (drawPoll).
//
//export DrawSync
//go:noinline
func DrawSync(mode int32) int32 {
	if mode == 0 {
		// Sony's starts a watch for the wait, then reads D2_CHCR and
		// GPUSTAT. The first look at the queue is made here and the
		// wait, if there is one, in drainRest: with the loop here the
		// function saved eight registers for it, and DrawSync(1), which
		// then restored them on its way out, took longer than Sony's
		// (spec 49).
		space(drawSyncEnterSpace)
		drawWatch.mark(drawNoLimit)
		space(drawSyncLookSpace)
		if !drainOnce() {
			drainRest()
		}
		space(drawSyncSpace)
		return 0
	}
	if n := drawPoll(); n >= 0 {
		return n
	}
	return drawPollMove()
}

// drawPoll is DrawSync for a mode other than 0. It returns the number of
// requests waiting behind the one running: Sony's counts 1 with a drawing
// running and one image queued behind it, and one more for each image after
// (probe spec/probes/qlen, spec 45). With none waiting it returns 1 if the
// channel or the GPU is still busy, and 0 if not.
//
// It reads what Sony's reads, at Sony's cycles (spec 49, probe
// spec/probes/imgsync). With none waiting: D2_CHCR, and with the channel
// busy nothing more; GPUSTAT 16 cycles later, and the return 6 cycles
// later with the GPU busy than with it ready. With requests waiting:
// D2_CHCR twice, 26 cycles apart. The queue moves on from the channel's
// interrupt, which a waiting request turns on; should the channel be seen
// idle with requests waiting, it returns -1, for DrawSync to move the
// queue on from here (drawPollMove), as every DrawSync(1) did before spec
// 49. A request that has gone out with none behind it is left for the next
// call that looks at the queue to retire, as the handler leaves it
// (handlerDrawDone).
//
// It is a function of its own, one that calls nothing, so that mode 0's
// instructions stay what they were whatever changes here (spec 47) and
// its way out is short enough for Sony's time.
//
// Its pads are loads as well as stores and nops (spec 50): each takes 2
// cycles in the emulator, and under a console's costs a load takes about 6
// where a store takes 2 and a nop 1. With stores alone a poll took Sony's
// time in the emulator and four fifths of it on a console, so a program
// polling behind a MoveImage or a ClearImage counted a fifth more polls
// than on Sony's. The counts below bring a warm poll to Sony's time under
// the costs too (probe spec/probes/pollsync). Behind a LoadImage the
// image's blocks hold the bus, and each load or store waits a block's time
// for it: so the path with no request waiting has no stores, and a
// multiply where two more loads would be, for a poll to wait out as many
// blocks as Sony's does (probe spec/probes/imgparts, the POLL lines). It lies in DrawSync's section,
// so that the image's packing puts it straight after DrawSync and the two
// never share cache lines: placed by size, it can land on DrawSync's own,
// some 10 to 20 cycles a poll. Whether either shares lines with a program's
// polling loop still depends on where the program lies.
//
//go:section .text.DrawSync
//go:noinline
func drawPoll() int32 {
	n := queued() - queue.running
	if n != 0 {
		// Loads and stores of its own (loadsApart, spaceApart), so that
		// the compiler does not merge this path's pad with the other's.
		loadsApart(drawPollWaitingLoads)
		spaceApart(drawPollWaitingSpace, n)
		if dmaGPUControl.Get()&dmaBusy == 0 {
			return -1
		}
		loadsApart(drawPollAgainLoads)
		spaceApart(drawPollAgainSpace, n)
		nops(1)
		dmaGPUControl.Get()
		loadsApart(drawPollCountLoads)
		space(drawPollCountSpace)
		return int32(n)
	}
	loads(drawPollChannelLoads)
	multsWide(drawPollChannelMults)
	nops(drawPollChannelNops)
	space(drawPollChannelSpace)
	if dmaGPUControl.Get()&dmaBusy != 0 {
		// An image's blocks may be going: no stores here, each of
		// which would wait a block's time for the bus (spec 50).
		loads(drawPollBusyLoads)
		mults(drawPollBusyMults)
		nops(drawPollBusyNops)
		return 1
	}
	loads(drawPollStatLoads)
	nops(drawPollStatNops)
	ready := gp1.Get()&gpuStatReady != 0
	loads(drawPollReadyLoads)
	if !ready {
		nops(drawPollDrawingNops)
		return 1
	}
	return 0
}

// drawPollMove is DrawSync(1) when the channel has finished with requests
// waiting: it moves the queue on and counts again.
//
//go:noinline
func drawPollMove() int32 {
	pollQueue()
	n := queued() - queue.running
	if n == 0 && !gpuIdle() {
		return 1
	}
	return int32(n)
}

// drawPoll's pads, in 2-cycle stores, loads and nops, and DrawSync(0)'s:
// before its watch, before its look at the queue and after it.
const (
	drawSyncEnterSpace = 15
	drawSyncLookSpace  = 0
	drawSyncSpace      = 0

	drawPollWaitingLoads = 5
	drawPollWaitingSpace = 6
	drawPollAgainLoads   = 3
	drawPollAgainSpace   = 3
	drawPollCountLoads   = 1
	drawPollCountSpace   = 4
	drawPollChannelLoads = 4
	drawPollChannelMults = 1
	drawPollChannelNops  = 3
	drawPollChannelSpace = 0
	drawPollBusyLoads    = 1
	drawPollBusyMults    = 1
	drawPollBusyNops     = 2
	drawPollStatLoads    = 1
	drawPollStatNops     = 1
	drawPollReadyLoads   = 2
	drawPollDrawingNops  = 3
)

// sendList queues the packet list at p for the GPU, and returns without
// waiting for it.
func sendList(p unsafe.Pointer) {
	submit(uintptr(p), nil)
}

// imageEnter begins LoadImage or StoreImage: it starts the watch, masks
// every interrupt, and looks at the queue. It returns I_MASK for imageLeave
// and how it found the GPU. Free (enterIdle), the call had no need to wait,
// which is when Sony's call clips the caller's rectangle (spec 21). Busy
// with what is queued, when the drawing's end will start an image behind
// it, it returns at once with enterLater, for the caller to queue the
// image (queueLoad, queueStore). With the channel done and the GPU still
// drawing, or the queue full, it waits in the call (enterWaited).
//
// Its accesses come at Sony's cycles (spec 49, probe spec/probes/imgsync):
// from the watch's read of GPUSTAT, I_MASK is read at 84 cycles and
// written at 86, D2_CHCR read at 132 and GPUSTAT at 158, and with the GPU
// free the transfer's own watch starts at 220. With requests waiting
// behind the one running, Sony's reads neither D2_CHCR nor GPUSTAT, and the
// image goes behind them; with none waiting and the channel busy, it reads
// D2_CHCR alone.
//
// It lies in DrawSync's section, as LoadImage and StoreImage do: placed by
// size it landed on LoadImage's own cache lines in one build, and each call
// then ran as if cold, some 350 cycles more under a console's costs (spec
// 50). The section is under 4 KB, so nothing in it shares a line.
//
//go:section .text.DrawSync
//go:noinline
func imageEnter() (m uint32, state enterState) {
	libStart()
	drawWatch.mark(drawNoLimit)
	spins(imageEnterMaskTurns)
	loads(imageEnterMaskLoads)
	nops(imageEnterMaskNops)
	space(imageEnterMaskSpace)
	m = iMask.Get()
	iMask.Set(0)
	if queued() > queue.running {
		// Requests wait behind the one running, and the end of each
		// starts the next: the image goes behind them, unless the
		// queue is full.
		if queued() >= queueLen {
			return imageWait(m), enterWaited
		}
		space(imageEnterWaitingSpace)
		return m, enterLater
	}
	loads(imageEnterChannelLoads)
	space(imageEnterChannelSpace)
	if dmaGPUControl.Get()&dmaBusy != 0 {
		// The channel's completion will start the image: it is made to
		// interrupt the CPU, as Sony's has it with a request waiting
		// (spec 43), DICR read 64 cycles after D2_CHCR and written 16
		// after that.
		space(imageEnterInterruptSpace)
		d := dmaInterrupt.Get() & 0x00ffffff
		space(imageEnterEnableSpace)
		dmaInterrupt.Set(d | dicrGPUEnable)
		return m, enterLater
	}
	loads(imageEnterStatLoads)
	space(imageEnterStatSpace)
	state = enterIdle
	if gp1.Get()&gpuStatReady == 0 {
		// The channel has finished and the GPU is drawing the last of
		// it: nothing would start a queued image. Sony's reads GPUSTAT
		// until the GPU is ready, with I_MASK 0.
		gpuWait()
		state = enterWaited
	}
	// What was running has gone out, and nothing waits behind it.
	queue.head += queue.running
	queue.running = 0
	space(imageEnterWatchSpace)
	// The transfer's own watch.
	drawWatch.mark(drawNoLimit)
	return m, state
}

// enterState is how imageEnter and packetEnter found the GPU: free
// (enterIdle), free after a wait in the call (enterWaited), or busy with a
// request that will start the call's own when it ends (enterLater), for
// the caller to queue it.
type enterState uint32

const (
	enterIdle enterState = iota
	enterWaited
	enterLater
)

// packetEnter is imageEnter for ClearImage, whose request is a packet for
// the channel: the same accesses at the same cycles, to the read of
// GPUSTAT, and no second watch. It returns the request at the queue's
// tail, taken into the queue, for the caller to fill in before it puts
// I_MASK back: when the state is enterIdle or enterWaited it is the one
// running, for the caller to start (packetStart), and when enterLater it
// waits. MoveImage has the same written out, with its packet made first,
// since Sony's starts its channel 24 cycles after the read of GPUSTAT,
// less than a return takes.
//
// It lies in MoveImage's section, as ClearImage does, so that neither call
// shares a cache line with it (see imageEnter).
//
//go:section .text.MoveImage
//go:noinline
func packetEnter() (uint32, enterState, *request) {
	drawWatch.mark(drawNoLimit)
	spins(imageEnterMaskTurns)
	loads(packetEnterMaskLoads)
	space(packetEnterMaskSpace)
	m := iMask.Get()
	iMask.Set(0)
	tail := queue.tail
	q := &queue.reqs[tail%queueLen]
	queue.tail = tail + 1
	if tail-queue.head > queue.running {
		return m, enterLater, q
	}
	loads(packetEnterChannelLoads)
	space(packetEnterChannelSpace)
	if dmaGPUControl.Get()&dmaBusy != 0 {
		space(packetEnterInterruptSpace)
		d := dmaInterrupt.Get() & 0x00ffffff
		space(packetEnterEnableSpace)
		dmaInterrupt.Set(d | dicrGPUEnable)
		return m, enterLater, q
	}
	space(packetEnterStatSpace)
	state := enterIdle
	if gp1.Get()&gpuStatReady == 0 {
		gpuWait()
		state = enterWaited
	}
	// What was running has gone out, and nothing waited behind it: the
	// request is at the queue's head.
	queue.head = tail
	queue.running = 1
	return m, state, q
}

// packetRoom begins MoveImage or ClearImage before packetEnter: it starts
// the library and waits while the queue is full.
//
//go:inline
func packetRoom() {
	libStart()
	if queued() >= queueLen {
		packetRoomWait()
	}
}

//go:noinline
func packetRoomWait() {
	for queued() >= queueLen {
		pollQueue()
	}
}

// packetMoved copies the request q, filled in at what was the queue's tail
// before interrupts were masked, to the tail as it is now, and returns it
// there: a callback that queued a request of its own in between took the
// place.
//
//go:noinline
func packetMoved(q *request) *request {
	n := &queue.reqs[queue.tail%queueLen]
	*n = *q
	return n
}

// packetStart starts the channel on the packet in the request q, which is
// at the queue's head with the GPU free: GP1 04000002h, then D2_MADR,
// D2_BCR and D2_CHCR, with as many nops before each of the three, for
// Sony's 8 cycles from one to the next. The address goes to the channel as
// it is, as Sony's sends it: the channel uses its low 24 bits.
//
//go:inline
func packetStart(q *request, addrNops, countNops, startNops int) {
	gp1.Set(gp1DMAToGPU)
	nops(addrNops)
	dmaGPUAddr.Set(uint32(uintptr(unsafe.Pointer(&q.env))))
	nops(countNops)
	dmaGPUBlock.Set(0)
	nops(startNops)
	dmaGPUControl.Set(dmaLinkedList)
}

// packetEnter's pads.
const (
	packetEnterMaskLoads      = 0
	packetEnterMaskSpace      = 6
	packetEnterChannelLoads   = 0
	packetEnterChannelSpace   = 8
	packetEnterInterruptSpace = 19
	packetEnterEnableSpace    = 5
	packetEnterStatSpace      = 8
)

// gpuWait reads GPUSTAT until the GPU is ready for a command. It is a
// function of its own so that the wait's loop adds nothing to its callers'
// usual paths.
//
//go:noinline
func gpuWait() {
	for gp1.Get()&gpuStatReady == 0 {
	}
}

// imageRefused is imageWait for an image call that imageEnter left for the
// queue and the queue would not take (queueLoad, queueStore): if imageEnter
// made the channel's completion interrupt the CPU for it, nothing waiting,
// that is undone first.
//
//go:noinline
func imageRefused(m uint32) uint32 {
	if queued() <= queue.running {
		dmaInterrupt.Set(dmaInterrupt.Get() & 0x00ffffff &^ dicrGPUEnable)
	}
	return imageWait(m)
}

// imageQueued ends an image call whose request went into the queue, after
// I_MASK is back: Sony's reads D2_CHCR 20 cycles later. Should the channel
// have finished by then, its completion may have come before it was made
// to interrupt, and nothing would start the request: the queue is moved on
// from here.
//
//go:inline
func imageQueued() {
	space(imageQueuedSpace)
	if dmaGPUControl.Get()&dmaBusy == 0 {
		pollQueue()
	}
}

// imageEnter's pads, in turns of spin and 2-cycle stores (spec 49), and
// loads, which take the emulator the same 2 cycles and a console 6 where a
// store takes 2 (spec 50): with stores alone the image calls took Sony's
// time in the emulator and some 100 to 200 cycles less on a console.
const (
	imageEnterMaskTurns      = 2
	imageEnterMaskLoads      = 7
	imageEnterMaskNops       = 1
	imageEnterMaskSpace      = 0
	imageEnterChannelLoads   = 0
	imageEnterChannelSpace   = 10
	imageEnterStatLoads      = 0
	imageEnterStatSpace      = 7
	imageEnterInterruptSpace = 22
	imageEnterWaitingSpace   = 19
	imageEnterEnableSpace    = 5
	imageQueuedSpace         = 7
	imageEnterWatchSpace     = 16
)

// queueLoad and queueStore put a LoadImage or StoreImage of the rectangle
// r, its size kept within video memory, and the pixels at p into the
// queue, after imageEnter found that it could go there (enterLater), and
// report whether they did. Interrupts must be off. Pixels on the stack are
// not queued, since they would be gone by the time the image goes: the
// library's own, the font's palette and the kanji's glyphs, are there.
// Then the caller waits (imageRefused). These are calls of their own, so
// that the image calls' code changes as little as it can when the GPU is
// free.
//
//go:noinline
func queueLoad(r RECT, p unsafe.Pointer) bool {
	nops(queueImageNops)
	if onStack(p) {
		return false
	}
	queueImage(r, p, imageLoad)
	return true
}

//go:noinline
func queueStore(r RECT, p unsafe.Pointer) bool {
	nops(queueImageNops)
	if onStack(p) {
		return false
	}
	queueImage(r, p, imageStore)
	return true
}

// queueImageNops are nops at the start of queueLoad and queueStore, 8
// cycles of the queued call's time, for their size. Without them each is
// 216 bytes, and the image's packing gave one of them the place
// libps_vsync_wait had, 8003C070h, and put that elsewhere, on other cache
// lines. Under a console's costs VSync(0) at 240 lines then returned 158 to
// 165 cycles after the interrupt in many fields, where it had returned
// within 155 (TestVSyncPoll, spec 49).
const queueImageNops = 4

// onStack reports whether p is on the stack, at or above the stack
// pointer, or in the scratchpad, which channel 2 cannot reach and where a
// program may keep its stack.
//
//go:noinline
func onStack(p unsafe.Pointer) bool {
	a := uintptr(p) & 0x1fffffff
	sp := device.AsmFull("move {}, $$sp", nil) & 0x1fffffff
	return a >= 0x1f800000 || sp < 0x800000 && a >= sp
}

// imageWait is imageEnter's wait, for an image call that found the GPU busy
// and cannot go into the queue: it waits for the queue, with interrupts
// on, starts the transfer's watch and returns I_MASK as imageEnter does.
// m is I_MASK as the call found it, with interrupts masked.
//
//go:noinline
func imageWait(m uint32) uint32 {
	for !queueFree() || queued() > queue.running {
		iMask.Set(m)
		drain()
		m = queueEnter()
	}
	queue.head += queue.running
	queue.running = 0
	drawWatch.mark(drawNoLimit)
	return m
}

// queueImage puts the image call kind, imageLoad or imageStore, of the
// rectangle r and the pixels at p into the queue. imageEnter has seen to it
// that the request ahead will start it: either one waits already, whose own
// start made the channel's completion interrupt the CPU, or imageEnter did
// (spec 49). An image of no size is queued as any other, and counted by
// DrawSync(1), as Sony's is (probe spec/probes/imgzero); its turn sends
// nothing (startImage).
func queueImage(r RECT, p unsafe.Pointer, kind uint32) {
	q := &queue.reqs[queue.tail%queueLen]
	q.list = 0
	q.env[0] = endOfList
	q.env[1] = pair(r.X, r.Y)
	q.env[2] = pair(r.W, r.H)
	q.env[3] = uint32(uintptr(p))
	q.restore = kind
	queue.lastImage = queue.tail
	queue.tail++
}

// startImage sends the image request r, which has come to the head of the
// queue with the GPU free, for startHead, and waits for the transfer to
// end: startHead then starts the request's empty packet list, which has
// the channel move the queue on as any request does. Interrupts must be
// off. With no image waiting behind it, channel 2's completion stops
// interrupting the CPU (queueImage); after ResetGraph(1) has emptied the
// queue with images in it, drawDone acknowledges it until the next.
//
//go:inline
func startImage(r *request) {
	last := queue.head == queue.lastImage
	if last {
		dmaInterrupt.Set(dmaInterrupt.Get()&0x00ffffff&^dicrGPUEnable | dicrGPUFlag)
	}
	for gp1.Get()&gpuStatReady == 0 {
	}
	rect := RECT{
		X: int16(r.env[1]),
		Y: int16(r.env[1] >> 16),
		W: int16(r.env[2]),
		H: int16(r.env[2] >> 16),
	}
	p := unsafe.Pointer(uintptr(r.env[3]))
	// An image of no size waits its turn as any other and then sends
	// nothing (spec 49).
	if t := imageTransfer(rect, p); t.none() {
	} else if r.restore == imageLoad {
		sendImage(t, p)
	} else {
		readImage(t, (*uint32)(p))
	}
	if last {
		for dmaGPUControl.Get()&dmaBusy != 0 {
		}
		return
	}
	imageBlocksWait()
}

// loadImage is LoadImage for the library's own images, the font's and the
// kanji's.
func loadImage(r RECT, p unsafe.Pointer) {
	LoadImage(&r, (*uint32)(p))
}

// transfer is an image transfer worked out before the GPU is asked whether
// it is ready, so that its words follow that at once: the place and size
// as the GPU takes them, how many words the CPU moves, the word for
// D2_BCR, blocks of 16 words and how many, and where the blocks start.
type transfer struct {
	place, size uint32
	head        uint32
	count       uint32
	addr        uint32
}

// blocks reports whether the transfer has blocks for the DMA.
//
//go:inline
func (t *transfer) blocks() bool { return t.count >= 1<<16 }

// none reports whether the transfer moves nothing, an image of no size.
//
//go:inline
func (t *transfer) none() bool { return t.head == 0 && !t.blocks() }

// imageTransfer works out the transfer of the rectangle r, of a size kept
// within video memory, and the pixels at p: the CPU moves the words that
// do not make a whole block of 16, from the start of the image, and DMA the
// rest. The pointer goes to the channel as given, as Sony's sends it: the
// channel uses its low 24 bits.
//
//go:inline
func imageTransfer(r RECT, p unsafe.Pointer) transfer {
	const block = 16
	words := (uint32(uint16(r.W))*uint32(uint16(r.H)) + 1) / 2
	head := words % block
	return transfer{
		place: pair(r.X, r.Y),
		size:  pair(r.W, r.H),
		head:  head,
		count: words/block<<16 | block,
		addr:  uint32(uintptr(p)) + 4*head,
	}
}

// leave stores the transfer's words, for its values to be at hand when the
// GPU is ready: a pad of four stores.
//
//go:inline
func (t *transfer) leave() {
	spaceWith(1, t.place)
	spaceWith(1, t.size)
	spaceWith(1, t.count)
	spaceWith(1, t.addr)
}

// sendImage sends the image at p to video memory, for LoadImage, with the
// GPU ready for it: the CPU's words, then the blocks by DMA, which it does
// not wait for. The stores between its accesses bring each to Sony's cycle
// (spec 49): from GP1 04000000h, the four GP0 words at 8, 18, 28 and 38
// cycles, the CPU's words from 58, 16 apart, and GP1 04000002h 16 after
// the last of them, or 56 with none; D2_MADR, D2_BCR and D2_CHCR 8, 18 and
// 26 after that.
//
//go:inline
func sendImage(t transfer, p unsafe.Pointer) {
	gp1.Set(gp1DMAOff)
	nops(sendImageClearNops)
	gp0.Set(0x01000000) // clear the texture cache
	loads(sendImageCommandLoads)
	space(sendImageCommandSpace)
	gp0.Set(0xa0000000)
	loads(sendImagePlaceLoads)
	space(sendImagePlaceSpace)
	gp0.Set(t.place)
	loads(sendImageSizeLoads)
	space(sendImageSizeSpace)
	gp0.Set(t.size)
	if t.head != 0 {
		space(sendImageWordsSpace)
		// The pad comes between two words, not after the last.
		q := unsafe.Pointer(at((*uint32)(p)))
		for n := t.head; ; {
			gp0.Set(*(*uint32)(q))
			q = unsafe.Add(q, 4)
			if n--; n == 0 {
				break
			}
			space(loadImageWordSpace)
		}
	}
	if t.blocks() {
		if t.head == 0 {
			loads(sendImageWholeSpace)
		} else {
			loads(sendImageMixedSpace)
		}
		gp1.Set(gp1DMAToGPU)
		loads(sendImageAddrSpace)
		dmaGPUAddr.Set(t.addr)
		loads(sendImageCountSpace)
		nops(sendImageCountNops)
		dmaGPUBlock.Set(t.count)
		nops(sendImageStartNops)
		dmaGPUControl.Set(dmaToGPU)
	}
}

// sendImage's pads, in 2-cycle stores, and in nops where the first store
// of a run, which forms the address, would make it 4.
const (
	sendImageClearNops    = 1
	sendImageCommandLoads = 3
	sendImageCommandSpace = 0
	sendImagePlaceLoads   = 4
	sendImagePlaceSpace   = 0
	sendImageSizeLoads    = 3
	sendImageSizeSpace    = 0
	sendImageWordsSpace   = 3
	sendImageWholeSpace   = 2
	sendImageMixedSpace   = 0
	sendImageAddrSpace    = 2
	sendImageCountSpace   = 1
	sendImageCountNops    = 1
	sendImageStartNops    = 1
)

// imageSize returns the size of an image kept within video memory: a negative
// width or height counts as 0 (spec 20, "LoadImage and StoreImage sizes").
func imageSize(r RECT) (w, h int16) {
	return int16(clamp(int(r.W), 1024)), int16(clamp(int(r.H), 512))
}

// pair packs two coordinates the way the GPU takes a point or a size: x in
// the low half, y in the high.
func pair(x, y int16) uint32 { return uint32(uint16(x)) | uint32(uint16(y))<<16 }

// imageCallLeaves puts in a0 and v1 what the four image calls leave there on Sony's:
// the interrupt mask, m, and its register's address (probe spec/probes/loadleft). A program
// can get a0 as an argument by passing none, as a call to srand() with no seed does.
// It comes last in each, with nothing else live in those registers. The mask is
// the one the call put back (queueLeave): Sony's does not read I_MASK again.
//
//go:inline
func imageCallLeaves(m uint32) {
	device.AsmFull(`.set push
	.set noat
	move $$4, {m}
	lui $$3, 0x1f80
	ori $$3, $$3, 0x1074
	.set pop`, map[string]interface{}{"m": m})
}

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
	// Sony's builds its packet for longer, and starts its watch 526
	// cycles into the call (spec 08, spec 49).
	spin(putDrawEnvEnterTurns)
	space(putDrawEnvEnterSpace)
	submit(0, (*[16]uint32)(unsafe.Pointer(&e.Dr_env))[:n+1])
	// Sony's copies the environment into its own state after queueing
	// it, a byte at a time (spec 08).
	spin(putDrawEnvTurns)
	nops(putDrawEnvNops)
}

// textureWindow is the E2h command's bits for the texture window w: its
// size as a mask in steps of 8 pixels, 0 for the whole page, and its
// position in the same steps.
func textureWindow(w RECT) uint32 {
	return uint32(-w.W)&0xff>>3 | (uint32(-w.H)&0xff>>3)<<5 |
		(uint32(w.X)>>3&0x1f)<<10 | (uint32(w.Y)>>3&0x1f)<<15
}

// clamp keeps v within 0 and hi. The low end takes no branch: Sony's PutDrawEnv takes the
// same time for a clip at y 0 as at y 240.
func clamp(v, hi int) int { return min(v&^(v>>31), hi) }

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
// GP0 80h, and returns 0. DrawSync waits for it. A rectangle of no width or
// no height is not sent: Sony's returns -1 for it at once, without a look
// at the GPU, and sends one of a negative size as it is (spec 49, probe
// spec/probes/imgsync).
//
// With the GPU free its accesses come at Sony's cycles: packetEnter's, then
// the channel started 24 cycles after the read of GPUSTAT (packetStart),
// I_MASK put back 34 after that, and the return.
//
//export MoveImage
//go:noinline
func MoveImage(r *RECT, x, y int32) int32 {
	if r.W == 0 {
		space(moveImageNoWidthSpace)
		return -1
	}
	if r.H == 0 {
		space(moveImageNoHeightSpace)
		return -1
	}
	// The packet is made in the queue before interrupts are masked: Sony's
	// starts the channel 24 cycles after its read of GPUSTAT, too soon to
	// make it there.
	packetRoom()
	tail := queue.tail
	q := &queue.reqs[tail%queueLen]
	q.list = 0
	q.restore = 0
	q.env[0] = 4<<24 | endOfList
	q.env[1] = 0x80000000
	q.env[2] = pair(r.X, r.Y)
	q.env[3] = pair(int16(x), int16(y))
	q.env[4] = pair(r.W, r.H)
	loads(moveImageEnterLoads)
	space(moveImageEnterSpace)
	// packetEnter, written out.
	drawWatch.mark(drawNoLimit)
	spins(imageEnterMaskTurns)
	space(moveImageMaskSpace)
	m := iMask.Get()
	iMask.Set(0)
	if queue.tail != tail {
		q = packetMoved(q)
		tail = queue.tail
	}
	queue.tail = tail + 1
	later := tail-queue.head > queue.running
	if !later {
		space(moveImageChannelSpace)
		if dmaGPUControl.Get()&dmaBusy != 0 {
			space(packetEnterInterruptSpace)
			d := dmaInterrupt.Get() & 0x00ffffff
			space(packetEnterEnableSpace)
			dmaInterrupt.Set(d | dicrGPUEnable)
			later = true
		}
	}
	if later {
		spins(moveImageQueuedTurns)
		space(moveImageQueuedSpace)
		imageLeave(m)
		imageQueued()
		space(moveImageQueuedLeaveSpace)
		imageCallLeaves(m)
		return 0
	}
	space(moveImageStatSpace)
	if gp1.Get()&gpuStatReady == 0 {
		gpuWait()
	}
	queue.head = tail
	queue.running = 1
	space(moveImageStartSpace)
	packetStart(q, 0, 0, 1)
	nops(moveImageLeavePad)
	space(moveImageLeaveSpace)
	imageLeave(m)
	nops(moveImageLeaveNops)
	imageCallLeaves(m)
	return 0
}

// MoveImage's pads, in 2-cycle stores, turns of spin and nops.
const (
	moveImageNoWidthSpace     = 17
	moveImageNoHeightSpace    = 19
	moveImageEnterLoads       = 0
	moveImageEnterSpace       = 22
	moveImageMaskSpace        = 10
	moveImageChannelSpace     = 3
	moveImageStatSpace        = 8
	moveImageStartSpace       = 0
	moveImageLeavePad         = 10
	moveImageLeaveSpace       = 4
	moveImageLeaveNops        = 6
	moveImageQueuedTurns      = 29
	moveImageQueuedSpace      = 2
	moveImageQueuedLeaveSpace = 15
)
