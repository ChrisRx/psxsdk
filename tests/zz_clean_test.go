package tests

import (
	"cmp"
	"debug/dwarf"
	"debug/elf"
	"fmt"
	"image"
	"image/png"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"go.chrisrx.dev/psx/emu"
	"go.chrisrx.dev/psx/emu/emutest"
)

// gpuWord is a word written to the GPU: to GP0, or to GP1 when gp1 is set.
type gpuWord struct {
	v   uint32
	gp1 bool
}

// spuWord is a halfword written to an SPU register, at its offset from
// 0x1F801C00.
type spuWord struct{ off, v uint32 }

func (w spuWord) String() string { return fmt.Sprintf("%03x=%04x", w.off, w.v) }

func (w gpuWord) String() string {
	if w.gp1 {
		return fmt.Sprintf("1:%08x", w.v)
	}
	return fmt.Sprintf("%08x", w.v)
}

// TestClean compares a program built against the clean-room library,
// internal/libps, with the same program built against a reference library.
// PSX_CLEAN names it: the reference disc is .build/NAME-ref/NAME-ref.cue
// and the clean one .build/NAME-clean/NAME-clean.cue (internal/libps/build.sh
// NAME-clean DIR). PSX_CLEAN_REF=sony (or another suffix) compares
// .build/NAME-sony/NAME-sony.cue, a build against Sony's own library,
// with .build/NAME-sony-clean/NAME-sony-clean.cue, the clean build padded
// to it (PAD_TO_REF=.build/NAME-sony/NAME-sony.bin build.sh
// NAME-sony-clean DIR). PSX_CLEAN_OTHER=SUFFIX puts .build/NAME-SUFFIX in
// the clean build's place, for a reference build padded to Sony's,
// PSX_CLEAN_REF=sony PSX_CLEAN_OTHER=sony-ref.
//
// The GPU streams are compared swap by swap: each run's words are cut
// before every GP1 05h word (a buffer swap), so a frame's words count
// wherever VSync's timing puts them. Recording starts at the program's
// entry point, after the BIOS. Everything before the first swap is
// compared as one sequence, the setup, less any leading GP1 03h words.
// PSX_CLEAN_FRAMES swaps are compared (120 by default). The test
// reports the TTY of each run, the first difference with the words around
// it, and how many pixels of the last picture differ. It writes both
// pictures to .build/NAME-clean/ref.png and clean.png, both runs' sound
// memory at the end to ref-spuram.bin and clean-spuram.bin, and both command
// streams to ref-gpu.txt and clean-gpu.txt there: a line for the setup,
// then a line per swap, each word in hex, a GP1 word written as "1:" and
// the word.
//
// The SPU's register writes are compared as one sequence, over the
// frames both runs made, since the frames they land in follow startup
// timing, and so is its sound memory
// at the end. The first frame whose writes differ, counting from the
// frame of the first swap, is logged but does not fail the test. The
// spu.txt files beside the GPU ones hold the writes by frame, each as
// offset=value.
//
// The programs in withCard get a blank memory card in slot 1, and what
// the runs leave on it is compared too.
//
// PSX_CLEAN_PAD, a button mask in hex as emu.Pad.Press takes it, holds
// those buttons on the controller in port 1 from the 30th swap to the
// 60th, so runs at different speeds see them in the same loops of the
// program. A comma list of masks presses each in turn, from the 30th
// swap: each held for 10 swaps, then let go for 10, to work a program's
// menus.
//
// Each run's frames are counted from its first swap, not from power on,
// so the time each build takes to load and start does not change how
// many frames the comparison covers: a run stops after PSX_CLEAN_FRAMES
// swaps or that many frames plus 600 after its first swap.
//
// The start-up clears .bss a word at a time, and the two libraries keep
// different amounts of zeroed state, which would put main at another
// point of the video frame in each run for no reason of the library's
// behavior. So the run with the smaller .bss has its start-up clear as
// many more words past _end as the other has, and both reach main on
// the same cycle (internal/libps/build.sh already pads the executables to
// the same size, which lines up their loading).
//
// The runs in timingSensitive differ from their references only in
// timing. TestClean still logs their differences, but skips rather than
// fails, with the reason, so regress.sh lists them apart from the
// failures that matter.
func TestClean(t *testing.T) {
	name := os.Getenv("PSX_CLEAN")
	if name == "" {
		t.Skip("set PSX_CLEAN to compare a clean-room library build with its reference")
	}
	swaps := cmp.Or(atoi(os.Getenv("PSX_CLEAN_FRAMES")), 120)
	steps := readPadSteps(t, os.Getenv("PSX_CLEAN_PAD"))
	const limit = 600 // frames to wait for the first swap
	key := name
	if pad := os.Getenv("PSX_CLEAN_PAD"); pad != "" {
		key += ":" + pad
	}
	r := &differences{T: t, timing: timingSensitive[key]}
	defer func() {
		if r.timing != "" && r.found {
			t.Skipf("timing only: %s", r.timing)
		}
	}()
	type run struct {
		tty   string
		gpu   [][]gpuWord
		spu   [][]spuWord
		ram   []byte
		card  []byte
		pic   *image.RGBA
		fault string
	}
	// play runs disc until it has made swaps buffer swaps, or for 600
	// frames more than that after its first swap.
	play := func(dir string, b bss, extra uint32) run {
		var r run
		m := emutest.Boot(t, emutest.BIOS(emutest.BIOSFromEnv(t)), emutest.Disc(dir+".cue"), emutest.AllowFaults())
		var card *emu.MemoryCard
		if withCard[name] {
			var err error
			if card, err = emu.NewMemoryCard(nil); err != nil {
				t.Fatal(err)
			}
			m.InsertCard(1, card)
		}
		// Both runs start recording at the program's entry, after the
		// BIOS, with the start-up lengthened in the one that needs it.
		start(t, m, b, extra)
		var words []gpuWord
		first := -1 // frame of the first swap
		m.OnGP0(func(v uint32) { words = append(words, gpuWord{v: v}) })
		m.OnGP1(func(v uint32) {
			if v>>24 == 0x05 {
				r.gpu = append(r.gpu, words)
				words = nil
				if first < 0 {
					first = len(r.spu)
				}
			}
			words = append(words, gpuWord{v: v, gp1: true})
		})
		var sound []spuWord
		m.OnSPU(func(off, v uint32) { sound = append(sound, spuWord{off, v}) })
		func() {
			defer func() {
				if e := recover(); e != nil {
					r.fault = fmt.Sprint(e)
				}
			}()
			for {
				m.Pad(1).Release(0xffff)
				if b := steps.held(len(r.gpu)); b != 0 {
					m.Pad(1).Press(b)
				}
				m.RunFrames(1)
				r.spu = append(r.spu, sound)
				sound = nil
				if len(r.gpu) > swaps || first < 0 && len(r.spu) >= limit ||
					first >= 0 && len(r.spu)-first >= swaps+600 {
					break
				}
			}
		}()
		m.OnGP0(nil)
		m.OnGP1(nil)
		m.OnSPU(nil)
		if len(r.gpu) > 0 {
			for len(r.gpu[0]) > 0 && bios(r.gpu[0][0]) {
				r.gpu[0] = r.gpu[0][1:]
			}
		}
		if first >= 0 {
			r.spu = setup(r.spu, first)
		}
		r.tty, r.pic, r.ram = m.TTY(), m.Display(), slices.Clone(m.SPURAM())
		if card != nil {
			r.card = slices.Clone(card.Data())
		}
		return r
	}
	refName, clName := name+"-ref", name+"-clean"
	if suffix := os.Getenv("PSX_CLEAN_REF"); suffix != "" && suffix != "ref" {
		refName, clName = name+"-"+suffix, name+"-"+suffix+"-clean"
	}
	if suffix := os.Getenv("PSX_CLEAN_OTHER"); suffix != "" {
		clName = name + "-" + suffix
	}
	refDir := "../.build/" + refName + "/"
	clDir := "../.build/" + clName + "/"
	refBSS, clBSS := readBSS(t, refDir), readBSS(t, clDir)
	t.Logf(".bss: reference %d words, clean %d", refBSS.words(), clBSS.words())
	ref := play(refDir+refName, refBSS, max(clBSS.words(), refBSS.words())-refBSS.words())
	cl := play(clDir+clName, clBSS, max(clBSS.words(), refBSS.words())-clBSS.words())
	t.Logf("swaps: reference %d, clean %d", len(ref.gpu)-1, len(cl.gpu)-1)
	if len(ref.gpu) <= 1 || len(cl.gpu) <= 1 {
		t.Fatalf("no buffer swap in %d frames", limit)
	}
	if len(ref.gpu) != len(cl.gpu) {
		r.Errorf("the runs made different numbers of swaps in %d frames after the first", swaps+600)
	}
	dir := clDir
	dump(t, dir+"ref-gpu.txt", "swap", ref.gpu)
	dump(t, dir+"clean-gpu.txt", "swap", cl.gpu)
	dump(t, dir+"ref-spu.txt", "frame", ref.spu)
	dump(t, dir+"clean-spu.txt", "frame", cl.spu)
	savePNG(t, dir+"ref.png", ref.pic)
	savePNG(t, dir+"clean.png", cl.pic)
	os.WriteFile(dir+"ref-spuram.bin", ref.ram, 0o666)
	os.WriteFile(dir+"clean-spuram.bin", cl.ram, 0o666)
	t.Logf("reference TTY:\n%s", tail(ref.tty, 1500))
	t.Logf("clean TTY:\n%s", tail(cl.tty, 1500))
	if cl.fault != "" {
		t.Errorf("clean run stopped: %s", cl.fault)
	}
	compare(r, "GPU", "swap", ref.gpu, cl.gpu)
	// The runs stop on their last swap, which pacing may put a frame or
	// two apart, and the sound tick writes every frame.
	frames := min(len(ref.spu), len(cl.spu))
	compare(r, "SPU", "frame", [][]spuWord{slices.Concat(ref.spu[:frames]...)}, [][]spuWord{slices.Concat(cl.spu[:frames]...)})
	compare(&logger{t}, "SPU", "frame", ref.spu, cl.spu)
	if os.Getenv("PSX_CLEAN_REF") == "cimg" {
		// On the retail BIOS the boot sound's reverb leaves samples here,
		// up to the moment the library's start-up stops it.
		clear(ref.ram[0x70000:])
		clear(cl.ram[0x70000:])
	}
	if n := countDiff(ref.ram, cl.ram); n > 0 {
		r.Errorf("%d bytes of sound memory differ", n)
	}
	if n := countDiff(ref.card, cl.card); n > 0 {
		r.Errorf("%d bytes of the memory card differ (first at %d)", n, firstDiff(ref.card, cl.card))
	}
	if ref.pic.Bounds() != cl.pic.Bounds() {
		r.Errorf("display sizes differ: reference %v, clean %v", ref.pic.Bounds(), cl.pic.Bounds())
		return
	}
	n := 0
	for i := 0; i < len(ref.pic.Pix); i += 4 {
		if [3]uint8(ref.pic.Pix[i:i+3]) != [3]uint8(cl.pic.Pix[i:i+3]) {
			n++
		}
	}
	if n > 0 {
		r.Errorf("%d pixels of the last frame differ", n)
	}
}

