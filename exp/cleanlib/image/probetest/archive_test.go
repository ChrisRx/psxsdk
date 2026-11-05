package probetest

import (
	"cmp"
	"flag"
	"fmt"
	"hash/fnv"
	"image"
	"image/draw"
	"image/png"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"

	"go.chrisrx.dev/psx/emu"
	"go.chrisrx.dev/psx/emu/emutest"
)

var archive = flag.Bool("archive", false, "compare the web archive's programs, on the discs archive.sh builds")

const carchive = "../../../../.build/carchive/"

// archiveRun is what one program did on one image.
type archiveRun struct {
	booted  bool
	changes []pictureChange
	tty     string
	parked  uint32 // the entry's low half, where it stopped for lack of an entry
	wild    uint32 // a PC outside memory
	frames  int    // how many frames it showed
}

// runArchive runs a disc until the library starts, or gives up, then for a number
// of frames, with Start and Cross held every 240.
func runArchive(t *testing.T, cue string, frames int) archiveRun {
	var r archiveRun
	m := emutest.Boot(t, emutest.BIOS(emutest.BIOSFromEnv(t)), emutest.Disc(cue), emutest.AllowFaults())
	for end := m.Cycles() + 30*emutest.Second; m.Cycles() < end && !r.booted; {
		for range 4096 {
			if m.Machine.Step() != nil {
				return r
			}
		}
		r.booted = strings.Contains(string(m.TTY()), "Execute !")
	}
	if !r.booted {
		return r
	}
	from := len(m.TTY())
	presses := map[int]bool{}
	for f := 121; f < frames; f += 240 {
		presses[f] = true
	}
	start := m.Frames()
	r.changes = pictureChanges(m, frames, presses, func() bool {
		pc := m.CPU.PC
		switch {
		case m.CPU.R[24] == 0xdead0000:
			r.parked = 0x10000 | m.CPU.R[25]&0xffff
		case pc >= 0x200000 && pc < 0x80000000, pc >= 0x80200000 && pc < 0xa0000000, pc >= 0xa0200000 && pc < 0xbfc00000, pc >= 0xbfc80000:
			r.wild = pc
		}
		return r.parked != 0 || r.wild != 0
	})
	r.frames = int(m.Frames() - start)
	r.tty = strings.ReplaceAll(string(m.TTY()[from:]), "\r", "")
	return r
}

// verdict sets the two runs against each other, as a word and the details.
func verdict(a, b archiveRun) (string, string) {
	if !a.booted || !b.booted {
		return "noboot", fmt.Sprintf("sony %v, clean %v", a.booted, b.booted)
	}
	note := fmt.Sprintf("changes %d/%d", len(a.changes), len(b.changes))
	if a.wild != 0 {
		note += fmt.Sprintf(", wild on sony %08x", a.wild)
	}
	if b.wild != 0 {
		note += fmt.Sprintf(", wild on clean %08x", b.wild)
	}
	if a.frames != b.frames && a.wild == 0 && b.wild == 0 && b.parked == 0 {
		return "stall", fmt.Sprintf("%s, frames shown %d on Sony's, %d on clean", note, a.frames, b.frames)
	}
	if b.parked != 0 {
		return "parked", fmt.Sprintf("%s, at entry low half %05x", note, b.parked)
	}
	for i := range min(len(a.changes), len(b.changes)) {
		if a.changes[i].sum != b.changes[i].sum {
			return "differ", fmt.Sprintf("%s, change %d, frame %d on Sony's, %d on clean", note, i, a.changes[i].frame, b.changes[i].frame)
		}
	}
	for i := range min(len(a.changes), len(b.changes)) {
		if a.changes[i].frame != b.changes[i].frame {
			return "shift", fmt.Sprintf("%s, change %d, frame %d on Sony's, %d on clean", note, i, a.changes[i].frame, b.changes[i].frame)
		}
	}
	if len(a.changes) != len(b.changes) {
		return "shift", note + ", one run has more"
	}
	if a.tty != b.tty {
		return "tty", fmt.Sprintf("%s, %d bytes on Sony's, %d on clean", note, len(a.tty), len(b.tty))
	}
	if b.wild != a.wild && b.wild != 0 {
		return "wild", note
	}
	return "same", note
}

