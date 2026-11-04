package tests

import (
	"cmp"
	"debug/elf"
	"fmt"
	"go.chrisrx.dev/psx/yaroze"
	"image"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"

	"go.chrisrx.dev/psx/emu"
	"go.chrisrx.dev/psx/emu/emutest"
)

// PROTOTYPE: programs built for the resident library, combined with the
// Go library's image in its place (PSX_LIBPS_IMAGE).
func TestImageHello(t *testing.T) {
	imageOnly(t)
	m := bootExample(t, "hello-img")
	m.RunUntilCount("main.screen", 30, 2*emutest.Second)
	t.Logf("pc %#x t8 %#x t9 %#x tty %q", m.CPU.PC, m.CPU.R[24], m.CPU.R[25], m.TTY())
	m.Golden("hello.png")
}

// TestImageTerraOriginal runs Team Fatal's TERRA.EXE, built with Sony's
// SDK, on Sony's library and on the Go library's image, pressing Start
// and Cross now and then, and compares the pictures.
func TestImageTerraOriginal(t *testing.T) {
	imageOnly(t)
	boot := func(v string) *emutest.Machine {
		m := emutest.Boot(t,
			emutest.BIOS(emutest.BIOSFromEnv(t)),
			emutest.Disc("../.build/terra-orig-"+v+"/terra-orig-"+v+".cue"),
		)
		m.RunUntilTTY("Execute !", 30*emutest.Second)
		return m
	}
	sony, gol := boot("sony"), boot("go")
	same := 0
	for i := range 150 {
		for _, m := range []*emutest.Machine{sony, gol} {
			if i%4 == 3 {
				m.Hold(1, emu.ButtonStart|emu.ButtonCross, 2)
				m.RunFrames(28)
			} else {
				m.RunFrames(30)
			}
		}
		if gol.CPU.R[24] == 0xdead0000 {
			t.Fatalf("frame %d: a function the Go library lacks, resident address low half %#x", 30*(i+1), gol.CPU.R[25])
		}
		a, b := sony.Display(), gol.Display()
		if a.Bounds() == b.Bounds() && string(a.Pix) == string(b.Pix) {
			same++
		} else {
			n, worst := 0, 0
			for k := range a.Pix {
				d := int(a.Pix[k]) - int(b.Pix[k])
				if d < 0 {
					d = -d
				}
				if d != 0 {
					n++
				}
				worst = max(worst, d)
			}
			t.Logf("%3d s: %d components differ, by up to %d", i+1, n, worst)
			if worst > 64 {
				emutest.Golden(t, b, fmt.Sprintf("zz-terra-go-%03d.png", i+1))
				emutest.Golden(t, a, fmt.Sprintf("zz-terra-sony-%03d.png", i+1))
			}
		}
	}
	t.Logf("%d of 150 pictures the same", same)
}

// TestImageFrameRates counts the program frames in 600 video frames under
// each library, from the main.screen counter.
func TestImageFrameRates(t *testing.T) {
	imageOnly(t)
	for _, name := range []string{"scene", "terra", "dungeon", "invaders"} {
		var out []string
		for _, dir := range []string{"../.build", "../.build/imgdiscs"} {
			for _, suffix := range []string{"", "-go"} {
				if dir != "../.build" && suffix != "" {
					continue
				}
				path := dir + "/" + name + suffix + "/" + name + suffix + ".cue"
				if _, err := os.Stat(path); err != nil {
					continue
				}
				m := emutest.Boot(t,
					emutest.BIOS(emutest.BIOSFromEnv(t)),
					emutest.Disc(path),
					emutest.Symbols(filepath.Dir(path)+"/prog.elf"),
				)
				m.RunUntilTTY("Execute !", 30*emutest.Second)
				m.RunFrames(120)
				a := m.Read32(m.Sym("main.screen"))
				m.RunFrames(600)
				b := m.Read32(m.Sym("main.screen"))
				out = append(out, fmt.Sprintf("%s: %d", filepath.Base(filepath.Dir(dir+"/"+name+suffix+"/x"))+map[bool]string{true: "(image)"}[dir != "../.build"], b-a))
			}
		}
		t.Log(name, out)
	}
}

// TestImageSceneLockstep finds the first frame at which scene's picture
// under the image departs from its picture under --libps go.
func TestImageSceneLockstep(t *testing.T) {
	imageOnly(t)
	boot := func(path string) *emutest.Machine {
		m := emutest.Boot(t,
			emutest.BIOS(emutest.BIOSFromEnv(t)),
			emutest.Disc(path),
			emutest.Symbols(filepath.Dir(path)+"/prog.elf"),
		)
		m.RunUntilTTY("Execute !", 30*emutest.Second)
		return m
	}
	for _, name := range []string{"scene", "terra"} {
		gom, img := boot("../.build/"+name+"-go/"+name+"-go.cue"), boot("../.build/"+name+"-img/"+name+"-img.cue")
		same := 0
		for n := uint32(5); n < 400; n += 5 {
			for _, m := range []*emutest.Machine{gom, img} {
				m.RunUntilCount("main.screen", n, 10*emutest.Second)
			}
			a, b := gom.Display(), img.Display()
			if string(a.Pix) != string(b.Pix) {
				emutest.Golden(t, a, "zz-"+name+"-go.png")
				emutest.Golden(t, b, "zz-"+name+"-img.png")
				t.Errorf("%s: program frame %d differs (%d matched)", name, n, same)
				break
			}
			same++
		}
		t.Logf("%s: %d pictures compared the same", name, same)
	}
}

// TestImageTerraOne runs the original Terra on one library alone
// (PSX_TERRA=sony or go), pressing as TestImageTerraOriginal does, and
// writes a picture every 15 seconds.
func TestImageTerraOne(t *testing.T) {
	imageOnly(t)
	v := os.Getenv("PSX_TERRA")
	m := emutest.Boot(t,
		emutest.BIOS(emutest.BIOSFromEnv(t)),
		emutest.Disc("../.build/terra-orig-"+v+"/terra-orig-"+v+".cue"),
	)
	m.RunUntilTTY("Execute !", 30*emutest.Second)
	for i := range 90 {
		if i%4 == 3 {
			m.Hold(1, emu.ButtonStart|emu.ButtonCross, 2)
			m.RunFrames(28)
		} else {
			m.RunFrames(30)
		}
		if i == 5 {
			t.Logf("tty: %q pc %#x", m.TTY(), m.CPU.PC)
		}
		if i%15 == 0 {
			emutest.Golden(t, m.Display(), fmt.Sprintf("zz-terra1-%s-%03d.png", v, i))
		}
	}
}

// TestImageTerraFirst steps the original Terra on both libraries a frame
// at a time with TestImageTerraOriginal's presses, and at the first frame
// whose picture differs compares the game's memory.
func TestImageTerraFirst(t *testing.T) {
	imageOnly(t)
	boot := func(v string) *emutest.Machine {
		m := emutest.Boot(t,
			emutest.BIOS(emutest.BIOSFromEnv(t)),
			emutest.Disc("../.build/terra-orig-"+v+"/terra-orig-"+v+".cue"),
		)
		m.RunUntilTTY("Execute !", 30*emutest.Second)
		return m
	}
	sony, gol := boot("sony"), boot("go")
	for f := range 30 * 30 {
		for _, m := range []*emutest.Machine{sony, gol} {
			if f%120 >= 90 && f%120 < 92 {
				m.Hold(1, emu.ButtonStart|emu.ButtonCross, 1)
			} else {
				m.RunFrames(1)
			}
		}
		a, b := sony.Display(), gol.Display()
		if string(a.Pix) == string(b.Pix) {
			continue
		}
		emutest.Golden(t, b, "zz-terra-first-go.png")
		emutest.Golden(t, a, "zz-terra-first-sony.png")
		n, first := 0, uint32(0)
		for addr := uint32(0x80100000); addr < 0x80180000; addr += 4 {
			if sony.Read32(addr) != gol.Read32(addr) {
				if n == 0 {
					first = addr
				}
				n++
				if n <= 12 {
					t.Logf("  %#x: sony %#08x go %#08x", addr, sony.Read32(addr), gol.Read32(addr))
				}
			}
		}
		t.Fatalf("frame %d: pictures differ; %d words of the game's memory differ, the first at %#x", f, n, first)
	}
}

// TestImageTerraMemory steps the original Terra on both libraries a frame
// at a time and reports the first frames at which the game's memory, from
// its load address to the end of its bss, differs.
func TestImageTerraMemory(t *testing.T) {
	imageOnly(t)
	boot := func(v string) *emutest.Machine {
		m := emutest.Boot(t,
			emutest.BIOS(emutest.BIOSFromEnv(t)),
			emutest.Disc("../.build/terra-orig-"+v+"/terra-orig-"+v+".cue"),
		)
		m.RunUntilTTY("Execute !", 30*emutest.Second)
		return m
	}
	sony, gol := boot("sony"), boot("go")
	seen := map[uint32]bool{}
	reports := 0
	for f := range 30 * 25 {
		for _, m := range []*emutest.Machine{sony, gol} {
			if f%120 >= 90 && f%120 < 92 {
				m.Hold(1, emu.ButtonStart|emu.ButtonCross, 1)
			} else {
				m.RunFrames(1)
			}
		}
		var fresh []string
		for addr := uint32(0x80100000); addr < 0x80180000; addr += 4 {
			a, b := sony.Read32(addr), gol.Read32(addr)
			if a != b && !seen[addr] {
				seen[addr] = true
				if len(fresh) < 10 {
					fresh = append(fresh, fmt.Sprintf("%#x: %#x/%#x", addr, a, b))
				}
			}
		}
		if len(fresh) > 0 {
			t.Logf("frame %d (pc %#x/%#x): %v", f, sony.CPU.PC, gol.CPU.PC, fresh)
			reports++
			if reports == 25 {
				return
			}
		}
	}
}

// TestImageTerraCommands runs the original Terra on both libraries to
// PSX_TERRA_AT seconds with TestImageTerraOriginal's presses, then records
// one frame's GP0 words from each and reports where the streams part.
func TestImageTerraCommands(t *testing.T) {
	imageOnly(t)
	at, _ := strconv.Atoi(os.Getenv("PSX_TERRA_AT"))
	boot := func(v string) *emutest.Machine {
		m := emutest.Boot(t,
			emutest.BIOS(emutest.BIOSFromEnv(t)),
			emutest.Disc("../.build/terra-orig-"+v+"/terra-orig-"+v+".cue"),
		)
		m.RunUntilTTY("Execute !", 30*emutest.Second)
		return m
	}
	sony, gol := boot("sony"), boot("go")
	for i := range at {
		for _, m := range []*emutest.Machine{sony, gol} {
			if i%4 == 3 {
				m.Hold(1, emu.ButtonStart|emu.ButtonCross, 2)
				m.RunFrames(28)
			} else {
				m.RunFrames(30)
			}
		}
	}
	var words [2][]uint32
	for k, m := range []*emutest.Machine{sony, gol} {
		m.OnGP0(func(v uint32) { words[k] = append(words[k], v) })
		m.RunFrames(1)
		m.OnGP0(nil)
	}
	a, b := words[0], words[1]
	t.Logf("sony %d words, go %d words; pictures same: %v", len(a), len(b), string(sony.Display().Pix) == string(gol.Display().Pix))
	var out strings.Builder
	for _, w := range a {
		fmt.Fprintf(&out, "%08x\n", w)
	}
	os.WriteFile("../.build/image/gp0-sony.txt", []byte(out.String()), 0o644)
	out.Reset()
	for _, w := range b {
		fmt.Fprintf(&out, "%08x\n", w)
	}
	os.WriteFile("../.build/image/gp0-go.txt", []byte(out.String()), 0o644)
}

