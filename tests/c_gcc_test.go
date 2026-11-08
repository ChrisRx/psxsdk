package tests

import (
	"bytes"
	"crypto/sha1"
	"debug/elf"
	"encoding/binary"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"go.chrisrx.dev/psx/internal/cproject"
	"go.chrisrx.dev/x/assert"
)

// gccInput presses Start and the face buttons and moves about, after the
// 1200 frames a program can take to start under OpenBIOS. A program that only
// draws its title screen without input has shown little of its code.
const gccInput = "1200;st:10;200;x:10;200;st:10;200;x:10;200;u:30;r:30;x:30;d:30;l:30;o:10;300"

// gccCFlags are yaroze.mk's flags, with those of toolchain.cFlags that ask for
// what a GCC of 1996 did, so that what differs is the compiler and not what it
// was asked for. -x c goes with them because an uppercase .C reads as C++.
var gccCFlags = []string{
	"-march=mips1", "-mtune=r3000", "-mabi=32", "-EL", "-msoft-float", "-G0", "-mno-abicalls", "-fno-pic",
	"-std=gnu89", "-fcommon", "-O2", "-ffreestanding", "-fno-builtin", "-fno-stack-protector",
	"-fno-asynchronous-unwind-tables", "-fomit-frame-pointer",
	"-w", "-fno-strict-aliasing", "-fwrapv", "-fno-delete-null-pointer-checks",
	"-D_R3000", "-Dpsx", "-Dunix",
}

// Projects from the Yaroze websites, built from their source by yaroze, have
// to mean what they mean to GCC. Sony's shipped executables cannot show this
// for a project that came without one, or for one that went unmatched by both
// compilers, and comparing with GCC found three bugs that they did not.
//
// Every project has to build under both compilers or neither, and its global
// variables have to have the same sizes and contents. Those on gccExact also
// have to draw the same video memory with and without input. The rest differ
// for reasons that are not a miscompile, such as the order rand is called in or
// a loop that runs at another rate, or draw nothing, so byte equality would
// say nothing about them. A project that matches and is not listed is logged
// as "exact:", for adding to the list.
//
// PSX_WEBARCHIVE is the archive's site directory. PSX_GCC is an SDK installed
// from the yaroze-toolchain repository, by make install or nix
// build .#libyaroze, with mipsel-none-elf-gcc on PATH. It takes a while:
//
//	go test ./tests -run TestCMatchesGCC -timeout 60m -parallel 10
func TestCMatchesGCC(t *testing.T) {
	root := os.Getenv("PSX_WEBARCHIVE")
	if root == "" {
		t.Skip("PSX_WEBARCHIVE is not set")
	}
	prefix := os.Getenv("PSX_GCC")
	if prefix == "" {
		t.Skip("PSX_GCC is not set to an installed SDK")
	}
	_, err := exec.LookPath("mipsel-none-elf-gcc")
	assert.NoError(t, err)
	yaroze := needCC(t)
	psxemu := filepath.Join(t.TempDir(), "psxemu")
	out, err := exec.Command("go", "build", "-o", psxemu, "go.chrisrx.dev/psx/tools/psxemu").CombinedOutput()
	assert.NoError(t, err, string(out))
	eco2exe := filepath.Join(t.TempDir(), "eco2exe")
	out, err = exec.Command("go", "build", "-o", eco2exe, "go.chrisrx.dev/psx/tools/eco2exe").CombinedOutput()
	assert.NoError(t, err, string(out))

	projects, err := gccProjects(root)
	assert.NoError(t, err)
	assert.Equal(t, true, len(projects) > 100, "the archive's projects should have been found")
	for _, name := range gccExact {
		if !slices.Contains(projects, name) {
			t.Errorf("%s is on gccExact and not a project in the archive", name)
		}
	}

	for _, name := range projects {
		t.Run(name, func(t *testing.T) {
			parallel(t)
			dir := filepath.Join(root, name)
			tmp := t.TempDir()
			ourELF, ourEXE := filepath.Join(tmp, "our.elf"), filepath.Join(tmp, "our.exe")
			ourErr := errors.Join(yarozeBuild(yaroze, dir, ourELF), yarozeBuild(yaroze, dir, ourEXE))
			gccELF, gccEXE := filepath.Join(tmp, "gcc.elf"), filepath.Join(tmp, "gcc.exe")
			gccErr := gccBuild(prefix, eco2exe, dir, filepath.Join(tmp, "gcc"), gccELF, gccEXE)
			if ourErr != nil || gccErr != nil {
				if (ourErr == nil) != (gccErr == nil) {
					t.Errorf("only one compiler builds it\nyaroze: %v\nGCC: %v", ourErr, gccErr)
				}
				return
			}

			differs, err := gccGlobals(ourELF, gccELF)
			assert.NoError(t, err)
			if len(differs) > 0 {
				t.Errorf("global variables differ: %s", strings.Join(differs, ", "))
			}

			same, drew := true, false
			for i, script := range []string{"1500", gccInput} {
				ours, ourHalt, err := gccRun(psxemu, ourEXE, script, filepath.Join(tmp, fmt.Sprintf("our%d.vram", i)))
				assert.NoError(t, err)
				gcc, gccHalt, err := gccRun(psxemu, gccEXE, script, filepath.Join(tmp, fmt.Sprintf("gcc%d.vram", i)))
				assert.NoError(t, err)
				same = same && bytes.Equal(ours, gcc) && ourHalt == gccHalt
				drew = drew || slices.ContainsFunc(ours, func(b byte) bool { return b != 0 })
			}
			listed := slices.Contains(gccExact, name)
			switch {
			case listed && !same:
				t.Error("video memory should match what the program built by GCC draws")
			case listed && !drew:
				t.Error("the program should have drawn something")
			case !listed && same && drew:
				t.Logf("exact: %s", name)
			}
		})
	}
}

