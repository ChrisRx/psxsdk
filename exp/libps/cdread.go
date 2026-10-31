//go:build yaroze

package libps

import (
	"unsafe"

	"go.chrisrx.dev/psx/internal/volatile"
)

// Reading sectors. CdRead sets the drive reading from the position last
// sent and takes the ready callback: each sector the drive announces is
// fetched into the program's buffer from the interrupt, after its header
// is checked against the sector expected, until the count is done, when
// the drive is paused and the program's callbacks put back. CdReadSync
// watches the count, restarting a read that has stalled.

// The read in progress.
//
// libps 0x8004D75C and on
var (
	cdReadSectors int32   // 0x8004D75C: how many sectors were asked for
	cdReadBuf     *uint32 // 0x8004D760: where they go
	cdReadAddr    *uint32 // 0x8004D764: where the next one goes
	cdReadMode    int32   // 0x8004D768: the mode the drive is read in
	cdReadWords   int32   // 0x8004D76C: the words of a sector in that mode
	cdReadLeft    int32   // 0x8004D770: sectors to come, -1 for a failure
	cdReadLast    int32   // 0x8004D774: when the last sector came
	cdReadStart   int32   // 0x8004D778: when the read began
	cdReadSector  int32   // 0x8004D77C: the sector expected next

	// The program's callbacks, kept while the read has the drive.
	cdSavedSyncCB  cdCallback // 0x8004D780
	cdSavedReadyCB cdCallback // 0x8004D784

	// A sector's header and subheader, read to check the sector, and the
	// mode byte sent to the drive: neither on the stack, since the
	// library hands their addresses on.
	cdHeader    [3]uint32
	cdModeParam uint8
)

// cdReadLeftNow is the count as the interrupt handler may have left it.
func cdReadLeftNow() int32 { return int32(volatile.LoadUint32((*uint32)(unsafe.Pointer(&cdReadLeft)))) }

// cdReadLastNow is the time of the last sector, likewise.
func cdReadLastNow() int32 { return int32(volatile.LoadUint32((*uint32)(unsafe.Pointer(&cdReadLast)))) }

// CdRead reads sectors into buf from the position the last Setloc gave,
// in the given mode, and returns 0 if the read could not be started. The
// read goes on in the background, and CdReadSync waits for it. The drive
// is set to deliver whole sectors, header and all, so that each can be
// checked; mode's size bits say how much of each the program gets.
//
// libps 0x8003BF60, 264 bytes
//
//export CdRead
func CdRead(sectors int32, buf *uint32, mode int32) int32 {
	cdStart()
	cdReadMode = mode
	switch mode & 0x30 {
	case 0:
		cdReadWords = 0x200
	case 0x20:
		cdReadWords = 0x249
	default:
		cdReadWords = 0x246
	}
	cdReadMode |= 0x20
	cdReadBuf = buf
	cdReadSectors = sectors
	cdSavedSyncCB = cdSetSyncCB(nil)
	cdSavedReadyCB = cdSetReadyCB(nil)
	cdReadStart = VSync(-1)
	if cdStatus&0xe0 != 0 {
		cdControl(cdlPause, nil, nil, cdWaitComplete)
	}
	return b2i(cdReadGo(false) > 0)
}

// cdReadGo starts the drive on the read, or starts it again after a
// failure: the shell is checked, the drive is paused and sent the
// position again for a retry, the mode is set when it differs from the
// drive's, and the read is sent with the ready callback in place. It
// returns the sectors to come, -1 for a failure.
//
// libps 0x8003BD88, 472 bytes
func cdReadGo(retry bool) int32 {
	cdSetSyncCB(nil)
	cdSetReadyCB(nil)
	if CdStatus()&0x10 != 0 {
		if VSync(-1)&0x3f == 0 {
			kputs("CdRead: Shell open...\n")
		}
		cdControl(cdlNop, nil, nil, cdNoWait)
		cdReadStart = VSync(-1)
		cdReadLeft = -1
		return cdReadLeft
	}
	if retry {
		kputs("CdRead: retry...\n")
		cdControl(cdlPause, nil, nil, cdWaitAck)
		if cdControl(cdlSetloc, &cdPos.Minute, nil, cdWaitAck) == 0 {
			cdReadLeft = -1
			return cdReadLeft
		}
	}
	cdFlush()
	cdModeParam = uint8(cdReadMode)
	if cdModeParam != cdMode || retry {
		if cdControl(cdlSetmode, &cdModeParam, nil, cdWaitAck) == 0 {
			cdReadLeft = -1
			return cdReadLeft
		}
	}
	cdReadSector = CdPosToInt(&cdPos)
	cdSetReadyCB(cdReadReady)
	cdReadAddr = cdReadBuf
	cdControl(cdlReadN, nil, nil, cdNoWait)
	cdReadLeft = cdReadSectors
	cdReadLast = VSync(-1)
	return cdReadLeft
}

// cdReadReady is the ready callback while a read is on: it takes the
// sector into the buffer, after checking its header against the sector
// expected when the program takes only the data, counts it, and when the
// read is done, or has failed, or has gone on for twenty seconds, pauses
// the drive, puts the program's callbacks back and tells its read
// callback.
//
// libps 0x8003BB64, 548 bytes
func cdReadReady(status uint8, result *[8]uint8) {
	if status != cdlDataReady {
		cdReadLeft = -1
	} else if cdReadLeft > 0 {
		if cdReadWords == 0x200 {
			cdGetSector(&cdHeader[0], 3)
			if CdPosToInt((*CdlLOC)(unsafe.Pointer(&cdHeader[0]))) != cdReadSector {
				kputs("CdRead: sector error\n")
				cdReadLeft = -1
			}
		}
		cdGetSector(cdReadAddr, cdReadWords)
		cdReadAddr = (*uint32)(unsafe.Add(unsafe.Pointer(cdReadAddr), cdReadWords*4))
		cdReadLeft--
		cdReadSector++
	}
	cdReadLast = VSync(-1)
	if cdReadLeft < 0 {
		cdReadGo(true)
	}
	if VSync(-1) > cdReadStart+1200 {
		cdReadLeft = -1
	}
	if cdReadLeft != 0 && VSync(-1) <= cdReadStart+1200 {
		return
	}
	cdSetSyncCB(cdSavedSyncCB)
	cdSetReadyCB(cdSavedReadyCB)
	cdControl(cdlPause, nil, nil, cdWaitAck)
	if cdReadDone != nil {
		status := uint8(cdlComplete)
		if cdReadLeft != 0 {
			status = cdlDiskError
		}
		cdReadDone(status, result)
	}
}

// CdReadSync waits for the read to finish and returns 0 when it has, the
// sectors still to come with mode 1, which does not wait, or -1 when the
// read has taken twenty seconds. A read that has failed, or has gone a
// second without a sector, is started again. result gets the drive's
// last reply.
//
// libps 0x8003C068, 204 bytes
//
//export CdReadSync
func CdReadSync(mode int32, result *uint8) int32 {
	var left int32
	for {
		switch {
		case VSync(-1) > cdReadStart+1200:
			left = -1
		case cdReadLeftNow() < 0 || VSync(-1) > cdReadLastNow()+60:
			cdReadGo(true)
			left = cdReadSectors
		default:
			left = cdReadLeftNow()
		}
		if mode != 0 || left <= 0 {
			break
		}
	}
	cdReady(1, result)
	return left
}

// cdSetReadDone sets the callback told when a read ends and returns the
// one before.
//
// libps 0x8003C134, 24 bytes
func cdSetReadDone(fn cdCallback) cdCallback {
	old := cdReadDone
	cdReadDone = fn
	return old
}