// TestArchive compares every program archive.sh built, by its picture changes.
func TestArchive(t *testing.T) {
	if !*archive {
		t.Skip("archive.sh runs this")
	}
	rows, err := os.ReadFile(carchive + "programs.tsv")
	if err != nil {
		t.Fatal(err)
	}
	frames, _ := strconv.Atoi(os.Getenv("ARCHIVE_FRAMES"))
	if frames == 0 {
		frames = 1200
	}
	file := carchive + "summary.txt"
	if os.Getenv("ARCHIVE_ONLY") != "" {
		file = carchive + "summary-only.txt"
	}
	out, err := os.Create(file)
	if err != nil {
		t.Fatal(err)
	}
	defer out.Close()
	var mu sync.Mutex
	t.Run("programs", func(t *testing.T) {
		for row := range strings.SplitSeq(strings.TrimSpace(string(rows)), "\n") {
			name, _, _ := strings.Cut(row, "\t")
			t.Run(name, func(t *testing.T) {
				t.Parallel()
				run := func(side string) archiveRun {
					cue := carchive + name + "-" + side + "/" + name + "-" + side + ".cue"
					if _, err := os.Stat(cue); err != nil {
						return archiveRun{}
					}
					return runArchive(t, cue, frames)
				}
				word, detail := verdict(run("sony"), run("clean"))
				mu.Lock()
				defer mu.Unlock()
				fmt.Fprintf(out, "%-50s %-7s %s\n", name, word, detail)
			})
		}
	})
}

// TestArchiveShow writes pictures of the program ARCHIVE_SHOW on both images at
// frame ARCHIVE_AT, and says where they differ.
func TestArchiveShow(t *testing.T) {
	name := os.Getenv("ARCHIVE_SHOW")
	if name == "" {
		t.Skip("ARCHIVE_SHOW names a program")
	}
	at, _ := strconv.Atoi(os.Getenv("ARCHIVE_AT"))
	var pics []*image.RGBA
	for _, side := range []string{"sony", "clean"} {
		dir := carchive + name + "-" + side + "/"
		m := emutest.Boot(t, emutest.BIOS(emutest.BIOSFromEnv(t)), emutest.Disc(dir+name+"-"+side+".cue"), emutest.AllowFaults())
		m.RunUntilTTY("Execute !", 30*emutest.Second)
		presses := map[int]bool{}
		for f := 121; f < at; f += 240 {
			presses[f] = true
		}
		pictureChanges(m, at, presses, nil)
		pic := m.Display()
		pics = append(pics, pic)
		f, err := os.Create(dir + "show.png")
		if err != nil {
			t.Fatal(err)
		}
		png.Encode(f, pic)
		f.Close()
		t.Logf("%s: frame %d, %v, pc %08x, TTY %q", side, at, pic.Bounds(), m.CPU.PC, m.TTY()[max(0, len(m.TTY())-400):])
	}
	a, b := pics[0], pics[1]
	if a.Bounds() != b.Bounds() {
		return
	}
	n, box := 0, image.Rectangle{}
	for y := range a.Bounds().Dy() {
		for x := range a.Bounds().Dx() {
			if a.RGBAAt(x, y) != b.RGBAAt(x, y) {
				n++
				box = box.Union(image.Rect(x, y, x+1, y+1))
			}
		}
	}
	t.Logf("%d pixels differ, in %v", n, box)
}

// firstSide runs a program one picture change at a time, keeping what it sends
// the GPU.
type firstSide struct {
	m     *emutest.Machine
	words []uint32
	frame int
	held  int
	last  uint64
}

