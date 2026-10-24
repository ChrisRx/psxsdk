package psxdisc

import (
	"bytes"
	"encoding/binary"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.chrisrx.dev/psx/format/iso9660"
)

// dataTrack lays out a filesystem of a known size to put audio behind.
func dataTrack(t *testing.T, files int) *iso9660.Writer {
	t.Helper()
	root := &iso9660.Dir{}
	for i := range files {
		root.Entries = append(root.Entries, &iso9660.Entry{
			Name: iso9660.FileIdentifier(string(rune('A'+i)) + ".dat"),
			Size: 10,
			Open: func() (io.ReadCloser, error) {
				return io.NopCloser(strings.NewReader("0123456789")), nil
			},
		})
	}
	w, err := iso9660.NewWriter(root, iso9660.Options{VolumeID: "TEST"})
	if err != nil {
		t.Fatal(err)
	}
	return w
}

// samples is a recognizable run of 16-bit stereo frames.
func samples(frames int) []byte {
	b := make([]byte, frames*audioBytesPerSample)
	for i := range frames {
		binary.LittleEndian.PutUint16(b[i*4:], uint16(i))
		binary.LittleEndian.PutUint16(b[i*4+2:], uint16(0x8000-i))
	}
	return b
}

func audioFromBytes(name string, b []byte) *AudioTrack {
	return &AudioTrack{
		Name: name,
		Size: int64(len(b)),
		Open: func() (io.ReadCloser, error) { return io.NopCloser(bytes.NewReader(b)), nil },
	}
}