// terraFuncs are the bindings of the library calls the original Terra
// makes, for their parameters' types.
var terraFuncs = map[string]any{"GsSortSprite": yaroze.GsSortSprite, "GsSortFastSprite": yaroze.GsSortFastSprite, "GsSortBoxFill": yaroze.GsSortBoxFill, "GsGetLs": yaroze.GsGetLs, "GsScaleScreen": yaroze.GsScaleScreen, "GsSortObject4": yaroze.GsSortObject4, "ApplyMatrix": yaroze.ApplyMatrix, "ApplyMatrixSV": yaroze.ApplyMatrixSV, "CdReadFile": yaroze.CdReadFile, "CdReadSync": yaroze.CdReadSync, "CdSearchFile": yaroze.CdSearchFile, "ClearImage": yaroze.ClearImage, "DrawSync": yaroze.DrawSync, "EnterCriticalSection": yaroze.EnterCriticalSection, "ExitCriticalSection": yaroze.ExitCriticalSection, "FntFlush": yaroze.FntFlush, "FntLoad": yaroze.FntLoad, "FntOpen": yaroze.FntOpen, "GetPadBuf": yaroze.GetPadBuf, "GsClearOt": yaroze.GsClearOt, "GsDefDispBuff": yaroze.GsDefDispBuff, "GsDrawOt": yaroze.GsDrawOt, "GsGetActiveBuff": yaroze.GsGetActiveBuff, "GsGetLws": yaroze.GsGetLws, "GsGetTimInfo": yaroze.GsGetTimInfo, "GsInit3D": yaroze.GsInit3D, "GsInitGraph": yaroze.GsInitGraph, "GsSetAmbient": yaroze.GsSetAmbient, "GsSetFlatLight": yaroze.GsSetFlatLight, "GsSetLightMatrix": yaroze.GsSetLightMatrix, "GsSetLsMatrix": yaroze.GsSetLsMatrix, "GsSetProjection": yaroze.GsSetProjection, "GsSetRefView2": yaroze.GsSetRefView2, "GsSetWorkBase": yaroze.GsSetWorkBase, "GsSwapDispBuff": yaroze.GsSwapDispBuff, "LoadImage": yaroze.LoadImage, "MoveImage": yaroze.MoveImage, "PopMatrix": yaroze.PopMatrix, "PushMatrix": yaroze.PushMatrix, "ResetGraph": yaroze.ResetGraph, "RotMatrix": yaroze.RotMatrix, "RotMatrixZ": yaroze.RotMatrixZ, "ScaleMatrix": yaroze.ScaleMatrix, "SsSeqClose": yaroze.SsSeqClose, "SsSeqGetVol": yaroze.SsSeqGetVol, "SsSeqOpen": yaroze.SsSeqOpen, "SsSeqPlay": yaroze.SsSeqPlay, "SsSeqSetVol": yaroze.SsSeqSetVol, "SsSeqStop": yaroze.SsSeqStop, "SsSetMVol": yaroze.SsSetMVol, "SsUtAllKeyOff": yaroze.SsUtAllKeyOff, "SsUtKeyOn": yaroze.SsUtKeyOn, "SsUtReverbOn": yaroze.SsUtReverbOn, "SsUtSetReverbDepth": yaroze.SsUtSetReverbDepth, "SsUtSetReverbType": yaroze.SsUtSetReverbType, "SsVabTransfer": yaroze.SsVabTransfer, "StoreImage": yaroze.StoreImage, "VSync": yaroze.VSync}

// mipsSize is the size of a value of type t on the console.
func mipsSize(t reflect.Type) uint32 {
	switch t.Kind() {
	case reflect.Pointer, reflect.UnsafePointer, reflect.Uintptr, reflect.Int, reflect.Uint, reflect.Func:
		return 4
	case reflect.Array:
		return uint32(t.Len()) * mipsSize(t.Elem())
	case reflect.Struct:
		var n, align uint32 = 0, 1
		for i := range t.NumField() {
			f := t.Field(i).Type
			a := min(mipsSize(f), 4)
			if f.Kind() == reflect.Array || f.Kind() == reflect.Struct {
				a = mipsAlign(f)
			}
			align = max(align, a)
			n = (n + a - 1) &^ (a - 1)
			n += mipsSize(f)
		}
		return (n + align - 1) &^ (align - 1)
	}
	return uint32(t.Size())
}

func mipsAlign(t reflect.Type) uint32 {
	switch t.Kind() {
	case reflect.Array:
		return mipsAlign(t.Elem())
	case reflect.Struct:
		a := uint32(1)
		for i := range t.NumField() {
			a = max(a, mipsAlign(t.Field(i).Type))
		}
		return a
	}
	return min(mipsSize(t), 4)
}

// terraCalls steps m until it has made n calls from the game into the
// library, and describes each: the arguments its binding takes, the bytes
// its pointer arguments reach, before and after, and its result.
func terraCalls(t *testing.T, m *emutest.Machine, names map[uint32]string, start string, n int) []string {
	var out []string
	type pending struct {
		desc   string
		ra, sp uint32
		ptrs   [][2]uint32
		masks  [][]bool
		result bool
	}
	var open []pending
	bytesAt := func(p, size uint32) string {
		var b strings.Builder
		for k := uint32(0); k < size; k++ {
			fmt.Fprintf(&b, "%02x", m.Bus.Read8(p+k))
		}
		return b.String()
	}
	for steps := 0; steps < 400_000_000 && len(out) < n; steps++ {
		pc := m.CPU.PC
		if len(open) > 0 && pc == open[0].ra && m.CPU.R[29] == open[0].sp {
			p := open[0]
			open = open[:0]
			s := p.desc
			if v := m.CPU.R[2]; p.result && v >= 0x80010000 && v < 0x80090000 {
				s += " -> (into the library)"
			} else if p.result {
				s += fmt.Sprintf(" -> %x", v)
			}
			for _, q := range p.ptrs {
				if strings.HasPrefix(p.desc, "GetPadBuf") || strings.HasPrefix(p.desc, "CdReadFile") {
					s += " | (into the library)"
					continue
				}
				s += " | " + maskBytes(bytesAt(q[0], q[1]), p.masks[indexOf(p.ptrs, q)])
			}
			out = append(out, s)
		}
		if name, ok := names[pc]; ok && len(open) == 0 {
			ra := m.CPU.R[31]
			if ra >= 0x80090000 && ra < 0x80200000 {
				d := name
				p := pending{ra: ra, sp: m.CPU.R[29]}
				if fn, ok := terraFuncs[name]; ok {
					ft := reflect.TypeOf(fn)
					p.result = ft.NumOut() > 0
					for i := range min(ft.NumIn(), 4) {
						v := m.CPU.R[4+i]
						pt := ft.In(i)
						if pt.Kind() == reflect.Pointer && v >= 0x80000000 && v < 0x80200000 {
							size := mipsSize(pt.Elem())
							d += " [ptr]"
							p.ptrs = append(p.ptrs, [2]uint32{v, size})
							p.masks = append(p.masks, padMask(pt.Elem()))
						} else {
							d += fmt.Sprintf(" %x", v)
						}
					}
				} else if c, ok := cSigs[name]; ok {
					p.result = c.result
					for i := range c.args {
						d += fmt.Sprintf(" %x", m.CPU.R[4+i])
					}
				} else {
					p.result = true
					d += fmt.Sprintf(" %x %x %x %x", m.CPU.R[4], m.CPU.R[5], m.CPU.R[6], m.CPU.R[7])
				}
				p.desc = d
				open = append(open, p)
			}
		}
		m.Step()
	}
	return out
}

// TestImageTerraCalls compares the original Terra's library calls under
// the two libraries, from a GsClearOt at PSX_TERRA_AT seconds.
func TestImageTerraCalls(t *testing.T) {
	imageOnly(t)
	at, _ := strconv.Atoi(os.Getenv("PSX_TERRA_AT"))
	names := map[uint32]string{}
	src, _ := os.ReadFile("../internal/toolchain/_tinygo/targets/playstation-yaroze-symusr-orig.S")
	for _, line := range strings.Split(string(src), "\n") {
		f := strings.Fields(strings.ReplaceAll(line, ",", " "))
		if len(f) == 3 && f[0] == ".set" && strings.HasSuffix(f[1], "_orig") {
			v, err := strconv.ParseUint(f[2], 0, 32)
			if err == nil {
				names[uint32(v)] = strings.TrimSuffix(f[1], "_orig")
			}
		}
	}
	boot := func(v string) *emutest.Machine {
		m := emutest.Boot(t,
			emutest.BIOS(emutest.BIOSFromEnv(t)),
			emutest.Disc("../.build/terra-orig-"+v+"/terra-orig-"+v+".cue"),
		)
		m.RunUntilTTY("Execute !", 30*emutest.Second)
		return m
	}
	sony, gol := boot("sony"), boot("go")
	for i := range at {
		for _, m := range []*emutest.Machine{sony, gol} {
			if i%4 == 3 {
				m.Hold(1, emu.ButtonStart|emu.ButtonCross, 2)
				m.RunFrames(28)
			} else {
				m.RunFrames(30)
			}
		}
	}
	a := terraCalls(t, sony, names, "GsClearOt", 3000)
	b := terraCalls(t, gol, names, "GsClearOt", 3000)
	os.WriteFile("../.build/image/calls-sony.txt", []byte(strings.Join(a, "\n")+"\n"), 0o644)
	os.WriteFile("../.build/image/calls-go.txt", []byte(strings.Join(b, "\n")+"\n"), 0o644)
	for i := range min(len(a), len(b)) {
		if a[i] != b[i] {
			t.Logf("call %d differs:\nsony %s\ngo   %s", i, a[i], b[i])
			return
		}
	}
	t.Logf("%d calls the same", min(len(a), len(b)))
}

// TestImageTerraGameFrames runs the original Terra on each library with
// the pad driven by the game's own frames, each GsSwapDispBuff call one,
// and compares the pictures frame for frame.
func TestImageTerraGameFrames(t *testing.T) {
	imageOnly(t)
	const swap = 0x80021920
	frames, _ := strconv.Atoi(os.Getenv("PSX_TERRA_FRAMES"))
	var vbl [2][]uint64
	run := func(v string) [][]byte {
		k := map[string]int{"sony": 0, "go": 1}[v]
		m := emutest.Boot(t,
			emutest.BIOS(emutest.BIOSFromEnv(t)),
			emutest.Disc("../.build/terra-orig-"+v+"/terra-orig-"+v+".cue"),
		)
		m.RunUntilTTY("Execute !", 30*emutest.Second)
		var pics [][]byte
		for g := range frames {
			m.RunUntil(func() bool {
				return m.CPU.PC == swap && m.CPU.R[31] >= 0x80100000 && m.CPU.R[31] < 0x80180000
			}, 10*emutest.Second, "a swap")
			m.Step()
			vbl[k] = append(vbl[k], m.Frames())
			if g%120 >= 90 && g%120 < 92 {
				m.Pad(1).Press(emu.ButtonStart | emu.ButtonCross)
			} else {
				m.Pad(1).Release(emu.ButtonStart | emu.ButtonCross)
			}
			pics = append(pics, append([]byte(nil), m.Display().Pix...))
		}
		return pics
	}
	a, b := run("sony"), run("go")
	if os.Getenv("PSX_TERRA_SAVE") != "" {
		for g := 500; g < frames; g += 1000 {
			emutest.Golden(t, pic(b[g]), fmt.Sprintf("zz-gf-go-%04d.png", g))
		}
	}
	start, worst := -1, 0
	for g := range frames + 1 {
		n := 0
		if g < frames {
			for k := range a[g] {
				if a[g][k] != b[g][k] {
					n++
				}
			}
		}
		switch {
		case n > 0 && start < 0:
			start, worst = g, n
		case n > 0:
			worst = max(worst, n)
		case start >= 0:
			t.Logf("game frames %d to %d differ, by up to %d components; video frames sony %v go %v", start, g-1, worst, vbl[0][max(start-3, 0):g], vbl[1][max(start-3, 0):g])
			if os.Getenv("PSX_TERRA_SAVE") != "" {
				emutest.Golden(t, pic(a[start]), fmt.Sprintf("zz-gf-sony-%04d.png", start))
				emutest.Golden(t, pic(b[start]), fmt.Sprintf("zz-gf-go-%04d.png", start))
			}
			start = -1
		}
	}
}

// pic is a 320x240 RGBA picture from its pixels.
func pic(pix []byte) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, 320, len(pix)/4/320))
	copy(img.Pix, pix)
	return img
}

