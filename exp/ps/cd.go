package ps

// CD audio: tracks of music recorded on the disc after the program,
// played by the drive while the program goes on and mixed by the sound
// processor with whatever the program plays itself. A track's number is
// what disc.Track gives a program; the data track is 1.

// SetDiscVolume turns the disc's audio into the sound processor's mix
// at a volume, left and right, 0 to 127. The main volume, SetVolume,
// has to be up as well.
func SetDiscVolume(left, right int) { discVolume(left, right) }

// PlayTracks plays the tracks from the one at index from on, once
// through, going on with the next when the drive has ended one. The
// list is kept, up to 15 tracks.
func PlayTracks(tracks []int, from int) { cdPlay(1, tracks, from) }

// LoopTracks is PlayTracks round and round.
func LoopTracks(tracks []int, from int) { cdPlay(2, tracks, from) }

// StopTracks stops the drive playing.
func StopTracks() { cdPlay(0, nil, 0) }

// PlayingTrack is the index in the list given to PlayTracks or
// LoopTracks of the track playing, or -1 for none. Asking is also what
// starts the next track when the drive has ended one, so a program asks
// once a frame.
func PlayingTrack() int { return cdPlay(3, nil, 0) }
