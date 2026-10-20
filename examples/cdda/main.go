// CD audio: tracks of music recorded on the disc after the program, played
// by the drive while the program goes on, with a level meter of what the
// SPU is mixing.
//
// A port of the SCEE tutorial's chapter 3, example 2
// (docs/manuals/tutorial-cd.md), which played the tracks of whatever music
// CD was in the drive: the Net Yaroze could only read its own boot disc, so
// a program's own music had to come from elsewhere. A disc built here can
// carry its own, so the tracks are the program's: three short ones that
// gen writes, declared with disc.Track, which records each as a track of
// CD audio after the data track and tells the program its number.
//
// CD audio costs the program nothing: the drive reads it and sends it
// straight to the SPU, which mixes it with whatever the program plays
// itself, as SsSetSerialAttr and SsSetSerialVol set up. CdPlay plays a list
// of tracks from one of them on, once or round and round, and says which is
// playing. Sound from the disc is CD quality, 44.1 kHz stereo, and takes
// 176 KB of disc a second, which is why games kept their music on it and
// their sound effects in the SPU's memory.
//
// Differences from the tutorial: the tracks are on the program's own disc,
// cross plays them all in turn and triangle round and round, where the
// tutorial played one at a time, and the program shows which is playing.
//
// # Controls
//
// Left and right choose a track. Cross plays from it to the last, triangle
// round and round, and circle stops.
package main

//go:generate go run ./gen audio

import (
	"go.chrisrx.dev/psx/disc"
	"go.chrisrx.dev/psx/yaroze"
)

// The tracks, recorded on the disc in this order after the data track.
var (
	track1 = disc.Track("audio/TRACK1.WAV")
	track2 = disc.Track("audio/TRACK2.WAV")
	track3 = disc.Track("audio/TRACK3.WAV")
)

var trackNames = [3]string{"RISING", "FALLING", "BEAT"}

var (
	screen = yaroze.Screen{Background: yaroze.CVECTOR{R: 10, G: 40, B: 40}, Depth: 1}
	pad    yaroze.Pad

	// tracks is the list CdPlay plays from, the tracks' numbers ending
	// with a 0, as it takes them.
	tracks [4]int32

	// chosen is the track the list plays from, and playing which is
	// playing, as CdPlay reports, -1 for none: both for a test to read.
	chosen  int32
	playing int32 = -1

	bars [3]yaroze.GsBOXF

	text    yaroze.Buffer
	textBuf [512]byte
)

func main() {
	screen.Open()
	screen.Font(16, 16, 288, 200, 512)
	text.Init(textBuf[:])
	tracks = [4]int32{track1.TrackNumber(), track2.TrackNumber(), track3.TrackNumber(), 0}

	// The disc's audio is mixed into the SPU's output at full volume.
	yaroze.SsSetMVol(127, 127)
	yaroze.SsSetSerialAttr(yaroze.SS_CD, yaroze.SS_MIX, yaroze.SS_SON)
	yaroze.SsSetSerialVol(yaroze.SS_CD, 127, 127)

	for i := range bars {
		bars[i] = yaroze.GsBOXF{X: 40 + int16(i)*90, Y: 200, W: 60, H: 8, R: 60, G: 120, B: 120}
	}

	for ot := range screen.Frames() {
		pad.Update()
		switch {
		case pad.Pressed(yaroze.PADLleft) && chosen > 0:
			chosen--
		case pad.Pressed(yaroze.PADLright) && chosen < 2:
			chosen++
		case pad.Pressed(yaroze.PADRdown):
			yaroze.CdPlay(1, &tracks[0], chosen)
		case pad.Pressed(yaroze.PADRup):
			yaroze.CdPlay(2, &tracks[0], chosen)
		case pad.Pressed(yaroze.PADRright):
			yaroze.CdPlay(0, &tracks[0], chosen)
		}
		playing = yaroze.CdPlay(3, &tracks[0], 0)

		text.Reset()
		text.WriteString("~c099CD AUDIO~c999\n\n")
		for i := range int32(3) {
			if i == chosen {
				text.WriteString("> ")
			} else {
				text.WriteString("  ")
			}
			text.WriteString("TRACK ").WriteInt(tracks[i]).WriteString("  ").WriteString(trackNames[i])
			if i == playing {
				text.WriteString("  ~c990PLAYING~c999")
			}
			text.WriteString("\n")
		}
		text.WriteString("\nCROSS PLAYS FROM THE CHOSEN TRACK\nTRIANGLE PLAYS ROUND AND ROUND\nCIRCLE STOPS\n")
		text.Print()

		// A bar for each track, lit while it plays and pulsing with the
		// frames.
		for i := range bars {
			b := &bars[i]
			if int32(i) == playing {
				pulse := uint8(screen.Frame() % 32 * 4)
				b.R, b.G, b.B = 255, 160+pulse/2, 40
				b.H = 8 + uint16(pulse/8)
			} else {
				b.R, b.G, b.B = 60, 120, 120
				b.H = 8
			}
			b.Y = 208 - int16(b.H)
			b.Sort(ot, 0)
		}
	}
}