// terraToGameFrame boots the original Terra on library v and runs it to
// game frame g, with the pad driven as TestImageTerraGameFrames drives it.
func terraToGameFrame(t *testing.T, v string, g int) *emutest.Machine {
	const swap = 0x80021920
	m := emutest.Boot(t,
		emutest.BIOS(emutest.BIOSFromEnv(t)),
		emutest.Disc("../.build/terra-orig-"+v+"/terra-orig-"+v+".cue"),
	)
	m.RunUntilTTY("Execute !", 30*emutest.Second)
	for f := range g {
		m.RunUntil(func() bool {
			return m.CPU.PC == swap && m.CPU.R[31] >= 0x80100000 && m.CPU.R[31] < 0x80180000
		}, 10*emutest.Second, "a swap")
		m.Step()
		if f%120 >= 90 && f%120 < 92 {
			m.Pad(1).Press(emu.ButtonStart | emu.ButtonCross)
		} else {
			m.Pad(1).Release(emu.ButtonStart | emu.ButtonCross)
		}
	}
	return m
}

// TestImageTerraGameCalls compares the library calls of the original Terra
// on the two libraries from game frame PSX_TERRA_FRAMES.
func TestImageTerraGameCalls(t *testing.T) {
	imageOnly(t)
	g, _ := strconv.Atoi(os.Getenv("PSX_TERRA_FRAMES"))
	calls := cmp.Or(atoi(os.Getenv("PSX_TERRA_CALLS")), 4000)
	names := terraNames()
	a := terraCalls(t, terraToGameFrame(t, "sony", g), names, "", calls)
	b := terraCalls(t, terraToGameFrame(t, "go", g), names, "", calls)
	os.WriteFile("../.build/image/calls-sony.txt", []byte(strings.Join(a, "\n")+"\n"), 0o644)
	os.WriteFile("../.build/image/calls-go.txt", []byte(strings.Join(b, "\n")+"\n"), 0o644)
	for i := range min(len(a), len(b)) {
		if a[i] != b[i] {
			t.Logf("call %d differs:\nsony %s\ngo   %s", i, a[i], b[i])
			return
		}
	}
	t.Logf("%d calls the same", min(len(a), len(b)))
}

// terraNames maps the resident library's entry points to their names.
func terraNames() map[uint32]string {
	names := map[uint32]string{}
	src, _ := os.ReadFile("../internal/toolchain/_tinygo/targets/playstation-yaroze-symusr-orig.S")
	for _, line := range strings.Split(string(src), "\n") {
		f := strings.Fields(strings.ReplaceAll(line, ",", " "))
		if len(f) == 3 && f[0] == ".set" && strings.HasSuffix(f[1], "_orig") {
			v, err := strconv.ParseUint(f[2], 0, 32)
			if err == nil {
				names[uint32(v)] = strings.TrimSuffix(f[1], "_orig")
			}
		}
	}
	return names
}

func atoi(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}

// cSigs are the argument counts and results of the C library calls the
// original Terra makes, which have no bindings.
var cSigs = map[string]struct {
	args   int
	result bool
}{
	"InitHeap": {2, false}, "SsSetTickMode": {1, false}, "rand": {0, true},
	"bcopy": {3, false}, "bzero": {2, false}, "malloc": {1, true}, "free": {1, false},
	"memset": {3, true}, "strlen": {1, true}, "FntPrint": {1, true},
}

// padMask marks the bytes of a value of type t that belong to fields
// named Pad, which hold whatever was in memory and mean nothing.
func padMask(t reflect.Type) []bool {
	mask := make([]bool, mipsSize(t))
	var walk func(t reflect.Type, at uint32, pad bool)
	walk = func(t reflect.Type, at uint32, pad bool) {
		switch t.Kind() {
		case reflect.Struct:
			off := uint32(0)
			for i := range t.NumField() {
				f := t.Field(i)
				a := mipsAlign(f.Type)
				off = (off + a - 1) &^ (a - 1)
				walk(f.Type, at+off, pad || strings.HasPrefix(f.Name, "Pad"))
				off += mipsSize(f.Type)
			}
		case reflect.Array:
			n := mipsSize(t.Elem())
			for i := range t.Len() {
				walk(t.Elem(), at+uint32(i)*n, pad)
			}
		default:
			if pad {
				for k := range mipsSize(t) {
					mask[at+k] = true
				}
			}
		}
	}
	walk(t, 0, false)
	return mask
}

// maskBytes replaces the masked bytes of hex, two digits a byte, by "..".
func maskBytes(hex string, mask []bool) string {
	b := []byte(hex)
	for i, m := range mask {
		if m && 2*i+1 < len(b) {
			b[2*i], b[2*i+1] = '.', '.'
		}
	}
	return string(b)
}

func indexOf(ptrs [][2]uint32, q [2]uint32) int {
	for i, p := range ptrs {
		if p == q {
			return i
		}
	}
	return 0
}

// imageOnly skips the prototype's tests unless PSX_IMAGE_TESTS is set.
func imageOnly(t *testing.T) {
	if os.Getenv("PSX_IMAGE_TESTS") == "" {
		t.Skip("set PSX_IMAGE_TESTS to run the library image prototype's tests")
	}
}

// TestImageArchive runs every program exp/libps/image/proto/archive.sh
// built from the web archive, on Sony's library and on the Go library's
// image, for 20 seconds with Start and Cross pressed every few, and
// compares their pictures a second apart.
func TestImageArchive(t *testing.T) {
	imageOnly(t)
	missingAt := uint32(0)
	if ef, err := elf.Open("../.build/image/image.elf"); err == nil {
		if syms, err := ef.Symbols(); err == nil {
			for _, s := range syms {
				if s.Name == "libps_image_missing" {
					missingAt = uint32(s.Value)
				}
			}
		}
		ef.Close()
	}
	names := terraNames()
	rows, _ := os.ReadFile("../.build/archive/programs.tsv")
	var summary []string
	var mu sync.Mutex
	t.Run("programs", func(t *testing.T) {
		for _, row := range strings.Split(strings.TrimSpace(string(rows)), "\n") {
			name := strings.Split(row, "\t")[0]
			t.Run(name, func(t *testing.T) {
				t.Parallel()
				run := func(v string) (pics [][]byte, note string) {
					cue := "../.build/archive/" + name + "-" + v + "/" + name + "-" + v + ".cue"
					if _, err := os.Stat(cue); err != nil {
						return nil, "no disc"
					}
					ok := t.Run(v, func(t *testing.T) {
						m := emutest.Boot(t, emutest.BIOS(emutest.BIOSFromEnv(t)), emutest.Disc(cue))
						m.RunUntilTTY("Execute !", 30*emutest.Second)
						for i := range 20 {
							if i%4 == 3 {
								m.Hold(1, emu.ButtonStart|emu.ButtonCross, 2)
								m.RunFrames(58)
							} else {
								m.RunFrames(60)
							}
							pics = append(pics, append([]byte(nil), m.Display().Pix...))
							if v == "go" && missingAt != 0 && m.CPU.PC >= missingAt && m.CPU.PC < missingAt+16 {
								note = fmt.Sprintf("calls %s, which the image lacks", names[0x80000000|m.CPU.R[25]&0xffff|lookupHigh(names, m.CPU.R[25])])
								return
							}
						}
					})
					if !ok && note == "" {
						note = "faulted"
					}
					return pics, note
				}
				a, na := run("sony")
				b, nb := run("go")
				same, first := 0, -1
				for i := range min(len(a), len(b)) {
					if string(a[i]) == string(b[i]) {
						same++
					} else if first < 0 {
						first = i + 1
					}
				}
				blank := true
				for _, p := range a {
					for k := 0; k < len(p); k += 4 {
						if p[k]|p[k+1]|p[k+2] != 0 {
							blank = false
							break
						}
					}
				}
				line := fmt.Sprintf("%-50s sony:%-10s go:%-40s same %2d/%d first diff %3d blank(sony) %v", name, cmp.Or(na, "ok"), cmp.Or(nb, "ok"), same, min(len(a), len(b)), first, blank)
				mu.Lock()
				summary = append(summary, line)
				mu.Unlock()
			})
		}
	})
	slices.Sort(summary)
	os.WriteFile("../.build/archive/summary.txt", []byte(strings.Join(summary, "\n")+"\n"), 0o644)
}

// lookupHigh finds the high half of the resident address whose low half
// is lo, for naming a call the image lacks.
func lookupHigh(names map[uint32]string, lo uint32) uint32 {
	for a := range names {
		if a&0xffff == lo&0xffff {
			return a &^ 0x8000ffff
		}
	}
	return 0
}

// TestImageArchiveOne writes the pictures of archive program PSX_ARCHIVE
// on both libraries after PSX_ARCHIVE_AT seconds, as zz-arch-V.png.
func TestImageArchiveOne(t *testing.T) {
	imageOnly(t)
	name := os.Getenv("PSX_ARCHIVE")
	at := cmp.Or(atoi(os.Getenv("PSX_ARCHIVE_AT")), 1)
	for _, v := range []string{"sony", "go"} {
		m := emutest.Boot(t, emutest.BIOS(emutest.BIOSFromEnv(t)), emutest.Disc("../.build/archive/"+name+"-"+v+"/"+name+"-"+v+".cue"))
		m.RunUntilTTY("Execute !", 30*emutest.Second)
		m.RunFrames(60 * at)
		emutest.Golden(t, m.Display(), "zz-arch-"+v+".png")
		t.Logf("%s: pc %#x ra %#x %q", v, m.CPU.PC, m.CPU.R[31], m.TTY()[max(0, len(m.TTY())-300):])
	}
}

// TestImageArchiveWild runs archive program PSX_ARCHIVE on the image until
// the PC leaves memory, and shows the instructions that took it there.
func TestImageArchiveWild(t *testing.T) {
	imageOnly(t)
	name := os.Getenv("PSX_ARCHIVE")
	m := emutest.Boot(t, emutest.BIOS(emutest.BIOSFromEnv(t)), emutest.Disc("../.build/archive/"+name+"-go/"+name+"-go.cue"))
	m.RunUntilTTY("Execute !", 30*emutest.Second)
	if os.Getenv("PSX_WATCH_SP_LOW") != "" {
		m.RunUntil(func() bool {
			sp := m.CPU.R[29]
			return sp >= 0x80010000 && sp < 0x80090000 && (sp < 0x8004d550 || sp > 0x8004e540)
		}, 20*emutest.Second, "the stack in the library")
	} else if r := atoi(os.Getenv("PSX_WATCH_REG")); r != 0 {
		v := uint32(atoi(os.Getenv("PSX_WATCH_VAL")))
		lo, _ := strconv.ParseUint(cmp.Or(os.Getenv("PSX_WATCH_FROM"), "0"), 0, 32)
		hi, _ := strconv.ParseUint(cmp.Or(os.Getenv("PSX_WATCH_TO"), "0xffffffff"), 0, 32)
		m.RunUntil(func() bool {
			return m.CPU.R[r] == v && m.CPU.PC >= uint32(lo) && m.CPU.PC < uint32(hi)
		}, 20*emutest.Second, "the register's value")
	} else {
		m.RunUntil(func() bool {
			pc := m.CPU.PC
			return pc >= 0x10000 && pc < 0x80000000 || pc >= 0x80200000 && pc < 0xbfc00000 || pc >= 0xbfc80000
		}, 20*emutest.Second, "the PC leaving memory")
	}
	m.RunUntil(func() bool { return false }, 1, "a report")
}

