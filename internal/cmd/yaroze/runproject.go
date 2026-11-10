package yaroze

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"go.chrisrx.dev/psx/format/auto"
	"go.chrisrx.dev/psx/internal/cmd/mkdisc"
	"go.chrisrx.dev/psx/internal/cproject"
	"go.chrisrx.dev/psx/internal/project"
	"go.chrisrx.dev/psx/internal/psxplay"
	"go.chrisrx.dev/psx/link"
)

// isBuildable reports whether dir holds something to build: a Go package
// or a C project.
func isBuildable(dir string) bool {
	if cproject.Is(dir) {
		return true
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if !e.IsDir() && strings.EqualFold(filepath.Ext(e.Name()), ".go") {
			return true
		}
	}
	return false
}

// openProject finds the project run is given, or returns nil if it is
// given a package to build: an auto file, or with arg a zip, a directory
// with nothing to build in it, or an executable. A program given, with an
// auto file, runs in place of the one it loads, and with a zip or a
// directory is the one in it to run.
func openProject(arg, autoPath, program string) (*project.Project, error) {
	if arg == "" {
		if autoPath != "" {
			return project.FromAuto(autoPath, program)
		}
		arg = "."
	}
	fi, err := os.Stat(arg)
	if err != nil {
		return nil, err
	}
	if fi.IsDir() && autoPath == "" && program == "" {
		if isBuildable(arg) {
			return nil, nil
		}
		p, err := project.Open(arg, project.Options{})
		if err != nil {
			return nil, fmt.Errorf("%s has no Go package or C project to build, and %w", arg, err)
		}
		return p, nil
	}
	return project.Open(arg, project.Options{Auto: autoPath, Program: program})
}

// runProject runs p from a disc with its directory as the root, booting
// the program put there beside its files, so that a program reading them
// from the CD finds them. If that disc cannot be made, the program is
// loaded on its own.
func runProject(p *project.Project, f *discFlags, play psxplay.Options) error {
	for _, name := range p.Missing {
		fmt.Fprintf(os.Stderr, "yaroze: %s loads %s, which is not there\n", filepath.Base(p.Auto), name)
	}
	if f.verbose {
		for _, line := range p.Skipped {
			fmt.Fprintf(os.Stderr, "yaroze: skipped from %s: %s\n", filepath.Base(p.Auto), line)
		}
	}
	exe, err := p.Executable(f.loads...)
	if err != nil {
		return err
	}
	tmp, err := os.MkdirTemp("", "yaroze-run-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	path := filepath.Join(tmp, "PROGRAM.EXE")
	if err := exe.WriteFile(path); err != nil {
		return err
	}
	play.Title = "yaroze - " + p.Name()
	play.EXE = ""

	boot := bootName(p.Dir)
	opts := mkdisc.NewOptions()
	opts.Output = filepath.Join(tmp, "disc.bin")
	opts.VolumeID = volumeID(p.Name())
	opts.Dir = p.Dir
	opts.Boot = boot
	opts.Files = append([]string{boot + "=" + path}, f.files...)
	opts.Audio = f.audio
	opts.Stdout = io.Discard
	if err := mkdisc.Build(opts); err != nil {
		fmt.Fprintf(os.Stderr, "yaroze: running %s without its files on a disc, which it could not be put on: %v\n", p.Name(), err)
		play.EXE = path
		return psxplay.Run(play)
	}
	play.Disc = filepath.Join(tmp, "disc.cue")
	return psxplay.Run(play)
}

// bootName is a name for the executable on the disc that none of the
// project's files has.
func bootName(dir string) string {
	for i := 0; ; i++ {
		name := "PROGRAM.EXE"
		if i > 0 {
			name = fmt.Sprintf("PROGRAM%d.EXE", i)
		}
		if _, taken := auto.Find(dir, name); !taken {
			return name
		}
	}
}

// runEXE runs the executable --exe names, with --load's files: a PSX-EXE
// as it is, and an ECOFF or an ELF made into one, as eco2exe does.
func runEXE(f *discFlags, play psxplay.Options) error {
	play.Title = "yaroze - " + filepath.Base(play.EXE)
	data, err := os.ReadFile(play.EXE)
	if err != nil {
		return err
	}
	if link.IsPSXEXE(data) {
		play.Loads = f.loads
		return psxplay.Run(play)
	}
	if !link.IsExecutable(data) {
		return errors.New(play.EXE + " is not a PSX-EXE, an ECOFF or an ELF")
	}
	exe, err := link.Runnable(play.EXE, f.loads...)
	if err != nil {
		return err
	}
	tmp, err := os.MkdirTemp("", "yaroze-run-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	play.EXE = filepath.Join(tmp, "PROGRAM.EXE")
	if err := exe.WriteFile(play.EXE); err != nil {
		return err
	}
	return psxplay.Run(play)
}

// volumeID is a disc label made of name: its letters in upper case, its
// digits, and underscores for the rest.
func volumeID(name string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z':
			return r - 'a' + 'A'
		case r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			return r
		}
		return '_'
	}, name)
}
