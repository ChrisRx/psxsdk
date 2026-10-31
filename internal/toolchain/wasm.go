package toolchain

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// Wasm is TinyGo compiled to WebAssembly, the playground's compiler. It
// reads everything from a sysroot, as it does in the browser: the standard
// library merged with TinyGo's, the target files, and a cache holding the
// target's C and assembly already compiled, since the compiler has no
// clang. The program and the modules it uses come from the host's file
// system.
//
// The compiler and the sysroot are those of the site's play/ directory,
// tinygo.wasm and sysroot.tar.gz: the ones given to Embed, as cmd/yaroze
// does, or else those of YAROZE_WASM when it names such a directory. YAROZE_WASM
// is ignored when there are embedded ones. Either is unpacked once into
// the user's cache directory, keyed by the compiler, and what the runtime
// makes of the module, which takes a while, is kept there too.
//
// The module runs under one of two runtimes. wazero is built in, so it is
// always there: it keeps the module loaded between the builds of a
// process, and a snapshot of the compiler's memory between processes (see
// wasm_snapshot.go). The wasmtime command, when there is one, runs the
// module as a command for each build, and quicker, since its code is
// better and its compiled module loads in no time.
type Wasm struct {
	dir     string // the unpacked compiler: tinygo.wasm and sysroot
	sysroot string
	env     sysrootEnv
	rt      wasmRuntime

	// The directory the module writes to, this process's own, and how
	// many builds it has made, which name their outputs.
	tmp    string
	builds int
}

// A wasmRuntime runs the compiler's module.
type wasmRuntime interface {
	// name is the runtime's, for messages: "wazero" or "wasmtime".
	name() string
	// describe says which runtime, and where it is, for yaroze env.
	describe() string
	// build runs one build with the compiler arguments given, writing the
	// compiler's output to b.Stderr, and returns an error for a build that
	// failed.
	build(ctx context.Context, b *Build, args []string) error
	// close lets go of whatever the runtime holds.
	close(ctx context.Context)
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
	guestTmp     = "/.yaroze/tmp"
	guestCache   = "/.yaroze/cache"
)

// The compiler and sysroot given to Embed.
var embeddedWasm, embeddedSysroot []byte

// Embed builds in the WebAssembly compiler, tinygo.wasm gzipped, and its
// sysroot.tar.gz, for New to use in place of any YAROZE_WASM.
func Embed(compiler, sysroot []byte) {
	embeddedWasm, embeddedSysroot = compiler, sysroot
}

// New finds the compiler, built in or from YAROZE_WASM, unpacks it if it has
// not been, and has it run under the runtime named: wazero, wasmtime, or
// with auto or nothing wasmtime when there is a wasmtime command and
// wazero otherwise.
func New(runtime string) (*Wasm, error) {
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
	} else if dir := os.Getenv("YAROZE_WASM"); dir != "" {
		wasmPath := filepath.Join(dir, "tinygo.wasm")
		sysPath := filepath.Join(dir, "sysroot.tar.gz")
		h := sha256.New()
		for _, p := range []string{wasmPath, sysPath} {
			info, err := os.Stat(p)
			if err != nil {
				return nil, fmt.Errorf("YAROZE_WASM: %w", err)
			}
			abs, _ := filepath.Abs(p)
			fmt.Fprintf(h, "%s %d %d\n", abs, info.Size(), info.ModTime().UnixNano())
		}
		key = hex.EncodeToString(h.Sum(nil))[:16]
		module = func() ([]byte, error) { return os.ReadFile(wasmPath) }
		sysroot = func() ([]byte, error) { return os.ReadFile(sysPath) }
	} else {
		return nil, errors.New("this yaroze has no compiler built in; set YAROZE_WASM to a directory holding tinygo.wasm and sysroot.tar.gz, as the site's play/ does")
	}

	w := &Wasm{dir: filepath.Join(cache, "wasm-"+key)}
	w.sysroot = filepath.Join(w.dir, "sysroot")
	if !complete(w.dir) {
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
		return nil, fmt.Errorf("the compiler's sysroot is for %s, not %s", w.env.Target, Target)
	}
	markUsed(w.dir)
	trimCache(cache, w.dir, time.Now())

	// The module's own directory, for what it writes: this process's,
	// since another may be building at the same time, removed by Close,
	// and those of processes that never got there swept out after a day.
	if entries, err := os.ReadDir(w.dir); err == nil {
		for _, e := range entries {
			if strings.HasPrefix(e.Name(), "tmp-") {
				if fi, err := e.Info(); err == nil && time.Since(fi.ModTime()) > 24*time.Hour {
					os.RemoveAll(filepath.Join(w.dir, e.Name()))
				}
			}
		}
	}
	w.tmp, err = os.MkdirTemp(w.dir, "tmp-")
	if err != nil {
		return nil, err
	}

	switch runtime {
	case "wazero":
		w.rt = newWazero(w)
	case "wasmtime":
		w.rt, err = newWasmtime(w)
	case "", "auto":
		if w.rt, err = newWasmtime(w); err != nil {
			w.rt = newWazero(w)
		}
		err = nil
	default:
		err = fmt.Errorf("runtime %q is not wazero, wasmtime or auto", runtime)
	}
	if err != nil {
		w.Close(context.Background())
		return nil, err
	}
	return w, nil
}

