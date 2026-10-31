//go:build yaroze

package main

import (
	"unsafe"

	"go.chrisrx.dev/psx/disc"
	"go.chrisrx.dev/psx/internal/volatile"
	"go.chrisrx.dev/psx/yaroze"
)

// The sequencer: breakout's bank and song are read off the disc, the bank
// is sent to the sound processor and the song played under each library,
// and the voices' registers are printed as the song goes, after a pause
// and a resume, after a stop, and after everything is closed.

//export SsVabTransfer_orig
func SsVabTransfer_orig(vh *uint8, vb *uint8, id int16, wait int16) int16

//export SsVabClose_orig
func SsVabClose_orig(id int16)

//export SsSeqOpen_orig
func SsSeqOpen_orig(p *uint32, vab int16) int16

//export SsSeqPlay_orig
func SsSeqPlay_orig(seq int16, mode uint8, count int16)

//export SsSeqPause_orig
func SsSeqPause_orig(seq int16)

//export SsSeqReplay_orig
func SsSeqReplay_orig(seq int16)

//export SsSeqStop_orig
func SsSeqStop_orig(seq int16)

//export SsSeqClose_orig
func SsSeqClose_orig(seq int16)

//export SsSeqSetVol_orig
func SsSeqSetVol_orig(seq int16, l int16, r int16)

//export SsSetMVol_orig
func SsSetMVol_orig(l int16, r int16)

//export SsIsEos_orig
func SsIsEos_orig(seq int16, track int16) int16

// The bank and the song, breakout's, which mkdisc places on the twin's
// disc from the example's assets.
var (
	sndVHFile  = disc.Register("STD0.VH", "../../../examples/breakout/assets/STD0.VH")
	sndVBFile  = disc.Register("STD0.VB", "../../../examples/breakout/assets/STD0.VB")
	sndSEQFile = disc.Register("GOGO.SEQ", "../../../examples/breakout/assets/GOGO.SEQ")

	sndVH  [61440 / 4]uint32
	sndVB  [410624 / 4]uint32
	sndSEQ [18432 / 4]uint32
)

// sndRead reads one of the files into its buffer, and reports whether
// the read went through.
func sndRead(mode int32, name string, buf *uint32) bool {
	var path [16]byte
	copy(path[:], "\\"+name+";1\x00")
	var res [8]uint8
	var r int32
	if mode == 0 {
		if CdReadFile_orig(&path[0], buf, 0) == 0 {
			return false
		}
		for {
			if r = CdReadSync_orig(1, &res[0]); r <= 0 {
				break
			}
			VSync_orig(0)
		}
		return r == 0
	}
	if yaroze.CdReadFile(&path[0], buf, 0) == 0 {
		return false
	}
	for {
		if r = yaroze.CdReadSync(1, &res[0]); r <= 0 {
			break
		}
		yaroze.VSync(0)
	}
	return r == 0
}