// wavFile writes a WAV file of the given samples, with a chunk before the
// data one so that the parser has to walk rather than assume a layout.
func wavFile(t *testing.T, dir string, pcm []byte, rate uint32, channels, bits uint16) string {
	t.Helper()
	var b bytes.Buffer

	fmtChunk := make([]byte, 16)
	binary.LittleEndian.PutUint16(fmtChunk[0:], 1) // uncompressed PCM
	binary.LittleEndian.PutUint16(fmtChunk[2:], channels)
	binary.LittleEndian.PutUint32(fmtChunk[4:], rate)
	binary.LittleEndian.PutUint32(fmtChunk[8:], rate*uint32(channels)*uint32(bits)/8)
	binary.LittleEndian.PutUint16(fmtChunk[12:], channels*bits/8)
	binary.LittleEndian.PutUint16(fmtChunk[14:], bits)

	// An odd length chunk is followed by a padding byte, which a reader
	// that forgets it will drift over.
	extra := []byte("psxsdk")

	body := new(bytes.Buffer)
	writeChunk := func(id string, data []byte) {
		body.WriteString(id)
		binary.Write(body, binary.LittleEndian, uint32(len(data)))
		body.Write(data)
		if len(data)%2 != 0 {
			body.WriteByte(0)
		}
	}
	writeChunk("fmt ", fmtChunk)
	writeChunk("LIST", extra[:5])
	writeChunk("data", pcm)

	b.WriteString("RIFF")
	binary.Write(&b, binary.LittleEndian, uint32(4+body.Len()))
	b.WriteString("WAVE")
	b.Write(body.Bytes())

	path := filepath.Join(dir, "track.wav")
	if err := os.WriteFile(path, b.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestAudioTrackFileWAV(t *testing.T) {
	pcm := samples(1000)
	path := wavFile(t, t.TempDir(), pcm, AudioSampleRate, AudioChannels, AudioBitsPerSample)

	track, err := AudioTrackFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if track.Size != int64(len(pcm)) {
		t.Errorf("track is %d bytes, want %d", track.Size, len(pcm))
	}

	rc, err := track.Open()
	if err != nil {
		t.Fatal(err)
	}
	defer rc.Close()
	got, err := io.ReadAll(rc)
	if err != nil {
		t.Fatal(err)
	}
	// Open seeks to the data chunk, so the samples arrive without the
	// header in front of them.
	if !bytes.Equal(got[:len(pcm)], pcm) {
		t.Error("the samples read back do not match the ones written")
	}
}

// A file in some other format cannot be recorded as it stands, and saying so
// beats writing a track of noise.
func TestAudioTrackFileRejectsWrongFormat(t *testing.T) {
	dir := t.TempDir()
	tests := []struct {
		name           string
		rate           uint32
		channels, bits uint16
		wantInMessage  string
	}{
		{"wrong rate", 22050, 2, 16, "22050 Hz"},
		{"mono", AudioSampleRate, 1, 16, "1 channel"},
		{"8 bit", AudioSampleRate, 2, 8, "8 bit"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := wavFile(t, dir, samples(100), tt.rate, tt.channels, tt.bits)
			_, err := AudioTrackFile(path)
			if err == nil {
				t.Fatal("the file was accepted")
			}
			if !strings.Contains(err.Error(), tt.wantInMessage) {
				t.Errorf("error does not mention %q: %v", tt.wantInMessage, err)
			}
		})
	}
}

// Anything that is not a WAV is taken to be bare samples, which is what the
// tools that produce PlayStation audio emit.
func TestAudioTrackFileRawPCM(t *testing.T) {
	pcm := samples(500)
	path := filepath.Join(t.TempDir(), "track.pcm")
	if err := os.WriteFile(path, pcm, 0o644); err != nil {
		t.Fatal(err)
	}

	track, err := AudioTrackFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if track.Size != int64(len(pcm)) {
		t.Errorf("track is %d bytes, want %d", track.Size, len(pcm))
	}

	// Half a frame is not audio, and recording it would shift one channel
	// against the other for the rest of the track.
	short := filepath.Join(t.TempDir(), "odd.pcm")
	if err := os.WriteFile(short, pcm[:len(pcm)-1], 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := AudioTrackFile(short); err == nil {
		t.Error("a file that is not a whole number of frames was accepted")
	}
}

func TestImageLayout(t *testing.T) {
	data := dataTrack(t, 3)
	first := samples(AudioSectorSize / audioBytesPerSample * 2) // two whole sectors
	second := samples(10)                                       // less than one

	img := &Image{
		Data:  data,
		Audio: []*AudioTrack{audioFromBytes("first.wav", first), audioFromBytes("second.wav", second)},
	}

	var buf bytes.Buffer
	l, err := img.Write(&buf)
	if err != nil {
		t.Fatal(err)
	}

	dataSectors := data.TotalBlocks()
	want := []Track{
		{Number: 1, Start: 0, Sectors: dataSectors},
		{Number: 2, Audio: true, Name: "first.wav", Pregap: dataSectors, Start: dataSectors + StandardPregap, Sectors: 2},
		{Number: 3, Audio: true, Name: "second.wav", Pregap: dataSectors + StandardPregap + 2, Start: dataSectors + 2*StandardPregap + 2, Sectors: 1},
	}
	if len(l.Tracks) != len(want) {
		t.Fatalf("image has %d tracks, want %d", len(l.Tracks), len(want))
	}
	for i := range want {
		if l.Tracks[i] != want[i] {
			t.Errorf("track %d = %+v, want %+v", i+1, l.Tracks[i], want[i])
		}
	}

	// Every track has to start where the file says it does, or a player
	// seeking to it lands in the middle of something else.
	if got := uint32(buf.Len()) / AudioSectorSize; got != l.Sectors() {
		t.Errorf("image is %d sectors but the layout says %d", got, l.Sectors())
	}
	if uint32(buf.Len())%AudioSectorSize != 0 {
		t.Errorf("image is %d bytes, which is not a whole number of sectors", buf.Len())
	}

	image := buf.Bytes()
	at := func(sector uint32) []byte {
		return image[sector*AudioSectorSize : (sector+1)*AudioSectorSize]
	}

	// The pause is silence, and the samples begin immediately after it.
	for s := dataSectors; s < dataSectors+StandardPregap; s++ {
		if !bytes.Equal(at(s), make([]byte, AudioSectorSize)) {
			t.Fatalf("sector %d of the pause is not silent", s)
		}
	}
	if !bytes.Equal(at(l.Tracks[1].Start), first[:AudioSectorSize]) {
		t.Error("the first audio track does not start with its samples")
	}

	// A track shorter than a sector is padded, not truncated or run into
	// the next track.
	last := at(l.Tracks[2].Start)
	if !bytes.Equal(last[:len(second)], second) {
		t.Error("the second audio track does not start with its samples")
	}
	if !bytes.Equal(last[len(second):], make([]byte, AudioSectorSize-len(second))) {
		t.Error("the rest of its sector is not silence")
	}
}

func TestCueSheet(t *testing.T) {
	data := dataTrack(t, 1)
	img := &Image{
		Data:  data,
		Audio: []*AudioTrack{audioFromBytes("a", samples(AudioSectorSize)), audioFromBytes("b", samples(AudioSectorSize))},
	}
	l, err := img.Write(io.Discard)
	if err != nil {
		t.Fatal(err)
	}

	var cue strings.Builder
	if err := WriteCueSheet(&cue, "/somewhere/else/game.bin", l); err != nil {
		t.Fatal(err)
	}
	got := cue.String()

	// The file is named without its path: a cue sheet is read from beside
	// the image it describes.
	if !strings.HasPrefix(got, "FILE \"game.bin\" BINARY\n") {
		t.Errorf("cue sheet starts %q", strings.SplitN(got, "\n", 2)[0])
	}
	for _, want := range []string{
		"  TRACK 01 MODE2/2352\n",
		"    INDEX 01 00:00:00\n",
		"  TRACK 02 AUDIO\n",
		"  TRACK 03 AUDIO\n",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("cue sheet has no %q:\n%s", want, got)
		}
	}

	// The recorded pause is index zero of the track it belongs to, which
	// every audio track has.
	if n := strings.Count(got, "INDEX 00"); n != 2 {
		t.Errorf("cue sheet has %d index zero lines, want 2:\n%s", n, got)
	}

	// A data track with no audio is the single track cue sheet a disc
	// holding only a filesystem needs.
	plain, err := (&Image{Data: dataTrack(t, 1)}).Write(io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	cue.Reset()
	if err := WriteCueSheet(&cue, "game.bin", plain); err != nil {
		t.Fatal(err)
	}
	if got := cue.String(); got != "FILE \"game.bin\" BINARY\n  TRACK 01 MODE2/2352\n    INDEX 01 00:00:00\n" {
		t.Errorf("single track cue sheet is:\n%s", got)
	}
}

func TestMSF(t *testing.T) {
	// 75 frames to the second, counted from the start of the file rather
	// than from the start of the disc.
	tests := []struct {
		sector uint32
		want   string
	}{
		{0, "00:00:00"},
		{74, "00:00:74"},
		{75, "00:01:00"},
		{75 * 60, "01:00:00"},
		{10000, "02:13:25"},
	}
	for _, tt := range tests {
		if got := msf(tt.sector); got != tt.want {
			t.Errorf("msf(%d) = %s, want %s", tt.sector, got, tt.want)
		}
	}
}

func TestImageWithoutDataTrack(t *testing.T) {
	if _, err := (&Image{}).Write(io.Discard); err == nil {
		t.Error("an image with no filesystem was written")
	}
}
