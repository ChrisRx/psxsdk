// Command gen writes the cdda example's three audio tracks, each three
// seconds of 44.1 kHz 16-bit stereo, the only thing a CD's audio holds: a
// rising arpeggio, a falling one in a square wave, and a beat.
package main

import (
	"encoding/binary"
	"log"
	"math"
	"os"
	"path/filepath"
)

const (
	rate    = 44100
	seconds = 3
)

func main() {
	dir := os.Args[1]
	write(filepath.Join(dir, "TRACK1.WAV"), func(t float64) (float64, float64) {
		notes := []float64{261.63, 329.63, 392.00, 523.25}
		f := notes[int(t*4)%len(notes)]
		v := 0.3 * math.Sin(2*math.Pi*f*t)
		return v, v * 0.6
	})
	write(filepath.Join(dir, "TRACK2.WAV"), func(t float64) (float64, float64) {
		notes := []float64{659.25, 587.33, 493.88, 440.00, 392.00, 329.63}
		f := notes[int(t*6)%len(notes)]
		v := 0.15
		if math.Sin(2*math.Pi*f*t) < 0 {
			v = -v
		}
		return v * 0.6, v
	})
	write(filepath.Join(dir, "TRACK3.WAV"), func(t float64) (float64, float64) {
		beat := math.Mod(t, 0.5)
		kick := 0.5 * math.Exp(-beat*20) * math.Sin(2*math.Pi*(60+200*math.Exp(-beat*30))*beat)
		bass := 0.2 * math.Sin(2*math.Pi*[]float64{55, 55, 65.4, 49}[int(t*2)%4]*t)
		return kick + bass, kick + bass
	})
}

func write(path string, f func(t float64) (float64, float64)) {
	n := rate * seconds
	b := []byte("RIFF")
	b = binary.LittleEndian.AppendUint32(b, uint32(36+4*n))
	b = append(b, "WAVEfmt "...)
	for _, v := range []any{uint32(16), uint16(1), uint16(2), uint32(rate), uint32(4 * rate), uint16(4), uint16(16)} {
		switch v := v.(type) {
		case uint16:
			b = binary.LittleEndian.AppendUint16(b, v)
		case uint32:
			b = binary.LittleEndian.AppendUint32(b, v)
		}
	}
	b = append(b, "data"...)
	b = binary.LittleEndian.AppendUint32(b, uint32(4*n))
	for i := range n {
		l, r := f(float64(i) / rate)
		// A short fade at each end, so a track starts and stops cleanly.
		fade := min(1, float64(i)/2000, float64(n-i)/2000)
		b = binary.LittleEndian.AppendUint16(b, uint16(int16(l*fade*32767)))
		b = binary.LittleEndian.AppendUint16(b, uint16(int16(r*fade*32767)))
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		log.Fatal(err)
	}
}
