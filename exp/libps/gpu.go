//go:build yaroze

package libps

import "unsafe"

// The GPU, as libgpu's sys.c (v1.120) drives it: a driver of small
// functions over the registers, and a queue of them, so that a transfer
// asked for while one is running waits its turn and runs from the DMA
// interrupt when the one before ends. The original reached the driver
// through a table of function pointers, jtb (libps 0x8004829C); here
// they are called by name.

// gpuOp is a driver function with the two arguments the queue keeps.
type gpuOp func(arg unsafe.Pointer, arg2 uint32)

// The environment the driver keeps.
//
// libps 0x800482E4, 128 bytes
var (
	gpuKind    uint8 // +0: the GPU's type, from reset
	gpuReady   uint8 // +1: ResetGraph has run
	gpuDebug   uint8 // +2: the debug level, SetGraphDebug
	gpuReverse uint8 // +3: SetGraphReverse
	gpuXMax    int16 // +4: the width of video memory
	gpuYMax    int16 // +6: its height
	// queueIdle is set while an operation is queued and cleared, with the
	// sync callback called, when the queue drains.
	queueIdle uint32 // +8
	// syncCallback is DrawSyncCallback's.
	syncCallback func()  // +12
	curDraw      DRAWENV // +16: the drawing environment in effect
	curDisp      DISPENV // +108: the display environment in effect
)

// The queue: 64 entries, each an operation, its first argument (a copy
// when the operation takes a struct, so the caller's can die) and its
// second, with the index of the next to run and the next to fill, the
// last operation run for the timeout report, and the masks saved while
// the queue is worked on.
//
// libps 0x800640C8, 64 by 96 bytes, and 0x800483D8 to 0x80048400
const queueSize = 64

type queueEntry struct {
	fn   gpuOp
	argp unsafe.Pointer
	arg2 uint32
	arg  [21]uint32
}

var (
	queue        [queueSize]queueEntry
	queueHead    int32 // 0x800483E8: the next entry to fill
	queueTail    int32 // 0x800483EC: the next entry to run
	lastFn       gpuOp // 0x800483D8
	lastArg      unsafe.Pointer
	lastArg2     uint32
	queueMask    uint16     // 0x800483F0
	exequeMask   uint16     // 0x800483F4
	resetMask    uint16     // 0x800483F8
	syncDeadline int32      // 0x800483FC: the vertical blank count a wait gives up at
	syncRetries  int32      // 0x80048400
	ctlBuf       [256]uint8 // 0x800605BC: the last GP1 command of each kind
	videoMode    int32      // 0x8004EA24
	clearPacket  [12]uint32 // 0x8006057C: the packet clearImage sends
)

// The width and height of video memory by GPU type.
//
// libps 0x80048364 and 0x80048378
var (
	xMaxByKind = [5]int16{1024, 1024, 1024, 1024, 1024}
	yMaxByKind = [5]int16{512, 1024, 1024, 512, 1024}
)

// ResetGraph resets the GPU and the library's hold on it. Mode 0 and 3
// are a full reset: the environment cleared, the interrupt handler
// installed, the GPU reset and its type found, the video memory size
// recorded, and the environments in effect forgotten, so the next
// PutDispEnv and PutDrawEnv apply. Mode 1 only cancels what the GPU is
// doing. It returns the GPU type, or 0 for the cancel.
//
// libps 0x80027CB0, 344 bytes
//
//export ResetGraph
func ResetGraph(mode int32) int32 {
	m := mode & 7
	if m == 0 || m == 3 {
		kprintf("ResetGraph:jtb=%08x,env=%08x\n", uint32(0x8004829c), uint32(0x800482e4))
		gpuKind, gpuReady, gpuDebug, gpuReverse, gpuXMax, gpuYMax = 0, 0, 0, 0, 0, 0
		queueIdle, syncCallback = 0, nil
		curDraw, curDisp = DRAWENV{}, DISPENV{}
		ResetCallback()
		gpuCw(0x8004829c & 0xffffff)
		var full uint32
		if m != 0 {
			full = 1
		}
		gpuKind = uint8(gpuReset(full))
		gpuReady = 1
		gpuXMax = xMaxByKind[gpuKind]
		gpuYMax = yMaxByKind[gpuKind]
		fill(unsafe.Pointer(&curDraw), 0xff, unsafe.Sizeof(curDraw))
		fill(unsafe.Pointer(&curDisp), 0xff, unsafe.Sizeof(curDisp))
		return int32(gpuKind)
	}
	if gpuDebug >= 2 {
		kprintf("ResetGraph(%d)...\n", mode)
	}
	return gpuReset(1)
}