// TestImageArchivePhase looks for the frame of archive program
// PSX_ARCHIVE on Sony's library, within 30 of PSX_ARCHIVE_AT seconds,
// whose picture is the image's at that time exactly.
func TestImageArchivePhase(t *testing.T) {
	imageOnly(t)
	name := os.Getenv("PSX_ARCHIVE")
	at := 60 * cmp.Or(atoi(os.Getenv("PSX_ARCHIVE_AT")), 3)
	boot := func(v string) *emutest.Machine {
		m := emutest.Boot(t, emutest.BIOS(emutest.BIOSFromEnv(t)), emutest.Disc("../.build/archive/"+name+"-"+v+"/"+name+"-"+v+".cue"))
		m.RunUntilTTY("Execute !", 30*emutest.Second)
		return m
	}
	g := boot("go")
	g.RunFrames(at)
	want := string(g.Display().Pix)
	s := boot("sony")
	s.RunFrames(at - 30)
	best, bestD, box := -1, 0, [4]int{}
	for d := -30; d <= 30; d++ {
		pix := s.Display().Pix
		if len(pix) == len(want) {
			n, b := 0, [4]int{1 << 30, 1 << 30, -1, -1}
			w := s.Display().Bounds().Dx()
			for k := 0; k < len(pix); k += 4 {
				if pix[k] != want[k] || pix[k+1] != want[k+1] || pix[k+2] != want[k+2] {
					n++
					x, y := k/4%w, k/4/w
					b = [4]int{min(b[0], x), min(b[1], y), max(b[2], x), max(b[3], y)}
				}
			}
			if best < 0 || n < best {
				best, bestD, box = n, d, b
			}
		}
		s.RunFrames(1)
	}
	t.Logf("%s: closest is Sony's frame %d (%+d), %d pixels differ in %v", name, at+bestD, bestD, best, box)
}

// TestImageArchiveInit reports the video frames at which the library's
// init starts and returns, for archive program PSX_ARCHIVE on each library.
func TestImageArchiveInit(t *testing.T) {
	imageOnly(t)
	name := os.Getenv("PSX_ARCHIVE")
	for _, v := range []string{"sony", "go"} {
		m := emutest.Boot(t, emutest.BIOS(emutest.BIOSFromEnv(t)), emutest.Disc("../.build/archive/"+name+"-"+v+"/"+name+"-"+v+".cue"))
		m.RunUntilTTY("Execute !", 30*emutest.Second)
		m.RunUntil(func() bool { return m.CPU.PC == 0x80010030 }, 10*emutest.Second, "the init")
		start, ra := m.Frames(), m.CPU.R[31]
		m.RunUntil(func() bool { return m.CPU.PC == ra }, 10*emutest.Second, "the init returning")
		t.Logf("%s: init from frame %d to %d", v, start, m.Frames())
	}
}

// TestImageInitSteps reports the video frame after each call of Sony's
// init routine, for archive program PSX_ARCHIVE.
func TestImageInitSteps(t *testing.T) {
	imageOnly(t)
	name := os.Getenv("PSX_ARCHIVE")
	m := emutest.Boot(t, emutest.BIOS(emutest.BIOSFromEnv(t)), emutest.Disc("../.build/archive/"+name+"-sony/"+name+"-sony.cue"))
	m.RunUntilTTY("Execute !", 30*emutest.Second)
	steps := []struct {
		pc   uint32
		what string
	}{{0x80010030, "start"}, {0x80010040, "ResetCallback"}, {0x80010048, "SetDispMask"}, {0x80010050, "ResetGraph"}, {0x80010058, "SetGraphDebug"}, {0x80010074, "InitPAD"}, {0x8001007c, "CdInit"}, {0x8001008c, "SsInit"}, {0x800100a0, "SsSetTableSize"}, {0x800100a8, "SsSetTickMode"}, {0x800100b0, "SsStart"}, {0x800100b8, "card events"}}
	for _, s := range steps {
		m.RunUntil(func() bool { return m.CPU.PC == s.pc }, 10*emutest.Second, s.what)
		t.Logf("%-15s frame %d cycles %d", s.what, m.Frames(), m.Cycles())
	}
}

// TestImageAllocs counts the callers of the image's runtime.alloc, at
// PSX_ALLOC, over the first seconds of archive program PSX_ARCHIVE.
func TestImageAllocs(t *testing.T) {
	imageOnly(t)
	name := os.Getenv("PSX_ARCHIVE")
	at, _ := strconv.ParseUint(os.Getenv("PSX_ALLOC"), 0, 32)
	m := emutest.Boot(t, emutest.BIOS(emutest.BIOSFromEnv(t)), emutest.Disc("../.build/archive/"+name+"-go/"+name+"-go.cue"))
	m.RunUntilTTY("Execute !", 30*emutest.Second)
	callers := map[[2]uint32]int{}
	n := 0
	m.RunUntil(func() bool {
		if m.CPU.PC == uint32(at) {
			callers[[2]uint32{m.CPU.R[31], m.CPU.R[4]}]++
			n++
		}
		return n >= 2000
	}, 8*emutest.Second, "allocations")
	for k, v := range callers {
		t.Logf("from %#x, %d bytes: %d times", k[0], k[1], v)
	}
}

// TestImageWatchWord runs archive program PSX_ARCHIVE on the image until
// the word at PSX_WATCH_ADDR changes, and shows the instructions before.
func TestImageWatchWord(t *testing.T) {
	imageOnly(t)
	name := os.Getenv("PSX_ARCHIVE")
	addr, _ := strconv.ParseUint(os.Getenv("PSX_WATCH_ADDR"), 0, 32)
	m := emutest.Boot(t, emutest.BIOS(emutest.BIOSFromEnv(t)), emutest.Disc("../.build/archive/"+name+"-go/"+name+"-go.cue"))
	m.RunUntilTTY("Execute !", 30*emutest.Second)
	was := m.Read32(uint32(addr))
	t.Logf("at Execute: %#x, pc %#x", was, m.CPU.PC)
	m.RunUntil(func() bool { return m.Read32(uint32(addr)) != was }, 20*emutest.Second, "the word changing")
	t.Logf("%#x: %#x -> %#x", addr, was, m.Read32(uint32(addr)))
	m.RunUntil(func() bool { return false }, 1, "a report")
}

// TestImageCalls logs the first calls to the image's address PSX_CALL by
// archive program PSX_ARCHIVE, with their arguments.
func TestImageCalls(t *testing.T) {
	imageOnly(t)
	name := os.Getenv("PSX_ARCHIVE")
	at, _ := strconv.ParseUint(os.Getenv("PSX_CALL"), 0, 32)
	m := emutest.Boot(t, emutest.BIOS(emutest.BIOSFromEnv(t)), emutest.Disc("../.build/archive/"+name+"-go/"+name+"-go.cue"))
	m.RunUntilTTY("Execute !", 30*emutest.Second)
	for range 6 {
		m.RunUntil(func() bool { return m.CPU.PC == uint32(at) }, 10*emutest.Second, "the call")
		t.Logf("a0 %#x a1 %#x a2 %#x a3 %#x ra %#x", m.CPU.R[4], m.CPU.R[5], m.CPU.R[6], m.CPU.R[7], m.CPU.R[31])
		m.Step()
	}
}

// TestImageArchiveCalls compares archive program PSX_ARCHIVE's library
// calls on the two libraries, from PSX_ARCHIVE_AT seconds.
func TestImageArchiveCalls(t *testing.T) {
	imageOnly(t)
	name := os.Getenv("PSX_ARCHIVE")
	at := cmp.Or(atoi(os.Getenv("PSX_ARCHIVE_AT")), 3)
	calls := cmp.Or(atoi(os.Getenv("PSX_TERRA_CALLS")), 3000)
	names := terraNames()
	run := func(v string) []string {
		m := emutest.Boot(t, emutest.BIOS(emutest.BIOSFromEnv(t)), emutest.Disc("../.build/archive/"+name+"-"+v+"/"+name+"-"+v+".cue"))
		m.RunUntilTTY("Execute !", 30*emutest.Second)
		m.RunFrames(60 * at)
		return terraCalls(t, m, names, "", calls)
	}
	a, b := run("sony"), run("go")
	os.WriteFile("../.build/image/calls-sony.txt", []byte(strings.Join(a, "\n")+"\n"), 0o644)
	os.WriteFile("../.build/image/calls-go.txt", []byte(strings.Join(b, "\n")+"\n"), 0o644)
}

// TestImageSortPackets records the packets each GsSortObject4 call of
// archive program PSX_ARCHIVE writes, from PSX_ARCHIVE_AT seconds, on both
// libraries, into .build/image/sort-V.txt: the call's arguments, the
// object's words, and the packet words from GsOUT_PACKET_P before to after.
func TestImageSortPackets(t *testing.T) {
	imageOnly(t)
	const sort, packetP = 0x80023E58, 0x80063D40
	name := os.Getenv("PSX_ARCHIVE")
	at := cmp.Or(atoi(os.Getenv("PSX_ARCHIVE_AT")), 4)
	for _, v := range []string{"sony", "go"} {
		m := emutest.Boot(t, emutest.BIOS(emutest.BIOSFromEnv(t)), emutest.Disc("../.build/archive/"+name+"-"+v+"/"+name+"-"+v+".cue"))
		m.RunUntilTTY("Execute !", 30*emutest.Second)
		m.RunFrames(60 * at)
		var out strings.Builder
		for range 40 {
			m.RunUntil(func() bool { return m.CPU.PC == sort }, 5*emutest.Second, "a sort")
			obj, ra, sp := m.CPU.R[4], m.CPU.R[31], m.CPU.R[29]
			fmt.Fprintf(&out, "call obj %x ot %x shift %x scratch %x\n  obj:", obj, m.CPU.R[5], m.CPU.R[6], m.CPU.R[7])
			for k := uint32(0); k < 4; k++ {
				fmt.Fprintf(&out, " %08x", m.Read32(obj+4*k))
			}
			g := m.CPU.GTE
			fmt.Fprintf(&out, "\n  gte RT %v TR %v L %v LR %v BK %v FC %v OF %x,%x H %x DQ %x,%x ZSF %x,%x", g.RT, g.TR, g.L, g.LR, g.BK, g.FC, g.OFX, g.OFY, g.H, g.DQA, g.DQB, g.ZSF3, g.ZSF4)
			from := m.Read32(packetP)
			m.Step()
			m.RunUntil(func() bool { return m.CPU.PC == ra && m.CPU.R[29] == sp }, 5*emutest.Second, "the sort returning")
			to := m.Read32(packetP)
			fmt.Fprintf(&out, "\n  packets %x..%x (%d words):", from, to, (to-from)/4)
			for p := from; p < to && p < from+4*400; p += 4 {
				if (p-from)%32 == 0 {
					out.WriteString("\n   ")
				}
				fmt.Fprintf(&out, " %08x", m.Read32(p))
			}
			out.WriteString("\n")
		}
		os.WriteFile("../.build/image/sort-"+v+".txt", []byte(out.String()), 0o644)
	}
}

// TestImageTMD dumps the TMD object at PSX_TMD in archive program
// PSX_ARCHIVE's memory, after PSX_ARCHIVE_AT seconds: its header and the
// first primitives' headers.
func TestImageTMD(t *testing.T) {
	imageOnly(t)
	name := os.Getenv("PSX_ARCHIVE")
	obj, _ := strconv.ParseUint(os.Getenv("PSX_TMD"), 0, 32)
	m := emutest.Boot(t, emutest.BIOS(emutest.BIOSFromEnv(t)), emutest.Disc("../.build/archive/"+name+"-sony/"+name+"-sony.cue"))
	m.RunUntilTTY("Execute !", 30*emutest.Second)
	m.RunFrames(60 * cmp.Or(atoi(os.Getenv("PSX_ARCHIVE_AT")), 4))
	o := uint32(obj)
	t.Logf("object: %08x %08x %08x %08x %08x %08x %08x", m.Read32(o), m.Read32(o+4), m.Read32(o+8), m.Read32(o+12), m.Read32(o+16), m.Read32(o+20), m.Read32(o+24))
	p := m.Read32(o + 16)
	for i := range min(int(m.Read32(o+20)), 6) {
		h := m.Read32(p)
		olen, ilen, flag, mode := h&0xff, h>>8&0xff, h>>16&0xff, h>>24
		var w []string
		for k := uint32(0); k <= ilen; k++ {
			w = append(w, fmt.Sprintf("%08x", m.Read32(p+4*k)))
		}
		t.Logf("prim %d: mode %02x flag %02x ilen %d olen %d: %v", i, mode, flag, ilen, olen, w)
		p += 4 * (ilen + 1)
	}
}

