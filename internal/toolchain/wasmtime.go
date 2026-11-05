package toolchain

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// wasmtimeRuntime runs the compiler's module under the wasmtime command,
// once per build. Its code runs the compiler about twice as fast as
// wazero's, and the module it compiles once, kept beside the unpacked
// compiler, loads in a few milliseconds, so a build takes about as long
// as one from a snapshot under wazero, with nothing kept in memory or in
// the temporary directory. The command takes the arguments from
// TINYGO_ARGS and calls tinygo_main, since it cannot write them into the
// module as wazero does (see cmd/tinygo-wasm in the compiler's patch).
type wasmtimeRuntime struct {
	w       *Wasm
	path    string // the command
	version string
	cwasm   string // the module compiled by this version of the command
}

// newWasmtime finds the wasmtime command, YAROZE_WASMTIME or wasmtime on the
// PATH, and its version, which its compiled modules are tied to.
func newWasmtime(w *Wasm) (*wasmtimeRuntime, error) {
	path := os.Getenv("YAROZE_WASMTIME")
	if path == "" {
		var err error
		if path, err = exec.LookPath("wasmtime"); err != nil {
			return nil, errors.New("there is no wasmtime command on the PATH")
		}
	}
	out, err := output(path, "--version")
	if err != nil {
		return nil, err
	}
	fields := strings.Fields(out)
	if len(fields) < 2 || fields[0] != "wasmtime" {
		return nil, fmt.Errorf("%s --version said %q, not a wasmtime version", path, out)
	}
	version := fields[len(fields)-1]
	return &wasmtimeRuntime{
		w:       w,
		path:    path,
		version: version,
		cwasm:   filepath.Join(w.dir, "wasmtime-"+version+".cwasm"),
	}, nil
}

func (r *wasmtimeRuntime) name() string { return "wasmtime" }

func (r *wasmtimeRuntime) describe() string {
	return fmt.Sprintf("wasmtime %s, %s", r.version, r.path)
}

func (r *wasmtimeRuntime) build(ctx context.Context, b *Build, args []string) error {
	if err := r.compile(b); err != nil {
		return err
	}
	w := r.w
	wd, err := os.Getwd()
	if err != nil {
		return err
	}
	// The host's file system is the compiler's, so that the program, a
	// replace directive's directory and the module cache are where the
	// host has them; the command has no read-only mount. Mounts on longer
	// paths take precedence.
	cmdArgs := []string{"run", "--allow-precompiled",
		"--dir", "/::/",
		"--dir", w.sysroot + "::" + guestSysroot,
		"--dir", w.tmp + "::" + guestTmp,
		"--dir", filepath.Join(w.sysroot, "cache") + "::" + guestCache + "/tinygo",
	}
	for k, v := range w.environ(wd) {
		cmdArgs = append(cmdArgs, "--env", k+"="+v)
	}
	cmdArgs = append(cmdArgs, "--env", "TINYGO_ARGS="+strings.Join(args, "\x1f"), "--invoke", "tinygo_main", r.cwasm)
	cmd := exec.CommandContext(ctx, r.path, cmdArgs...)
	cmd.Stdout = b.Stderr
	cmd.Stderr = b.Stderr
	err = cmd.Run()
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return fmt.Errorf("tinygo (wasm) exited with %d", exit.ExitCode())
	} else if err != nil {
		return fmt.Errorf("tinygo (wasm) under wasmtime: %w", err)
	}
	return nil
}

// compile has the command's compiled module there.
func (r *wasmtimeRuntime) compile(b *Build) error {
	return r.precompile(b.Stderr, b.logf, filepath.Join(r.w.dir, "tinygo.wasm"), r.cwasm)
}

// precompile has the command compile module to cwasm, unless that is done,
// by way of a file beside it, so that a compilation cut short is never run.
func (r *wasmtimeRuntime) precompile(stderr io.Writer, logf func(string, ...any), module, cwasm string) error {
	if _, err := os.Stat(cwasm); err == nil {
		return nil
	}
	start := time.Now()
	f, err := os.CreateTemp(filepath.Dir(cwasm), "wasmtime-*.cwasm.tmp")
	if err != nil {
		return err
	}
	f.Close()
	cmd := exec.Command(r.path, "compile", "-o", f.Name(), module)
	cmd.Stdout = stderr
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		os.Remove(f.Name())
		return fmt.Errorf("wasmtime compile: %w", err)
	}
	if err := os.Rename(f.Name(), cwasm); err != nil {
		os.Remove(f.Name())
		return err
	}
	logf("wasm: wasmtime %s compiled %s in %s", r.version, filepath.Base(module), since(start))
	return nil
}

// runCommand runs a wasmtime command line, writing its output to stderr.
func runCommand(ctx context.Context, stderr io.Writer, path string, args []string) error {
	cmd := exec.CommandContext(ctx, path, args...)
	cmd.Stdout = stderr
	cmd.Stderr = stderr
	err := cmd.Run()
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return fmt.Errorf("exited with %d", exit.ExitCode())
	}
	return err
}

func (r *wasmtimeRuntime) close(ctx context.Context) {}