// gccProjects returns the archive's C projects, relative to root: directories
// with an auto file and C sources, and only the first with the same sources.
func gccProjects(root string) ([]string, error) {
	var projects []string
	seen := make(map[[sha1.Size]byte]bool)
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || !d.IsDir() {
			return err
		}
		entries, err := os.ReadDir(path)
		if err != nil {
			return err
		}
		hash := sha1.New()
		auto, sources := false, 0
		for _, e := range entries {
			switch {
			case e.IsDir():
			case strings.EqualFold(e.Name(), "auto"):
				auto = true
			case strings.EqualFold(filepath.Ext(e.Name()), ".c"):
				data, err := os.ReadFile(filepath.Join(path, e.Name()))
				if err != nil {
					return err
				}
				hash.Write(data)
				sources++
			}
		}
		if !auto || sources == 0 {
			return nil
		}
		key := [sha1.Size]byte(hash.Sum(nil))
		if seen[key] {
			return nil
		}
		seen[key] = true
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		projects = append(projects, rel)
		return nil
	})
	return projects, err
}

func yarozeBuild(yaroze, dir, out string) error {
	if data, err := exec.Command(yaroze, "build", "-o", out, dir).CombinedOutput(); err != nil {
		return fmt.Errorf("%w: %s", err, data)
	}
	return nil
}

// gccCommand runs GCC without the flags Nix's wrapper would otherwise add for
// hardening, which mean nothing on an R3000 and change the code it generates.
func gccCommand(args ...string) *exec.Cmd {
	cmd := exec.Command("mipsel-none-elf-gcc", args...)
	cmd.Env = append(os.Environ(), "NIX_HARDENING_ENABLE=")
	return cmd
}