// sonyMachine boots a disc on Sony's library and runs it until the
// library has been running a while, for calling its functions directly.
func sonyMachine(t *testing.T) *emutest.Machine {
	v := cmp.Or(os.Getenv("PSX_PROBE_LIB"), "sony")
	m := emutest.Boot(t, emutest.BIOS(emutest.BIOSFromEnv(t)), emutest.Disc("../.build/terra-orig-"+v+"/terra-orig-"+v+".cue"))
	m.RunUntilTTY("Execute !", 30*emutest.Second)
	m.RunFrames(30)
	return m
}

// probeCall calls fn on m with the registers and the words for the
// stack's argument slots given, and returns v0, v1 and the RAM words
// that changed outside the call's stack, the stack's region excluded.
func probeCall(t *testing.T, m *emutest.Machine, fn uint32, a [4]uint32, stack []uint32) (v0, v1 uint32, changed map[uint32][2]uint32) {
	const sp, ret = 0x801f0000, 0x80001000
	before := make([]uint32, 0x200000/4)
	for i := range before {
		before[i] = m.Read32(0x80000000 + uint32(i)*4)
	}
	for i, w := range stack {
		m.Bus.Write32(sp+16+uint32(i)*4, w)
	}
	saved := m.CPU.R
	savedPC := m.CPU.PC
	copy(m.CPU.R[4:8], a[:])
	m.CPU.R[29], m.CPU.R[31] = sp, ret
	m.CPU.PC = fn
	m.CPU.NextPC = fn + 4
	m.RunUntil(func() bool { return m.CPU.PC == ret }, 5*emutest.Second, "the call returning")
	v0, v1 = m.CPU.R[2], m.CPU.R[3]
	changed = map[uint32][2]uint32{}
	for i := range before {
		addr := 0x80000000 + uint32(i)*4
		if addr >= sp-0x1000 && addr < sp+0x100 {
			continue
		}
		if w := m.Read32(addr); w != before[i] {
			changed[addr] = [2]uint32{before[i], w}
		}
	}
	m.CPU.R = saved
	m.CPU.PC, m.CPU.NextPC = savedPC, savedPC+4
	return
}

// TestImageProbeErrMath calls Sony's _err_math with codes and function
// numbers and reports what it changes, prints and leaves for _get_errno.
func TestImageProbeErrMath(t *testing.T) {
	imageOnly(t)
	const errMath, getErrno, mathErrno = 0x80042860, 0x80043008, 0x8004EA28
	m := sonyMachine(t)
	for _, c := range [][2]uint32{{0x21, 0x13}, {0x22, 0x13}, {0x22, 0x01}, {0x21, 0x05}, {0, 0}, {7, 0x30}} {
		tty := len(m.TTY())
		v0, v1, ch := probeCall(t, m, errMath, [4]uint32{c[0], c[1], 0, 0}, nil)
		e, _, _ := probeCall(t, m, getErrno, [4]uint32{}, nil)
		t.Logf("_err_math(%#x, %#x) -> v0 %#x v1 %#x; math_errno %#x; _get_errno %#x; printed %q; changed %v", c[0], c[1], v0, v1, m.Read32(mathErrno), e, m.TTY()[tty:], ch)
	}
}

// TestImageProbeDblShift calls Sony's _dbl_shift(p, a1, lo, hi, n) over a
// grid and reports what it leaves at p, and compares _get_errno with the
// kernel's B-function 54h.
func TestImageProbeDblShift(t *testing.T) {
	imageOnly(t)
	const dblShift, getErrno, dst = 0x8003E3A0, 0x80043008, 0x801e0000
	m := sonyMachine(t)
	e1, _, _ := probeCall(t, m, getErrno, [4]uint32{}, nil)
	m.CPU.R[9] = 0x54
	e2, _, _ := probeCall(t, m, 0xb0, [4]uint32{}, nil)
	t.Logf("_get_errno %#x, B(54h) %#x", e1, e2)
	values := [][2]uint32{{0x789abcde, 0x00123456}, {0xffffffff, 0xffffffff}, {0x00000001, 0x80000000}, {0x12345678, 0x9abcdef0}}
	r := uint32(12345)
	for range 12 {
		r = r*1103515245 + 12345
		lo := r
		r = r*1103515245 + 12345
		values = append(values, [2]uint32{lo, r})
	}
	for _, a1 := range []uint32{0, 1, 2, 0xffffffff} {
		for _, v := range values {
			for n := uint32(0xfffffffb); n != 71; n++ {
				m.Bus.Write32(dst, 0xdeadbeef)
				m.Bus.Write32(dst+4, 0xcafef00d)
				v0, v1, ch := probeCall(t, m, dblShift, [4]uint32{dst, a1, v[0], v[1]}, []uint32{n})
				delete(ch, dst)
				delete(ch, dst+4)
				t.Logf("a1 %d v %08x:%08x n %d -> %08x:%08x v0 %x other %d", int32(a1), v[1], v[0], int32(n), m.Read32(dst+4), m.Read32(dst), v0, len(ch))
				_ = v1
			}
		}
	}
}

// probeK is probeCall stepping by itself for at most limit instructions:
// it also records each kernel call made, as "B(35h) a0 a1 a2 a3", and says
// whether the call returned.
func probeK(t *testing.T, m *emutest.Machine, fn uint32, a [4]uint32, stack []uint32, limit int) (v0, v1 uint32, changed map[uint32][2]uint32, kernel []string, returned bool) {
	return probeKIn(t, m, fn, a, stack, limit, nil)
}

// probeKIn is probeK with the kernel's read, B(34h), answered from input
// when input is not nil: a byte at a time, then 0 bytes when it runs out
// (-1 for every read when input is empty but not nil... see eof).
func probeKIn(t *testing.T, m *emutest.Machine, fn uint32, a [4]uint32, stack []uint32, limit int, input []byte) (v0, v1 uint32, changed map[uint32][2]uint32, kernel []string, returned bool) {
	const sp, ret = 0x801f0000, 0x80001000
	before := make([]uint32, 0x200000/4)
	for i := range before {
		before[i] = m.Read32(0x80000000 + uint32(i)*4)
	}
	for i, w := range stack {
		m.Bus.Write32(sp+16+uint32(i)*4, w)
	}
	saved, savedPC := m.CPU.R, m.CPU.PC
	copy(m.CPU.R[4:8], a[:])
	m.CPU.R[29], m.CPU.R[31] = sp, ret
	m.CPU.PC, m.CPU.NextPC = fn, fn+4
	depth := 0
	for range limit {
		pc := m.CPU.PC
		if pc == ret {
			returned = true
			break
		}
		if v := pc & 0x1fffffff; (v == 0xa0 || v == 0xb0 || v == 0xc0) && depth == 0 {
			kernel = append(kernel, fmt.Sprintf("%c(%02xh) %x %x %x %x", "ABC"[(v-0xa0)/0x10], m.CPU.R[9], m.CPU.R[4], m.CPU.R[5], m.CPU.R[6], m.CPU.R[7]))
			if v == 0xb0 && m.CPU.R[9] == 0x34 && input != nil {
				if len(input) > 0 {
					m.Bus.Write8(m.CPU.R[5], input[0])
					input = input[1:]
					m.CPU.R[2] = 1
				} else if os.Getenv("PSX_READ_ZERO") != "" {
					m.CPU.R[2] = 0
				} else {
					m.CPU.R[2] = 0xffffffff
				}
				m.CPU.PC, m.CPU.NextPC = m.CPU.R[31], m.CPU.R[31]+4
				continue
			}
		}
		m.Step()
	}
	v0, v1 = m.CPU.R[2], m.CPU.R[3]
	changed = map[uint32][2]uint32{}
	for i := range before {
		addr := 0x80000000 + uint32(i)*4
		if addr >= sp-0x1000 && addr < sp+0x100 {
			continue
		}
		if w := m.Read32(addr); w != before[i] {
			changed[addr] = [2]uint32{before[i], w}
		}
	}
	m.CPU.R = saved
	m.CPU.PC, m.CPU.NextPC = savedPC, savedPC+4
	return
}

// TestImageProbeWrappers probes the kernel wrappers and the console
// functions with a few arguments each.
func TestImageProbeWrappers(t *testing.T) {
	imageOnly(t)
	m := sonyMachine(t)
	const name, buf = 0x801e0000, 0x801e0100
	for i, c := range []byte("cdrom:\\NOFILE.EXE;1\x00") {
		m.Bus.Write8(name+uint32(i), c)
	}
	calls := []struct {
		what  string
		fn    uint32
		a     [4]uint32
		limit int
	}{
		{"LoadTest", 0x80042DF8, [4]uint32{name, buf, 0, 0}, 12},
		{"Load", 0x80042E08, [4]uint32{name, buf, 0, 0}, 12},
		{"FlushCache", 0x80042E28, [4]uint32{1, 2, 3, 4}, 2_000_000},
		{"Exec", 0x80042E18, [4]uint32{buf, 1, 2, 3}, 12},
		{"putc('A', 1)", 0x80043764, [4]uint32{'A', 1, 0, 0}, 2_000_000},
		{"putc('\\n', 1)", 0x80043764, [4]uint32{'\n', 1, 0, 0}, 2_000_000},
		{"putc('\\t', 1)", 0x80043764, [4]uint32{'\t', 1, 0, 0}, 2_000_000},
		{"putc('B', 0)", 0x80043764, [4]uint32{'B', 0, 0, 0}, 2_000_000},
		{"putc('C', 2)", 0x80043764, [4]uint32{'C', 2, 0, 0}, 2_000_000},
		{"getc(0)", 0x80043420, [4]uint32{0, 0, 0, 0}, 400_000},
		{"getc(3)", 0x80043420, [4]uint32{3, 0, 0, 0}, 400_000},
		{"getchar", 0x80043450, [4]uint32{0, 0, 0, 0}, 400_000},
		{"gets(buf)", 0x80043484, [4]uint32{buf, 0, 0, 0}, 400_000},
	}
	for _, c := range calls {
		for k := uint32(0); k < 64; k += 4 {
			m.Bus.Write32(buf+k, 0x11111111)
		}
		tty := len(m.TTY())
		v0, v1, ch, kern, ok := probeK(t, m, c.fn, c.a, nil, c.limit)
		var mem []string
		for a, w := range ch {
			if a >= buf && a < buf+64 {
				mem = append(mem, fmt.Sprintf("buf+%x=%08x", a-buf, w[1]))
			}
		}
		slices.Sort(mem)
		t.Logf("%-18s returned %v v0 %x v1 %x kernel %q printed %q buf %v (%d words changed)", c.what, ok, v0, v1, kern, m.TTY()[tty:], mem, len(ch))
	}
}

// TestImageProbeStdio probes getc, getchar and gets with input given to
// the kernel's read, and the kernel's read and write's own results.
func TestImageProbeStdio(t *testing.T) {
	imageOnly(t)
	m := sonyMachine(t)
	const buf, slot = 0x801e0100, 0x801efff0
	m.Bus.Write8(buf, 'Z')
	m.CPU.R[9] = 0x35
	w1, _, _, _, _ := probeK(t, m, 0xb0, [4]uint32{1, buf, 1, 0}, nil, 2_000_000)
	m.CPU.R[9] = 0x35
	w2, _, _, _, _ := probeK(t, m, 0xb0, [4]uint32{2, buf, 1, 0}, nil, 2_000_000)
	t.Logf("kernel write(1) %x, write(2) %x", w1, w2)
	for _, c := range []struct {
		what  string
		fn    uint32
		a     uint32
		input string
		pre   uint32
	}{
		{"getc(0) 'q'", 0x80043420, 0, "q", 0x5a5a5a5a},
		{"getc(0) 0xff", 0x80043420, 0, "\xff", 0x5a5a5a5a},
		{"getc(0) 0x80", 0x80043420, 0, "\x80", 0x5a5a5a5a},
		{"getc(0) eof", 0x80043420, 0, "", 0x5a5a5a5a},
		{"getchar 'r'", 0x80043450, 0, "r", 0x5a5a5a5a},
		{"getchar eof", 0x80043450, 0, "", 0x5a5a5a5a},
		{"gets ab\\n", 0x80043484, buf, "ab\nxy", 0},
		{"gets ab\\r", 0x80043484, buf, "ab\rxy", 0},
		{"gets a\\bc\\n", 0x80043484, buf, "a\bc\n", 0},
		{"gets a\\x7fc\\n", 0x80043484, buf, "a\x7fc\n", 0},
		{"gets \\b\\n", 0x80043484, buf, "\b\n", 0},
		{"gets \\t\\n", 0x80043484, buf, "x\ty\n", 0},
		{"gets eof", 0x80043484, buf, "ab", 0},
		{"gets empty", 0x80043484, buf, "\n", 0},
	} {
		for k := uint32(0); k < 16; k += 4 {
			m.Bus.Write32(buf+k, 0x11111111)
		}
		m.Bus.Write32(slot, c.pre)
		tty := len(m.TTY())
		in := []byte(c.input)
		v0, _, _, kern, ok := probeKIn(t, m, c.fn, [4]uint32{c.a, 0, 0, 0}, nil, 400_000, in)
		var got []byte
		for k := uint32(0); k < 12; k++ {
			got = append(got, m.Bus.Read8(buf+k))
		}
		t.Logf("%-16s returned %v v0 %x buf % x printed %q kernel %d calls", c.what, ok, v0, got, m.TTY()[tty:], len(kern))
	}
}

