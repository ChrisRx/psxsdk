//go:build yaroze

package main

import (
	"go.chrisrx.dev/psx/disc"
	"go.chrisrx.dev/psx/yaroze"
)

// The CD-ROM: the position conversions in place, and a script that
// initializes the drive, finds a file on the disc, reads it by name and
// by sector, and prints what came back, under the originals and the Go
// versions.

// The originals beyond the bindings, at the addresses the generated
// symbol file gives them.

//export CdInit_orig
func CdInit_orig() int32

//export CdSearchFile_orig
func CdSearchFile_orig(fp *yaroze.CdlFILE, name *byte) *yaroze.CdlFILE

//export CdReadFile_orig
func CdReadFile_orig(file *byte, addr *uint32, nbyte int32) int32

//export CdRead_orig
func CdRead_orig(sectors int32, buf *uint32, mode int32) int32

//export CdReadSync_orig
func CdReadSync_orig(mode int32, result *uint8) int32

//export CdControl_orig
func CdControl_orig(com uint8, param *uint8, result *uint8) int32

//export CdControlB_orig
func CdControlB_orig(com uint8, param *uint8, result *uint8) int32

//export CdIntToPos_orig
func CdIntToPos_orig(i int32, p *yaroze.CdlLOC) *yaroze.CdlLOC

//export CdPosToInt_orig
func CdPosToInt_orig(p *yaroze.CdlLOC) int32

//export CdStatus_orig
func CdStatus_orig() uint8

// The Go library's, by the names the bindings lack.

//export CdInit
func CdInit() int32

//export CdStatus
func CdStatus() uint8

// twinData is the file the script reads: 5000 bytes, byte i being
// (i*7+3)&0xff, which mkdisc places on the disc.
var twinData = disc.Register("TWIN.DAT", "twin.dat")

var (
	cdName [16]byte
	cdFile yaroze.CdlFILE
	cdLoc  yaroze.CdlLOC
	cdRes  [8]uint8
	cdData [3 * 512]uint32
)

// checkCdPos compares the conversions between a sector number and the
// drive's position over sector numbers around each boundary.
func checkCdPos() {
	var a, b yaroze.CdlLOC
	for _, i := range [...]int32{0, 1, 74, 75, 149, 150, 151, 4499, 4500, 4650, 12345, 65535, 100000, 330000} {
		CdIntToPos_orig(i, &a)
		yaroze.CdIntToPos(i, &b)
		name := itoa(int(i))
		checkInt("CdIntToPos "+name, int32(a.Minute)<<16|int32(a.Second)<<8|int32(a.Sector), int32(b.Minute)<<16|int32(b.Second)<<8|int32(b.Sector))
		checkInt("CdPosToInt "+name, CdPosToInt_orig(&a), yaroze.CdPosToInt(&b))
	}
}

// cdScript drives the disc and prints what came of it.
func cdScript(mode int32) {
	copy(cdName[:], "\\TWIN.DAT;1\x00")
	var r int32
	if mode == 0 {
		print("cdinit ", CdInit_orig(), "\n")
		printFound(CdSearchFile_orig(&cdFile, &cdName[0]) != nil)
		print("readfile ", CdReadFile_orig(&cdName[0], &cdData[0], 0), "\n")
		for {
			if r = CdReadSync_orig(1, &cdRes[0]); r <= 0 {
				break
			}
			VSync_orig(0)
		}
		print("readsync ", r, " ", hex(cdRes[0]), "\n")
		printData()
		clearData()
		CdIntToPos_orig(int32(twinData.Sector()), &cdLoc)
		print("setloc ", CdControl_orig(yaroze.CdlSetloc, &cdLoc.Minute, nil), "\n")
		print("read ", CdRead_orig(3, &cdData[0], yaroze.CdlModeSpeed), "\n")
		print("readsync0 ", CdReadSync_orig(0, &cdRes[0]), " ", hex(cdRes[0]), "\n")
		printData()
		print("status ", hex(CdStatus_orig()), "\n")
		// Command 1 asks for the status alone.
		print("nop ", CdControlB_orig(1, nil, &cdRes[0]), " ", hex(cdRes[0]), "\n")
		return
	}
	print("cdinit ", CdInit(), "\n")
	printFound(yaroze.CdSearchFile(&cdFile, &cdName[0]) != nil)
	print("readfile ", yaroze.CdReadFile(&cdName[0], &cdData[0], 0), "\n")
	for {
		if r = yaroze.CdReadSync(1, &cdRes[0]); r <= 0 {
			break
		}
		yaroze.VSync(0)
	}
	print("readsync ", r, " ", hex(cdRes[0]), "\n")
	printData()
	clearData()
	yaroze.CdIntToPos(int32(twinData.Sector()), &cdLoc)
	print("setloc ", yaroze.CdControl(yaroze.CdlSetloc, &cdLoc.Minute, nil), "\n")
	print("read ", yaroze.CdRead(3, &cdData[0], yaroze.CdlModeSpeed), "\n")
	print("readsync0 ", yaroze.CdReadSync(0, &cdRes[0]), " ", hex(cdRes[0]), "\n")
	printData()
	print("status ", hex(CdStatus()), "\n")
	print("nop ", yaroze.CdControlB(1, nil, &cdRes[0]), " ", hex(cdRes[0]), "\n")
}

// printFound prints what the search found.
func printFound(found bool) {
	print("search ", found, " ", hex(cdFile.Pos.Minute), hex(cdFile.Pos.Second), hex(cdFile.Pos.Sector), " ", cdFile.Size, " ")
	for _, c := range cdFile.Name {
		if c == 0 {
			break
		}
		print(string(rune(c)))
	}
	print("\n")
}

// printData prints a sum of the file's bytes and whether each is what
// the file holds.
func printData() {
	sum := uint32(0)
	ok := true
	for i := 0; i < 5000; i++ {
		b := uint8(cdData[i/4] >> (8 * (i % 4)))
		sum += uint32(b)
		if b != uint8(i*7+3) {
			ok = false
		}
	}
	if ok {
		print("data ok ", sum, "\n")
	} else {
		print("data bad ", sum, "\n")
	}
}

func clearData() {
	for i := range cdData {
		cdData[i] = 0
	}
}