// Runtime says which runtime the compiler runs under, and where it is.
func (w *Wasm) Runtime() string { return w.rt.describe() }

// Dir is where the compiler is unpacked.
func (w *Wasm) Dir() string { return w.dir }

// opt returns the optimization level to build b at: the one it names,
// which the cache has to hold, or else DefaultOpt, or the first level the
// cache holds if it does not hold that. The playground's sysroot holds only
// z, the level the playground builds at, and task yaroze adds 2 to its own.
func (w *Wasm) opt(b *Build) (string, error) {
	if len(w.env.Opts) == 0 {
		if b.Opt == "" {
			return DefaultOpt, nil
		}
		return b.Opt, nil
	}
	if b.Opt != "" {
		if !slices.Contains(w.env.Opts, b.Opt) {
			return "", fmt.Errorf("the compiler's cache holds the target's C and assembly compiled only at -opt=%s, so it cannot build at -opt=%s", strings.Join(w.env.Opts, " and -opt="), b.Opt)
		}
		return b.Opt, nil
	}
	if slices.Contains(w.env.Opts, DefaultOpt) {
		return DefaultOpt, nil
	}
	// Said whether or not the build is verbose, since it is not the
	// executable the same build with the tinygo command writes.
	fmt.Fprintf(b.Stderr, "yaroze: building at -opt=%s, the level the compiler's cache holds, rather than -opt=%s\n", w.env.Opts[0], DefaultOpt)
	return w.env.Opts[0], nil
}

// Compile builds b, writing the ELF to b.Output.
func (w *Wasm) Compile(ctx context.Context, b *Build) error {
	opt, err := w.opt(b)
	if err != nil {
		return err
	}
	pkg, err := filepath.Abs(b.Package)
	if err != nil {
		return err
	}
	// The compiler is told the working directory, where the main module's
	// go.mod is looked for, rather than left to its own: a module restored
	// from a snapshot has another process's.
	wd, err := os.Getwd()
	if err != nil {
		return err
	}
	w.builds++
	out := fmt.Sprintf("%s/prog%d.elf", guestTmp, w.builds)
	target := Target
	if b.Target != "" {
		abs, err := filepath.Abs(b.Target)
		if err != nil {
			return err
		}
		target = filepath.ToSlash(abs)
	}
	args := []string{"-target=" + target, "-opt=" + opt, "-C", filepath.ToSlash(wd), "-o", out}
	if len(b.Tags) > 0 {
		args = append(args, "-tags="+strings.Join(b.Tags, " "))
	}
	args = append(args, b.hotArgs()...)
	args = append(args, filepath.ToSlash(pkg))

	start := time.Now()
	if err := w.rt.build(ctx, b, args); err != nil {
		return err
	}
	b.logf("wasm: tinygo ran under %s in %s", w.rt.name(), since(start))

	hostOut := filepath.Join(w.tmp, filepath.Base(out))
	elf, err := os.ReadFile(hostOut)
	if err != nil {
		return err
	}
	os.Remove(hostOut)
	return os.WriteFile(b.Output, elf, 0o666)
}

// environ is the compiler's environment: where its directories are, and
// the working directory.
func (w *Wasm) environ(wd string) map[string]string {
	return map[string]string{
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
	}
}

// Close lets the runtime go and removes what the module wrote.
func (w *Wasm) Close(ctx context.Context) {
	if w.rt != nil {
		w.rt.close(ctx)
	}
	if w.tmp != "" {
		os.RemoveAll(w.tmp)
	}
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
//
// Several processes may unpack the same compiler at once, as the examples'
// discs are built, and the first to finish is the one the rest run from:
// a dir that is complete is never removed or replaced, since a process may
// be running from it, and a copy made too late is thrown away.
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
	if err := os.WriteFile(filepath.Join(tmp, completeMark), nil, 0o666); err != nil {
		return err
	}
	for range 2 {
		if complete(dir) {
			return nil
		}
		err := os.Rename(tmp, dir)
		if err == nil || complete(dir) {
			return nil
		}
		// What is there is not a compiler: an unpacking cut short before
		// the mark was written, from a version that wrote it last. Clear
		// it and try once more.
		os.RemoveAll(dir)
	}
	return fmt.Errorf("could not put the compiler in %s", dir)
}

// completeMark is the file an unpacked compiler's directory ends with,
// which says the whole of it is there.
const completeMark = ".complete"

// complete reports whether dir holds a whole compiler.
func complete(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, completeMark))
	return err == nil
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