// fill sets n bytes at p to c.
//
// libps 0x8002A9D0, 44 bytes
func fill(p unsafe.Pointer, c byte, n uintptr) {
	b := unsafe.Slice((*byte)(p), n)
	for i := range b {
		b[i] = c
	}
}

// gpuReset is the driver's reset: the queue emptied and, for a full
// reset, the GPU DMA stopped and the GPU reset, or for a cancel its
// command buffer cleared. A full reset finds the GPU's type.
//
// libps 0x8002A4D0, 644 bytes
func gpuReset(mode uint32) int32 {
	old := SetIntrMask(0)
	queueHead, queueTail = 0, 0
	resetMask = old
	switch mode & 7 {
	case 0:
		dma2CHCR.Set(0x401)
		dpcr.Set(dpcr.Get() | 0x800)
		gp1.Set(0)
		ctlBuf = [256]uint8{}
		queue = [queueSize]queueEntry{}
	case 1:
		dma2CHCR.Set(0x401)
		dpcr.Set(dpcr.Get() | 0x800)
		gp1.Set(0x02000000)
		gp1.Set(0x01000000)
	}
	SetIntrMask(resetMask)
	if mode&7 == 0 {
		return gpuDetect(mode)
	}
	return 0
}

// gpuDetect asks the GPU its version and returns its type: 3 for the
// GPU that answers 2, or 4 with bit 3 of mode, which also puts it in its
// other mode; for the older GPU, 0 when its status does not take the
// texture page setting and 1 or 2 by the same bit.
//
// libps 0x8002A8F4, 220 bytes
func gpuDetect(mode uint32) int32 {
	gp1.Set(0x10000007)
	if gp0.Get()&0xffffff == 2 {
		if mode&8 != 0 {
			gp1.Set(0x09000001)
			return 4
		}
		return 3
	}
	gp0.Set(0xe1001000 | gp1.Get()&0x3fff)
	_ = gp0.Get()
	if gp1.Get()&0x1000 == 0 {
		return 0
	}
	if mode&8 != 0 {
		gp1.Set(0x20000504)
		return 2
	}
	return 1
}

// GetGraphType returns the GPU's type, as ResetGraph found it.
//
// libps 0x80028034, 16 bytes
//
//export GetGraphType
func GetGraphType() int32 { return int32(gpuKind) }

// GetGraphDebug returns the debug level.
//
// libps 0x80028044, 16 bytes
//
//export GetGraphDebug
func GetGraphDebug() int32 { return int32(gpuDebug) }

// SetGraphDebug sets the debug level, at which the library reports what it
// is asked to do: 1 checks the rectangles it is given, 2 names every call.
// It returns the level before.
//
// libps 0x80027F1C, 108 bytes
//
//export SetGraphDebug
func SetGraphDebug(level int32) int32 {
	old := gpuDebug
	gpuDebug = uint8(level)
	if gpuDebug != 0 {
		kprintf("SetGraphDebug:level:%d,type:%d reverse:%d\n", uint32(gpuDebug), uint32(gpuKind), uint32(gpuReverse))
	}
	return int32(old)
}

// SetDispMask turns the display on (1) or off (0). Off forgets the display
// environment in effect, so the next PutDispEnv applies in full.
//
// libps 0x800280B0, 156 bytes
//
//export SetDispMask
func SetDispMask(mask int32) {
	if gpuDebug >= 2 {
		kprintf("SetDispMask(%d)...\n", mask)
	}
	if mask == 0 {
		fill(unsafe.Pointer(&curDisp), 0xff, unsafe.Sizeof(curDisp))
		gpuCtl(0x03000001)
	} else {
		gpuCtl(0x03000000)
	}
}

// gpuCtl sends a GP1 command and remembers it by its kind.
//
// libps 0x80029DC4, 40 bytes
func gpuCtl(cmd uint32) {
	gp1.Set(cmd)
	ctlBuf[cmd>>24] = uint8(cmd)
}