// TestImageProbeGets probes gets with long lines and unusual characters.
func TestImageProbeGets(t *testing.T) {
	imageOnly(t)
	m := sonyMachine(t)
	const buf = 0x801d0000
	for _, in := range []string{
		strings.Repeat("x", 300) + "\n",
		strings.Repeat("x", 124) + "\n",
		strings.Repeat("x", 125) + "\n",
		strings.Repeat("x", 126) + "\n",
		strings.Repeat("x", 130) + "\b\bz\n",
		"a\x01b\x1bc\n",
		"\x80\xfe\xffz\n",
		"ab\r\ncd\n",
		"abc\b\b\b\bd\n",
	} {
		for k := uint32(0); k < 400; k += 4 {
			m.Bus.Write32(buf+k, 0x11111111)
		}
		tty := len(m.TTY())
		v0, _, _, kern, ok := probeKIn(t, m, 0x80043484, [4]uint32{buf, 0, 0, 0}, nil, 2_000_000, []byte(in))
		n := 0
		for m.Bus.Read8(buf+uint32(n)) != 0 && n < 400 {
			n++
		}
		got := make([]byte, min(n+2, 400))
		for k := range got {
			got[k] = m.Bus.Read8(buf + uint32(k))
		}
		reads := 0
		for _, k := range kern {
			if strings.HasPrefix(k, "B(34h)") {
				reads++
			}
		}
		out := m.TTY()[tty:]
		t.Logf("input %d bytes: returned %v v0 %x, string %d bytes, % x, %d reads, printed %d x %d bells %d other: %q", len(in), ok, v0, n, got[:min(len(got), 16)], reads, strings.Count(out, "x"), strings.Count(out, "\a"), len(out)-strings.Count(out, "x")-strings.Count(out, "\a"), out[max(0, len(out)-12):])
	}
}

// TestImageProbeJmp probes setjmp and longjmp: which registers setjmp
// keeps where, and what longjmp restores from where, and where it goes.
func TestImageProbeJmp(t *testing.T) {
	imageOnly(t)
	const setjmp, longjmp, buf = 0x8004550C, 0x80045548, 0x801d0000
	m := sonyMachine(t)
	run := func(fn uint32, a0, a1 uint32, regs map[int]uint32, stopAt func() bool, limit int) (r [32]uint32, pc uint32, n int) {
		saved, savedPC := m.CPU.R, m.CPU.PC
		for k, v := range regs {
			m.CPU.R[k] = v
		}
		m.CPU.R[4], m.CPU.R[5] = a0, a1
		m.CPU.PC, m.CPU.NextPC = fn, fn+4
		for n = 0; n < limit && !stopAt(); n++ {
			m.Step()
		}
		r, pc = m.CPU.R, m.CPU.PC
		m.CPU.R = saved
		m.CPU.PC, m.CPU.NextPC = savedPC, savedPC+4
		return
	}
	for k := uint32(0); k < 80; k += 4 {
		m.Bus.Write32(buf+k, 0xeeeeeeee)
	}
	regs := map[int]uint32{29: 0x801f0000, 31: 0x80001000, 30: 0x1e1e1e1e, 28: 0x1c1c1c1c}
	for s := 16; s <= 23; s++ {
		regs[s] = 0x10101010 * uint32(s-15)
	}
	r, _, n := run(setjmp, buf, 0, regs, func() bool { return m.CPU.PC == 0x80001000 }, 2_000_000)
	var words []string
	for k := uint32(0); k < 80; k += 4 {
		words = append(words, fmt.Sprintf("%08x", m.Read32(buf+k)))
	}
	t.Logf("setjmp: %d steps, v0 %x, buf %v", n, r[2], words)
	for _, val := range []uint32{0, 1, 7} {
		for k := uint32(0); k < 80; k += 4 {
			m.Bus.Write32(buf+k, 0xa0000000|k)
		}
		m.Bus.Write32(buf, 0x80001100)
		r, pc, n := run(longjmp, buf, val, map[int]uint32{29: 0x801f0000, 31: 0x80001000}, func() bool { return m.CPU.PC == 0x80001100 || m.CPU.PC == 0x80001000 }, 200)
		var got []string
		for _, k := range []int{2, 3, 16, 17, 18, 19, 20, 21, 22, 23, 28, 29, 30, 31} {
			got = append(got, fmt.Sprintf("r%d=%x", k, r[k]))
		}
		t.Logf("longjmp(buf, %d): %d steps, pc %x, %v", val, n, pc, got)
	}
}

// printfCases are formats and their arguments as words, laid out as the
// SDK's compiler passes variable arguments: from a2 on, a double on an
// even word.
var printfCases = func() []struct {
	f    string
	args []uint32
} {
	d := func(v float64) []uint32 { b := math.Float64bits(v); return []uint32{uint32(b), uint32(b >> 32)} }
	cat := func(parts ...[]uint32) []uint32 {
		var w []uint32
		for _, p := range parts {
			w = append(w, p...)
		}
		return w
	}
	i := func(v ...int32) []uint32 {
		var w []uint32
		for _, x := range v {
			w = append(w, uint32(x))
		}
		return w
	}
	type c = struct {
		f    string
		args []uint32
	}
	cases := []c{
		{"%d|%5d|%-5d|%05d|%+d|% d", i(-42, 42, 42, -42, 42, 42)},
		{"%x|%X|%#x|%08x|%o|%#o|%u", i(255, 255, 255, 255, 8, 8, -1)},
		{"%c|%3c|%-3c|%%|%5%|%k", i('z', 'y', 'x')},
		{"%ld|%lu|%hd|%lx|%hhd", i(-7, -7, 70000, 65535, 300)},
		{"[%*d] [%-*d] [%.*d] [%*.*d]", i(6, 42, 6, 42, 4, 42, 7, 3, 9)},
		{"%.3d|%.0d|%5.2d|%-8.3x|", i(7, 0, 3, 255)},
		{"%p|%10p", []uint32{0x801d0000, 0x1234}},
		{"%f|%.2f", cat(d(3.14159), d(-2.5))},
		{"%e|%E", cat(d(1e10), d(-1.5e-5))},
		{"%g|%G|%g|%g", cat(d(123456789), d(0.0001), d(100), d(1.5))},
		{"%8.3f|%-8.1f|%08.2f|%+.1f", cat(d(9.96), d(1), d(-3.14159), d(2))},
		{"%.0f|%#.0f|%.10g|%f", cat(d(2.5), d(2.5), d(1.0/3), d(0))},
		{"%d %f %d", cat(i(5), i(0), d(1.25), i(9))},
		{"%f|%f|%e", cat(d(1e20), d(-0.0), d(0))},
	}
	return cases
}()

// TestImageProbePrintf runs printfCases through sprintf and sprintf2, and
// printf and printf2, on the library PSX_PROBE_LIB names.
func TestImageProbePrintf(t *testing.T) {
	imageOnly(t)
	const buf, fmtAt = 0x801d0000, 0x801d1000
	m := sonyMachine(t)
	fns := []struct {
		name  string
		addr  uint32
		toBuf bool
	}{{"sprintf", 0x80044BE0, true}, {"sprintf2", 0x8004133C, true}, {"printf", 0x80044518, false}, {"printf2", 0x800412FC, false}}
	for _, f := range fns {
		for _, c := range printfCases {
			for k, b := range []byte(c.f + "\x00") {
				m.Bus.Write8(fmtAt+uint32(k), b)
			}
			for k := uint32(0); k < 128; k += 4 {
				m.Bus.Write32(buf+k, 0x11111111)
			}
			args := append([]uint32(nil), c.args...)
			var a [4]uint32
			var stack []uint32
			if f.toBuf {
				a[0], a[1] = buf, fmtAt
				for k, w := range args {
					if k < 2 {
						a[2+k] = w
					} else {
						stack = append(stack, w)
					}
				}
			} else {
				a[0] = fmtAt
				for k, w := range args {
					if k < 3 {
						a[1+k] = w
					} else {
						stack = append(stack, w)
					}
				}
			}
			// A double's words start on an even argument: printf's first
			// variable argument is a1, so its doubles move up a word.
			if !f.toBuf && strings.ContainsAny(c.f, "fFeEgG") {
				a = [4]uint32{fmtAt, 0, 0, 0}
				all := append([]uint32{0}, args...)
				a[1], a[2], a[3] = all[0], all[1], all[2]
				stack = append([]uint32(nil), all[3:]...)
			}
			tty := len(m.TTY())
			v0, _, _, _, ok := probeK(t, m, f.addr, a, stack, 2_000_000)
			out := m.TTY()[tty:]
			if f.toBuf {
				var b []byte
				for k := uint32(0); k < 128; k++ {
					c := m.Bus.Read8(buf + k)
					if c == 0 {
						break
					}
					b = append(b, c)
				}
				out = string(b)
			}
			t.Logf("%-8s %-30q -> %q (v0 %d, returned %v)", f.name, c.f, out, int32(v0), ok)
		}
	}
}

// TestImageProbePrintfCount probes what printf2 and sprintf2 return, with
// simple formats.
func TestImageProbePrintfCount(t *testing.T) {
	imageOnly(t)
	const buf, fmtAt = 0x801d0000, 0x801d1000
	m := sonyMachine(t)
	for _, c := range []struct {
		f string
		a uint32
	}{{"", 0}, {"a", 0}, {"abc", 0}, {"%d", 5}, {"%d", 12345}, {"x%d", 5}, {"%dx", 5}, {"%5d", 5}, {"%-5d", 5}, {"%05d", 5}, {"%s", 0x801d1100}, {"%10s", 0x801d1100}, {"%c", 'q'}, {"%3c", 'q'}, {"ab%%cd", 0}, {"%x", 255}, {"%8x", 255}, {"\n", 0}, {"a\nb", 0}} {
		for k, b := range []byte(c.f + "\x00") {
			m.Bus.Write8(fmtAt+uint32(k), b)
		}
		for k, b := range []byte("hey\x00") {
			m.Bus.Write8(0x801d1100+uint32(k), b)
		}
		v1, _, _, _, _ := probeK(t, m, 0x8004133C, [4]uint32{buf, fmtAt, c.a, 0}, nil, 2_000_000)
		tty := len(m.TTY())
		v2, _, _, _, _ := probeK(t, m, 0x800412FC, [4]uint32{fmtAt, c.a, 0, 0}, nil, 2_000_000)
		v3, _, _, _, _ := probeK(t, m, 0x80044BE0, [4]uint32{buf, fmtAt, c.a, 0}, nil, 2_000_000)
		v4, _, _, _, _ := probeK(t, m, 0x80044518, [4]uint32{fmtAt, c.a, 0, 0}, nil, 2_000_000)
		t.Logf("%-8q printed %-12q sprintf2 %d printf2 %d sprintf %d printf %d", c.f, m.TTY()[tty:], int32(v1), int32(v2), int32(v3), int32(v4))
	}
}

