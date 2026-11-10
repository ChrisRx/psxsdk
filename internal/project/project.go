// Package project finds a Net Yaroze program to run in what a person was
// handed: a directory, or a zip such as the showcase's, holding the
// executable, its data and the auto file that says what goes where, or the
// executable alone. The auto file is the project: it names the program and
// the files loaded beside it. Without one, a lone executable is.
package project

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"go.chrisrx.dev/psx/format/auto"
	"go.chrisrx.dev/psx/format/psx"
	"go.chrisrx.dev/psx/link"
)

// A Project is a program to run and what goes with it.
type Project struct {
	// Dir is the directory the program's files are in, which goes on a
	// disc as its root, for a program that reads them from the CD.
	Dir string

	// Program is the executable: a PSX-EXE, an ECOFF or an ELF.
	Program string

	// Auto is the auto file the project is from, or empty.
	Auto string

	// Loads are the files put into memory before the program runs.
	Loads []auto.Load

	// Missing names the files the auto file loads that are not there.
	Missing []string

	// Skipped are the auto file's lines that load nothing and start
	// nothing, which running the project does without.
	Skipped []string

	tmp string // the directory a zip was unpacked into
}

// Close removes what Open unpacked.
func (p *Project) Close() error {
	if p.tmp == "" {
		return nil
	}
	return os.RemoveAll(p.tmp)
}

// Name is the project's name: its program's, without the extension.
func (p *Project) Name() string {
	base := filepath.Base(p.Program)
	return strings.TrimSuffix(base, filepath.Ext(base))
}

// Executable is the program as a PSX-EXE that runs without a Net Yaroze's
// boot disc, with its loads in it, then extra, as link.Runnable makes it.
func (p *Project) Executable(extra ...link.Load) (*psx.File, error) {
	var loads []link.Load
	for _, l := range p.Loads {
		data, err := os.ReadFile(l.File)
		if err != nil {
			return nil, err
		}
		loads = append(loads, link.Load{Addr: l.Addr, Data: data})
	}
	return link.Runnable(p.Program, append(loads, extra...)...)
}

// FromAuto reads the project the auto file at path describes. A program
// given overrides the one it loads.
func FromAuto(path, program string) (*Project, error) {
	b, err := auto.Read(path)
	if err != nil {
		return nil, err
	}
	p := &Project{
		Dir:     b.Dir,
		Program: b.Program(),
		Auto:    path,
		Missing: b.Missing,
		Skipped: b.Skipped,
	}
	for _, l := range b.Loads {
		p.Loads = append(p.Loads, l)
	}
	if program != "" {
		p.Program = program
	}
	if p.Program == "" {
		if len(b.Missing) > 0 {
			return nil, fmt.Errorf("%s: the program it loads is not there: %s", path, strings.Join(b.Missing, ", "))
		}
		return nil, fmt.Errorf("%s loads no program", path)
	}
	return p, nil
}

// Options choose within a directory or a zip.
type Options struct {
	// Auto is the auto file to run, a path inside as DOS would take it.
	Auto string

	// Program is the executable to run, a path inside as DOS would take
	// it, with the files an auto file beside it that loads it gives.
	Program string
}