// gpuGetCtl is the last GP1 command of a kind, as gpuCtl remembered it.
//
// libps 0x80029DEC, 20 bytes
func gpuGetCtl(kind uint32) uint8 { return ctlBuf[kind] }

// gpuParam asks the GPU for one of its parameters.
//
// libps 0x80029E9C, 48 bytes
func gpuParam(n uint32) uint32 {
	gp1.Set(0x10000000 | n)
	return gp0.Get() & 0xffffff
}

// gpuStatus is the GPU's status word.
//
// libps 0x800295BC, 24 bytes
func gpuStatus() uint32 { return gp1.Get() }

// gpuCwb sends n command words through GP0 directly, DMA off.
//
// libps 0x80029E00, 80 bytes
func gpuCwb(words *uint32, n uint32) {
	gp1.Set(0x04000000)
	for _, w := range unsafe.Slice(words, n) {
		gp0.Set(w)
	}
}

// gpuCwc sends a linked list of packets by DMA: the ordering table draw.
//
// libps 0x80029E50, 76 bytes
func gpuCwc(packets unsafe.Pointer, _ uint32) {
	gp1.Set(0x04000002)
	dma2MADR.Set(uint32(uintptr(packets)))
	dma2BCR.Set(0)
	dma2CHCR.Set(0x01000401)
}

// gpuOtc clears an ordering table of n entries ending at ot by DMA
// channel 6, which writes the chain backwards, and waits for it.
//
// libps 0x800295D4, 232 bytes
func gpuOtc(ot unsafe.Pointer, n uint32) uint32 {
	dpcr.Set(dpcr.Get() | 0x08000000)
	dma6CHCR.Set(0)
	dma6MADR.Set(uint32(uintptr(ot)) + n*4 - 4)
	dma6BCR.Set(n)
	dma6CHCR.Set(0x11000002)
	syncStart()
	for dma6CHCR.Get()&dmaBusy != 0 {
		if syncTimeout() != 0 {
			return ^uint32(0)
		}
	}
	return n
}

// gpuDws sends a rectangle of pixels to video memory: the rectangle is
// clipped to video memory, the words that do not make a block of sixteen
// go through GP0 directly, and the blocks by DMA.
//
// libps 0x800298D4, 596 bytes
func gpuDws(arg unsafe.Pointer, arg2 uint32) {
	r := (*RECT)(arg)
	p := (*uint32)(at(uintptr(arg2)))
	clipRect(r, 0)
	n := (int32(r.W)*int32(r.H) + 1) / 2
	if n <= 0 {
		return
	}
	blocks := n >> 4
	rest := n - blocks<<4
	if !waitReady(gpuReadyForCommands) {
		return
	}
	gp1.Set(0x04000000)
	gp0.Set(0x01000000)
	gp0.Set(0xa0000000)
	gp0.Set(*(*uint32)(unsafe.Pointer(r)))
	gp0.Set(*(*uint32)(unsafe.Add(unsafe.Pointer(r), 4)))
	words := unsafe.Slice(p, n)
	for _, w := range words[:rest] {
		gp0.Set(w)
	}
	if blocks != 0 {
		gp1.Set(0x04000002)
		dma2MADR.Set(uint32(uintptr(unsafe.Pointer(&words[rest]))))
		dma2BCR.Set(uint32(blocks)<<16 | 0x10)
		dma2CHCR.Set(0x01000201)
	}
}

// clipRect keeps a rectangle's size inside video memory: a negative
// size is 0, and one past the edge is the edge, which the transfers take
// as the width and height themselves and the fill as one less.
func clipRect(r *RECT, slack int16) {
	switch {
	case r.W < 0:
		r.W = 0
	case r.W > gpuXMax-slack:
		r.W = gpuXMax - slack
	}
	switch {
	case r.H < 0:
		r.H = 0
	case r.H > gpuYMax-slack:
		r.H = gpuYMax - slack
	}
}

// waitReady waits for a GPU status bit, giving up after the sync timeout.
func waitReady(bit uint32) bool {
	if gp1.Get()&bit != 0 {
		return true
	}
	for {
		if syncTimeout() != 0 {
			return false
		}
		if gp1.Get()&bit != 0 {
			return true
		}
	}
}

