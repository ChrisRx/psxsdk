//go:build yaroze

package libps

import (
	"unsafe"

	"go.chrisrx.dev/psx/internal/volatile"
)

// The CD-ROM, as libcd drives it. A command goes to the drive through
// its four byte registers and is answered by an interrupt or two: an
// acknowledge, a completion, a sector ready, the end of the data, or an
// error. The interrupt handler reads each reply, keeps its bytes, and sets
// one of two statuses, the sync status for the command and the ready
// status for the data, which the library's waits poll and the program's
// callbacks are told of. A sector's data comes over DMA channel 3 when
// asked for.

// The drive's commands, by number.
const (
	cdlSync     = 0x00
	cdlNop      = 0x01
	cdlSetloc   = 0x02
	cdlPlay     = 0x03
	cdlReadN    = 0x06
	cdlPause    = 0x09
	cdlInit     = 0x0a
	cdlDemute   = 0x0c
	cdlSetmode  = 0x0e
	cdlGetlocL  = 0x10
	cdlGetlocP  = 0x11
	cdlSeekL    = 0x15
	cdlSeekP    = 0x16
	cdlTest     = 0x19
	cdlReadS    = 0x1b
	cdlCommands = 0x1c
)

// The replies, which are the statuses too.
const (
	cdlNoIntr      = 0
	cdlDataReady   = 1
	cdlComplete    = 2
	cdlAcknowledge = 3
	cdlDataEnd     = 4
	cdlDiskError   = 5
)

// What the library knows of each command.
//
// libps 0x8004D3C4, 0x8004D508, 0x8004D588, 0x8004D608 and 0x8004D688
var (
	// cdNeedsPos are the commands CdControl sends the position to first.
	cdNeedsPos = [32]bool{cdlPlay: true, cdlReadN: true, cdlSeekL: true, cdlSeekP: true, cdlReadS: true}

	// cdTwoReplies are the commands answered twice, acknowledged and
	// then completed; the others are complete when acknowledged.
	cdTwoReplies = [32]bool{0x07: true, 0x08: true, cdlPause: true, cdlInit: true, 0x12: true, cdlSeekL: true, cdlSeekP: true, 0x1a: true}

	// cdClearsReady are the commands that start the ready status over.
	cdClearsReady = [32]bool{cdlPlay: true, 0x04: true, 0x05: true, cdlReadN: true, cdlSeekL: true, cdlSeekP: true, cdlReadS: true}

	// cdParams is how many parameter bytes each command takes. The test
	// command's count is set by a program.
	cdParams = [32]int32{cdlSetloc: 3, 0x0d: 2, cdlSetmode: 1, 0x12: 1, 0x14: 1}
)

// cdAckHasStatus reports whether a command's acknowledge carries the
// drive's status: all but the two that answer with a position.
func cdAckHasStatus(com uint8) bool { return com != cdlGetlocL && com != cdlGetlocP }

// The names, for the messages.
//
// libps 0x8004D468 and 0x8004D4E8
var (
	cdComNames = [cdlCommands]string{"CdlSync", "CdlNop", "CdlSetloc", "CdlPlay", "CdlForward", "CdlBackword", "CdlReadN", "CdlStandby", "CdlStop", "CdlPause", "CdlReset", "CdlMute", "CdlDemute", "CdlSetfilter", "CdlSetmode", "?", "CdlGetlocL", "CdlGetlocP", "?", "CdlGetTN", "CdlGetTD", "CdlSeekL", "CdlSeekP", "?", "?", "?", "?", "CdlReadS"}
	cdIntNames = [7]string{"NoIntr", "DataReady", "Complete", "Acknowledge", "DataEnd", "DiskError", "?"}
)

// cdComstr names a command, or "none".
//
// libps 0x80039408, 52 bytes
func cdComstr(com uint8) string {
	if com < cdlCommands {
		return cdComNames[com]
	}
	return "none"
}