// sndScript plays the song and prints the voices as it goes.
func sndScript(mode int32) {
	// The files' sectors, which keeps their declarations in the program
	// for mkdisc to find.
	print("sndfiles ", sndVHFile.Sector(), " ", sndVBFile.Sector(), " ", sndSEQFile.Sector(), "\n")
	// The sequencer's first call starts it under the Go library, which
	// resets the processor; the resident's startup did that before the
	// drive was initialized, so the drive is initialized again after it
	// under both, which leaves the processor as the startup's order did.
	if mode == 0 {
		SsSetMVol_orig(127, 127)
		CdInit_orig()
	} else {
		yaroze.SsSetMVol(127, 127)
		CdInit()
	}
	ok := sndRead(mode, "STD0.VH", &sndVH[0]) && sndRead(mode, "STD0.VB", &sndVB[0]) && sndRead(mode, "GOGO.SEQ", &sndSEQ[0])
	print("sndread ", ok, "\n")
	if !ok {
		return
	}
	vh := (*uint8)(unsafe.Pointer(&sndVH[0]))
	vb := (*uint8)(unsafe.Pointer(&sndVB[0]))
	waits := [...]int{2, 10, 30, 60, 120, 240}
	if mode == 0 {
		SsSetMVol_orig(127, 127)
		vab := SsVabTransfer_orig(vh, vb, -1, 1)
		print("vab ", vab, " ", sndProgSum(), "\n")
		seq := SsSeqOpen_orig(&sndSEQ[0], vab)
		print("seq ", seq, "\n")
		SsSeqSetVol_orig(seq, 127, 127)
		SsSeqPlay_orig(seq, 1, 0)
		for _, n := range waits {
			for range n {
				VSync_orig(0)
			}
			printSPU("play "+itoa(n), SsIsEos_orig(seq, 0))
		}
		SsSeqPause_orig(seq)
		for range 10 {
			VSync_orig(0)
		}
		printSPU("pause", SsIsEos_orig(seq, 0))
		SsSeqReplay_orig(seq)
		for range 60 {
			VSync_orig(0)
		}
		printSPU("replay", SsIsEos_orig(seq, 0))
		SsSeqStop_orig(seq)
		for range 3 {
			VSync_orig(0)
		}
		printSPU("stop", SsIsEos_orig(seq, 0))
		SsSeqClose_orig(seq)
		SsVabClose_orig(vab)
		for range 3 {
			VSync_orig(0)
		}
		printSPU("close", 0)
		return
	}
	yaroze.SsSetMVol(127, 127)
	vab := yaroze.SsVabTransfer(vh, vb, -1, 1)
	print("vab ", vab, " ", sndProgSum(), "\n")
	seq := yaroze.SsSeqOpen(&sndSEQ[0], vab)
	print("seq ", seq, "\n")
	yaroze.SsSeqSetVol(seq, 127, 127)
	yaroze.SsSeqPlay(seq, 1, 0)
	for _, n := range waits {
		for range n {
			yaroze.VSync(0)
		}
		printSPU("play "+itoa(n), yaroze.SsIsEos(seq, 0))
	}
	yaroze.SsSeqPause(seq)
	for range 10 {
		yaroze.VSync(0)
	}
	printSPU("pause", yaroze.SsIsEos(seq, 0))
	yaroze.SsSeqReplay(seq)
	for range 60 {
		yaroze.VSync(0)
	}
	printSPU("replay", yaroze.SsIsEos(seq, 0))
	yaroze.SsSeqStop(seq)
	for range 3 {
		yaroze.VSync(0)
	}
	printSPU("stop", yaroze.SsIsEos(seq, 0))
	yaroze.SsSeqClose(seq)
	yaroze.SsVabClose(vab)
	for range 3 {
		yaroze.VSync(0)
	}
	printSPU("close", 0)
}

// sndProgSum sums the bank's program table, which the open fills with
// the programs' indexes and the samples' addresses.
func sndProgSum() uint32 {
	sum := uint32(0)
	for i := 8; i < 8+128*4; i++ {
		sum += sndVH[i]
	}
	return sum
}

// spuReg reads one of the sound processor's registers, by offset from
// 0x1f801c00.
func spuReg(off uintptr) uint16 {
	return (*volatile.Register16)(unsafe.Pointer(uintptr(0x1f801c00) + off)).Get()
}

// printSPU prints the voices' volumes, pitches, addresses and envelopes,
// the registers the library writes, and the common ones, under a name.
func printSPU(name string, eos int16) {
	print("spu ", name, " eos ", eos, "\n")
	for v := uintptr(0); v < 24; v++ {
		print("v", int(v), " ")
		for i := uintptr(0); i < 12; i += 2 {
			print(hex16(spuReg(v*16+i)), " ")
		}
		print("\n")
	}
	print("common ", hex16(spuReg(0x180)), " ", hex16(spuReg(0x182)), " ", hex16(spuReg(0x184)), " ", hex16(spuReg(0x186)),
		" ", hex16(spuReg(0x198)), " ", hex16(spuReg(0x19a)), " ", hex16(spuReg(0x1a6)), " ", hex16(spuReg(0x1aa)), "\n")
}

func hex16(v uint16) string { return hex(uint8(v>>8)) + hex(uint8(v)) }