// syncStart starts a wait on the GPU: it may take four seconds of
// vertical blanks, or a great many polls.
//
// libps 0x8002A754, 52 bytes
func syncStart() {
	syncDeadline = VSync(-1) + 240
	syncRetries = 0
}

// syncTimeout is one poll of a wait: 0 to go on, or -1 when the wait has
// run out, in which case what the GPU was doing is reported and the GPU
// and the queue are reset.
//
// libps 0x8002A788, 364 bytes
func syncTimeout() int32 {
	if VSync(-1) <= syncDeadline {
		old := syncRetries
		syncRetries++
		if old <= 0xf0000 {
			return 0
		}
	}
	kprintf("GPU timeout:que=%d,stat=%08x,chcr=%08x,madr=%08x\n",
		uint32(queueHead-queueTail)&63, gp1.Get(), dma2CHCR.Get(), dma2MADR.Get())
	kprintf("func=(%08x)(%08x,%08x)\n", uint32(opAddr(lastFn)), uint32(uintptr(lastArg)), lastArg2)
	old := SetIntrMask(0)
	queueHead, queueTail = 0, 0
	resetMask = old
	dma2CHCR.Set(0x401)
	dpcr.Set(dpcr.Get() | 0x800)
	gp1.Set(0x02000000)
	gp1.Set(0x01000000)
	SetIntrMask(resetMask)
	return -1
}

// opAddr is an operation's function value's first word, for the report.
func opAddr(f gpuOp) uintptr {
	if f == nil {
		return 0
	}
	return (*(*[2]uintptr)(unsafe.Pointer(&f)))[0]
}

// addque queues an operation, or runs it at once when the queue is empty
// and the GPU idle. size is how many bytes of what arg points at to copy
// into the queue, 0 to pass the pointer itself. It returns the number
// queued, or -1 when the queue stayed full past the timeout.
//
// libps 0x80029EF0, 740 bytes (and 0x80029ECC, the form without arg2)
func addque(fn gpuOp, arg unsafe.Pointer, size uint32, arg2 uint32) int32 {
	syncStart()
	for (queueHead+1)&63 == queueTail {
		if syncTimeout() != 0 {
			return -1
		}
		exeque()
	}
	queueMask = SetIntrMask(0)
	queueIdle = 1
	if gpuReady != 0 && queueHead == queueTail && dma2CHCR.Get()&dmaBusy == 0 && syncCallback == nil {
		for gp1.Get()&gpuReadyForCommands == 0 {
		}
		fn(arg, arg2)
		lastFn, lastArg, lastArg2 = fn, arg, arg2
		SetIntrMask(queueMask)
		return 0
	}
	DMACallback(2, exequeCallback)
	e := &queue[queueHead]
	if size != 0 {
		words := unsafe.Slice((*uint32)(arg), size/4)
		copy(e.arg[:], words)
		e.argp = unsafe.Pointer(&e.arg[0])
	} else {
		e.argp = arg
	}
	e.arg2 = arg2
	e.fn = fn
	queueHead = (queueHead + 1) & 63
	SetIntrMask(queueMask)
	exeque()
	return (queueHead - queueTail) & 63
}

// exequeCallback is exeque as the DMA callback, which returns nothing.
func exequeCallback() { exeque() }

// exeque runs what is queued, each operation once the GPU is ready for
// it and until one starts a transfer, which runs it again when done. It
// returns the number still queued, or 1 while a transfer runs.
//
// libps 0x8002A1D4, 764 bytes
func exeque() int32 {
	if dma2CHCR.Get()&dmaBusy != 0 {
		return 1
	}
	exequeMask = SetIntrMask(0)
	for queueHead != queueTail {
		if (queueTail+1)&63 == queueHead && syncCallback == nil {
			DMACallback(2, nil)
		}
		for gp1.Get()&gpuReadyForCommands == 0 {
		}
		e := &queue[queueTail]
		e.fn(e.argp, e.arg2)
		lastFn, lastArg, lastArg2 = e.fn, e.argp, e.arg2
		queueTail = (queueTail + 1) & 63
		if dma2CHCR.Get()&dmaBusy != 0 {
			break
		}
	}
	SetIntrMask(exequeMask)
	if queueHead == queueTail && dma2CHCR.Get()&dmaBusy == 0 && queueIdle != 0 && syncCallback != nil {
		queueIdle = 0
		syncCallback()
	}
	return (queueHead - queueTail) & 63
}

