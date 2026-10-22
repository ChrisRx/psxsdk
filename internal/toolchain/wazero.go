package toolchain

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/ChrisRx/wazero"
	"github.com/ChrisRx/wazero/api"
	"github.com/ChrisRx/wazero/experimental"
	"github.com/ChrisRx/wazero/imports/wasi_snapshot_preview1"
	"github.com/ChrisRx/wazero/sys"
)

// wazeroRuntime runs the compiler's module in-process under wazero. The
// module, loaded by the first build, is kept for the next (see load), as a
// process making more than one build, yaroze run making patches say, pays
// for loading once and gets what the compiler keeps between builds, the
// standard library parsed and type-checked. Across processes a snapshot of
// the compiler's memory does the same (see wasm_snapshot.go).
type wazeroRuntime struct {
	w   *Wasm
	rt  wazero.Runtime
	mod api.Module
	out *switchWriter

	// The snapshot the module is restored from and written to, or empty
	// with why in snapshotErr; whether the module loaded was restored from
	// it, and whether it has been written by this process.
	snapshot    string
	snapshotErr error
	restored    bool
	saved       bool
}

func newWazero(w *Wasm) *wazeroRuntime {
	r := &wazeroRuntime{w: w}
	r.snapshot, r.snapshotErr = snapshotPath(w)
	return r
}

func (r *wazeroRuntime) name() string { return "wazero" }

func (r *wazeroRuntime) describe() string { return "wazero, built in" }

func (r *wazeroRuntime) build(ctx context.Context, b *Build, args []string) error {
	for attempt := 0; ; attempt++ {
		if err := r.load(ctx, b); err != nil {
			return err
		}
		r.out.w = b.Stderr
		err := buildWithReactor(ctx, r.mod, args)
		r.out.w = io.Discard
		var exit *sys.ExitError
		if errors.As(err, &exit) {
			if exit.ExitCode() != 0 {
				return fmt.Errorf("tinygo (wasm) exited with %d", exit.ExitCode())
			}
		} else if err != nil {
			// The module is in no state to build again: the next build
			// loads it afresh. When it was restored from a snapshot, the
			// snapshot may be what is wrong: it goes, and the build is
			// tried once more from a fresh start.
			r.unload(ctx)
			if r.restored && attempt == 0 {
				b.logf("wasm: the compiler restored from %s failed: %v; starting it afresh", r.snapshot, err)
				os.Remove(r.snapshot)
				continue
			}
			return fmt.Errorf("tinygo (wasm): %w", err)
		}
		break
	}
	r.save(b)
	return nil
}