// padSteps is PSX_CLEAN_PAD read: the button masks to press in turn.
type padSteps []uint16

// readPadSteps parses s, a comma list of masks in hex.
func readPadSteps(t *testing.T, s string) padSteps {
	t.Helper()
	var steps padSteps
	if s == "" {
		return nil
	}
	for _, f := range strings.Split(s, ",") {
		b, err := strconv.ParseUint(f, 16, 16)
		if err != nil {
			t.Fatalf("PSX_CLEAN_PAD: %v", err)
		}
		steps = append(steps, uint16(b))
	}
	return steps
}

// held returns the buttons held at swap n: a single mask from the 30th
// swap to the 60th, or each of several in turn from the 30th, 10 swaps
// down and 10 up.
func (p padSteps) held(n int) uint16 {
	if n < 30 || len(p) == 0 {
		return 0
	}
	if len(p) == 1 {
		if n < 60 {
			return p[0]
		}
		return 0
	}
	i, j := (n-30)/20, (n-30)%20
	if i >= len(p) || j >= 10 {
		return 0
	}
	return p[i]
}

// withCard lists the programs, by PSX_CLEAN, that get a blank memory card
// in slot 1. TestClean compares what each run leaves on it.
var withCard = map[string]bool{
	"card": true,
}

// timingSensitive lists the runs, by PSX_CLEAN and PSX_CLEAN_PAD as
// "name:pad", that differ from their references only in timing, with
// why. A run's readout of its own frame time differs because the two
// libraries take different times; the others have an event within a few
// thousand cycles of a frame's edge, which the libraries' speed
// differences move across it (internal/libps/spec/08-timing.md).
var timingSensitive = map[string]string{
	"timers":    "its readout of the lines its frame took",
	"flame":     "its readout of its frame time",
	"morph":     "its readout of its frame time",
	"offscr":    "its readout of its frame time",
	"dungeon":   "its readout of its frame time",
	"check":     "its total time readout",
	"terra":     "a sound tick at a frame's edge",
	"square":    "its loading bar: a read's last interrupt within a few hundred cycles of a vertical blank",
	"dino":      "its reverb memory, which holds audio of the sample its key ons land on",
	"cdda:4000": "its CD capture memory, which holds audio of the sample the play lands on",
}

