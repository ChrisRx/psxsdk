// Package toolchain compiles a Go program for the Net Yaroze, for yaroze,
// with TinyGo compiled to WebAssembly: the compiler the playground runs in
// the browser, so that no tinygo command, go command or C toolchain is
// needed. It runs under wazero, built in, or under the wasmtime command
// when there is one, which runs it quicker (see Wasm).
//
// The compiler writes the ELF TinyGo links. What follows is yaroze's:
// link.MIPS1Traps and RemoveTraps, then link.Executable.
package toolchain

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Target is the TinyGo target yaroze builds for.
const Target = "playstation-yaroze"

// DefaultOpt is the optimization level a build has when it names none: for
// speed, since a game's frame has a budget. TinyGo's own default is z, for
// size.
const DefaultOpt = "2"

// A Build is one program to compile.
type Build struct {
	Package string   // the directory of the main package
	Opt     string   // TinyGo's optimization level: 0, 1, 2, s or z, or empty for DefaultOpt
	Tags    []string // extra build tags
	Output  string   // the ELF to write

	// Hot builds a program that can take hot reload patches, and HotBase
	// and HotAt build a patch for the program whose ELF HotBase names,
	// linked at HotAt, instead of a program (see internal/hot).
	Hot     bool
	HotBase string
	HotAt   uint32

	Stderr io.Writer // the compiler's output
	Log    func(format string, args ...any)
}

// hotArgs are the compiler flags for b's hot reload settings.
func (b *Build) hotArgs() []string {
	var args []string
	if b.Hot {
		args = append(args, "-hot")
	}
	if b.HotBase != "" {
		args = append(args, "-hot-base="+b.HotBase, fmt.Sprintf("-hot-at=0x%x", b.HotAt))
	}
	return args
}

func (b *Build) logf(format string, args ...any) {
	if b.Log != nil {
		b.Log(format, args...)
	}
}

// cacheDir is yaroze's cache directory, YAROZE_CACHE or yaroze under the user's,
// made if need be.
func cacheDir() (string, error) {
	dir := os.Getenv("YAROZE_CACHE")
	if dir == "" {
		base, err := os.UserCacheDir()
		if err != nil {
			return "", err
		}
		dir = filepath.Join(base, "yaroze")
	}
	return dir, os.MkdirAll(dir, 0o777)
}

// since returns the time since start, rounded for a log line.
func since(start time.Time) time.Duration {
	return time.Since(start).Round(time.Millisecond)
}

// output runs a command and returns its standard output, trimmed.
func output(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	cmd.Stderr = os.Stderr
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("%s %s: %w", name, strings.Join(args, " "), err)
	}
	return strings.TrimSpace(string(out)), nil
}
