//go:build yaroze

package main

import (
	"go.chrisrx.dev/psx/disc"
	"go.chrisrx.dev/psx/yaroze"
)

// CD audio: one of cdda's tracks is recorded on the twin's disc, and the
// script plays it under each library, asks what is playing, and stops it,
// printing what CdPlay says each time and the volumes the serial calls
// set.

//export CdPlay_orig
func CdPlay_orig(mode int32, tracks *int32, offset int32) int32

//export SsSetSerialAttr_orig
func SsSetSerialAttr_orig(source uint8, attr uint8, on uint8)

//export SsSetSerialVol_orig
func SsSetSerialVol_orig(source uint8, l int16, r int16)

//export SsGetSerialAttr_orig
func SsGetSerialAttr_orig(source uint8, attr uint8) uint8

// The Go library's, by the names the bindings lack.

//export CdPlay
func CdPlay(mode int32, tracks *int32, offset int32) int32

//export SsSetSerialAttr
func SsSetSerialAttr(source uint8, attr uint8, on uint8)

//export SsSetSerialVol
func SsSetSerialVol(source uint8, l int16, r int16)

//export SsGetSerialAttr
func SsGetSerialAttr(source uint8, attr uint8) uint8

var (
	cdTrack  = disc.Track("../../../examples/cdda/audio/TRACK1.WAV")
	cdTracks [2]int32
)

// cdPlayScript plays the track, asks after it, and stops it.
func cdPlayScript(mode int32) {
	cdTracks[0] = cdTrack.TrackNumber()
	print("cdtrack ", cdTracks[0], "\n")
	if mode == 0 {
		SsSetSerialAttr_orig(0, 0, 1)
		SsSetSerialVol_orig(0, 100, 50)
		print("serial ", SsGetSerialAttr_orig(0, 0), " ", SsGetSerialAttr_orig(0, 1), " ", hex16(spuReg(0x1b0)), " ", hex16(spuReg(0x1b2)), "\n")
		print("cdplay ", CdPlay_orig(1, &cdTracks[0], 0), "\n")
		for range 30 {
			VSync_orig(0)
		}
		print("cdplaying ", CdPlay_orig(3, &cdTracks[0], 0), "\n")
		print("cdstop ", CdPlay_orig(0, &cdTracks[0], 0), "\n")
		for range 5 {
			VSync_orig(0)
		}
		print("cdstopped ", CdPlay_orig(3, &cdTracks[0], 0), "\n")
		return
	}
	SsSetSerialAttr(0, 0, 1)
	SsSetSerialVol(0, 100, 50)
	print("serial ", SsGetSerialAttr(0, 0), " ", SsGetSerialAttr(0, 1), " ", hex16(spuReg(0x1b0)), " ", hex16(spuReg(0x1b2)), "\n")
	print("cdplay ", CdPlay(1, &cdTracks[0], 0), "\n")
	for range 30 {
		yaroze.VSync(0)
	}
	print("cdplaying ", CdPlay(3, &cdTracks[0], 0), "\n")
	print("cdstop ", CdPlay(0, &cdTracks[0], 0), "\n")
	for range 5 {
		yaroze.VSync(0)
	}
	print("cdstopped ", CdPlay(3, &cdTracks[0], 0), "\n")
}
