//go:build js && wasm

// Command web runs the emulator in a browser, for the site built into docs/
// (task site). It runs in the page's worker, static/worker.js, and exposes
// a global psx object to it:
//
//	psx.boot(bios, cue, files)  boots a BIOS, a Uint8Array, with the disc
//	                            whose cue sheet is named cue in files, an
//	                            object of Uint8Arrays by file name. It
//	                            returns an error message, or null.
//	psx.runFrame(pixels)        runs to the next vertical blank, copies the
//	                            display as RGBA into pixels, a
//	                            Uint8ClampedArray, if it is large enough,
//	                            and returns {width, height}, or {error}.
//	psx.setButtons(buttons)     sets the buttons held on the pad in port 1,
//	                            as emu's Button bits.
//	psx.takeTTY()               returns what the BIOS has printed since the
//	                            last call.
//	psx.setAudio(on)            turns the sound on or off, now and for
//	                            machines booted later.
//	psx.takeAudio()             returns the sound made since the last
//	                            call, as a Uint8Array of interleaved left
//	                            and right 16-bit little-endian samples at
//	                            44.1kHz, or null if there is none.
package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"syscall/js"
	"testing/fstest"

	"go.chrisrx.dev/psx/emu"
)

// frameBudget bounds the instructions runFrame runs looking for a vertical
// blank, which comes every 560,000 cycles or so.
const frameBudget = 4_000_000

var (
	machine *emu.Machine
	disc    *emu.Disc
	frame   *image.RGBA
	tty     bytes.Buffer

	audioOn bool
	sound   []int16
	soundJS []byte
)

func main() {
	js.Global().Set("psx", js.ValueOf(map[string]any{
		"boot":       js.FuncOf(boot),
		"runFrame":   js.FuncOf(runFrame),
		"setButtons": js.FuncOf(setButtons),
		"takeTTY":    js.FuncOf(takeTTY),
		"setAudio":   js.FuncOf(setAudio),
		"takeAudio":  js.FuncOf(takeAudio),
	}))
	select {}
}

func boot(_ js.Value, args []js.Value) any {
	if len(args) != 3 {
		return "boot takes a BIOS, a cue sheet name and the disc's files"
	}
	bios := bytesFromJS(args[0])
	fsys := fstest.MapFS{}
	files := args[2]
	names := js.Global().Get("Object").Call("keys", files)
	for i := range names.Length() {
		name := names.Index(i).String()
		fsys[name] = &fstest.MapFile{Data: bytesFromJS(files.Get(name))}
	}

	m, err := emu.New(bios)
	if err != nil {
		return err.Error()
	}
	d, err := emu.OpenDiscFS(fsys, args[1].String())
	if err != nil {
		return err.Error()
	}
	if disc != nil {
		disc.Close()
	}
	tty.Reset()
	m.TTY = &tty
	m.SetAudio(audioOn)
	m.InsertDisc(d)
	machine, disc = m, d
	return nil
}

func runFrame(_ js.Value, args []js.Value) any {
	if machine == nil {
		return map[string]any{"error": "nothing is running"}
	}
	target := machine.Frames() + 1
	if _, err := machine.RunFrame(frameBudget); err != nil {
		machine = nil
		return map[string]any{"error": err.Error()}
	}
	if machine.Frames() < target {
		return map[string]any{"error": fmt.Sprintf("no vertical blank in %d instructions", frameBudget)}
	}

	frame = machine.DisplayInto(frame)
	w, h := frame.Rect.Dx(), frame.Rect.Dy()
	if len(args) > 0 && args[0].Truthy() && args[0].Length() >= len(frame.Pix) {
		js.CopyBytesToJS(args[0], frame.Pix)
	}
	return map[string]any{"width": w, "height": h}
}

func setButtons(_ js.Value, args []js.Value) any {
	if machine == nil || len(args) == 0 {
		return nil
	}
	pad := machine.Pad(1)
	pad.Release(0xffff)
	pad.Press(uint16(args[0].Int()))
	return nil
}

func takeTTY(js.Value, []js.Value) any {
	s := tty.String()
	tty.Reset()
	return s
}

func bytesFromJS(v js.Value) []byte {
	b := make([]byte, v.Length())
	js.CopyBytesToGo(b, v)
	return b
}

func setAudio(_ js.Value, args []js.Value) any {
	audioOn = len(args) > 0 && args[0].Truthy()
	if machine != nil {
		machine.SetAudio(audioOn)
	}
	return nil
}

func takeAudio(js.Value, []js.Value) any {
	if machine == nil {
		return nil
	}
	sound = machine.Audio(sound[:0])
	if len(sound) == 0 {
		return nil
	}
	soundJS = soundJS[:0]
	for _, v := range sound {
		soundJS = binary.LittleEndian.AppendUint16(soundJS, uint16(v))
	}
	out := js.Global().Get("Uint8Array").New(len(soundJS))
	js.CopyBytesToJS(out, soundJS)
	return out
}
