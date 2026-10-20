// Package toolchain compiles a Go program for the Net Yaroze with TinyGo,
// for nyc. TinyGo comes in one of two ways:
//
//   - Native runs the tinygo command. A TinyGo without the PlayStation
//     targets gets them from the fork, whose files are embedded here and
//     laid over a copy of its root made of links (Overlay).
//   - Wasm runs TinyGo compiled to WebAssembly, the compiler the
//     playground runs in the browser, under wazero, with no tinygo or go
//     command needed at all.
//
// Either writes the ELF TinyGo links. What follows is the same for both:
// link.MIPS1Traps and RemoveTraps, then link.Executable.
package toolchain

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"
)

// Target is the TinyGo target nyc builds for.
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

	Stderr io.Writer // the compiler's output
	Log    func(format string, args ...any)
}

func (b *Build) logf(format string, args ...any) {
	if b.Log != nil {
		b.Log(format, args...)
	}
}

// A Compiler compiles programs for the target.
type Compiler interface {
	// Name is the compiler's name, for messages: "tinygo" or "wasm".
	Name() string
	// Compile builds b, writing the ELF to b.Output.
	Compile(ctx context.Context, b *Build) error
}

// Find returns the compiler named, or with "auto" the native one when there
// is a tinygo command and the WebAssembly one otherwise.
func Find(name string) (Compiler, error) {
	switch name {
	case "tinygo", "native":
		return NewNative()
	case "wasm":
		return NewWasm()
	case "", "auto":
		if c, err := NewNative(); err == nil {
			return c, nil
		}
		c, err := NewWasm()
		if err != nil {
			return nil, fmt.Errorf("no compiler: there is no tinygo command, and %w", err)
		}
		return c, nil
	}
	return nil, fmt.Errorf("compiler %q is not tinygo, wasm or auto", name)
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