// differences reports TestClean's differences: as failures, or, for a
// timing-sensitive run, as log lines, noting that there were some.
type differences struct {
	*testing.T
	timing string
	found  bool
}

func (d *differences) Errorf(format string, args ...any) {
	d.Helper()
	d.found = true
	if d.timing != "" {
		d.Logf(format, args...)
		return
	}
	d.T.Errorf(format, args...)
}

// bss is a build's entry point and its .bss, from _fbss up to _end.
type bss struct{ entry, start, end uint32 }

func (b bss) words() uint32 { return (b.end - b.start) / 4 }

// readBSS reads the bss of the build in dir, from its prog.elf.
func readBSS(t *testing.T, dir string) bss {
	t.Helper()
	f, err := elf.Open(dir + "prog.elf")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	syms, err := f.Symbols()
	if err != nil {
		t.Fatal(err)
	}
	b := bss{entry: uint32(f.Entry)}
	for _, s := range syms {
		switch s.Name {
		case "_fbss":
			b.start = uint32(s.Value)
		case "_end":
			b.end = uint32(s.Value)
		}
	}
	if b.start == 0 || b.end < b.start {
		t.Fatalf("%sprog.elf: no _fbss and _end", dir)
	}
	return b
}

// start runs m to its entry point. With extra words, it goes on through
// the start-up until it holds _end in t1 as the end of its clearing, then
// moves that end on by extra words, so that the start-up takes 10 cycles
// a word longer. Both start-ups load _end into t1 for the clear before
// anything else uses t1 for it.
func start(t *testing.T, m *emutest.Machine, b bss, extra uint32) {
	t.Helper()
	m.RunUntil(func() bool { return m.CPU.PC == b.entry }, 200*emutest.Second, "the entry point")
	if extra == 0 {
		return
	}
	for range 64 {
		m.Step()
		if m.CPU.R[9] == b.end {
			m.CPU.R[9] += 4 * extra
			return
		}
	}
	t.Fatal("the start-up did not load _end into t1")
}