func newFirstSide(t *testing.T, cue string) *firstSide {
	m := emutest.Boot(t, emutest.BIOS(emutest.BIOSFromEnv(t)), emutest.Disc(cue), emutest.AllowFaults())
	m.RunUntilTTY("Execute !", 30*emutest.Second)
	s := &firstSide{m: m}
	m.OnGP0(func(v uint32) { s.words = append(s.words, v) })
	return s
}

// next runs to the next picture change, or says there was none within the frames.
func (s *firstSide) next(frames int) (uint64, bool) {
	for s.frame < frames {
		s.frame++
		if s.frame%240 == 121 {
			s.m.Pad(1).Press(emu.ButtonStart | emu.ButtonCross)
			s.held = s.frame + 2
		}
		if s.held != 0 && s.frame >= s.held {
			s.m.Pad(1).Release(emu.ButtonStart | emu.ButtonCross)
			s.held = 0
		}
		before := s.m.Frames()
		if _, err := s.m.Machine.RunFrame(1 << 20); err != nil || s.m.Frames() == before {
			return 0, false
		}
		h := fnv.New64a()
		h.Write(s.m.Display().Pix)
		if v := h.Sum64(); v != s.last {
			s.last = v
			return v, true
		}
	}
	return 0, false
}

// firstDifference runs both sides of a program to their first different picture and
// returns a report, with both pictures side by side in carchive/first/NAME.png.
func firstDifference(t *testing.T, name string, frames int) string {
	cue := func(side string) string { return carchive + name + "-" + side + "/" + name + "-" + side + ".cue" }
	a, b := newFirstSide(t, cue("sony")), newFirstSide(t, cue("clean"))
	var out strings.Builder
	fmt.Fprintf(&out, "%s:", name)
	shifted := false
	change := 0
	for ; ; change++ {
		sa, oka := a.next(frames)
		sb, okb := b.next(frames)
		if !oka || !okb {
			fmt.Fprintf(&out, " no different picture by change %d (sony %v at %d, clean %v at %d)", change, oka, a.frame, okb, b.frame)
			break
		}
		if a.frame != b.frame && !shifted {
			shifted = true
			fmt.Fprintf(&out, " first shift change %d frame %d/%d;", change, a.frame, b.frame)
		}
		if sa != sb {
			pa, pb := a.m.Display(), b.m.Display()
			n, box := 0, image.Rectangle{}
			if pa.Bounds() == pb.Bounds() {
				for y := range pa.Bounds().Dy() {
					for x := range pa.Bounds().Dx() {
						if pa.RGBAAt(x, y) != pb.RGBAAt(x, y) {
							n++
							box = box.Union(image.Rect(x, y, x+1, y+1))
						}
					}
				}
			}
			fmt.Fprintf(&out, " change %d frame %d/%d, %d pixels in %v of %v/%v", change, a.frame, b.frame, n, box, pa.Bounds(), pb.Bounds())
			both := image.NewRGBA(image.Rect(0, 0, pa.Bounds().Dx()+pb.Bounds().Dx()+4, max(pa.Bounds().Dy(), pb.Bounds().Dy())))
			draw.Draw(both, pa.Bounds(), pa, image.Point{}, draw.Src)
			draw.Draw(both, pb.Bounds().Add(image.Pt(pa.Bounds().Dx()+4, 0)), pb, image.Point{}, draw.Src)
			if f, err := os.Create(carchive + "first/" + name + ".png"); err == nil {
				png.Encode(f, both)
				f.Close()
			}
			break
		}
	}
	fmt.Fprintf(&out, "\n  gp0 %d/%d words", len(a.words), len(b.words))
	for i := range min(len(a.words), len(b.words)) {
		if a.words[i] != b.words[i] {
			fmt.Fprintf(&out, ", first different at %d\n", i)
			from, to := max(i-8, 0), min(i+8, min(len(a.words), len(b.words)))
			fmt.Fprintf(&out, "  sony  %08x\n  clean %08x\n", a.words[from:to], b.words[from:to])
			break
		}
	}
	return out.String()
}

