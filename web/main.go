//go:build js && wasm

// Command web runs the emulator in a browser, for the site GitHub Pages
// serves (task site). It runs in the page's worker, static/worker.js, and
// exposes a global psx object to it:
//
//	psx.boot(bios, cue, files, elf)
//	                            boots a BIOS, a Uint8Array, with the disc
//	                            whose cue sheet is named cue in files, an
//	                            object of Uint8Arrays by file name. elf, if
//	                            given, is the program's ELF executable, whose
//	                            symbols name addresses in the log. It
//	                            returns an error message, or null.
//	psx.runFrame(pixels)        runs to the next vertical blank, copies the
//	                            display as RGBA into pixels, a
//	                            Uint8ClampedArray, if it is large enough,
//	                            and returns {width, height}, or {error}.
//	                            With null for pixels the display is not
//	                            read at all, and the size is the last
//	                            one's.
//	psx.runFast(ms)             runs frames as fast as they go, for up to ms
//	                            milliseconds, until the display first shows
//	                            something, without reading the display and
//	                            dropping their sound: the fast start, which
//	                            gets through the BIOS and a program's
//	                            loading without waiting on them in real
//	                            time. It returns {shown, frames}, shown
//	                            being whether the fast start is over, which
//	                            it is too after half a minute of frames,
//	                            for a program that never shows anything,
//	                            or {error}.
//	psx.setButtons(buttons)     sets the buttons held on the pad in port 1,
//	                            as emu's Button bits.
//	psx.takeOutput()            returns what the BIOS and the program have
//	                            printed since the last call, and what the
//	                            emulator has logged, in the order it came:
//	                            an array of {level, text}, where level is ""
//	                            for printed text and otherwise the log
//	                            record's level, as "ERROR" or "DEBUG".
//	psx.setLogLevel(level)      logs at level and above, one of "trace",
//	                            "debug", "info", "warn" and "error", or with
//	                            "off", stops logging, which is how a machine
//	                            starts. It holds for machines booted later.
//	psx.symbol(name)            returns the address of the program's variable
//	                            of that name, from the ELF boot was given, or
//	                            -1: the playground's way to a running
//	                            program's scene.
//	psx.read(addr, n)           returns n bytes of RAM at addr as a
//	                            Uint8Array, or null when the range is not
//	                            RAM or nothing is running.
//	psx.write(addr, bytes)      puts a Uint8Array into RAM at addr, and
//	                            returns whether it could.
//	psx.setAudio(on)            turns the sound on or off, now and for
//	                            machines booted later.
//	psx.takeAudio()             returns the sound made since the last
//	                            call, as a Uint8Array of interleaved left
//	                            and right 16-bit little-endian samples at
//	                            44.1kHz, or null if there is none.
package main

import (
	"context"
	"debug/elf"
	"encoding/binary"
	"fmt"
	"image"
	"log/slog"
	"strings"
	"syscall/js"
	"testing/fstest"
	"time"

	"go.chrisrx.dev/psx/emu"
)

// frameBudget bounds the instructions runFrame runs looking for a vertical
// blank, which comes every 560,000 cycles or so.
const frameBudget = 4_000_000

var (
	machine *emu.Machine
	disc    *emu.Disc
	frame   *image.RGBA
	syms    []emu.Symbol
	funcs   map[string]uint32 // the program's functions, by name

	out      output
	logLevel slog.LevelVar
	logging  bool

	audioOn bool
	sound   []int16
	soundJS []byte

	// fastFrames is how many frames the fast start has run.
	fastFrames int
)

// maxFastFrames is the most frames the fast start runs, half a minute of
// them.
const maxFastFrames = 30 * 60

func main() {
	js.Global().Set("psx", js.ValueOf(map[string]any{
		"boot":        js.FuncOf(boot),
		"runFrame":    js.FuncOf(runFrame),
		"runFast":     js.FuncOf(runFast),
		"setButtons":  js.FuncOf(setButtons),
		"takeOutput":  js.FuncOf(takeOutput),
		"setLogLevel": js.FuncOf(setLogLevel),
		"setAudio":    js.FuncOf(setAudio),
		"takeAudio":   js.FuncOf(takeAudio),
		"symbol":      js.FuncOf(symbol),
		"read":        js.FuncOf(readMemory),
		"write":       js.FuncOf(writeMemory),
		"setBreak":    js.FuncOf(setBreak),
		"clearBreak":  js.FuncOf(clearBreak),
		"step":        js.FuncOf(step),
		"registers":   js.FuncOf(registers),
		"hotRegion":   js.FuncOf(hotRegion),
		"patch":       js.FuncOf(patch),
	}))
	select {}
}