// gpuSync is the driver's wait. Mode 0 waits for the queue to drain and
// the GPU to go idle, -1 if that takes too long. Any other mode runs what
// it can and returns how much is left: the number queued, or 1 while a
// transfer runs with nothing queued behind it.
//
// libps 0x8002A60C, 328 bytes
func gpuSync(mode int32) int32 {
	if mode == 0 {
		syncStart()
		for queueHead != queueTail {
			exeque()
			if syncTimeout() != 0 {
				return -1
			}
		}
		for dma2CHCR.Get()&dmaBusy != 0 || gp1.Get()&gpuReadyForCommands == 0 {
			if syncTimeout() != 0 {
				return -1
			}
		}
		return 0
	}
	n := (queueHead - queueTail) & 63
	if n != 0 {
		exeque()
	}
	if dma2CHCR.Get()&dmaBusy != 0 || gp1.Get()&gpuReadyForCommands == 0 {
		if n == 0 {
			return 1
		}
	}
	return n
}

// DrawSync waits for the GPU with mode 0, and with any other mode asks
// how much drawing is left; see gpuSync.
//
// libps 0x8002814C, 108 bytes
//
//export DrawSync
func DrawSync(mode int32) int32 {
	if gpuDebug >= 2 {
		kprintf("DrawSync(%d)...\n", mode)
	}
	return gpuSync(mode)
}

// LoadImage sends the pixels at p to the rectangle rect of video memory,
// through the queue, and returns how many operations are queued ahead.
//
// libps 0x80028374, 100 bytes
//
//export LoadImage
func LoadImage(rect *RECT, p *uint32) int32 {
	checkRect("LoadImage", rect)
	return addque(gpuDws, unsafe.Pointer(rect), 8, uint32(uintptr(unsafe.Pointer(p))))
}

// checkRect reports a rectangle at the debug levels: at 1 one that
// reaches outside video memory, at 2 every one.
//
// libps 0x800281B8, 296 bytes
func checkRect(name string, r *RECT) {
	switch gpuDebug {
	case 1:
		if r.Y < 0 || int32(r.Y)+int32(r.H) > int32(gpuYMax) || r.X < 0 || int32(r.X)+int32(r.W) > int32(gpuXMax) {
			kprintf("%s:bad RECT", name)
			kprintf("(%d,%d)-(%d,%d)\n", int32(r.X), int32(r.Y), int32(r.W), int32(r.H))
		}
	case 2:
		kprintf("%s:", name)
		kprintf("(%d,%d)-(%d,%d)\n", int32(r.X), int32(r.Y), int32(r.W), int32(r.H))
	}
}

// GetTPage makes the texture page value for a page at x, y in video
// memory, with the color depth tp (0 for 4-bit, 1 for 8-bit, 2 for
// 16-bit) and the blending rate abr. The fields' places differ between
// the GPU types: the older ones put the depth at bit 9.
//
// libps 0x80027404, 200 bytes
//
//export GetTPage
func GetTPage(tp, abr, x, y int32) uint16 {
	if k := GetGraphType(); k == 1 || k == 2 {
		return uint16((tp&3)<<9 | (abr&3)<<7 | (y&0x300)>>3 | (x&0x3ff)>>6)
	}
	return uint16((tp&3)<<7 | (abr&3)<<5 | (y&0x100)>>4 | (x&0x3ff)>>6 | (y&0x200)<<2)
}

// SetVideoMode sets the video mode, 0 for NTSC and 1 for PAL, and returns
// the one before. The SDK's video.o did this and then set the sequencer's
// tick mode to the vertical blank; the sequencer is not here, so the mode
// is recorded for GetVideoMode alone.
//
// libps 0x8003D72C, 24 bytes, and video.o
//
//export SetVideoMode
func SetVideoMode(mode int32) int32 {
	old := videoMode
	videoMode = mode
	ssSetTickMode(5)
	return old
}

// GetVideoMode returns the video mode SetVideoMode set.
//
// libps 0x8003D744, 16 bytes
//
//export GetVideoMode
func GetVideoMode() int32 { return videoMode }