// cdIntstr names a reply, or "none".
//
// libps 0x8003943C, 52 bytes
func cdIntstr(intr uint8) string {
	if intr < 7 {
		return cdIntNames[intr]
	}
	return "none"
}

// A cdCallback is told a status and the eight bytes of the reply that set
// it.
type cdCallback func(status uint8, result *[8]uint8)

// The library's state.
//
// libps 0x8004D444 and on
var (
	cdSyncCB   cdCallback // 0x8004D444: told of each command's completion
	cdReadyCB  cdCallback // 0x8004D448: told of each sector
	cdReadDone cdCallback // 0x8004D758: told when a read ends
	cdDebug    int32      // 0x8004D450: how much to say
	cdStatus   uint32     // 0x8004D454: the drive's status byte from its last reply
	cdStatus1  uint32     // 0x8004D458: the byte after it
	cdOpens    int32      // 0x8004D45C: how many times the shell has been found open
	cdPos      CdlLOC     // 0x8004D460: the last position sent
	cdMode     uint8      // 0x8004D464: the last mode set
	cdCom      uint8      // 0x8004D465: the last command sent
)

func init() { cdPos.Minute = 2 }

// The statuses the interrupt handler sets, which the waits read through
// volatile loads: the sync status, the ready status, and the end of the
// data, which is reported through the ready status too.
//
// libps 0x8004D720, 0x8004D721 and 0x8004D722
var cdStat [3]uint8

// The replies kept for each status, eight bytes apiece.
//
// libps 0x800606D4, 0x800606DC and 0x800606E4
var cdSyncResult, cdReadyResult, cdEndResult [8]uint8

// A wait's deadline and name, for the timeout message.
//
// libps 0x800606EC and 0x800606F4
var (
	cdDeadline int32
	cdWaitName string
)

// cdStatAt reads one of the statuses as the handler may have left it.
func cdStatAt(i int) uint8 { return volatile.LoadUint8(&cdStat[i]) }

// cdSetResult copies a reply into a program's buffer, if it gave one.
func cdSetResult(dst *uint8, src *[8]uint8) {
	if dst == nil {
		return
	}
	out := unsafe.Slice(dst, 8)
	for i := range src {
		out[i] = volatile.LoadUint8(&src[i])
	}
}

// cdIntr takes one reply from the drive: the interrupt's kind and the
// bytes of the reply, which it acknowledges. It keeps the drive's status,
// counts the shell opening, and sets the status the kind of reply decides
// with the reply's bytes. It returns which waits the reply satisfies, 2
// for the sync status and 4 for the ready status, 0 for no reply, and -1
// for a kind it does not know.
//
// libps 0x80039A98, 1400 bytes
func cdIntr() int32 {
	cdIndex.Set(1)
	intr := cdReg3.Get() & 7
	if intr == 0 {
		return 0
	}
	// Read again until two reads agree.
	for cdReg3.Get()&7 != intr {
		intr = cdReg3.Get() & 7
	}
	var result [8]uint8
	n := 0
	for n < 8 && cdIndex.Get()&0x20 != 0 {
		result[n] = cdReg1.Get()
		n++
	}
	cdIndex.Set(1)
	cdReg3.Set(7)
	cdReg2.Set(7)
	var errors uint8
	if intr != cdlAcknowledge || cdAckHasStatus(cdCom) {
		if cdStatus&0x10 == 0 && result[0]&0x10 != 0 {
			cdOpens++
		}
		cdStatus = uint32(result[0])
		cdStatus1 = uint32(result[1])
		errors = result[0] & 0x1d
	}
	if intr == cdlDiskError {
		kputs("DiskError: ")
		if cdDebug > 0 {
			kprintf("com=%s,code=(%02x:%02x)\n", cdComstr(cdCom), cdStatus, cdStatus1)
		}
	}
	switch intr {
	case cdlDataReady:
		status := uint8(cdlDataReady)
		if errors != 0 && n != 1 {
			status = cdlDiskError
		}
		cdStat[1] = status
		cdReadyResult = result
		cdIndex.Set(0)
		cdReg3.Set(0)
		return 4
	case cdlComplete:
		status := uint8(cdlComplete)
		if errors != 0 {
			status = cdlDiskError
		}
		cdStat[0] = status
		cdSyncResult = result
		return 2
	case cdlAcknowledge:
		if errors != 0 {
			cdStat[0] = cdlDiskError
			cdSyncResult = result
			return 2
		}
		if cdTwoReplies[cdCom] {
			cdStat[0] = cdlAcknowledge
			cdSyncResult = result
			return 1
		}
		cdStat[0] = cdlComplete
		cdSyncResult = result
		return 2
	case cdlDataEnd:
		cdStat[2] = cdlDataEnd
		cdStat[1] = cdStat[2]
		cdEndResult = result
		cdReadyResult = result
		return 4
	case cdlDiskError:
		cdStat[1] = cdlDiskError
		cdStat[0] = cdStat[1]
		cdSyncResult = result
		cdReadyResult = result
		return 6
	}
	kputs("CDROM: unknown intr")
	kprintf("(%d)\n", int32(intr))
	return -1
}

