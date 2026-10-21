package toolchain

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/experimental"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
	"github.com/tetratelabs/wazero/sys"
)

// Wasm is TinyGo compiled to WebAssembly, the playground's compiler, run
// under wazero. It reads everything from a sysroot, as it does in the
// browser: the standard library merged with TinyGo's, the target files,
// and a cache holding the target's C and assembly already compiled, since
// the compiler has no clang. The program and the modules it uses come from
// the host's file system, read only.
//
// The compiler and the sysroot are those of the site's play/ directory,
// tinygo.wasm and sysroot.tar.gz: the ones given to Embed, as cmd/nycw
// does, or else those of NYC_WASM when it names such a directory. NYC_WASM
// is ignored when there are embedded ones. Either is unpacked once into
// the user's cache directory, and wazero's compilation of the module,
// which takes a while, is kept there too.
type Wasm struct {
	dir     string // the unpacked compiler: tinygo.wasm and sysroot
	sysroot string
	env     sysrootEnv

	// The module, loaded by the first build and kept for the next (see
	// load), its output's writer, the directory it writes to, and how
	// many builds it has made, which name their outputs.
	rt     wazero.Runtime
	mod    api.Module
	out    *switchWriter
	tmp    string
	builds int
}

// sysrootEnv is the sysroot's env.json, as tinygo-sysroot and mkcache.sh
// write it.
type sysrootEnv struct {
	GOROOT     string `json:"GOROOT"`
	GOVERSION  string `json:"GOVERSION"`
	GOTOOLTAGS string `json:"GOTOOLTAGS"`
	TINYGOROOT string `json:"TINYGOROOT"`
	Target     string `json:"target"`
	BuildID    string `json:"buildID"`
	// Opts are the optimization levels the cache holds the target's C
	// and assembly compiled at, which are the only levels the compiler,
	// having no clang, can build at. A sysroot written before they were
	// recorded leaves it empty.
	Opts []string `json:"opts"`
}

// Where the compiler sees its own directories. The sysroot has to be at
// /sysroot, where mkcache.sh compiled the target's files, since the cache
// entries for them are keyed by their paths.
const (
	guestSysroot = "/sysroot"
	guestTmp     = "/.nyc/tmp"
	guestCache   = "/.nyc/cache"
)

// The compiler and sysroot given to Embed.
var embeddedWasm, embeddedSysroot []byte

// Embed builds in the WebAssembly compiler, tinygo.wasm gzipped, and its
// sysroot.tar.gz, for NewWasm to use in place of any NYC_WASM.
func Embed(compiler, sysroot []byte) {
	embeddedWasm, embeddedSysroot = compiler, sysroot
}

// NewWasm finds the WebAssembly compiler, built in or from NYC_WASM, and
// unpacks it if it has not been.
func NewWasm() (*Wasm, error) {
	cache, err := cacheDir()
	if err != nil {
		return nil, err
	}
	var key string
	var module func() ([]byte, error)
	var sysroot func() ([]byte, error)
	if len(embeddedWasm) > 0 {
		h := sha256.New()
		h.Write(embeddedWasm)
		h.Write(embeddedSysroot)
		key = hex.EncodeToString(h.Sum(nil))[:16]
		module = func() ([]byte, error) { return gunzip(embeddedWasm) }
		sysroot = func() ([]byte, error) { return embeddedSysroot, nil }
	} else if dir := os.Getenv("NYC_WASM"); dir != "" {
		wasmPath := filepath.Join(dir, "tinygo.wasm")
		sysPath := filepath.Join(dir, "sysroot.tar.gz")
		h := sha256.New()
		for _, p := range []string{wasmPath, sysPath} {
			info, err := os.Stat(p)
			if err != nil {
				return nil, fmt.Errorf("NYC_WASM: %w", err)
			}
			abs, _ := filepath.Abs(p)
			fmt.Fprintf(h, "%s %d %d\n", abs, info.Size(), info.ModTime().UnixNano())
		}
		key = hex.EncodeToString(h.Sum(nil))[:16]
		module = func() ([]byte, error) { return os.ReadFile(wasmPath) }
		sysroot = func() ([]byte, error) { return os.ReadFile(sysPath) }
	} else {
		return nil, errors.New("this nyc has no WebAssembly compiler built in (nycw has one); set NYC_WASM to a directory holding tinygo.wasm and sysroot.tar.gz, as the site's play/ does")
	}

	w := &Wasm{dir: filepath.Join(cache, "wasm-"+key)}
	w.sysroot = filepath.Join(w.dir, "sysroot")
	if _, err := os.Stat(filepath.Join(w.dir, ".complete")); err != nil {
		if err := unpack(cache, w.dir, module, sysroot); err != nil {
			return nil, err
		}
	}
	data, err := os.ReadFile(filepath.Join(w.sysroot, "env.json"))
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(data, &w.env); err != nil {
		return nil, fmt.Errorf("%s: %w", filepath.Join(w.sysroot, "env.json"), err)
	}
	if w.env.Target != Target {
		return nil, fmt.Errorf("the WebAssembly compiler's sysroot is for %s, not %s", w.env.Target, Target)
	}
	return w, nil
}