// bios is the BIOS's leading GP1 03h word, which TestClean drops.
func bios(w gpuWord) bool { return w == gpuWord{v: 0x03000000, gp1: true} }

// setup returns frames from f on, with everything before it gathered in
// front as one frame.
func setup[W any](frames [][]W, f int) [][]W {
	var all []W
	for _, words := range frames[:f] {
		all = append(all, words...)
	}
	return append([][]W{all}, frames[f:]...)
}

// reporter is where compare reports a difference: a *testing.T fails the
// test, a logger only logs it.
type reporter interface {
	Helper()
	Errorf(format string, args ...any)
}

type logger struct{ *testing.T }

func (l *logger) Errorf(format string, args ...any) { l.Logf(format, args...) }

// compare reports the first difference between two runs' streams, by
// unit (a swap or a frame) after setup's.
func compare[W comparable](t reporter, what, unit string, ref, cl [][]W) {
	t.Helper()
	for f := range min(len(ref), len(cl)) {
		a, b := ref[f], cl[f]
		i := 0
		for i < len(a) && i < len(b) && a[i] == b[i] {
			i++
		}
		if i == len(a) && i == len(b) {
			continue
		}
		where := fmt.Sprintf("%s %d", unit, f)
		if f == 0 {
			where = "setup"
		}
		lo := max(0, i-4)
		t.Errorf("%s: %s words differ at %d (reference wrote %d, clean %d)\nreference %v\nclean     %v",
			where, what, i, len(a), len(b), a[lo:min(len(a), i+12)], b[lo:min(len(b), i+12)])
		return
	}
}