// floatCases are single doubles and formats for probing sprintf2.
var floatCases = []struct {
	f string
	v float64
}{
	{"%.20f", 0.1}, {"%.2f", 2.675}, {"%.2f", 0.125}, {"%.2f", 0.375}, {"%.0f", 0.5}, {"%.0f", 1.5}, {"%.0f", 2.5},
	{"%f", 1e23}, {"%.30f", 1e-10}, {"%e", 1e-300}, {"%e", 1.7976931348623157e308}, {"%.15e", 0.1},
	{"%g", 0.0001234}, {"%g", 0.00001234}, {"%g", 123456}, {"%g", 1234567}, {"%.0g", 123}, {"%#g", 1.5}, {"%g", 100000},
	{"%.3g", 0.0009995}, {"%f", math.Inf(1)}, {"%f", math.Inf(-1)}, {"%f", math.NaN()}, {"%e", math.Inf(1)},
	{"%f", -0.0}, {"%e", -0.0}, {"%g", -0.0}, {"%+f", 0}, {"% e", 1}, {"%10.3e", -12345.678}, {"%-12g|", 3.5}, {"%012.3f", -1.5},
	{"%.3f", 1.0005}, {"%.3f", 1.0015}, {"%f", 123456789.123456789}, {"%.17g", 0.1}, {"%g", 1e-5}, {"%G", 1e-10},
	{"%f", 4.35}, {"%.1f", 0.05}, {"%.1f", 0.25}, {"%.1f", 0.35}, {"%e", 9.9999999e10}, {"%.2e", 9.995}, {"%g", 9.9999995},
}

// TestImageProbeFloat runs floatCases through sprintf2 on PSX_PROBE_LIB's
// library.
func TestImageProbeFloat(t *testing.T) {
	imageOnly(t)
	const buf, fmtAt = 0x801d0000, 0x801d1000
	m := sonyMachine(t)
	for _, c := range floatCases {
		for k, b := range []byte(c.f + "\x00") {
			m.Bus.Write8(fmtAt+uint32(k), b)
		}
		b := math.Float64bits(c.v)
		v0, _, _, _, _ := probeK(t, m, 0x8004133C, [4]uint32{buf, fmtAt, uint32(b), uint32(b >> 32)}, nil, 4_000_000)
		var out []byte
		for k := uint32(0); k < 400; k++ {
			ch := m.Bus.Read8(buf + k)
			if ch == 0 {
				break
			}
			out = append(out, ch)
		}
		t.Logf("%-7s %-24v -> %q (%d)", c.f, c.v, out, int32(v0))
	}
}

// TestImageProbeDigits writes sprintf2's "%.25e" of many doubles on
// PSX_PROBE_LIB's library to .build/image/digits-V.txt, as "bits output".
func TestImageProbeDigits(t *testing.T) {
	imageOnly(t)
	const buf, fmtAt = 0x801d0000, 0x801d1000
	m := sonyMachine(t)
	for k, b := range []byte("%.25e\x00") {
		m.Bus.Write8(fmtAt+uint32(k), b)
	}
	if os.Getenv("PSX_PROBE_EDGE") != "" {
		vals := []float64{1e9 + 0.5, 4294967295.5, 4294967296.25, 1e10 + 0.5, 1e12 + 0.5, 1e15 + 0.5, 4503599627370495.5, 9007199254740991, 9007199254740993, 1e16, 1.2345678901234567e16, 9.999999999999999e16, 1.2345678901234567e17, 1.2345678901234567e18, 123456789012345678901.0, 0.35, 0.0009995, 0.45, 0.55, 1.005, 2.5e-5, 7.5e-10}
		var out strings.Builder
		for _, v := range vals {
			b := math.Float64bits(v)
			probeK(t, m, 0x8004133C, [4]uint32{buf, fmtAt, uint32(b), uint32(b >> 32)}, nil, 8_000_000)
			var s []byte
			for k := uint32(0); k < 200; k++ {
				ch := m.Bus.Read8(buf + k)
				if ch == 0 {
					break
				}
				s = append(s, ch)
			}
			fmt.Fprintf(&out, "%016x %s\n", b, s)
		}
		os.WriteFile("../.build/image/edge-"+cmp.Or(os.Getenv("PSX_PROBE_LIB"), "sony")+".txt", []byte(out.String()), 0o644)
		return
	}
	vals := []float64{1, 2, 3, 0.1, 0.2, 0.3, 1.0 / 3, 2.0 / 3, 10, 100, 1e10, 1e22, 1e23, 1e100, 1e-1, 1e-5, 1e-10, 1e-100, 1e-300, 1e300, 5e-324, 2.2250738585072014e-308, 1.7976931348623157e308, math.Pi, math.E, 123.456, 0.000123, 9.999999999999999, 7.0e-10}
	r := uint64(88172645463325252)
	for range 300 {
		r ^= r << 13
		r ^= r >> 7
		r ^= r << 17
		v := math.Float64frombits(r &^ (1 << 63))
		if math.IsNaN(v) || math.IsInf(v, 0) {
			continue
		}
		vals = append(vals, v)
	}
	var out strings.Builder
	for _, v := range vals {
		b := math.Float64bits(v)
		probeK(t, m, 0x8004133C, [4]uint32{buf, fmtAt, uint32(b), uint32(b >> 32)}, nil, 8_000_000)
		var s []byte
		for k := uint32(0); k < 200; k++ {
			ch := m.Bus.Read8(buf + k)
			if ch == 0 {
				break
			}
			s = append(s, ch)
		}
		fmt.Fprintf(&out, "%016x %s\n", b, s)
	}
	os.WriteFile("../.build/image/digits-"+cmp.Or(os.Getenv("PSX_PROBE_LIB"), "sony")+".txt", []byte(out.String()), 0o644)
}

// TestImageProbeFntPrint prints formats with FntPrint on Sony's library
// and finds the text it stored in memory by its marker.
func TestImageProbeFntPrint(t *testing.T) {
	imageOnly(t)
	const fntPrint, fntOpen, fntLoad, fmtAt = 0x800258FC, 0x80025320, 0x8002527C, 0x801d1000
	m := sonyMachine(t)
	probeK(t, m, fntLoad, [4]uint32{960, 256, 0, 0}, nil, 2_000_000)
	id, _, _, _, _ := probeK(t, m, fntOpen, [4]uint32{0, 0, 256, 200}, []uint32{0, 512}, 2_000_000)
	t.Logf("FntOpen -> %d", int32(id))
	b := math.Float64bits(1.25)
	for i, c := range []struct {
		f    string
		args [3]uint32
	}{
		{"QZ%d|%5x|%-3c|%p|%k|", [3]uint32{42, 255, 'q'}},
		{"QZ%f|%d|", [3]uint32{uint32(b), uint32(b >> 32), 7}},
		{"QZ%s|%hd|", [3]uint32{0x801d1100, 70000, 0}},
	} {
		f := fmt.Sprintf("Q%d", i) + c.f[1:]
		for k, ch := range []byte(f + "\x00") {
			m.Bus.Write8(fmtAt+uint32(k), ch)
		}
		for k, ch := range []byte("hey\x00") {
			m.Bus.Write8(0x801d1100+uint32(k), ch)
		}
		v0, _, ch, _, _ := probeK(t, m, fntPrint, [4]uint32{fmtAt, c.args[0], c.args[1], c.args[2]}, nil, 2_000_000)
		marker := fmt.Sprintf("Q%dZ", i)
		found := ""
		for a := range ch {
			_ = a
		}
		for addr := uint32(0x80010000); addr < 0x80200000 && found == ""; addr++ {
			if addr >= fmtAt && addr < fmtAt+64 {
				continue
			}
			if m.Bus.Read8(addr) == marker[0] && m.Bus.Read8(addr+1) == marker[1] && m.Bus.Read8(addr+2) == marker[2] {
				var s []byte
				for k := uint32(0); k < 64; k++ {
					x := m.Bus.Read8(addr + k)
					if x == 0 {
						break
					}
					s = append(s, x)
				}
				found = fmt.Sprintf("%x: %q", addr, s)
			}
		}
		t.Logf("FntPrint(%q) -> %d, stored %s", f, int32(v0), found)
	}
}

// fntCase is a FntPrint call: its arguments as words, from a0 on, with
// "%F" in the text standing for the format's address and "%M" for a
// marker the stored text is found by.
type fntCase struct {
	what string
	f    string
	args []uint32
}

// TestImageProbeFnt probes FntPrint on PSX_PROBE_LIB's library: what text
// each call leaves in the stream, found by the marker it starts with, and
// what it returns. The stream is flushed before each call.
func TestImageProbeFnt(t *testing.T) {
	imageOnly(t)
	const fntOpen, fntLoad = 0x80025320, 0x8002527C
	const strAt = 0x801d1400
	m := sonyMachine(t)
	probeK(t, m, fntLoad, [4]uint32{960, 256, 0, 0}, nil, 2_000_000)
	id0, _, _, _, _ := probeK(t, m, fntOpen, [4]uint32{16, 16, 256, 200}, []uint32{0, 300}, 2_000_000)
	id1, _, _, _, _ := probeK(t, m, fntOpen, [4]uint32{16, 100, 256, 100}, []uint32{0, 40}, 2_000_000)
	t.Logf("FntOpen -> %d, %d", int32(id0), int32(id1))
	for k, b := range []byte("hey\x00") {
		m.Bus.Write8(strAt+uint32(k), b)
	}
	d := func(v float64) []uint32 { b := math.Float64bits(v); return []uint32{uint32(b), uint32(b >> 32)} }
	if os.Getenv("PSX_FNT_IDS") != "" {
		long := strings.Repeat("0123456789", 6)
		runFnt(t, m, []fntCase{{"short", "%Mab%d", []uint32{5}}}, []int{0, 1, 2, 3, 7, 8, 100, 0x7fffffff, -1}, id0, id1)
		runFnt(t, m, []fntCase{{"long", "%M" + long, nil}}, []int{1, 0}, id0, id1)
		return
	}
	if dirs := os.Getenv("PSX_FNT_SINGLE"); dirs != "" {
		var cs []fntCase
		for _, dir := range strings.Fields(dirs) {
			first := uint32(123)
			if v := os.Getenv("PSX_FNT_ARG"); v != "" {
				first = uint32(atoi(v))
			}
			cs = append(cs, fntCase{dir, "%M" + dir + "|", []uint32{first, strAt, 9, 10, 11}})
		}
		runFnt(t, m, cs, []int{-1}, id0, id1)
		return
	}
	cases := []fntCase{
		{"ints", "%M%d|%5d|%-5d|%05d|%+d|% d", []uint32{0xfffffffe, 42, 42, 0xffffffd6, 42, 42}},
		{"hex", "%M%x|%X|%#x|%08x|%o|%#o|%u", []uint32{255, 255, 255, 255, 8, 8, 0xffffffff}},
		{"chars", "%M%c|%3c|%-3c|%%|%5%|", []uint32{'z', 'y', 'x'}},
		{"unknown", "%M%k|after", nil},
		{"pointer", "%M%p|%10p", []uint32{0x801d0000, 0x1234}},
		{"string", "%M%s|%5s|%-5s|%.2s", []uint32{strAt, strAt, strAt, strAt}},
		{"lengths", "%M%ld|%hd|%lu", []uint32{0xfffffff9, 70000, 0xfffffff9}},
		{"star", "%M[%*d][%.*d]", []uint32{6, 42, 4, 42}},
		{"precision", "%M%.3d|%.0d|%5.2d", []uint32{7, 0, 3}},
		{"float", "%M%f|%d", append(append([]uint32{0}, d(1.25)...), 9)},
		{"float e", "%M%e", append([]uint32{0}, d(1.25)...)},
		{"newline tab", "%Ma\nb\tc~\x7f\x80\xff", nil},
		{"empty", "%M", nil},
	}
	runFnt(t, m, cases, []int{-1, int(id0), int(id1), 9, -5}, id0, id1)
}

// runFnt runs FntPrint cases on m, with each of ids first (-1 for none),
// flushing streams f0 and f1 before each call.
func runFnt(t *testing.T, m *emutest.Machine, cases []fntCase, ids []int, id0, id1 uint32) {
	runFntAt(t, m, cases, ids, id0, id1, 0x800258FC, 0x800255D8, "fntBuf")
}