// cdDispatch takes every reply waiting at the drive and runs the callbacks
// the replies satisfy, leaving the register index as it found it. It is
// the CD-ROM interrupt's handler, and what a wait does for itself when it
// runs inside a callback, where no interrupt is delivered.
//
// libps 0x8003B0B8, 224 bytes
func cdDispatch() {
	index := cdIndex.Get() & 3
	for {
		r := cdIntr()
		if r == 0 {
			break
		}
		if r&4 != 0 && cdReadyCB != nil {
			cdReadyCB(cdStat[1], &cdReadyResult)
		}
		if r&2 != 0 && cdSyncCB != nil {
			cdSyncCB(cdStat[0], &cdSyncResult)
		}
	}
	cdIndex.Set(index)
}

// cdStartWait begins a wait of up to 960 vertical blanks.
func cdStartWait(name string) {
	cdDeadline = VSync(-1) + 960
	cdWaitName = name
}

// cdTimedOut reports whether the wait has run out, saying so with the
// command and the statuses, and flushing the drive, when it has. The
// original also gave up after 3932160 turns of its loop, which came
// later than the deadline; a faster loop would make that sooner, so the
// deadline is the only limit here.
func cdTimedOut() bool {
	if VSync(-1) <= cdDeadline {
		return false
	}
	kputs("CD timeout: ")
	kprintf("%s:(%s) Sync=%s, Ready=%s\n", cdWaitName, cdComstr(cdCom), cdIntstr(cdStat[0]), cdIntstr(cdStat[1]))
	cdFlush()
	return true
}

// cdSync waits for the last command to complete, or to fail, and returns
// which, copying the reply to result; with mode 1 it returns 0 at once
// instead of waiting. The status is left at complete, so that the next
// wait returns at once too.
//
// libps 0x8003A010, 640 bytes
func cdSync(mode int32, result *uint8) int32 {
	cdStartWait("CD_sync")
	for {
		if cdTimedOut() {
			return -1
		}
		if inInterrupt != 0 {
			cdDispatch()
		}
		status := cdStatAt(0)
		if status == cdlComplete || status == cdlDiskError {
			cdStat[0] = cdlComplete
			cdSetResult(result, &cdSyncResult)
			return int32(status)
		}
		if mode != 0 {
			return 0
		}
	}
}

// cdReady waits for a sector, the end of the data, or a failure, and
// returns which, copying the reply to result and taking the status; with
// mode 1 it returns 0 at once instead of waiting.
//
// libps 0x8003A290, 716 bytes
func cdReady(mode int32, result *uint8) int32 {
	cdStartWait("CD_ready")
	for {
		if cdTimedOut() {
			return -1
		}
		if inInterrupt != 0 {
			cdDispatch()
		}
		if status := cdStatAt(2); status != 0 {
			cdStat[2] = 0
			cdSetResult(result, &cdEndResult)
			return int32(status)
		}
		if status := cdStatAt(1); status != 0 {
			cdStat[1] = 0
			cdSetResult(result, &cdReadyResult)
			return int32(status)
		}
		if mode != 0 {
			return 0
		}
	}
}