// The sequencer's tick: the mode SsSetTickMode was given and the rate it
// works out from it, 60 a second for NTSC and 50 for PAL under mode 5,
// the vertical blank. Nothing ticks, since the sequencer is not here.
//
// libps 0x80046C40, 0x80046C44 and 0x800658CC
var (
	tickMode     int32
	tickModeFlag int32
	tickRate     int32
)

// ssSetTickMode records the sequencer's tick mode. Only mode 5, the
// vertical blank, which SetVideoMode sets, is worked out: 60 ticks a
// second for NTSC and 50 for PAL. The original's table of the other
// modes drives a sequencer that is not here.
//
// libps 0x800147EC, 364 bytes
func ssSetTickMode(mode int32) {
	if mode&0x1000 != 0 {
		tickModeFlag = 1
		tickMode = mode & 0xfff
	} else {
		tickModeFlag = 0
		tickMode = mode
	}
	if tickMode == 5 {
		tickRate = 60
		if GetVideoMode() == 1 {
			tickRate = 50
		}
	}
}

// clearBlock is what gpuClr sends: a fill when the rectangle sits on
// 64-pixel columns, otherwise a rectangle drawn with the drawing area
// opened to all of video memory, and a second packet that puts the area
// and the offset back.
//
// libps 0x8006057C, 48 bytes
var clearBlock [12]uint32

// gpuClr fills a rectangle of video memory with a color: the fill
// command where it can be used, else a flat rectangle outside any clip.
//
// libps 0x800296BC, 536 bytes
func gpuClr(arg unsafe.Pointer, color uint32) {
	r := (*RECT)(arg)
	clipRect(r, 1)
	xy := *(*uint32)(arg)
	wh := *(*uint32)(unsafe.Add(arg, 4))
	if r.X&0x3f == 0 && r.W&0x3f == 0 {
		clearBlock[0] = 0x04ffffff
		clearBlock[1] = 0xe6000000
		clearBlock[2] = color&0xffffff | 0x02000000
		clearBlock[3], clearBlock[4] = xy, wh
	} else {
		clearBlock[0] = 0x07000000 | uint32(uintptr(unsafe.Pointer(&clearBlock[8])))&0xffffff
		clearBlock[1] = 0xe3000000
		clearBlock[2] = 0xe4ffffff
		clearBlock[3] = 0xe5000000
		clearBlock[4] = 0xe6000000
		clearBlock[5] = color&0xffffff | 0x60000000
		clearBlock[6], clearBlock[7] = xy, wh
		clearBlock[8] = 0x03ffffff
		clearBlock[9] = gpuParam(3) | 0xe3000000
		clearBlock[10] = gpuParam(4) | 0xe4000000
		clearBlock[11] = gpuParam(5) | 0xe5000000
	}
	gpuCwc(unsafe.Pointer(&clearBlock[0]), 0)
}

// gpuDrs reads a rectangle of video memory into memory: the rectangle is
// clipped to video memory, the words that do not make a block of sixteen
// come through GP0 directly, and the blocks by DMA.
//
// libps 0x80029B28, 668 bytes
func gpuDrs(arg unsafe.Pointer, arg2 uint32) {
	syncStart()
	r := (*RECT)(arg)
	p := (*uint32)(at(uintptr(arg2)))
	clipRect(r, 0)
	n := (int32(r.W)*int32(r.H) + 1) / 2
	if n <= 0 {
		return
	}
	blocks := n >> 4
	rest := n - blocks<<4
	if !waitReady(gpuReadyForCommands) {
		return
	}
	gp1.Set(0x04000000)
	gp0.Set(0x01000000)
	gp0.Set(0xc0000000)
	gp0.Set(*(*uint32)(unsafe.Pointer(r)))
	gp0.Set(*(*uint32)(unsafe.Add(unsafe.Pointer(r), 4)))
	if !waitReady(gpuReadyToSend) {
		return
	}
	words := unsafe.Slice(p, n)
	for i := range words[:rest] {
		words[i] = gp0.Get()
	}
	if blocks != 0 {
		gp1.Set(0x04000003)
		dma2MADR.Set(uint32(uintptr(unsafe.Pointer(&words[rest]))))
		dma2BCR.Set(uint32(blocks)<<16 | 0x10)
		dma2CHCR.Set(0x01000200)
	}
}
