package toolchain

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"slices"
)

// BaseTarget is the TinyGo target every build is for: the PlayStation
// alone, a program that owns the console with only the BIOS underneath.
// What makes a program a Net Yaroze's is laid over it, a Mode at a time.
const BaseTarget = "playstation"

// TargetDir is the directory under the compiler's TINYGOROOT that holds
// what is laid over BaseTarget: the files of internal/toolchain/yaroze,
// which the tasks that make a sysroot copy there, and the target files
// WriteTargets writes beside them.
const TargetDir = "yaroze"

// A Mode is a way of building a program for the Net Yaroze: which library
// it gets, and with it the build tags, the flags, the link script and the
// C and assembly that go over BaseTarget.
type Mode string

const (
	// Static links the clean library, internal/libps, into the program,
	// which loads where the resident library would be, at 0x80010000. It
	// is what yaroze builds unless told otherwise.
	Static Mode = "static"

	// Resident lays the program over the resident library, as a Net
	// Yaroze runs one: the boot disc's library on real hardware, and the
	// image link.Libps holds otherwise. The program loads above it, at
	// 0x80090000.
	Resident Mode = "resident"

	// Fidelity is Static with Sony's timing and leftover memory kept, as
	// the resident image has them, for comparing with a reference
	// (internal/libps/build.sh). No cache holds its C and assembly
	// compiled, so only the compiler with clang builds it.
	Fidelity Mode = "fidelity"
)

// Modes are the modes WriteTargets writes a target file for.
var Modes = []Mode{Static, Resident, Fidelity}

// A targetFile is a TinyGo target file, the fields yaroze sets.
type targetFile struct {
	Comment      string   `json:"comment,omitempty"`
	Inherits     []string `json:"inherits,omitempty"`
	BuildTags    []string `json:"build-tags,omitempty"`
	CFlags       []string `json:"cflags,omitempty"`
	LDFlags      []string `json:"ldflags,omitempty"`
	LinkerScript string   `json:"linkerscript,omitempty"`
	ExtraFiles   []string `json:"extra-files,omitempty"`
}

// firstTarget is the name of the target file every mode inherits ahead of
// BaseTarget. TinyGo links a target's files in the order its target files
// name them, parents first, and these have to come before the base's:
// the start-up is first on the link line so that _start is at the
// program's address, and the resident library's entry points, which are
// weak, come before the base target's weak C library so that the linker
// takes the library's memcpy and the rest, the first of two weak
// definitions, over the base's.
const firstTarget = "first"

// first is the target file of that name: the start-up, the resident
// library's entry points, which are empty in a static build, and the
// members and BIOS and GTE calls both libraries' programs link.
func first() targetFile {
	return targetFile{
		Comment: "The Net Yaroze files that go ahead of the playstation target's on the link line. Written by yaroze (internal/toolchain); inherited by the mode targets beside it and nothing to build for.",
		ExtraFiles: targetPaths(
			"start.S",
			"symusr.S",
			"symusr-orig.S",
			"cd.S",
			"bios.S",
			"members.S",
			"gte.S",
		),
	}
}

// target is the target file of a mode, for a compiler that sees the
// target files in dir. What it sets goes over BaseTarget's: the link
// script in place of its own, and runtime.c, the runtime's hooks answered
// by the library, in place of the weak defaults its files define.
func (m Mode) target(dir string) (targetFile, error) {
	t := targetFile{
		Inherits:     []string{path.Join(dir, firstTarget+".json"), BaseTarget},
		BuildTags:    []string{"yaroze"},
		LinkerScript: path.Join(TargetDir, "yaroze.ld"),
	}
	switch m {
	case Resident:
		t.Comment = "A Net Yaroze program laid over the resident library: it loads above the library, at 0x80090000, and calls into it at fixed addresses rather than linking a copy."
		t.ExtraFiles = targetPaths("runtime.c")
		return t, nil
	case Static, Fidelity:
		t.Comment = "A Net Yaroze program with its library linked in: internal/libps, a clean-room reimplementation in Go, loaded where the resident library sits on a Net Yaroze, so only what the program calls is linked."
		t.BuildTags = append(t.BuildTags, "cleanlib")
		t.CFlags = []string{"-DLIBPS_GO", "-DLIBPS_CLEAN"}
		t.LDFlags = []string{"--defsym=__load=0x80010000", "--defsym=__libps_heap_size=0"}
		t.ExtraFiles = targetPaths("libc.c", "libps-data.c", "runtime.c")
		if m == Fidelity {
			t.Comment = "The static build with Sony's timing and leftover memory kept, as the resident image has them, for comparing with a reference (internal/libps/build.sh). Programs are built static, which leaves them out."
			t.BuildTags = append(t.BuildTags, "fidelity")
			t.CFlags = append(t.CFlags, "-DLIBPS_FIDELITY")
		}
		return t, nil
	}
	return targetFile{}, fmt.Errorf("%q is no way to build for the Net Yaroze: static, resident or fidelity", string(m))
}

// targetPaths are the files named, of TargetDir, as a target file names
// them: from the compiler's TINYGOROOT.
func targetPaths(names ...string) []string {
	paths := make([]string, len(names))
	for i, name := range names {
		paths[i] = path.Join(TargetDir, name)
	}
	return paths
}

// TargetPath is the target file of a mode among those WriteTargets wrote
// to a directory the compiler sees as dir.
func (m Mode) TargetPath(dir string) string {
	return path.Join(dir, string(m)+".json")
}

// WriteTargets writes the target file of every mode into hostDir, for a
// compiler that sees that directory as dir: a sysroot's
// TINYGOROOT/yaroze, which the compiler has at /sysroot/tinygo/yaroze.
// A target file names the ones it inherits by path, so the files work
// only where the compiler sees them at dir.
func WriteTargets(hostDir, dir string) error {
	files := map[string]targetFile{firstTarget: first()}
	for _, m := range Modes {
		t, err := m.target(dir)
		if err != nil {
			return err
		}
		t.Comment += " Written by yaroze (internal/toolchain)."
		files[string(m)] = t
	}
	for name, t := range files {
		if err := writeTarget(filepath.Join(hostDir, name+".json"), t); err != nil {
			return err
		}
	}
	return nil
}

func writeTarget(file string, t any) error {
	data, err := json.MarshalIndent(t, "", "\t")
	if err != nil {
		return err
	}
	return os.WriteFile(file, append(data, '\n'), 0o666)
}

// overTarget writes a copy of the target file a program brings, src, to
// dst with the target file at base in place of BaseTarget among what it
// inherits, so that what the program's file sets goes over a mode's. A
// program's target file inherits BaseTarget, as any target file for the
// console does, and yaroze is what puts the Net Yaroze under it.
func overTarget(src, dst, base string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return fmt.Errorf("%s: %w", src, err)
	}
	var inherits []string
	if raw, ok := fields["inherits"]; ok {
		if err := json.Unmarshal(raw, &inherits); err != nil {
			return fmt.Errorf("%s: inherits: %w", src, err)
		}
	}
	i := slices.Index(inherits, BaseTarget)
	if i < 0 {
		return fmt.Errorf("%s does not inherit %s: a target file for yaroze inherits it, \"inherits\": [%q], and yaroze lays it over the Net Yaroze library's own files", src, BaseTarget, BaseTarget)
	}
	inherits[i] = base
	if fields["inherits"], err = json.Marshal(inherits); err != nil {
		return err
	}
	return writeTarget(dst, fields)
}