// gccBuild builds the project in dir with GCC from the same prepared sources
// yaroze compiles, linked where it says and with what its auto file loads.
func gccBuild(prefix, eco2exe, dir, work, outELF, outEXE string) error {
	include := filepath.Join(prefix, "include")
	lib := filepath.Join(prefix, "lib")
	p, err := cproject.Read(dir)
	if err != nil {
		return err
	}
	sources, err := p.Prepare(filepath.Join(work, "src"), func(name string) bool {
		fi, err := os.Stat(filepath.Join(include, name))
		return err == nil && !fi.IsDir()
	})
	if err != nil {
		return err
	}
	var objects []string
	for i, src := range sources {
		language := "c"
		if strings.EqualFold(filepath.Ext(src), ".s") {
			language = "assembler-with-cpp"
		}
		obj := filepath.Join(work, fmt.Sprintf("%d.o", i))
		args := slices.Clone(gccCFlags)
		for _, d := range p.Defines {
			args = append(args, "-D"+d)
		}
		args = append(args, "-I"+include, "-x", language, "-c", src, "-o", obj)
		if data, err := gccCommand(args...).CombinedOutput(); err != nil {
			return fmt.Errorf("%s: %w: %s", src, err, data)
		}
		objects = append(objects, obj)
	}
	args := []string{
		"-march=mips1", "-mabi=32", "-EL", "-msoft-float", "-G0", "-mno-abicalls", "-fno-pic",
		"-nostdlib", "-nostartfiles", "-Wl,--build-id=none",
		fmt.Sprintf("-Wl,--defsym=__load=0x%08x", p.Addr),
		"-L" + filepath.Join(prefix, "ldscripts"), "-T", "yaroze.ld", "-o", outELF,
		filepath.Join(lib, "startup.o"), filepath.Join(lib, "symusr.o"),
	}
	args = append(args, objects...)
	args = append(args, "-L"+lib, "-lyaroze", "-lgcc")
	if data, err := gccCommand(args...).CombinedOutput(); err != nil {
		return fmt.Errorf("link: %w: %s", err, data)
	}
	var loads []string
	for _, l := range p.Loads {
		loads = append(loads, "--load", fmt.Sprintf("%s@%08x", l.File, l.Addr))
	}
	if data, err := exec.Command(eco2exe, append(loads, outELF, outEXE)...).CombinedOutput(); err != nil {
		return fmt.Errorf("eco2exe: %w: %s", err, data)
	}
	return nil
}

// gccRun runs exe through script and returns its video memory and whether it
// halted. It is not --quiet, which would hide the halt, and a script psxemu
// rejects is an error rather than empty video memory that compares equal.
func gccRun(psxemu, exe, script, vram string) ([]byte, bool, error) {
	out, err := exec.Command(psxemu, "--bios", "../emu/openbios/openbios.bin", "--exe", exe, "--script", script, "--vram", vram).CombinedOutput()
	if err != nil {
		return nil, false, fmt.Errorf("%w: %s", err, out)
	}
	data, err := os.ReadFile(vram)
	return data, bytes.Contains(out, []byte("halting")), err
}

// A gccGlobal is a global variable in a linked program. data is nil for one in
// .bss.
type gccGlobal struct {
	size uint64
	data []byte
}

func gccReadGlobals(path string) (map[string]gccGlobal, error) {
	f, err := elf.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	syms, err := f.Symbols()
	if err != nil {
		return nil, err
	}
	globals := make(map[string]gccGlobal)
	for _, s := range syms {
		if s.Size == 0 || elf.ST_BIND(s.Info) != elf.STB_GLOBAL || strings.HasPrefix(s.Name, "_") {
			continue
		}
		if int(s.Section) >= len(f.Sections) || s.Section == elf.SHN_UNDEF {
			continue
		}
		sec := f.Sections[s.Section]
		if sec.Flags&elf.SHF_ALLOC == 0 || sec.Flags&elf.SHF_EXECINSTR != 0 {
			continue
		}
		g := gccGlobal{size: s.Size}
		if sec.Type == elf.SHT_PROGBITS {
			data, err := sec.Data()
			if err != nil {
				return nil, err
			}
			g.data = data[s.Value-sec.Addr:][:s.Size]
		}
		globals[s.Name] = g
	}
	return globals, nil
}