func (w *Wasm) Name() string { return "wasm" }

// opt returns the optimization level to build b at: the one it names,
// which the cache has to hold, or else DefaultOpt, or the first level the
// cache holds if it does not hold that. The playground's sysroot holds only
// z, the level the playground builds at, and task nyc adds 2 to its own.
func (w *Wasm) opt(b *Build) (string, error) {
	if len(w.env.Opts) == 0 {
		if b.Opt == "" {
			return DefaultOpt, nil
		}
		return b.Opt, nil
	}
	if b.Opt != "" {
		if !slices.Contains(w.env.Opts, b.Opt) {
			return "", fmt.Errorf("the WebAssembly compiler's cache holds the target's C and assembly compiled only at -opt=%s, so it cannot build at -opt=%s", strings.Join(w.env.Opts, " and -opt="), b.Opt)
		}
		return b.Opt, nil
	}
	if slices.Contains(w.env.Opts, DefaultOpt) {
		return DefaultOpt, nil
	}
	// Said whether or not the build is verbose, since it is not the
	// executable the same build with the tinygo command writes.
	fmt.Fprintf(b.Stderr, "nyc: building at -opt=%s, the level the WebAssembly compiler's cache holds, rather than -opt=%s\n", w.env.Opts[0], DefaultOpt)
	return w.env.Opts[0], nil
}

func (w *Wasm) Compile(ctx context.Context, b *Build) error {
	opt, err := w.opt(b)
	if err != nil {
		return err
	}
	if err := w.load(ctx, b); err != nil {
		return err
	}
	pkg, err := filepath.Abs(b.Package)
	if err != nil {
		return err
	}
	w.builds++
	out := fmt.Sprintf("%s/prog%d.elf", guestTmp, w.builds)
	args := []string{"-target=" + Target, "-opt=" + opt, "-o", out}
	if len(b.Tags) > 0 {
		args = append(args, "-tags="+strings.Join(b.Tags, " "))
	}
	args = append(args, b.hotArgs()...)
	args = append(args, filepath.ToSlash(pkg))

	start := time.Now()
	w.out.w = b.Stderr
	err = buildWithReactor(ctx, w.mod, args)
	w.out.w = io.Discard
	var exit *sys.ExitError
	if errors.As(err, &exit) {
		if exit.ExitCode() != 0 {
			return fmt.Errorf("tinygo (wasm) exited with %d", exit.ExitCode())
		}
	} else if err != nil {
		// The module is in no state to build again: the next build loads
		// it afresh.
		w.unload(ctx)
		return fmt.Errorf("tinygo (wasm): %w", err)
	}
	b.logf("wasm: tinygo ran in %s", since(start))

	hostOut := filepath.Join(w.tmp, filepath.Base(out))
	elf, err := os.ReadFile(hostOut)
	if err != nil {
		return err
	}
	os.Remove(hostOut)
	return os.WriteFile(b.Output, elf, 0o666)
}