func boot(_ js.Value, args []js.Value) any {
	if len(args) < 3 {
		return "boot takes a BIOS, a cue sheet name and the disc's files"
	}
	syms = nil
	globals = nil
	funcs = nil
	setHotRegion(nil)
	if len(args) > 3 && args[3].Truthy() {
		elfData := bytesFromJS(args[3])
		setHotRegion(elfData)
		s, err := emu.SymbolsFromELF(elfData)
		if err != nil {
			return fmt.Sprintf("reading the program's symbols: %v", err)
		}
		syms = s
		globals, _ = globalsFromELF(elfData)
		funcs = make(map[string]uint32, len(syms))
		for _, s := range syms {
			if _, ok := funcs[s.Name]; !ok {
				funcs[s.Name] = s.Addr
			}
		}
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
	out.reset()
	m.TTY = ttyWriter{}
	m.SetSymbols(syms)
	if logging {
		m.SetLogger(slog.New(handler{}))
	}
	m.SetAudio(audioOn)
	m.InsertDisc(d)
	machine, disc = m, d
	fastFrames = 0
	hits = nil
	patched = false
	return nil
}

func runFast(_ js.Value, args []js.Value) any {
	if machine == nil {
		return map[string]any{"error": "nothing is running"}
	}
	ms := 12
	if len(args) > 0 {
		ms = args[0].Int()
	}
	deadline := time.Now().Add(time.Duration(ms) * time.Millisecond)
	n := 0
	for !machine.Shown() && fastFrames < maxFastFrames && time.Now().Before(deadline) {
		if _, err := machine.RunFrame(frameBudget); err == emu.ErrBreak {
			return withHits(map[string]any{"stopped": int(machine.CPU.PC), "frames": n})
		} else if err != nil {
			machine = nil
			return map[string]any{"error": err.Error()}
		}
		sound = machine.Audio(sound[:0])
		fastFrames++
		n++
	}
	shown := machine.Shown() || fastFrames >= maxFastFrames
	return withHits(map[string]any{"shown": shown, "frames": n})
}

func runFrame(_ js.Value, args []js.Value) any {
	if machine == nil {
		return map[string]any{"error": "nothing is running"}
	}
	target := machine.Frames() + 1
	if _, err := machine.RunFrame(frameBudget); err == emu.ErrBreak {
		return withHits(map[string]any{"stopped": int(machine.CPU.PC)})
	} else if err != nil {
		machine = nil
		return map[string]any{"error": err.Error()}
	}
	if machine.Frames() < target {
		return map[string]any{"error": fmt.Sprintf("no vertical blank in %d instructions", frameBudget)}
	}

	// A frontend behind on its frames runs them without the display.
	if len(args) > 0 && args[0].IsNull() {
		var w, h int
		if frame != nil {
			w, h = frame.Rect.Dx(), frame.Rect.Dy()
		}
		return withHits(map[string]any{"width": w, "height": h})
	}
	frame = machine.DisplayInto(frame)
	w, h := frame.Rect.Dx(), frame.Rect.Dy()
	if len(args) > 0 && args[0].Truthy() && args[0].Length() >= len(frame.Pix) {
		js.CopyBytesToJS(args[0], frame.Pix)
	}
	return withHits(map[string]any{"width": w, "height": h})
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

func takeOutput(js.Value, []js.Value) any {
	chunks := out.take()
	arr := make([]any, len(chunks))
	for i, c := range chunks {
		arr[i] = map[string]any{"level": c.level, "text": c.text}
	}
	return arr
}

func setLogLevel(_ js.Value, args []js.Value) any {
	name := "off"
	if len(args) > 0 {
		name = args[0].String()
	}
	levels := map[string]slog.Level{
		"trace": emu.LevelTrace, "debug": slog.LevelDebug, "info": slog.LevelInfo,
		"warn": slog.LevelWarn, "error": slog.LevelError,
	}
	level, ok := levels[name]
	logging = ok
	if ok {
		logLevel.Set(level)
	}
	if machine != nil {
		if logging {
			machine.SetLogger(slog.New(handler{}))
		} else {
			machine.SetLogger(nil)
		}
	}
	return nil
}

// A chunk of output: printed text, with no level, or a log record.
type chunk struct {
	level, text string
}

// output is what has been printed and logged since it was last taken,
// with a run of the same log record collapsed into one and a count, and
// with no more kept than a page can show, should logging at a low level
// flood it.
type output struct {
	chunks  []chunk
	lines   int
	dropped int
	last    string // the last log record, for collapsing repeats
	repeats int
}

const maxLines = 2000

func (o *output) reset() { *o = output{} }

func (o *output) add(c chunk) {
	if o.lines >= maxLines {
		if c.level != "" {
			o.dropped++
		}
		return
	}
	o.lines += strings.Count(c.text, "\n")
	if n := len(o.chunks); n > 0 && c.level == "" && o.chunks[n-1].level == "" {
		o.chunks[n-1].text += c.text
		return
	}
	o.chunks = append(o.chunks, c)
}

func (o *output) flushRepeats() {
	if o.repeats > 0 {
		o.add(chunk{"REPEAT", fmt.Sprintf("    (and %d times more)\n", o.repeats)})
		o.repeats = 0
	}
}

func (o *output) take() []chunk {
	o.flushRepeats()
	if o.dropped > 0 {
		o.chunks = append(o.chunks, chunk{"WARN", fmt.Sprintf("WARN %d log records left out, too many to show\n", o.dropped)})
	}
	chunks := o.chunks
	o.chunks, o.lines, o.dropped = nil, 0, 0
	return chunks
}

// ttyWriter is the machine's TTY, which adds to the output. The Net
// Yaroze library's putchar ends a line with a carriage return too, one
// character a write, which a page would show as a line break of its own,
// so carriage returns are left out.
type ttyWriter struct{}

func (ttyWriter) Write(p []byte) (int, error) {
	text := strings.ReplaceAll(string(p), "\r", "")
	if text == "" {
		return len(p), nil
	}
	out.flushRepeats()
	out.last = ""
	out.add(chunk{"", text})
	return len(p), nil
}

// handler is a slog.Handler that adds each record to the output as a line
// of its own, with its attributes on the lines under it.
type handler struct{}

func (handler) Enabled(_ context.Context, l slog.Level) bool { return l >= logLevel.Level() }
func (h handler) WithAttrs([]slog.Attr) slog.Handler         { return h }
func (h handler) WithGroup(string) slog.Handler              { return h }

func (handler) Handle(_ context.Context, r slog.Record) error {
	level := r.Level.String()
	if r.Level == emu.LevelTrace {
		level = "TRACE"
	}
	var b strings.Builder
	b.WriteString(level)
	b.WriteByte(' ')
	b.WriteString(r.Message)
	b.WriteByte('\n')
	r.Attrs(func(a slog.Attr) bool {
		b.WriteString("    ")
		b.WriteString(a.Key)
		b.WriteString(": ")
		b.WriteString(strings.ReplaceAll(a.Value.String(), "\n", "\n      "))
		b.WriteByte('\n')
		return true
	})
	text := b.String()
	if text == out.last {
		out.repeats++
		return nil
	}
	out.flushRepeats()
	out.last = text
	out.add(chunk{level, text})
	return nil
}

func bytesFromJS(v js.Value) []byte {
	b := make([]byte, v.Length())
	js.CopyBytesToGo(b, v)
	return b
}

// The program's variables, by name, for the playground's live view of a
// scene: emu's symbols are the functions, for naming addresses in the log.
var globals map[string]uint32

// globalsFromELF reads the addresses of an executable's variables.
func globalsFromELF(data []byte) (map[string]uint32, error) {
	syms, err := emu.ELFSymbols(data, func(s elf.Symbol) bool {
		return elf.ST_TYPE(s.Info) == elf.STT_OBJECT
	})
	if err != nil {
		return nil, err
	}
	out := make(map[string]uint32, len(syms))
	for _, s := range syms {
		out[s.Name] = s.Addr
	}
	return out, nil
}

// symbol returns a variable's address, or failing that a function's, or
// -1 when the program has none of that name.
func symbol(_ js.Value, args []js.Value) any {
	if len(args) < 1 {
		return -1
	}
	name := args[0].String()
	if addr, ok := globals[name]; ok {
		return int(addr)
	}
	if addr, ok := funcs[name]; ok {
		return int(addr)
	}
	return -1
}

// inRAM says whether n bytes at addr lie in the console's RAM, in any of
// the segments it is seen through.
func inRAM(addr uint32, n int) bool {
	off := addr &^ 0xe0000000
	return n >= 0 && off < 2<<20 && uint64(off)+uint64(n) <= 2<<20
}

// readMemory returns n bytes of RAM at addr as a Uint8Array, or null when
// nothing is running or the range is not RAM.
func readMemory(_ js.Value, args []js.Value) any {
	if machine == nil || len(args) < 2 {
		return nil
	}
	addr, n := uint32(args[0].Int()), args[1].Int()
	if !inRAM(addr, n) {
		return nil
	}
	b := make([]byte, n)
	for i := range b {
		b[i] = machine.Bus.Read8(addr + uint32(i))
	}
	out := js.Global().Get("Uint8Array").New(n)
	js.CopyBytesToJS(out, b)
	return out
}

// writeMemory puts bytes, a Uint8Array, into RAM at addr, and returns
// whether it could.
func writeMemory(_ js.Value, args []js.Value) any {
	if machine == nil || len(args) < 2 {
		return false
	}
	addr := uint32(args[0].Int())
	b := bytesFromJS(args[1])
	if !inRAM(addr, len(b)) {
		return false
	}
	for i, c := range b {
		machine.Bus.Write8(addr+uint32(i), c)
	}
	return true
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