// gccGlobals returns the names of the global variables the two programs both
// have that differ in size or in contents. Two words that are both addresses
// in RAM do not count as differing, since the programs are laid out
// differently, and neither does a variable in .bss in one of them only.
func gccGlobals(a, b string) ([]string, error) {
	as, err := gccReadGlobals(a)
	if err != nil {
		return nil, err
	}
	bs, err := gccReadGlobals(b)
	if err != nil {
		return nil, err
	}
	var differs []string
	for name, x := range as {
		y, ok := bs[name]
		switch {
		case !ok:
		case x.size != y.size:
			differs = append(differs, fmt.Sprintf("%s (size %d and %d)", name, x.size, y.size))
		case x.data != nil && y.data != nil && !gccSameData(x.data, y.data):
			differs = append(differs, name)
		}
	}
	slices.Sort(differs)
	return differs, nil
}

func gccSameData(a, b []byte) bool {
	n := len(a) &^ 3
	for i := 0; i < n; i += 4 {
		x, y := binary.LittleEndian.Uint32(a[i:]), binary.LittleEndian.Uint32(b[i:])
		if x != y && (x>>24 != 0x80 || y>>24 != 0x80) {
			return false
		}
	}
	return bytes.Equal(a[n:], b[n:])
}

// gccExact is the projects whose programs draw the same video memory under
// both compilers, with and without input.
var gccExact = []string{
	"yaroze/ftp/scee/demos/3d/chrome.zip.x",
	"yaroze/ftp/scee/demos/3d/maketmd.zip.x",
	"yaroze/ftp/scee/demos/3d/viewtmd.zip.x",
	"yaroze/ftp/scee/demos/allother.zip.x/tod1.zip.x",
	"yaroze/ftp/scee/demos/other/t11.zip.x",
	"yaroze/ftp/scee/topten/motcap.zip.x/motionCapture",
	"yaroze/ftp/scee/topten/roller.zip.x",
	"yaroze/ftp/scee/tutor/chap1.zip.x/1",
	"yaroze/ftp/scee/tutor/chap1.zip.x/2",
	"yaroze/ftp/scee/tutor/chap1.zip.x/3",
	"yaroze/ftp/scee/tutor/chap1.zip.x/4",
	"yaroze/ftp/scee/tutor/chap1.zip.x/5",
	"yaroze/ftp/scee/tutor/chap1.zip.x/6",
	"yaroze/ftp/scee/tutor/chap2.zip.x/1",
	"yaroze/ftp/scee/tutor/chap2.zip.x/10",
	"yaroze/ftp/scee/tutor/chap2.zip.x/2",
	"yaroze/ftp/scee/tutor/chap2.zip.x/3",
	"yaroze/ftp/scee/tutor/chap2.zip.x/4",
	"yaroze/ftp/scee/tutor/chap2.zip.x/5",
	"yaroze/ftp/scee/tutor/chap2.zip.x/6",
	"yaroze/ftp/scee/tutor/chap2.zip.x/7",
	"yaroze/ftp/scee/tutor/chap2.zip.x/8",
	"yaroze/ftp/scee/tutor/chap2.zip.x/9",
	"yaroze/ftp/scee/tutor/chap3.zip.x/1",
	"yaroze/ftp/scee/tutor/chap3.zip.x/2",
	"yaroze/ftp/scee/tutor/chap4.zip.x/1",
	"yaroze/ftp/scee/tutor/chap4.zip.x/2",
	"yaroze/ftp/scee/tutor/chap4.zip.x/3",
	"yaroze/ftp/scee/tutor/chap4.zip.x/4",
	"yaroze/ftp/scee/tutor/chap4.zip.x/5",
	"yaroze/ftp/scee/tutor/chap5.zip.x/1",
	"yaroze/ftp/scee/tutor/chap5.zip.x/2",
	"yaroze/ftp/scee/tutor/chap5.zip.x/3",
	"yaroze/ftp/scee/tutor/chap6.zip.x/1",
	"yaroze/ftp/scee/tutor/chap6.zip.x/2",
	"yaroze/ftp/scee/tutor/chap7.zip.x/1",
	"yaroze/ftp/scee/tutor/chap7.zip.x/2",
	"yaroze/ftp/scee/tutor/chap7.zip.x/3",
	"yaroze/ftp/scee/tutor/chap7.zip.x/4",
	"yaroze/ftp/scee/tutor/chap7.zip.x/5",
	"yaroze/ftp/scee/tutor/chap7.zip.x/6",
	"yaroze/ftp/scee/tutor/chap8.zip.x/1",
	"yaroze/ftp/scee/tutor/chap8.zip.x/2",
	"yaroze/ftp/scee/tutor/chap8.zip.x/3",
	"yaroze/ftp/scee/tutor/chap8.zip.x/4",
	"yaroze/ftp/scee/tutor/chap8.zip.x/5",
	"yaroze/ftp/scee/tutor/chap8.zip.x/6",
	"yaroze/ftp/scee/tutor/chap8.zip.x/7",
	"yaroze/ftp/scee/tutor/chap9.zip.x/1",
	"yaroze/ftp/scei/sample/graphics/cube.zip.x",
	"yaroze_scea/demos/scea/step1.zip.x",
	"yaroze_scea/demos/scee/chap1.zip.x/1",
	"yaroze_scea/demos/scee/chap1.zip.x/2",
	"yaroze_scea/demos/scee/chap1.zip.x/3",
	"yaroze_scea/demos/scee/chap1.zip.x/4",
	"yaroze_scea/demos/scee/chap1.zip.x/5",
	"yaroze_scea/demos/scee/chap1.zip.x/6",
	"yaroze_scea/demos/scee/chap2.zip.x/1",
	"yaroze_scea/demos/scee/chap2.zip.x/10",
	"yaroze_scea/demos/scee/chap2.zip.x/2",
	"yaroze_scea/demos/scee/chap2.zip.x/3",
	"yaroze_scea/demos/scee/chap2.zip.x/4",
	"yaroze_scea/demos/scee/chap2.zip.x/5",
	"yaroze_scea/demos/scee/chap2.zip.x/6",
	"yaroze_scea/demos/scee/chap2.zip.x/7",
	"yaroze_scea/demos/scee/chap2.zip.x/8",
	"yaroze_scea/demos/scee/chap2.zip.x/9",
	"yaroze_scea/demos/scee/chap3.zip.x/1",
	"yaroze_scea/demos/scee/chap3.zip.x/2",
	"yaroze_scea/demos/scee/chap4.zip.x/1",
	"yaroze_scea/demos/scee/chap4.zip.x/2",
	"yaroze_scea/demos/scee/chap4.zip.x/3",
	"yaroze_scea/demos/scee/chap5.zip.x/1",
	"yaroze_scea/demos/scee/chap5.zip.x/2",
	"yaroze_scea/demos/scee/chap5.zip.x/3",
	"yaroze_scea/demos/scee/chap6.zip.x/1",
	"yaroze_scea/demos/scee/chap6.zip.x/2",
	"yaroze_scea/demos/scee/chap7.zip.x/1",
	"yaroze_scea/demos/scee/chap7.zip.x/2",
	"yaroze_scea/demos/scee/chap7.zip.x/3",
	"yaroze_scea/demos/scee/chap7.zip.x/4",
	"yaroze_scea/demos/scee/chap7.zip.x/5",
	"yaroze_scea/demos/scee/chap7.zip.x/6",
	"yaroze_scea/demos/scee/chap8.zip.x/1",
	"yaroze_scea/demos/scee/chap8.zip.x/2",
	"yaroze_scea/demos/scee/chap8.zip.x/3",
	"yaroze_scea/demos/scee/chap8.zip.x/4",
	"yaroze_scea/demos/scee/chap8.zip.x/5",
	"yaroze_scea/demos/scee/chap8.zip.x/6",
	"yaroze_scea/demos/scee/chap8.zip.x/7",
	"yaroze_scea/demos/scei/BREZZA01.ZIP.x",
	"yaroze_scea/demos/scei/CAR01.ZIP.x",
	"yaroze_scea/demos/scei/MITEXT.ZIP.x",
	"yaroze_scea/demos/scei/MO96SUM.ZIP.x",
	"yaroze_scea/demos/scei/PADTEST.ZIP.x",
	"yaroze_scei/forum/program/archive/dosvsoft/kdf020.lzh.x/kdf2/kdx",
	"yaroze_scei/forum/program/archive/event/event001/EBG.ZIP.x/ebg/ebg",
	"yaroze_scei/forum/program/archive/event/event001/G-F01.LZH.x/G-F01",
	"yaroze_scei/forum/program/archive/event/event001/KBOPZL01.LZH.x/PZL16",
	"yaroze_scei/forum/program/archive/event/event001/UCH829R1.LZH.x/RON829",
	"yaroze_scei/forum/program/archive/event/event002/eglib002.lzh.x/egform.lzh.x",
	"yaroze_scei/forum/program/archive/event/event002/eglib002.lzh.x/sample.lzh.x/SAMPLE/1",
	"yaroze_scei/forum/program/archive/event/event002/eglib002.lzh.x/sample.lzh.x/SAMPLE/2",
	"yaroze_scei/forum/program/archive/event/event002/eglib002.lzh.x/sample.lzh.x/SAMPLE/3",
	"yaroze_scei/forum/program/archive/event/event002/eglib002.lzh.x/sample.lzh.x/SAMPLE/4",
	"yaroze_scei/forum/program/archive/event/event002/eglib002.lzh.x/sample.lzh.x/SAMPLE/5",
	"yaroze_scei/forum/program/archive/event/event003/free/hijump.lzh.x/hijump",
	"yaroze_scei/forum/program/archive/pssoft/PLADEL-2.lzh.x",
	"yaroze_scei/forum/program/archive/test/DEMO_FM.LZH.x",
	"yaroze_scei/forum/program/archive/test/PADTEST.LZH.x",
	"yaroze_scei/forum/program/archive/test/PANIC101.LZH.x/PANIC",
	"yaroze_scei/forum/program/archive/test/PLADEL.lzh.x",
	"yaroze_scei/forum/program/archive/test/STDPLAY.LZH.x",
	"yaroze_scei/forum/program/archive/test/TEXT.LZH.x/TEXT",
	"yaroze_scei/forum/program/archive/test/eglib005.lzh.x/egform.lzh.x",
	"yaroze_scei/forum/program/archive/test/eglib005.lzh.x/sample.lzh.x/1",
	"yaroze_scei/forum/program/archive/test/eglib005.lzh.x/sample.lzh.x/2",
	"yaroze_scei/forum/program/archive/test/eglib005.lzh.x/sample.lzh.x/3",
	"yaroze_scei/forum/program/archive/test/eglib005.lzh.x/sample.lzh.x/4",
	"yaroze_scei/forum/program/archive/test/eglib005.lzh.x/sample.lzh.x/5",
	"yaroze_scei/forum/program/archive/test/eglib005.lzh.x/sample.lzh.x/7",
	"yaroze_scei/forum/program/archive/test/kdf010.lzh.x/kdf/example",
	"yaroze_scei/forum/program/archive/test/mkspr.lzh.x",
}