// firstDiff returns the offset of the first byte a and b differ in.
func firstDiff(a, b []byte) int {
	for i := range min(len(a), len(b)) {
		if a[i] != b[i] {
			return i
		}
	}
	return min(len(a), len(b))
}

func countDiff(a, b []byte) int {
	n := 0
	for i := range min(len(a), len(b)) {
		if a[i] != b[i] {
			n++
		}
	}
	return n + max(len(a), len(b)) - min(len(a), len(b))
}

// trimPath shortens a source path to the part after the module or
// GOROOT directory.
func trimPath(p string) string {
	for _, marker := range []string{"go.chrisrx.dev/psx/", "/src/"} {
		if _, after, ok := strings.Cut(p, marker); ok {
			return after
		}
	}
	return p
}

func tail(s string, n int) string {
	if len(s) > n {
		return "..." + s[len(s)-n:]
	}
	return s
}

func savePNG(t *testing.T, path string, img image.Image) {
	f, err := os.Create(path)
	if err != nil {
		t.Log(err)
		return
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Log(err)
	}
}

func dump[W fmt.Stringer](t *testing.T, path, unit string, frames [][]W) {
	var b []byte
	for f, words := range frames {
		b = fmt.Appendf(b, "%s %d:", unit, f)
		for _, w := range words {
			b = fmt.Appendf(b, " %v", w)
		}
		b = append(b, '\n')
	}
	if err := os.WriteFile(path, b, 0o666); err != nil {
		t.Log(err)
	}
}

// TestCleanShot runs the disc PSX_SHOT names (a .cue path from the
// repository root) for PSX_CLEAN_FRAMES frames, 120 by default, and
// writes its picture beside the disc as shot.png, for probe programs.
func TestCleanShot(t *testing.T) {
	cue := os.Getenv("PSX_SHOT")
	if cue == "" {
		t.Skip("set PSX_SHOT to a disc to take its picture")
	}
	m := emutest.Boot(t, emutest.BIOS(emutest.BIOSFromEnv(t)), emutest.Disc("../"+cue))
	m.RunFrames(cmp.Or(atoi(os.Getenv("PSX_CLEAN_FRAMES")), 120))
	savePNG(t, "../"+filepath.Dir(cue)+"/shot.png", m.Display())
}