// load has the module loaded and initialized, if it is not: it stays so
// between builds, keeping what the compiler keeps, the standard library
// parsed and type-checked, so that a process that builds more than once,
// nyc run making patches say, pays for loading once.
func (w *Wasm) load(ctx context.Context, b *Build) error {
	if w.mod != nil {
		return nil
	}
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
	// compiler ten times slower than under wasmtime. Interrupting nyc ends
	// the process, and the module with it.
	r := wazero.NewRuntimeWithConfig(ctx, wazero.NewRuntimeConfig().
		WithCompilationCache(ccache))
	wasi_snapshot_preview1.MustInstantiate(ctx, r)
	b.logf("wasm: read %d MB in %s", len(module)>>20, since(start))

	// The first compilation of the module is most of a minute on one
	// thread, which is wazero's default, and the cache makes it once.
	start = time.Now()
	compiled, err := r.CompileModule(experimental.WithCompilationWorkers(ctx, runtime.GOMAXPROCS(0)), module)
	if err != nil {
		r.Close(ctx)
		return fmt.Errorf("compiling tinygo.wasm: %w", err)
	}
	b.logf("wasm: compiled the module in %s", since(start))
	if compiled.ExportedFunctions()["tinygo_build"] == nil {
		r.Close(ctx)
		return errors.New("tinygo.wasm is not a build of the compiler this nyc runs: it has no tinygo_build")
	}

	wd, err := os.Getwd()
	if err != nil {
		r.Close(ctx)
		return err
	}
	// The module's own directory, for what it writes: cleared for each
	// process, since it is the same one for every build.
	w.tmp = filepath.Join(w.dir, "tmp")
	os.RemoveAll(w.tmp)
	if err := os.MkdirAll(w.tmp, 0o777); err != nil {
		r.Close(ctx)
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
	w.out = &switchWriter{w: io.Discard}
	cfg := wazero.NewModuleConfig().
		WithName("").
		WithArgs("tinygo").
		WithFSConfig(fsc).
		WithStdout(w.out).
		WithStderr(w.out).
		WithRandSource(rand.Reader).
		WithSysWalltime().
		WithSysNanotime().
		WithSysNanosleep().
		WithStartFunctions("_initialize")
	for k, v := range map[string]string{
		"GOROOT":          guestSysroot + "/" + w.env.GOROOT,
		"GOVERSION":       w.env.GOVERSION,
		"GOTOOLTAGS":      w.env.GOTOOLTAGS,
		"TINYGOROOT":      guestSysroot + "/" + w.env.TINYGOROOT,
		"GOMODCACHE":      modCache(),
		"HOME":            guestTmp,
		"TMPDIR":          guestTmp,
		"XDG_CACHE_HOME":  guestCache,
		"PWD":             filepath.ToSlash(wd),
		"TINYGO_BUILD_ID": w.env.BuildID,
	} {
		cfg = cfg.WithEnv(k, v)
	}
	start = time.Now()
	mod, err := r.InstantiateModule(ctx, compiled, cfg)
	if err != nil {
		if mod != nil {
			mod.Close(ctx)
		}
		r.Close(ctx)
		return fmt.Errorf("tinygo (wasm): %w", err)
	}
	b.logf("wasm: loaded the module in %s", since(start))
	w.rt, w.mod = r, mod
	return nil
}

// unload lets the module go.
func (w *Wasm) unload(ctx context.Context) {
	if w.mod != nil {
		w.mod.Close(ctx)
		w.mod = nil
	}
	if w.rt != nil {
		w.rt.Close(ctx)
		w.rt = nil
	}
}

// Close lets the module go and removes what it wrote.
func (w *Wasm) Close(ctx context.Context) {
	w.unload(ctx)
	if w.tmp != "" {
		os.RemoveAll(w.tmp)
	}
}

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

// modCache is the host's module cache, as the go command finds it, without
// asking the go command, which may not be there.
func modCache() string {
	if dir := os.Getenv("GOMODCACHE"); dir != "" {
		return filepath.ToSlash(dir)
	}
	gopath := os.Getenv("GOPATH")
	if gopath == "" {
		home, _ := os.UserHomeDir()
		gopath = filepath.Join(home, "go")
	}
	gopath, _, _ = strings.Cut(gopath, string(os.PathListSeparator))
	return filepath.ToSlash(filepath.Join(gopath, "pkg", "mod"))
}

// unpack writes the compiler's module and its sysroot into dir, by way of a
// directory beside it, so that an unpacking cut short is never used.
func unpack(cache, dir string, module, sysroot func() ([]byte, error)) error {
	tmp, err := os.MkdirTemp(cache, "wasm-tmp-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	data, err := module()
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(tmp, "tinygo.wasm"), data, 0o666); err != nil {
		return err
	}
	data, err = sysroot()
	if err != nil {
		return err
	}
	if err := untar(data, tmp); err != nil {
		return fmt.Errorf("unpacking the sysroot: %w", err)
	}
	if err := os.WriteFile(filepath.Join(tmp, ".complete"), nil, 0o666); err != nil {
		return err
	}
	os.RemoveAll(dir)
	return os.Rename(tmp, dir)
}

// untar extracts a gzipped tar archive into dir, refusing any name that
// would land outside it.
func untar(data []byte, dir string) error {
	zr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return err
	}
	tr := tar.NewReader(zr)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		name := filepath.Clean(filepath.FromSlash(h.Name))
		if !filepath.IsLocal(name) {
			return fmt.Errorf("%q is outside the archive", h.Name)
		}
		at := filepath.Join(dir, name)
		switch h.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(at, 0o777); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(at), 0o777); err != nil {
				return err
			}
			f, err := os.Create(at)
			if err != nil {
				return err
			}
			if _, err := io.Copy(f, tr); err != nil {
				f.Close()
				return err
			}
			if err := f.Close(); err != nil {
				return err
			}
		}
	}
}

func gunzip(data []byte) ([]byte, error) {
	zr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	return io.ReadAll(zr)
}

// Dir is where the compiler is unpacked.
func (w *Wasm) Dir() string { return w.dir }