// TestArchiveFirst reports every program in summary.txt that wasn't the same, or those
// named in ARCHIVE_FIRST, into first.txt.
func TestArchiveFirst(t *testing.T) {
	if !*archive {
		t.Skip("archive.sh's discs")
	}
	var names []string
	if s := os.Getenv("ARCHIVE_FIRST"); s != "" {
		names = strings.Fields(s)
	} else {
		rows, err := os.ReadFile(carchive + "summary.txt")
		if err != nil {
			t.Fatal(err)
		}
		for row := range strings.SplitSeq(strings.TrimSpace(string(rows)), "\n") {
			f := strings.Fields(row)
			if f[1] != "same" && f[1] != "noboot" {
				names = append(names, f[0])
			}
		}
	}
	os.MkdirAll(carchive+"first", 0o755)
	var mu sync.Mutex
	reports := map[string]string{}
	t.Run("programs", func(t *testing.T) {
		for _, name := range names {
			t.Run(name, func(t *testing.T) {
				t.Parallel()
				r := firstDifference(t, name, 1200)
				mu.Lock()
				defer mu.Unlock()
				reports[name] = r
			})
		}
	})
	var all strings.Builder
	for _, name := range names {
		all.WriteString(reports[name] + "\n")
	}
	if err := os.WriteFile(carchive+"first.txt", []byte(all.String()), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestArchiveWatch runs the side ARCHIVE_SIDE (clean) of ARCHIVE_SHOW up to ARCHIVE_AT
// frames, stops at the first time the PC is the hex address ARCHIVE_STOP, and logs the
// registers, the memory at a0 to a3, and return addresses on the stack.
func TestArchiveWatch(t *testing.T) {
	name := os.Getenv("ARCHIVE_SHOW")
	stop, _ := strconv.ParseUint(os.Getenv("ARCHIVE_STOP"), 16, 32)
	if name == "" || stop == 0 {
		t.Skip("ARCHIVE_SHOW and ARCHIVE_STOP name a program and an address")
	}
	side := os.Getenv("ARCHIVE_SIDE")
	if side == "" {
		side = "clean"
	}
	at, _ := strconv.Atoi(os.Getenv("ARCHIVE_AT"))
	if at == 0 {
		at = 1200
	}
	skip, _ := strconv.Atoi(os.Getenv("ARCHIVE_SKIP"))
	dir := carchive + name + "-" + side + "/"
	m := emutest.Boot(t, emutest.BIOS(emutest.BIOSFromEnv(t)), emutest.Disc(dir+name+"-"+side+".cue"), emutest.AllowFaults())
	m.RunUntilTTY("Execute !", 30*emutest.Second)
	end := m.Frames() + uint64(at)
	for m.Frames() < end {
		if m.Machine.Step() != nil {
			break
		}
		if m.CPU.PC != uint32(stop) {
			continue
		}
		if cond, ok := watchConds[os.Getenv("ARCHIVE_COND")]; ok && !cond(m) {
			continue
		}
		if skip > 0 {
			skip--
			continue
		}
		r := m.CPU.R
		t.Logf("at %08x frame %d: a0 %08x a1 %08x a2 %08x a3 %08x ra %08x sp %08x", m.CPU.PC, m.Frames(), r[4], r[5], r[6], r[7], r[31], r[29])
		for i := 4; i < 8; i++ {
			if a := r[i]; a>>24 == 0x80 || a>>24 == 0 && a < 0x200000 {
				var w [16]uint32
				for j := range w {
					w[j] = m.Read32(a&^3 + uint32(j)*4)
				}
				t.Logf("a%d: %08x", i-4, w)
			}
		}
		var ras []string
		for a := r[29]; a < r[29]+0x200 && a < 0x80200000; a += 4 {
			if v := m.Read32(a); v>>24 == 0x80 && v&3 == 0 && v > 0x80010000 && v < 0x801ff000 {
				ras = append(ras, fmt.Sprintf("%08x", v))
			}
		}
		t.Logf("on the stack: %v", ras)
		// ARCHIVE_DUMP lists hex addresses to log 8 words at, a leading * following a pointer.
		for _, d := range strings.Split(os.Getenv("ARCHIVE_DUMP"), ",") {
			if d == "" {
				continue
			}
			a, _ := strconv.ParseUint(strings.TrimPrefix(d, "*"), 16, 32)
			addr := uint32(a)
			if d[0] == '*' {
				addr = m.Read32(addr)
			}
			var w [8]uint32
			for j := range w {
				w[j] = m.Read32(addr + uint32(j)*4)
			}
			t.Logf("%s at %08x: %08x", d, addr, w)
		}
		if os.Getenv("ARCHIVE_GTE") != "" {
			t.Logf("gte: %+v", m.CPU.GTE)
		}
		return
	}
	t.Logf("never reached %08x", stop)
}

// watchConds narrow TestArchiveWatch to the calls that look odd.
var watchConds = map[string]func(m *emutest.Machine) bool{
	// GsClearOt(offset, point, ot) on a table with no memory or an outsize length.
	"clearot": func(m *emutest.Machine) bool {
		ot := m.CPU.R[6]
		return ot == 0 || m.Read32(ot) > 14 || m.Read32(ot+4) == 0
	},
	// LoadImage(rect, p) with no pixels or a negative size.
	"loadimage": func(m *emutest.Machine) bool {
		r := m.CPU.R[4]
		w, h := int16(m.Read32(r+4)), int16(m.Read32(r+4)>>16)
		return m.CPU.R[5] == 0 || w <= 0 || h <= 0
	},
	"nil": func(m *emutest.Machine) bool { return m.CPU.R[4] == 0 },
}

// entryNames names the library's entry points, from the image's first link.
func entryNames() map[uint32]string {
	names := map[uint32]string{}
	b, err := os.ReadFile("../../../../.build/cleanimage/map1.txt")
	if err != nil {
		return names
	}
	for line := range strings.SplitSeq(string(b), "\n") {
		f := strings.Fields(line)
		if len(f) == 5 && strings.HasPrefix(f[4], ".fix.") {
			a, _ := strconv.ParseUint(f[0], 16, 32)
			names[uint32(a)] = strings.TrimPrefix(f[4], ".fix.")
		}
	}
	return names
}

// programCall is one call from a program into the library.
type programCall struct {
	frame, cycles uint64
	text          string
	left          [3]uint32 // a1 to a3 on return
}

// programCalls records the calls a program makes into the library over its first
// frames: entries reached from program code, not the library's calls to itself.
func programCalls(t *testing.T, name, side string, names map[uint32]string, frames int) []programCall {
	dir := carchive + name + "-" + side + "/"
	m := emutest.Boot(t, emutest.BIOS(emutest.BIOSFromEnv(t)), emutest.Disc(dir+name+"-"+side+".cue"), emutest.AllowFaults())
	m.RunUntilTTY("Execute !", 30*emutest.Second)
	const programStart = 0x80090000
	var calls []programCall
	open, ra, sp := -1, uint32(0), uint32(0)
	start := m.Frames()
	for steps := frames << 20; int(m.Frames()-start) < frames && steps > 0; steps-- {
		if m.Machine.Step() != nil {
			break
		}
		pc, r := m.CPU.PC, &m.CPU.R
		if open >= 0 && pc == ra && r[29] == sp {
			calls[open].text += fmt.Sprintf(" = %08x", r[2])
			calls[open].left = [3]uint32{r[5], r[6], r[7]}
			open = -1
		}
		if e, ok := names[pc]; ok && open < 0 && r[31] >= programStart {
			calls = append(calls, programCall{
				frame:  m.Frames() - start,
				cycles: m.CPU.TotalCycles,
				text:   fmt.Sprintf("%s(%08x %08x %08x %08x)", e, r[4], r[5], r[6], r[7]),
			})
			open, ra, sp = len(calls)-1, r[31], r[29]
		}
	}
	return calls
}

// TestArchiveCalls compares the calls programs make into the library on both images,
// over ARCHIVE_CALLS, each NAME:FRAMES. Each side's list goes to calls/NAME-SIDE.txt,
// and the first call that differs, in entry, arguments or result, to the log.
func TestArchiveCalls(t *testing.T) {
	list := os.Getenv("ARCHIVE_CALLS")
	if list == "" {
		t.Skip("ARCHIVE_CALLS names the programs")
	}
	names := entryNames()
	if err := os.MkdirAll(carchive+"calls", 0o755); err != nil {
		t.Fatal(err)
	}
	for _, spec := range strings.Fields(list) {
		name, n, _ := strings.Cut(spec, ":")
		frames, _ := strconv.Atoi(n)
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			sides := map[string][]programCall{}
			for _, side := range []string{"sony", "clean"} {
				sides[side] = programCalls(t, name, side, names, frames)
				var out strings.Builder
				for _, c := range sides[side] {
					fmt.Fprintf(&out, "%5d %12d %s left %08x\n", c.frame, c.cycles, c.text, c.left)
				}
				if err := os.WriteFile(carchive+"calls/"+name+"-"+side+".txt", []byte(out.String()), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			a, b := sides["sony"], sides["clean"]
			for i := range min(len(a), len(b)) {
				if a[i].text != b[i].text {
					t.Logf("%s: call %d of %d/%d: frame %d %s on Sony's, frame %d %s on clean", name, i, len(a), len(b), a[i].frame, a[i].text, b[i].frame, b[i].text)
					return
				}
			}
			t.Logf("%s: the first %d calls agree, of %d/%d", name, min(len(a), len(b)), len(a), len(b))
		})
	}
}

// TestArchiveCosts compares time in each library entry over the first frames of the
// programs in ARCHIVE_COSTS, each NAME:FRAMES, largest difference first.
func TestArchiveCosts(t *testing.T) {
	list := os.Getenv("ARCHIVE_COSTS")
	if list == "" {
		t.Skip("ARCHIVE_COSTS names the programs")
	}
	names := entryNames()
	for _, spec := range strings.Fields(list) {
		name, n, _ := strings.Cut(spec, ":")
		frames, _ := strconv.Atoi(n)
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			run := func(side string) map[uint32]entryCost {
				dir := carchive + name + "-" + side + "/"
				m := emutest.Boot(t, emutest.BIOS(emutest.BIOSFromEnv(t)), emutest.Disc(dir+name+"-"+side+".cue"), emutest.AllowFaults())
				m.RunUntilTTY("Execute !", 30*emutest.Second)
				return libraryCosts(m, frames)
			}
			sony, clean := run("sony"), run("clean")
			var entries []uint32
			for e := range sony {
				entries = append(entries, e)
			}
			for e := range clean {
				if _, ok := sony[e]; !ok {
					entries = append(entries, e)
				}
			}
			diff := func(e uint32) int64 { return int64(sony[e].cycles) - int64(clean[e].cycles) }
			slices.SortFunc(entries, func(a, b uint32) int {
				return cmp.Compare(max(diff(b), -diff(b)), max(diff(a), -diff(a)))
			})
			var out strings.Builder
			fmt.Fprintf(&out, "%s to frame %d\n", name, frames)
			for _, e := range entries[:min(9, len(entries))] {
				fmt.Fprintf(&out, "  %-20s Sony %5d calls %10d, clean %5d calls %10d, diff %d\n", names[e]+fmt.Sprintf("/%x", e&0xfffff),
					sony[e].calls, sony[e].cycles, clean[e].calls, clean[e].cycles, diff(e))
			}
			t.Log(out.String())
		})
	}
}