// TestCleanProfile profiles a build: PSX_PROFILE names the disc,
// .build/NAME/NAME.cue with prog.elf beside it (so NAME is flame-clean
// or flame-ref, say). It runs to the first buffer swap and 60 frames on,
// then steps PSX_CLEAN_FRAMES frames (10 by default), attributing each
// instruction's cycles to the function it is in (by the ELF's function
// symbols), and prints the 30 costliest per frame. PSX_CLEAN_PAD holds
// buttons for the whole profiled stretch. Time spent waiting in VSync
// shows under VSync's functions: that is the frame's spare time.
//
// TinyGo inlines most of the library into main, so built with DWARF
// (TGOFLAGS=-debug internal/libps/build.sh NAME DIR) the test also
// attributes cycles to source lines, and prints the 15 costliest files
// and 40 costliest lines.
func TestCleanProfile(t *testing.T) {
	name := os.Getenv("PSX_PROFILE")
	if name == "" {
		t.Skip("set PSX_PROFILE to a build to profile")
	}
	frames := cmp.Or(atoi(os.Getenv("PSX_CLEAN_FRAMES")), 10)
	dir := "../.build/" + name + "/"
	f, err := elf.Open(dir + "prog.elf")
	if err != nil {
		t.Fatal(err)
	}
	syms, err := f.Symbols()
	type row struct {
		addr  uint32
		where string
	}
	var rows []row
	if d, err := f.DWARF(); err == nil {
		r := d.Reader()
		for {
			cu, err := r.Next()
			if err != nil || cu == nil {
				break
			}
			if cu.Tag != dwarf.TagCompileUnit {
				r.SkipChildren()
				continue
			}
			lr, err := d.LineReader(cu)
			if err != nil || lr == nil {
				continue
			}
			var e dwarf.LineEntry
			for lr.Next(&e) == nil {
				if !e.EndSequence && e.File != nil {
					rows = append(rows, row{uint32(e.Address), fmt.Sprintf("%s:%d", trimPath(e.File.Name), e.Line)})
				}
			}
		}
		slices.SortStableFunc(rows, func(a, b row) int { return cmp.Compare(a.addr, b.addr) })
	}
	line := func(pc uint32) string {
		i, found := slices.BinarySearchFunc(rows, pc, func(r row, pc uint32) int { return cmp.Compare(r.addr, pc) })
		if !found {
			i--
		}
		if i < 0 {
			return "?"
		}
		return rows[i].where
	}
	f.Close()
	if err != nil {
		t.Fatal(err)
	}
	type fn struct {
		name       string
		start, end uint32
	}
	var fns []fn
	for _, s := range syms {
		if elf.ST_TYPE(s.Info) == elf.STT_FUNC && s.Size > 0 && s.Section != elf.SHN_ABS {
			fns = append(fns, fn{s.Name, uint32(s.Value), uint32(s.Value + s.Size)})
		}
	}
	slices.SortFunc(fns, func(a, b fn) int { return cmp.Compare(a.start, b.start) })
	lookup := func(pc uint32) string {
		i, _ := slices.BinarySearchFunc(fns, pc, func(f fn, pc uint32) int {
			switch {
			case pc < f.start:
				return 1
			case pc >= f.end:
				return -1
			}
			return 0
		})
		if i < len(fns) && pc >= fns[i].start && pc < fns[i].end {
			return fns[i].name
		}
		if pc >= 0xbfc00000 || pc < 0x10000 {
			return "(kernel/BIOS)"
		}
		return "(unknown)"
	}

	m := emutest.Boot(t, emutest.BIOS(emutest.BIOSFromEnv(t)), emutest.Disc(dir+name+".cue"))
	swapped := false
	m.OnGP1(func(v uint32) {
		if v>>24 == 0x05 {
			swapped = true
		}
	})
	m.RunUntil(func() bool { return swapped }, 60*emutest.Second, "the first swap")
	m.OnGP1(nil)
	m.RunFrames(60)
	if buttons, _ := strconv.ParseUint(os.Getenv("PSX_CLEAN_PAD"), 16, 16); buttons != 0 {
		m.Pad(1).Press(uint16(buttons))
		m.RunFrames(30)
	}
	cycles, lines := map[string]uint64{}, map[string]uint64{}
	start, end := m.Frames(), m.Frames()+uint64(frames)
	for m.Frames() < end {
		pc, before := m.CPU.PC, m.CPU.TotalCycles
		m.Step()
		c := m.CPU.TotalCycles - before
		cycles[lookup(pc)] += c
		if rows != nil {
			lines[line(pc)] += c
		}
	}
	var total uint64
	for _, c := range cycles {
		total += c
	}
	per := float64(end - start)
	t.Logf("%d frames, %.0f cycles a frame", end-start, float64(total)/per)
	top := func(title string, m map[string]uint64, n int) {
		names := slices.Collect(maps.Keys(m))
		slices.SortFunc(names, func(a, b string) int { return cmp.Compare(m[b], m[a]) })
		t.Logf("%s:", title)
		for _, k := range names[:min(n, len(names))] {
			t.Logf("%10.0f %5.1f%%  %s", float64(m[k])/per, 100*float64(m[k])/float64(total), k)
		}
	}
	top("functions", cycles, 30)
	if rows != nil {
		files := map[string]uint64{}
		for k, c := range lines {
			file, _, _ := strings.Cut(k, ":")
			files[file] += c
		}
		top("files", files, 15)
		top("lines", lines, 40)
	}
}