// cdCw sends a command with its parameter bytes, after waiting for the
// one before to complete, and unless nowait is set waits for the drive's
// first reply, copying it to result. It returns 0 when the command went
// well, -1 for a failure or a timeout, and -2 for a command that needed
// a parameter and got none.
//
// libps 0x8003A55C, 1096 bytes
func cdCw(com uint8, param *uint8, result *uint8, nowait bool) int32 {
	if cdDebug >= 2 {
		kprintf("%s...\n", cdComstr(com))
	}
	if cdParams[com] != 0 && param == nil {
		if cdDebug > 0 {
			kprintf("%s: no param\n", cdComstr(com))
		}
		return -2
	}
	cdSync(0, nil)
	if com == cdlSetloc {
		cdPos = *(*CdlLOC)(unsafe.Pointer(param))
	}
	cdStat[0] = 0
	if cdClearsReady[com] {
		cdStat[1] = 0
	}
	cdIndex.Set(0)
	if n := cdParams[com]; n > 0 {
		for _, b := range unsafe.Slice(param, n) {
			cdReg2.Set(b)
		}
	}
	cdCom = com
	cdReg1.Set(com)
	if nowait {
		return 0
	}
	cdStartWait("CD_cw")
	for cdStatAt(0) == 0 {
		if cdTimedOut() {
			return -1
		}
		if inInterrupt != 0 {
			cdDispatch()
		}
	}
	if cdStatAt(0) == cdlComplete && com == cdlSetmode {
		cdMode = *param
	}
	cdSetResult(result, &cdSyncResult)
	if cdStatAt(0) == cdlDiskError {
		return -1
	}
	return 0
}

// cdFlush acknowledges whatever the drive has pending, sets the statuses
// to complete and nothing ready, and restores the bus timing.
//
// libps 0x8003AA2C, 224 bytes
func cdFlush() {
	cdIndex.Set(1)
	for cdReg3.Get()&7 != 0 {
		cdIndex.Set(1)
		cdReg3.Set(7)
		cdReg2.Set(7)
	}
	cdStat[2] = 0
	cdStat[1] = cdStat[2]
	cdStat[0] = cdlComplete
	cdIndex.Set(0)
	cdReg3.Set(0)
	comDelay.Set(0x1325)
}

// cdVol sets the drive's audio volume: left to left, left to right,
// right to right, right to left.
//
// libps 0x8003A9A4, 136 bytes
func cdVol(vol *[4]uint8) int32 {
	cdIndex.Set(2)
	cdReg2.Set(vol[0])
	cdReg3.Set(vol[1])
	cdIndex.Set(3)
	cdReg1.Set(vol[2])
	cdReg2.Set(vol[3])
	cdReg3.Set(0x20)
	return 0
}

// cdInitVol sets the sound processor to play the drive's audio: the main
// volume if it is off, the CD volume, and the control register's CD
// audio bit, then the drive's own volume to half each side.
//
// libps 0x8003AB0C, 244 bytes
func cdInitVol() int32 {
	if spuReg(0x1b8).Get() == 0 && spuReg(0x1ba).Get() == 0 {
		spuReg(0x180).Set(0x3fff)
		spuReg(0x182).Set(0x3fff)
	}
	spuReg(0x1b0).Set(0x3fff)
	spuReg(0x1b2).Set(0x3fff)
	spuReg(0x1aa).Set(0xc001)
	vol := [4]uint8{0x80, 0, 0x80, 0}
	return cdVol(&vol)
}

// cdInstall clears the library's record of the drive and takes the
// CD-ROM interrupt.
//
// libps 0x8003AC00, 80 bytes
func cdInstall() {
	cdReadyCB = nil
	cdSyncCB = nil
	cdStatus1 = 0
	cdStatus = 0
	ResetCallback()
	InterruptCallback(2, cdDispatch)
}

