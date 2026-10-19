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
// tinygo.wasm and sysroot.tar.gz, from NYC_WASM when it names such a
// directory, or else built into nyc with the nyc_embed build tag (task
// nyc). Either is unpacked once into the user's cache directory, and
// wazero's compilation of the module, which takes a while, is kept there
// too.
type Wasm struct {
	dir     string // the unpacked compiler: tinygo.wasm and sysroot
	sysroot string
	env     sysrootEnv
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

// NewWasm finds the WebAssembly compiler, from NYC_WASM or built in, and
// unpacks it if it has not been.
func NewWasm() (*Wasm, error) {
	cache, err := cacheDir()
	if err != nil {
		return nil, err
	}
	var key string
	var module func() ([]byte, error)
	var sysroot func() ([]byte, error)
	if dir := os.Getenv("NYC_WASM"); dir != "" {
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
	} else if len(embeddedWasm) > 0 {
		h := sha256.New()
		h.Write(embeddedWasm)
		h.Write(embeddedSysroot)
		key = hex.EncodeToString(h.Sum(nil))[:16]
		module = func() ([]byte, error) { return gunzip(embeddedWasm) }
		sysroot = func() ([]byte, error) { return embeddedSysroot, nil }
	} else {
		return nil, errors.New("this nyc has no WebAssembly compiler built in (task nyc builds one that does); set NYC_WASM to a directory holding tinygo.wasm and sysroot.tar.gz, as the site's play/ does")
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
	start := time.Now()
	module, err := os.ReadFile(filepath.Join(w.dir, "tinygo.wasm"))
	if err != nil {
		return err
	}
	ccache, err := wazero.NewCompilationCacheWithDir(filepath.Join(w.dir, "wazero"))
	if err != nil {
		return err
	}
	defer ccache.Close(ctx)
	// No WithCloseOnContextDone: to stop a module when the context ends,
	// wazero calls out to Go at the head of every loop, which made the
	// compiler ten times slower than under wasmtime. Interrupting nyc ends
	// the process, and the module with it.
	r := wazero.NewRuntimeWithConfig(ctx, wazero.NewRuntimeConfig().
		WithCompilationCache(ccache))
	defer r.Close(ctx)
	wasi_snapshot_preview1.MustInstantiate(ctx, r)
	b.logf("wasm: read %d MB in %s", len(module)>>20, since(start))

	// The first compilation of the module is most of a minute on one
	// thread, which is wazero's default, and the cache makes it once.
	start = time.Now()
	compiled, err := r.CompileModule(experimental.WithCompilationWorkers(ctx, runtime.GOMAXPROCS(0)), module)
	if err != nil {
		return fmt.Errorf("compiling tinygo.wasm: %w", err)
	}
	b.logf("wasm: compiled the module in %s", since(start))

	pkg, err := filepath.Abs(b.Package)
	if err != nil {
		return err
	}
	wd, err := os.Getwd()
	if err != nil {
		return err
	}
	tmp, err := os.MkdirTemp("", "nyc-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	cache := filepath.Join(w.sysroot, "cache")

	// The host's file system is the compiler's, read only, so that the
	// program, a replace directive's directory and the module cache are
	// where the host has them. Mounts on longer paths take precedence.
	fsc := wazero.NewFSConfig().
		WithReadOnlyDirMount("/", "/").
		WithReadOnlyDirMount(w.sysroot, guestSysroot).
		WithDirMount(tmp, guestTmp).
		WithDirMount(cache, guestCache+"/tinygo")
	args := []string{"tinygo", "-target=" + Target, "-opt=" + opt, "-o", guestTmp + "/prog.elf"}
	if len(b.Tags) > 0 {
		args = append(args, "-tags="+strings.Join(b.Tags, " "))
	}
	args = append(args, filepath.ToSlash(pkg))
	cfg := wazero.NewModuleConfig().
		WithName("").
		WithArgs(args...).
		WithFSConfig(fsc).
		WithStdout(b.Stderr).
		WithStderr(b.Stderr).
		WithRandSource(rand.Reader).
		WithSysWalltime().
		WithSysNanotime().
		WithSysNanosleep()
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
	if mod != nil {
		mod.Close(ctx)
	}
	var exit *sys.ExitError
	if errors.As(err, &exit) {
		if exit.ExitCode() != 0 {
			return fmt.Errorf("tinygo (wasm) exited with %d", exit.ExitCode())
		}
	} else if err != nil {
		return fmt.Errorf("tinygo (wasm): %w", err)
	}
	b.logf("wasm: tinygo ran in %s", since(start))

	elf, err := os.ReadFile(filepath.Join(tmp, "prog.elf"))
	if err != nil {
		return err
	}
	return os.WriteFile(b.Output, elf, 0o666)
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