// TestCleanStart observes a program's start-up from outside: PSX_START
// names a disc, .build/NAME/NAME.cue with prog.elf beside it. At the entry
// point it fills the RAM from _fbss to 64KB below the stack with a
// pattern, then steps to C main, logging kernel calls, system calls and
// the call to InitHeap with the cycles since entry, and reports the
// registers at entry and at main and which words of the pattern changed.
func TestCleanStart(t *testing.T) {
	name := os.Getenv("PSX_START")
	if name == "" {
		t.Skip("set PSX_START to a build to observe its start-up")
	}
	dir := "../.build/" + name + "/"
	f, err := elf.Open(dir + "prog.elf")
	if err != nil {
		t.Fatal(err)
	}
	syms, err := f.Symbols()
	f.Close()
	if err != nil {
		t.Fatal(err)
	}
	sym := func(n string) uint32 {
		for _, s := range syms {
			if s.Name == n && (n != "main" || elf.ST_TYPE(s.Info) == elf.STT_FUNC) {
				return uint32(s.Value)
			}
		}
		t.Fatalf("no symbol %s", n)
		return 0
	}
	entry, fbss, end, main, initHeap := uint32(f.Entry), sym("_fbss"), sym("_end"), sym("main"), sym("InitHeap")
	t.Logf("entry %08x, _fbss %08x, _end %08x (%d words), _gp %08x", entry, fbss, end, (end-fbss)/4, sym("_gp"))
	m := emutest.Boot(t, emutest.BIOS(emutest.BIOSFromEnv(t)), emutest.Disc(dir+name+".cue"), emutest.AllowFaults())
	m.RunUntil(func() bool { return m.CPU.PC == entry }, 200*emutest.Second, "the entry point")
	regs := func(when string) {
		var b strings.Builder
		for _, r := range []struct {
			name string
			i    int
		}{{"v0", 2}, {"v1", 3}, {"a0", 4}, {"a1", 5}, {"a2", 6}, {"a3", 7}, {"s0", 16}, {"s1", 17}, {"k0", 26}, {"k1", 27}, {"gp", 28}, {"sp", 29}, {"fp", 30}, {"ra", 31}} {
			fmt.Fprintf(&b, " %s=%08x", r.name, m.CPU.R[r.i])
		}
		t.Logf("%s:%s SR=%08x", when, b.String(), m.CPU.COP0[12])
	}
	regs("at entry")
	const pattern = 0xa5a5a5a5
	lo, hi := fbss, m.CPU.R[29]-0x10000
	for a := lo; a < hi; a += 4 {
		m.Bus.Write32(a, pattern)
	}
	start := m.CPU.TotalCycles
	for steps := 0; m.CPU.PC != main; steps++ {
		if steps == 10_000_000 {
			t.Fatal("main not reached")
		}
		pc, r := m.CPU.PC, &m.CPU.R
		switch {
		case pc == initHeap:
			t.Logf("+%d InitHeap(%08x, %08x)", m.CPU.TotalCycles-start, r[4], r[5])
		case pc == 0xa0 || pc == 0xb0 || pc == 0xc0:
			t.Logf("+%d kernel %02X(%02Xh) a0=%08x a1=%08x", m.CPU.TotalCycles-start, pc, r[9], r[4], r[5])
		case pc >= 0x80000000 && m.Read32(pc) == 0x0000000c:
			t.Logf("+%d syscall a0=%d", m.CPU.TotalCycles-start, r[4])
		}
		m.Step()
	}
	t.Logf("main at +%d", m.CPU.TotalCycles-start)
	regs("at main")
	run := uint32(0)
	for a := lo; a <= hi; a += 4 {
		changed := a < hi && m.Read32(a) != pattern
		switch {
		case changed && run == 0:
			run = a
		case !changed && run != 0:
			t.Logf("changed %08x..%08x (%d words), first %08x, last %08x", run, a, (a-run)/4, m.Read32(run), m.Read32(a-4))
			run = 0
		}
	}
}

func atoi(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}