// runFntAt is runFnt with the print and flush functions' addresses, and
// the image's staging buffer to skip, given.
func runFntAt(t *testing.T, m *emutest.Machine, cases []fntCase, ids []int, id0, id1, fntPrint, fntFlush uint32, staging string) {
	const fmtAt = 0x801d1000
	// On the image, the C FntPrint stages its text in fntBuf first: the
	// stream's copy is the one wanted.
	var skipFrom, skipTo uint32
	if os.Getenv("PSX_PROBE_LIB") == "go" {
		if ef, err := elf.Open("../.build/image/image.elf"); err == nil {
			if syms, err := ef.Symbols(); err == nil {
				for _, sy := range syms {
					if sy.Name == staging {
						skipFrom, skipTo = uint32(sy.Value), uint32(sy.Value+sy.Size)
					}
				}
			}
			ef.Close()
		}
	}
	for i, c := range cases {
		for _, withID := range ids {
			marker := fmt.Sprintf("@%c%c@", 'A'+i, 'a'+(withID+5)%26)
			f := strings.Replace(c.f, "%M", marker, 1)
			for k, b := range []byte(f + "\x00") {
				m.Bus.Write8(fmtAt+uint32(k), b)
			}
			probeK(t, m, fntFlush, [4]uint32{id0, 0, 0, 0}, nil, 2_000_000)
			probeK(t, m, fntFlush, [4]uint32{id1, 0, 0, 0}, nil, 2_000_000)
			words := append([]uint32(nil), c.args...)
			if withID != -1 {
				words = append([]uint32{fmtAt}, words...)
				words = append([]uint32{uint32(int32(withID))}, words...)
			} else {
				words = append([]uint32{fmtAt}, words...)
			}
			// A double in the variable arguments starts on an even word:
			// the cases give a padding word for the format alone, which an
			// id first takes the place of.
			if strings.Contains(c.what, "float") && withID != -1 {
				words = append(words[:2], words[3:]...)
			}
			var a [4]uint32
			copy(a[:], words)
			var stack []uint32
			if len(words) > 4 {
				stack = words[4:]
			}
			v0, _, _, _, _ := probeK(t, m, fntPrint, a, stack, 2_000_000)
			found := "(none)"
			for addr := uint32(0x80010000); addr < 0x80200000; addr++ {
				if addr == fmtAt {
					addr += 0x100
					continue
				}
				if addr >= skipFrom && addr < skipTo {
					continue
				}
				if m.Bus.Read8(addr) == '@' && m.Bus.Read8(addr+1) == marker[1] && m.Bus.Read8(addr+2) == marker[2] && m.Bus.Read8(addr+3) == '@' {
					var s []byte
					for k := uint32(4); k < 400; k++ {
						x := m.Bus.Read8(addr + k)
						if x == 0 {
							break
						}
						s = append(s, x)
					}
					found = fmt.Sprintf("%q", s)
					break
				}
			}
			t.Logf("%-12s id %3d -> %3d stored %s", c.what, withID, int32(v0), found)
		}
	}
}

// TestImageProbeKanji runs TestImageProbeFnt's cases through KanjiFntPrint
// on PSX_PROBE_LIB's library, with Kanji streams opened.
func TestImageProbeKanji(t *testing.T) {
	imageOnly(t)
	const kOpen, kPrint, kFlush, strAt = 0x8002632C, 0x80026A2C, 0x8002665C, 0x801d1400
	m := sonyMachine(t)
	id0, _, _, _, _ := probeK(t, m, kOpen, [4]uint32{16, 16, 256, 200}, []uint32{0, 0, 640, 480, 0, 300}, 2_000_000)
	id1, _, _, _, _ := probeK(t, m, kOpen, [4]uint32{16, 100, 256, 100}, []uint32{0, 0, 640, 481, 0, 40}, 2_000_000)
	t.Logf("KanjiFntOpen -> %d, %d", int32(id0), int32(id1))
	str := "hey"
	if v := os.Getenv("PSX_FNT_STR"); v != "" {
		str = v
	}
	for k, b := range []byte(str + "\x00") {
		m.Bus.Write8(strAt+uint32(k), b)
	}
	staging := "fntBuf"
	run := func(cs []fntCase, ids []int) { runFntAt(t, m, cs, ids, id0, id1, kPrint, kFlush, staging) }
	d := func(v float64) []uint32 { b := math.Float64bits(v); return []uint32{uint32(b), uint32(b >> 32)} }
	if dirs := os.Getenv("PSX_FNT_SINGLE"); dirs != "" {
		var cs []fntCase
		for _, dir := range strings.Fields(dirs) {
			first := uint32(123)
			if v := os.Getenv("PSX_FNT_ARG"); v != "" {
				first = uint32(atoi(v))
			}
			cs = append(cs, fntCase{dir, "%M" + dir + "|", []uint32{first, strAt, 9, 10, 11}})
		}
		run(cs, []int{-1})
		return
	}
	if os.Getenv("PSX_FNT_IDS") != "" {
		long := strings.Repeat("0123456789", 6)
		run([]fntCase{{"short", "%Mab%d", []uint32{5}}}, []int{0, 1, 2, 3, 7, 8, 100, 0x7fffffff, -1})
		run([]fntCase{{"long", "%M" + long, nil}}, []int{1, 0})
		for _, n := range []int{70, 120, 300} {
			run([]fntCase{{fmt.Sprintf("len %d", n), "%M" + strings.Repeat("x", n-4), nil}}, []int{1})
		}
		return
	}
	run([]fntCase{
		{"ints", "%M%d|%5d|%05d|%x|%08x", []uint32{0xfffffffe, 42, 42, 255, 255}},
		{"chars", "%M%c|%3c|%%|", []uint32{'z', 'y'}},
		{"string", "%M%s|%5s", []uint32{strAt, strAt}},
		{"float", "%M%f|%d", append(append([]uint32{0}, d(1.25)...), 9)},
		{"kanji", "%M\x82\xa0\x88\x9f", nil},
		{"newline tab", "%Ma\nb\tc~", nil},
	}, []int{-1, int(id0), int(id1), 9})
}

// callFast calls fn on m with a0 to a3 and returns v0 and v1, nothing
// else recorded.
func callFast(m *emutest.Machine, fn uint32, a [4]uint32) (uint32, uint32) {
	const sp, ret = 0x801f0000, 0x80001000
	saved, savedPC := m.CPU.R, m.CPU.PC
	copy(m.CPU.R[4:8], a[:])
	m.CPU.R[29], m.CPU.R[31] = sp, ret
	m.CPU.PC, m.CPU.NextPC = fn, fn+4
	for n := 0; n < 5_000_000 && m.CPU.PC != ret; n++ {
		m.Step()
	}
	v0, v1 := m.CPU.R[2], m.CPU.R[3]
	m.CPU.R = saved
	m.CPU.PC, m.CPU.NextPC = savedPC, savedPC+4
	return v0, v1
}

// TestImageProbeMath calls the math library's functions on many values on
// PSX_PROBE_LIB's library and writes the results' bits to
// .build/image/math-V.txt.
func TestImageProbeMath(t *testing.T) {
	imageOnly(t)
	m := sonyMachine(t)
	unary := []struct {
		name string
		addr uint32
	}{{"sin", 0x80040CC4}, {"cos", 0x80040D9C}, {"tan", 0x80040C3C}, {"asin", 0x8003DBF8}, {"acos", 0x8003DD7C}, {"atan", 0x8003D754}, {"exp", 0x8003F1C4}, {"log", 0x80040274}, {"log10", 0x800404BC}, {"sinh", 0x80040ED4}, {"cosh", 0x8004100C}, {"tanh", 0x8004107C}, {"sqrt", 0x800411CC}, {"floor", 0x8003F4E8}, {"ceil", 0x8003F5C0}}
	binary := []struct {
		name string
		addr uint32
	}{{"atan2", 0x8003D960}, {"pow", 0x80040740}, {"fmod", 0x800405FC}, {"hypot", 0x8003F7A0}}
	var vals []float64
	for _, v := range []float64{0, 0.5, 1, 2, 3, 10, 100, 0.1, 1e-5, 1e5, math.Pi, math.Pi / 2, math.Pi / 4, 2 * math.Pi, 0.999, 1.001, 700, 1e300, -0.5, -1, -2, -3.5, -100} {
		vals = append(vals, v)
	}
	r := uint64(12345)
	for range 60 {
		r = r*6364136223846793005 + 1442695040888963407
		vals = append(vals, (float64(r>>11)/(1<<53))*20-10)
	}
	var out strings.Builder
	split := func(v float64) (uint32, uint32) { b := math.Float64bits(v); return uint32(b), uint32(b >> 32) }
	for _, f := range unary {
		for _, v := range vals {
			lo, hi := split(v)
			r0, r1 := callFast(m, f.addr, [4]uint32{lo, hi, 0, 0})
			fmt.Fprintf(&out, "%s(%v) %08x%08x\n", f.name, v, r1, r0)
		}
	}
	for _, f := range binary {
		for i, v := range vals {
			w := vals[(i*7+3)%len(vals)]
			lo, hi := split(v)
			lo2, hi2 := split(w)
			r0, r1 := callFast(m, f.addr, [4]uint32{lo, hi, lo2, hi2})
			fmt.Fprintf(&out, "%s(%v,%v) %08x%08x\n", f.name, v, w, r1, r0)
		}
	}
	os.WriteFile("../.build/image/math-"+cmp.Or(os.Getenv("PSX_PROBE_LIB"), "sony")+".txt", []byte(out.String()), 0o644)
}

// TestImageBgData reads the GsBG, map, index and cells archive program
// PSX_ARCHIVE gives GsSortFixBg16 after PSX_ARCHIVE_AT seconds on both
// libraries, and reports where they differ.
func TestImageBgData(t *testing.T) {
	imageOnly(t)
	const sortBg = 0x80021D3C
	name := os.Getenv("PSX_ARCHIVE")
	at := cmp.Or(atoi(os.Getenv("PSX_ARCHIVE_AT")), 4)
	names := terraNames()
	var sortAt uint32
	for a, n := range names {
		if n == "GsSortFixBg16" {
			sortAt = a
		}
	}
	_ = sortBg
	type dump struct {
		bg, mp []uint32
		index  []uint16
		cells  []uint32
	}
	read := func(v string) dump {
		m := emutest.Boot(t, emutest.BIOS(emutest.BIOSFromEnv(t)), emutest.Disc("../.build/archive/"+name+"-"+v+"/"+name+"-"+v+".cue"))
		m.RunUntilTTY("Execute !", 30*emutest.Second)
		m.RunFrames(60 * at)
		m.RunUntil(func() bool { return m.CPU.PC == sortAt }, 5*emutest.Second, "a background sort")
		var d dump
		bg := m.CPU.R[4]
		for k := uint32(0); k < 36; k += 4 {
			d.bg = append(d.bg, m.Read32(bg+k))
		}
		mp := d.bg[5]
		for k := uint32(0); k < 16; k += 4 {
			d.mp = append(d.mp, m.Read32(mp+k))
		}
		nw, nh := d.mp[0]>>16, d.mp[1]&0xffff
		base, index := d.mp[2], d.mp[3]
		max := uint16(0)
		for i := uint32(0); i < nw*nh && i < 65536; i++ {
			x := uint16(m.Read32(index+2*i&^3) >> (16 * (i & 1)))
			d.index = append(d.index, x)
			if x > max && x != 0xffff {
				max = x
			}
		}
		for i := uint32(0); i <= uint32(max); i++ {
			d.cells = append(d.cells, m.Read32(base+8*i), m.Read32(base+8*i+4))
		}
		return d
	}
	a, b := read("sony"), read("go")
	t.Logf("bg sony %x\nbg go   %x\nmap sony %x\nmap go   %x", a.bg, b.bg, a.mp, b.mp)
	n := 0
	for i := range min(len(a.index), len(b.index)) {
		if a.index[i] != b.index[i] {
			if n < 10 {
				t.Logf("index %d: sony %x go %x", i, a.index[i], b.index[i])
			}
			n++
		}
	}
	c := 0
	for i := range min(len(a.cells), len(b.cells)) {
		if a.cells[i] != b.cells[i] {
			c++
		}
	}
	t.Logf("%d of %d index entries differ, %d of %d cell words", n, len(a.index), c, len(a.cells))
}