// TestArchiveAllCosts totals time by library entry over the first ARCHIVE_ALLCOSTS
// frames of every program that boots, on both images, into costs.txt: the calls and
// the mean cycles a call. Only programs whose calls to an entry agree are counted for
// that entry, so a program that ran further on one image doesn't skew it.
func TestArchiveAllCosts(t *testing.T) {
	frames, _ := strconv.Atoi(os.Getenv("ARCHIVE_ALLCOSTS"))
	if frames == 0 {
		t.Skip("ARCHIVE_ALLCOSTS gives the frames")
	}
	names := map[uint32]string{}
	if b, err := os.ReadFile("../../../../.build/cleanimage/map1.txt"); err == nil {
		for line := range strings.SplitSeq(string(b), "\n") {
			f := strings.Fields(line)
			if len(f) == 5 && strings.HasPrefix(f[4], ".fix.") {
				a, _ := strconv.ParseUint(f[0], 16, 32)
				names[uint32(a)] = strings.TrimPrefix(f[4], ".fix.")
			}
		}
	}
	rows, err := os.ReadFile(carchive + "summary.txt")
	if err != nil {
		t.Fatal(err)
	}
	type total struct {
		calls, sony, clean uint64
		programs           int
	}
	var mu sync.Mutex
	totals := map[uint32]*total{}
	t.Run("programs", func(t *testing.T) {
		for row := range strings.SplitSeq(strings.TrimSpace(string(rows)), "\n") {
			f := strings.Fields(row)
			if f[1] == "noboot" {
				continue
			}
			name := f[0]
			t.Run(name, func(t *testing.T) {
				t.Parallel()
				run := func(side string) map[uint32]entryCost {
					dir := carchive + name + "-" + side + "/"
					m := emutest.Boot(t, emutest.BIOS(emutest.BIOSFromEnv(t)), emutest.Disc(dir+name+"-"+side+".cue"), emutest.AllowFaults())
					m.RunUntilTTY("Execute !", 30*emutest.Second)
					return libraryCosts(m, frames)
				}
				sony, clean := run("sony"), run("clean")
				mu.Lock()
				defer mu.Unlock()
				for e, a := range sony {
					b, ok := clean[e]
					if !ok || e == 0 {
						continue
					}
					// Equal counts mean the same calls, near enough to compare.
					if a.calls != b.calls {
						continue
					}
					c := totals[e]
					if c == nil {
						c = &total{}
						totals[e] = c
					}
					c.calls += uint64(a.calls)
					c.sony += a.cycles
					c.clean += b.cycles
					c.programs++
				}
			})
		}
	})
	var entries []uint32
	for e := range totals {
		entries = append(entries, e)
	}
	diff := func(e uint32) int64 { return int64(totals[e].sony) - int64(totals[e].clean) }
	slices.SortFunc(entries, func(a, b uint32) int {
		return cmp.Compare(max(diff(b), -diff(b)), max(diff(a), -diff(a)))
	})
	var out strings.Builder
	fmt.Fprintf(&out, "%-24s %8s %5s %10s %10s %14s\n", "entry", "calls", "progs", "sony/call", "clean/call", "sony-clean")
	for _, e := range entries {
		c := totals[e]
		fmt.Fprintf(&out, "%-24s %8d %5d %10d %10d %14d\n", names[e]+fmt.Sprintf("/%x", e&0xfffff), c.calls, c.programs, c.sony/c.calls, c.clean/c.calls, diff(e))
	}
	if err := os.WriteFile(carchive+"costs.txt", []byte(out.String()), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestArchiveStart logs, for the programs in ARCHIVE_START on both images, the cycles
// from "Execute !" at each line printed and each entry into a library call up to the
// frame ARCHIVE_AT (2), and at the first instruction back in the program.
func TestArchiveStart(t *testing.T) {
	list := os.Getenv("ARCHIVE_START")
	if list == "" {
		t.Skip("ARCHIVE_START names the programs")
	}
	for _, name := range strings.Fields(list) {
		for _, side := range []string{"sony", "clean"} {
			dir := carchive + name + "-" + side + "/"
			m := emutest.Boot(t, emutest.BIOS(emutest.BIOSFromEnv(t)), emutest.Disc(dir+name+"-"+side+".cue"), emutest.AllowFaults())
			m.RunUntilTTY("Execute !", 30*emutest.Second)
			from, seen, frames := m.CPU.TotalCycles, len(m.TTY()), m.Frames()
			var out strings.Builder
			fmt.Fprintf(&out, "%s %s\n", name, side)
			inProgram := func(pc uint32) bool { return pc >= 0x80090000 && pc < 0x80200000 }
			was, next := false, from
			for n := 0; n < 1<<25 && m.Frames() < frames+8; n++ {
				if m.Machine.Step() != nil {
					break
				}
				if m.CPU.TotalCycles >= next {
					fmt.Fprintf(&out, "  %9d frame %d at %08x ra %08x\n", m.CPU.TotalCycles-from, m.Frames()-frames, m.CPU.PC, m.CPU.R[31])
					next += 150000
				}
				if tty := m.TTY(); len(tty) != seen && tty[len(tty)-1] == '\n' {
					fmt.Fprintf(&out, "  %9d frame %d tty %q\n", m.CPU.TotalCycles-from, m.Frames()-frames, tty[seen:])
					seen = len(tty)
				}
				now := inProgram(m.CPU.PC)
				if now != was {
					fmt.Fprintf(&out, "  %9d frame %d pc %08x program %v\n", m.CPU.TotalCycles-from, m.Frames()-frames, m.CPU.PC, now)
					was = now
				}
			}
			t.Log(out.String())
		}
	}
}

// TestArchiveKernel logs, for the programs in ARCHIVE_KERNEL on both images, the
// cycles from "Execute !" at each kernel call until the program's main code runs.
func TestArchiveKernel(t *testing.T) {
	list := os.Getenv("ARCHIVE_KERNEL")
	if list == "" {
		t.Skip("ARCHIVE_KERNEL names the programs")
	}
	for _, name := range strings.Fields(list) {
		sides := map[string][]string{}
		for _, side := range []string{"sony", "clean"} {
			dir := carchive + name + "-" + side + "/"
			m := emutest.Boot(t, emutest.BIOS(emutest.BIOSFromEnv(t)), emutest.Disc(dir+name+"-"+side+".cue"), emutest.AllowFaults())
			m.RunUntilTTY("Execute !", 30*emutest.Second)
			from := m.CPU.TotalCycles
			for n := 0; n < 1<<25; n++ {
				if m.Machine.Step() != nil {
					break
				}
				switch pc := m.CPU.PC; pc {
				case 0xa0, 0xb0, 0xc0:
					sides[side] = append(sides[side], fmt.Sprintf("%9d %02x:%02x", m.CPU.TotalCycles-from, pc, m.CPU.R[9]))
				}
				if m.CPU.TotalCycles-from > 3800000 {
					break
				}
			}
		}
		var out strings.Builder
		fmt.Fprintf(&out, "%s, sony then clean\n", name)
		a, b := sides["sony"], sides["clean"]
		for i := range max(len(a), len(b)) {
			var x, y string
			if i < len(a) {
				x = a[i]
			}
			if i < len(b) {
				y = b[i]
			}
			fmt.Fprintf(&out, "%-18s %s\n", x, y)
		}
		t.Log(out.String())
	}
}