// Open finds the project in path: a directory, a zip, or an executable.
//
// In a directory or a zip, the auto file is the project, and there has to
// be one, unless the options name one, or the program. An auto file beside
// another called auto.bat is the one, which SIOCONS ran. With no auto
// file, an executable is the project, if it is the only one.
//
// Given an executable, an auto file beside it that loads it gives the
// files that go with it.
func Open(path string, o Options) (*Project, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if fi.IsDir() {
		return openDir(path, o)
	}
	if strings.EqualFold(filepath.Ext(path), ".zip") {
		tmp, err := os.MkdirTemp("", "yaroze-project-")
		if err != nil {
			return nil, err
		}
		if err := unzip(path, tmp); err != nil {
			os.RemoveAll(tmp)
			return nil, err
		}
		p, err := openDir(tmp, o)
		if err != nil {
			os.RemoveAll(tmp)
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		p.tmp = tmp
		return p, nil
	}
	if o != (Options{}) {
		return nil, errors.New("an auto file or a program is chosen in a directory or a zip")
	}
	return openProgram(path)
}

func openDir(dir string, o Options) (*Project, error) {
	switch {
	case o.Auto != "":
		path, ok := auto.Find(dir, o.Auto)
		if !ok {
			return nil, fmt.Errorf("no %s in it", o.Auto)
		}
		p, err := FromAuto(path, "")
		if err != nil || o.Program == "" {
			return p, err
		}
		prog, ok := auto.Find(dir, o.Program)
		if !ok {
			return nil, fmt.Errorf("no %s in it", o.Program)
		}
		p.Program = prog
		return p, nil
	case o.Program != "":
		path, ok := auto.Find(dir, o.Program)
		if !ok {
			return nil, fmt.Errorf("no %s in it", o.Program)
		}
		return openProgram(path)
	}
	autos, programs, err := scan(dir)
	if err != nil {
		return nil, err
	}
	autos = siocons(autos)
	switch {
	case len(autos) == 1:
		return FromAuto(autos[0], "")
	case len(autos) > 1:
		return nil, fmt.Errorf("it holds %d auto files, and one has to be chosen: %s", len(autos), relList(dir, autos))
	case len(programs) == 1:
		return &Project{Dir: filepath.Dir(programs[0]), Program: programs[0]}, nil
	case len(programs) > 1:
		return nil, fmt.Errorf("it has no auto file, and %d executables, of which one has to be chosen: %s", len(programs), relList(dir, programs))
	}
	return nil, errors.New("it has no auto file and no executable")
}

// siocons drops each auto.bat beside an auto, the one SIOCONS ran, which
// some samples kept beside the batch file for the earlier tools.
func siocons(autos []string) []string {
	plain := map[string]bool{}
	for _, a := range autos {
		if strings.EqualFold(filepath.Base(a), "auto") {
			plain[filepath.Dir(a)] = true
		}
	}
	return slices.DeleteFunc(autos, func(a string) bool {
		return !strings.EqualFold(filepath.Base(a), "auto") && plain[filepath.Dir(a)]
	})
}

// openProgram is the project of an executable, with the files an auto file
// beside it loads, if one there loads it.
func openProgram(path string) (*Project, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	p := &Project{Dir: filepath.Dir(abs), Program: abs}
	entries, err := os.ReadDir(p.Dir)
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if e.IsDir() || !auto.IsName(e.Name()) {
			continue
		}
		a, err := FromAuto(filepath.Join(p.Dir, e.Name()), "")
		if err != nil || !sameFile(a.Program, abs) {
			continue
		}
		return a, nil
	}
	return p, nil
}

func sameFile(a, b string) bool {
	fa, err := os.Stat(a)
	if err != nil {
		return false
	}
	fb, err := os.Stat(b)
	return err == nil && os.SameFile(fa, fb)
}

// scan finds the auto files and the executables under dir.
func scan(dir string) (autos, programs []string, err error) {
	err = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == "__MACOSX" {
				return filepath.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() {
			return nil
		}
		if auto.IsName(d.Name()) {
			autos = append(autos, path)
			return nil
		}
		if isExecutable(path) {
			programs = append(programs, path)
		}
		return nil
	})
	return autos, programs, err
}

// isExecutable reports whether the file at path is an executable Runnable
// takes, by its first bytes and then by reading it as one.
func isExecutable(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	head := make([]byte, 8)
	if _, err := io.ReadFull(f, head); err != nil || !link.IsExecutable(head) {
		return false
	}
	if link.IsPSXEXE(head) {
		return true
	}
	_, err = link.ReadImage(f)
	return err == nil
}

func relList(dir string, paths []string) string {
	var names []string
	for _, p := range paths {
		r, err := filepath.Rel(dir, p)
		if err != nil {
			r = p
		}
		names = append(names, filepath.ToSlash(r))
	}
	slices.Sort(names)
	return strings.Join(names, ", ")
}

// ZipHasCue reports whether the zip at path holds a cue sheet, which makes
// it a disc rather than a project.
func ZipHasCue(path string) (bool, error) {
	r, err := zip.OpenReader(path)
	if err != nil {
		return false, err
	}
	defer r.Close()
	for _, f := range r.File {
		if strings.EqualFold(filepath.Ext(f.Name), ".cue") {
			return true, nil
		}
	}
	return false, nil
}

// unzip unpacks the zip at path into dir, refusing any name that would
// land outside it.
func unzip(path, dir string) error {
	r, err := zip.OpenReader(path)
	if err != nil {
		return err
	}
	defer r.Close()
	for _, f := range r.File {
		name := filepath.FromSlash(strings.ReplaceAll(f.Name, `\`, "/"))
		if !filepath.IsLocal(name) {
			return fmt.Errorf("%s: %q is outside the archive", path, f.Name)
		}
		dst := filepath.Join(dir, name)
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(dst, 0o755); err != nil {
				return err
			}
			continue
		}
		if !f.Mode().IsRegular() {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		if err := extract(f, dst); err != nil {
			return err
		}
	}
	return nil
}

func extract(f *zip.File, dst string) error {
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, rc); err != nil {
		return fmt.Errorf("%s: %w", f.Name, err)
	}
	return os.WriteFile(dst, buf.Bytes(), 0o644)
}
