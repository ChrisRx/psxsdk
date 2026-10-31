// CD audio: tracks of music recorded on the disc after the program, played
// by the drive while the program goes on, with a level meter of what the
// SPU is mixing.
//
// examples/cdda on the exp/ps API: the tracks are a slice of numbers that
// ps.PlayTracks and ps.LoopTracks take, ps.PlayingTrack says which is
// playing, and ps.SetDiscVolume mixes the disc in.
//
// # Controls
//
// Left and right choose a track. Cross plays from it to the last, triangle
// round and round, and circle stops.
package main

import (
	"go.chrisrx.dev/psx/disc"
	"go.chrisrx.dev/psx/exp/ps"
)

// The tracks, recorded on the disc in this order after the data track.
var (
	track1 = disc.Track("../../../examples/cdda/audio/TRACK1.WAV")
	track2 = disc.Track("../../../examples/cdda/audio/TRACK2.WAV")
	track3 = disc.Track("../../../examples/cdda/audio/TRACK3.WAV")
)

var trackNames = [3]string{"RISING", "FALLING", "BEAT"}

var (
	screen = ps.Screen{
		Background: ps.Color{R: 10, G: 40, B: 40},
		Depth:      1,
	}
	pad ps.Controller

	// tracks is the list the tracks are played from, their numbers.
	tracks [3]int

	// chosen is the track the list plays from, and playing which is
	// playing, -1 for none: both for a test to read.
	chosen  int
	playing = -1

	bars [3]ps.Box

	text ps.Text
)

func main() {
	screen.Open()
	text = screen.Font(16, 16, 288, 200, 512)
	tracks = [3]int{
		int(track1.TrackNumber()),
		int(track2.TrackNumber()),
		int(track3.TrackNumber()),
	}

	// The disc's audio is mixed into the SPU's output at full volume.
	ps.SetVolume(127, 127)
	ps.SetDiscVolume(127, 127)

	for i := range bars {
		bars[i] = ps.Box{
			W:     60,
			H:     8,
			Color: ps.Color{R: 60, G: 120, B: 120},
		}
		bars[i].MoveTo(40+i*90, 200)
	}

	for frame := range screen.Frames() {
		pad.Update()
		switch {
		case pad.Pressed(ps.Left) && chosen > 0:
			chosen--
		case pad.Pressed(ps.Right) && chosen < 2:
			chosen++
		case pad.Pressed(ps.Cross):
			ps.PlayTracks(tracks[:], chosen)
		case pad.Pressed(ps.Triangle):
			ps.LoopTracks(tracks[:], chosen)
		case pad.Pressed(ps.Circle):
			ps.StopTracks()
		}
		playing = ps.PlayingTrack()

		text.Print("~c099CD AUDIO~c999\n\n")
		for i := range tracks {
			if i == chosen {
				text.Print("> ")
			} else {
				text.Print("  ")
			}
			text.Print("TRACK ").Int(tracks[i]).Print("  ").Print(trackNames[i])
			if i == playing {
				text.Print("  ~c990PLAYING~c999")
			}
			text.Print("\n")
		}
		text.Print("\nCROSS PLAYS FROM THE CHOSEN TRACK\nTRIANGLE PLAYS ROUND AND ROUND\nCIRCLE STOPS\n")

		// A bar for each track, lit while it plays and pulsing with the
		// frames.
		for i := range bars {
			b := &bars[i]
			r := b.Rect()
			if i == playing {
				pulse := frame.Number() % 32 * 4
				b.Color = ps.Color{R: 255, G: uint8(160 + pulse/2), B: 40}
				r.H = 8 + pulse/8
			} else {
				b.Color = ps.Color{R: 60, G: 120, B: 120}
				r.H = 8
			}
			r.Y = 208 - r.H
			b.SetRect(r)
			b.Draw(frame, 0)
		}
	}
}