// cdInit resets the drive: the interrupt taken, whatever is pending
// flushed, and the drive asked for its status, initialized and unmuted.
// It returns -1 when any of that fails.
//
// libps 0x8003AC50, 492 bytes
func cdInit() int32 {
	kputs("CD_init:")
	// The original printed the address of its statuses' pointer, and so
	// does this, to print the same line.
	kprintf("addr=%08x\n", uint32(0x8004d724))
	cdCom = 0
	cdMode = 0
	cdInstall()
	cdIndex.Set(1)
	for cdReg3.Get()&7 != 0 {
		cdIndex.Set(1)
		cdReg3.Set(7)
		cdReg2.Set(7)
	}
	cdStat[2] = 0
	cdStat[1] = cdStat[2]
	cdStat[0] = cdlComplete
	cdIndex.Set(0)
	cdReg3.Set(0)
	comDelay.Set(0x1325)
	cdCw(cdlNop, nil, nil, false)
	if cdStatus&0x10 != 0 {
		cdCw(cdlNop, nil, nil, false)
	}
	if cdCw(cdlInit, nil, nil, false) != 0 {
		return -1
	}
	if cdCw(cdlDemute, nil, nil, false) != 0 {
		return -1
	}
	if cdSync(0, nil) != cdlComplete {
		return -1
	}
	return 0
}

// cdReset is CdReset: mode 2 takes the interrupt only, the others reset
// the drive, and mode 1 sets the audio up too. It returns 1 when that
// went well.
//
// libps 0x80039364, 108 bytes
func cdReset(mode int32) int32 {
	if mode == 2 {
		cdInstall()
		return 1
	}
	if cdInit() != 0 {
		return 0
	}
	if mode == 1 && cdInitVol() != 0 {
		return 0
	}
	return 1
}

// The callbacks CdInit installs, which deliver the kernel's CD-ROM events
// to whatever the program opened on them.
//
// libps 0x800392AC, 0x800392D4 and 0x800392FC
func cdEventSync(uint8, *[8]uint8)  { deliverEvent(0xf0000003, 0x20) }
func cdEventReady(uint8, *[8]uint8) { deliverEvent(0xf0000003, 0x40) }
func cdEventRead(uint8, *[8]uint8)  { deliverEvent(0xf0000003, 0x40) }

// CdInit resets the drive, trying five times, and installs the callbacks
// that deliver its events. It returns 1 when the drive answered.
//
// libps 0x8003921C, 144 bytes
//
//export CdInit
func CdInit() int32 {
	cdStarted = true
	for range 5 {
		if cdReset(1) == 1 {
			cdSyncCB = cdEventSync
			cdReadyCB = cdEventReady
			cdReadDone = cdEventRead
			return 1
		}
	}
	kprintf("CdInit: Init failed\n")
	return 0
}

// cdStarted is whether the drive has been initialized. The resident
// library's startup ran CdInit before the program; this library runs it
// the first time the program asks the drive for anything.
var cdStarted bool

func cdStart() {
	if !cdStarted {
		CdInit()
	}
}

// cdGetSector takes the sector the drive has ready into memory at madr,
// words words of it, over DMA channel 3: the drive is asked for the
// data, the channel is set up, and the transfer is waited for.
//
// libps 0x8003AFA8, 256 bytes
func cdGetSector(madr *uint32, words int32) int32 {
	cdIndex.Set(0)
	cdReg3.Set(0x80)
	cdDelay.Set(0x20943)
	comDelay.Set(0x1323)
	dpcr.Set(dpcr.Get() | 0x8000)
	dma3MADR.Set(uint32(uintptr(unsafe.Pointer(madr))))
	dma3BCR.Set(uint32(words) | 0x10000)
	for cdIndex.Get()&0x40 == 0 {
	}
	dma3CHCR.Set(0x11000000)
	for dma3CHCR.Get()&dmaBusy != 0 {
	}
	comDelay.Set(0x1325)
	return 0
}
