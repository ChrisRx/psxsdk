//go:build yaroze

package libps

// What a control call waits for.
const (
	cdWaitAck      = iota // the drive's acknowledge
	cdWaitComplete        // the command's completion
	cdNoWait              // nothing
)

// cdControl sends a command for the three control calls. A command that
// reads or seeks is sent the position first, as a Setloc, when the
// program gave one; the sync callback is kept out of the way while that
// goes; and the whole is tried four times. The drive's status is
// refreshed first when the shell was open. It returns 1 when the command
// went, and for cdWaitComplete only when it completed.
//
// libps 0x800394E0, 0x80039618 and 0x80039744
func cdControl(com uint8, param, result *uint8, wait int) int32 {
	saved := cdSyncCB
	for range 4 {
		cdSyncCB = nil
		if com != cdlNop && cdStatus&0x10 != 0 {
			cdCw(cdlNop, nil, nil, false)
		}
		r := result
		if wait == cdNoWait {
			r = nil
		}
		if param != nil && cdNeedsPos[com] && cdCw(cdlSetloc, param, r, false) != 0 {
			continue
		}
		cdSyncCB = saved
		if cdCw(com, param, r, wait == cdNoWait) != 0 {
			continue
		}
		if wait == cdWaitComplete {
			return b2i(cdSync(0, result) == cdlComplete)
		}
		return 1
	}
	cdSyncCB = saved
	return 0
}

func b2i(b bool) int32 {
	if b {
		return 1
	}
	return 0
}

// CdControl sends a command and waits for the drive to acknowledge it.
// param is the command's parameter, if it takes one, and result gets the
// reply when it is not nil. It returns 0 if the command could not be
// sent.
//
// libps 0x800394E0, 312 bytes
//
//export CdControl
func CdControl(com uint8, param *uint8, result *uint8) int32 {
	cdStart()
	return cdControl(com, param, result, cdWaitAck)
}

// CdControlB is CdControl waiting for the command to complete.
//
// libps 0x80039744, 324 bytes
//
//export CdControlB
func CdControlB(com uint8, param *uint8, result *uint8) int32 {
	cdStart()
	return cdControl(com, param, result, cdWaitComplete)
}

// CdControlF is CdControl without the wait: it returns as soon as the
// command is sent, and result is ignored.
//
// libps 0x80039618, 300 bytes
//
//export CdControlF
func CdControlF(com uint8, param *uint8, result *uint8) int32 {
	cdStart()
	return cdControl(com, param, result, cdNoWait)
}

// CdSync waits for the last command to complete and returns its status,
// CdlComplete or CdlDiskError, with the reply in result; mode 1 returns
// CdlNoIntr at once if it has not.
//
// libps 0x80039470, 32 bytes
//
//export CdSync
func CdSync(mode int32, result *uint8) int32 { return cdSync(mode, result) }

// CdReady waits for a sector to be ready and returns the status, taking
// it; mode 1 returns CdlNoIntr at once if none is.
//
// libps 0x80039490, 32 bytes
//
//export CdReady
func CdReady(mode int32, result *uint8) int32 { return cdReady(mode, result) }

// cdSetSyncCB and cdSetReadyCB set the callbacks and return the ones
// before.
//
// libps 0x800394B0 and 0x800394C8, 24 bytes each
func cdSetSyncCB(fn cdCallback) cdCallback {
	old := cdSyncCB
	cdSyncCB = fn
	return old
}

func cdSetReadyCB(fn cdCallback) cdCallback {
	old := cdReadyCB
	cdReadyCB = fn
	return old
}

// CdSetDebug sets how much the library says and returns the level before:
// 1 for errors, 2 for every command.
//
// libps 0x800393F0, 24 bytes
//
//export CdSetDebug
func CdSetDebug(level int32) int32 {
	old := cdDebug
	cdDebug = level
	return old
}

// CdFlush acknowledges whatever the drive has pending.
//
// libps 0x800393D0, 32 bytes
//
//export CdFlush
func CdFlush() { cdFlush() }

// CdStatus is the drive's status byte from its last reply.
//
// libps 0x80039324, 16 bytes
//
//export CdStatus
func CdStatus() uint8 { return uint8(cdStatus) }

// CdMode is the mode last set.
//
// libps 0x80039334, 16 bytes
//
//export CdMode
func CdMode() uint8 { return cdMode }

// CdLastCom is the command last sent.
//
// libps 0x80039344, 16 bytes
//
//export CdLastCom
func CdLastCom() uint8 { return cdCom }

// CdLastPos is the position last sent.
//
// libps 0x80039354, 16 bytes
//
//export CdLastPos
func CdLastPos() *CdlLOC { return &cdPos }

// CdIntToPos converts a sector number to the position the drive takes:
// minutes, seconds and sectors in binary coded decimal, counted from two
// seconds before the data, which is where the disc begins.
//
// libps 0x80039914, 260 bytes
//
//export CdIntToPos
func CdIntToPos(i int32, p *CdlLOC) *CdlLOC {
	i += 150
	seconds := i / 75
	sector := i - seconds*75
	minutes := seconds / 60
	seconds -= minutes * 60
	p.Minute = itob(minutes)
	p.Second = itob(seconds)
	p.Sector = itob(sector)
	return p
}

// CdPosToInt converts a position back to its sector number.
//
// libps 0x80039A18, 128 bytes
//
//export CdPosToInt
func CdPosToInt(p *CdlLOC) int32 {
	return (btoi(p.Minute)*60+btoi(p.Second))*75 + btoi(p.Sector) - 150
}

// itob and btoi convert between a number and its binary coded decimal.
func itob(v int32) uint8 { return uint8(v/10<<4 | v%10) }
func btoi(b uint8) int32 { return int32(b>>4)*10 + int32(b&15) }