// load has the module loaded and initialized, if it is not.
func (r *wazeroRuntime) load(ctx context.Context, b *Build) error {
	if r.mod != nil {
		return nil
	}
	w := r.w
	start := time.Now()
	module, err := os.ReadFile(filepath.Join(w.dir, "tinygo.wasm"))
	if err != nil {
		return err
	}
	ccache, err := wazero.NewCompilationCacheWithDir(filepath.Join(w.dir, "wazero"))
	if err != nil {
		return err
	}
	// No WithCloseOnContextDone: to stop a module when the context ends,
	// wazero calls out to Go at the head of every loop, which made the
	// compiler ten times slower than under wasmtime. Interrupting yaroze ends
	// the process, and the module with it. The wazero is a fork for the
	// other option: validating the 41MB module's function bodies again
	// when its compiled code is in the cache was most of loading it.
	rt := wazero.NewRuntimeWithConfig(ctx, wazero.NewRuntimeConfig().
		WithCompilationCache(ccache).
		WithSkipValidationOnCacheHit(true))
	wasi_snapshot_preview1.MustInstantiate(ctx, rt)
	b.logf("wasm: read %d MB in %s", len(module)>>20, since(start))

	// The first compilation of the module is most of a minute on one
	// thread, which is wazero's default, and the cache makes it once.
	start = time.Now()
	compiled, err := rt.CompileModule(experimental.WithCompilationWorkers(ctx, runtime.GOMAXPROCS(0)), module)
	if err != nil {
		rt.Close(ctx)
		return fmt.Errorf("compiling tinygo.wasm: %w", err)
	}
	b.logf("wasm: compiled the module in %s", since(start))
	if compiled.ExportedFunctions()["tinygo_build"] == nil {
		rt.Close(ctx)
		return errors.New("tinygo.wasm is not a build of the compiler this yaroze runs: it has no tinygo_build")
	}

	wd, err := os.Getwd()
	if err != nil {
		rt.Close(ctx)
		return err
	}
	cache := filepath.Join(w.sysroot, "cache")

	// The host's file system is the compiler's, read only, so that the
	// program, a replace directive's directory and the module cache are
	// where the host has them. Mounts on longer paths take precedence.
	fsc := wazero.NewFSConfig().
		WithReadOnlyDirMount("/", "/").
		WithReadOnlyDirMount(w.sysroot, guestSysroot).
		WithDirMount(w.tmp, guestTmp).
		WithDirMount(cache, guestCache+"/tinygo")
	r.out = &switchWriter{w: io.Discard}
	cfg := wazero.NewModuleConfig().
		WithName("").
		WithArgs("tinygo").
		WithFSConfig(fsc).
		WithStdout(r.out).
		WithStderr(r.out).
		WithRandSource(rand.Reader).
		WithSysWalltime().
		WithSysNanotime().
		WithSysNanosleep()
	for k, v := range w.environ(wd) {
		cfg = cfg.WithEnv(k, v)
	}

	// A snapshot, when there is one, stands in for the compiler's start.
	var rs restorer
	if r.snapshot != "" {
		f, size, err := openSnapshot(r.snapshot)
		switch {
		case err == nil:
			rs = newRestorer(f, size, int64(len(module)))
		case !errors.Is(err, fs.ErrNotExist):
			b.logf("wasm: not restoring %s: %v", r.snapshot, err)
		}
	} else if r.snapshotErr != nil {
		b.logf("wasm: keeping no snapshot: %v", r.snapshotErr)
	}
	start = time.Now()
	var mod api.Module
	if rs != nil {
		mod, err = rt.InstantiateModule(rs.context(ctx), compiled, cfg.WithStartFunctions())
		if err == nil {
			err = rs.apply(mod)
		}
		rs.close()
		if err != nil {
			// The compiler is started afresh instead, and the snapshot goes.
			b.logf("wasm: not restored from %s: %v", r.snapshot, err)
			os.Remove(r.snapshot)
			if mod != nil {
				mod.Close(ctx)
			}
			rs = nil
		}
	}
	if rs == nil {
		mod, err = rt.InstantiateModule(ctx, compiled, cfg.WithStartFunctions("_initialize"))
	}
	if err != nil {
		if mod != nil {
			mod.Close(ctx)
		}
		rt.Close(ctx)
		return fmt.Errorf("tinygo (wasm): %w", err)
	}
	r.restored = rs != nil
	if r.restored {
		touch(r.snapshot, usedInterval)
		b.logf("wasm: restored the compiler from %s in %s", r.snapshot, since(start))
	} else {
		b.logf("wasm: loaded the module in %s", since(start))
	}
	r.rt, r.mod = rt, mod
	return nil
}

// save keeps the module's memory as the snapshot, after the first build of
// a process that did not restore one, if there is still none.
func (r *wazeroRuntime) save(b *Build) {
	if r.snapshot == "" || r.restored || r.saved {
		return
	}
	r.saved = true
	if _, err := os.Lstat(r.snapshot); err == nil {
		return
	}
	start := time.Now()
	n, err := writeSnapshot(r.mod, r.snapshot)
	if err != nil {
		b.logf("wasm: not keeping a snapshot: %v", err)
		return
	}
	b.logf("wasm: kept the compiler's state, %d MB, at %s in %s", n>>20, r.snapshot, since(start))
}

// unload lets the module go.
func (r *wazeroRuntime) unload(ctx context.Context) {
	if r.mod != nil {
		r.mod.Close(ctx)
		r.mod = nil
	}
	if r.rt != nil {
		r.rt.Close(ctx)
		r.rt = nil
	}
}

func (r *wazeroRuntime) close(ctx context.Context) { r.unload(ctx) }

// A switchWriter writes to whichever writer it holds: the module's output
// goes to the build being made.
type switchWriter struct {
	w io.Writer
}

func (s *switchWriter) Write(p []byte) (int, error) { return s.w.Write(p) }

// buildWithReactor asks the initialized reactor module for one build: the
// arguments go, separated by NUL bytes, where tinygo_args says, and
// tinygo_build returns the status a run would have exited with.
func buildWithReactor(ctx context.Context, mod api.Module, args []string) error {
	at, err := mod.ExportedFunction("tinygo_args").Call(ctx)
	if err != nil {
		return err
	}
	joined := []byte(strings.Join(args, "\x00"))
	if !mod.Memory().Write(uint32(at[0]), joined) {
		return errors.New("the arguments do not fit the module's buffer")
	}
	status, err := mod.ExportedFunction("tinygo_build").Call(ctx, uint64(len(joined)))
	if err != nil {
		return err
	}
	if code := int32(status[0]); code != 0 {
		return sys.NewExitError(uint32(code))
	}
	return nil
}
