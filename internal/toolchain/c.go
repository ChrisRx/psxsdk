package toolchain

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ChrisRx/wazero"
	"github.com/ChrisRx/wazero/api"
	"github.com/ChrisRx/wazero/imports/wasi_snapshot_preview1"
	"github.com/ChrisRx/wazero/sys"
)

// cFiles are the headers a C program compiles against, written for psxsdk
// from the published Library Reference, and rt.c.
//
//go:embed c
var cFiles embed.FS

// The compiler that ships has no clang, so a C program is built by the full
// compiler, tinygo-full.wasm, which is too large to embed for what most
// programs never need. It is downloaded the first time, from where the file
// embedded with EmbedFull says, and kept in the cache directory like any
// other compiler. YAROZE_CC names one on disk instead, for working on it.
type fullManifest struct {
	URL    string `json:"url"`
	SHA256 string `json:"sha256"`
	Size   int64  `json:"size"`
}

var embeddedFull []byte

// EmbedFull builds in the manifest of the full compiler to download, the
// JSON web/compiler/release.sh writes.
func EmbedFull(manifest []byte) { embeddedFull = manifest }

// HasHeader reports whether the compiler's own include directory has the
// header name.
func HasHeader(name string) bool {
	fi, err := fs.Stat(cFiles, "c/include/"+name)
	return err == nil && !fi.IsDir()
}

// A CBuild is one C program to compile and link.
type CBuild struct {
	Sources []string // the C and assembly files
	Defines []string // without the -D
	Addr    uint32   // where it is linked
	Opt     string   // the optimization level, or empty for DefaultOpt
	Output  string   // the ELF to write

	Stderr io.Writer
	Log    func(format string, args ...any)
}

func (b *CBuild) logf(format string, args ...any) {
	if b.Log != nil {
		b.Log(format, args...)
	}
}

// The target's files a C program links, in order: the crt0 first, so that
// _start is at the program's address.
var cTargetFiles = []string{
	"playstation-yaroze.S",
	"playstation-yaroze-symusr.S",
	"playstation-yaroze-libps.S",
	"playstation-yaroze-gte.S",
	"r3000-builtins.c",
}

// Where the compiler sees the directory of C files, include and rt.
const guestC = "/.yaroze/c"

// cFlags are clang's flags for the target. What follows -w is what lets
// C written for a GCC of 1996 mean what it meant: gnu89, with GCC's common
// symbols, the helpers it called by name left as calls, and none of the
// optimizations on undefined behavior it did not make, since this code casts
// packet memory between types, counts on overflow wrapping and reads what it
// never set: given an uninitialized pointer, clang deletes the rest of the
// function and what follows its call, and the program runs on into whatever
// comes next in memory.
func cFlags(opt string) []string {
	return []string{
		"--target=mipsel-unknown-none-elf", "-march=mips1", "-mfloat-abi=soft", "-mno-abicalls", "-fno-pic", "-G0",
		"-mno-check-zero-division", "-O" + opt, "-ffunction-sections", "-fdata-sections",
		"-w", "-std=gnu89", "-fcommon", "-ffreestanding", "-fno-builtin", "-nostdinc",
		"-fno-strict-aliasing", "-fwrapv", "-fno-delete-null-pointer-checks", "-ftrivial-auto-var-init=zero",
		"-Wno-return-mismatch", "-Wno-int-conversion", "-Wno-incompatible-pointer-types",
		"-Wno-implicit-function-declaration", "-Wno-implicit-int",
	}
}

// CompileC builds b, writing the ELF to b.Output.
func (w *Wasm) CompileC(ctx context.Context, b *CBuild) error {
	opt := b.Opt
	if opt == "" {
		opt = DefaultOpt
	}
	tool, err := w.fullTool(ctx, b)
	if err != nil {
		return err
	}
	defer tool.close(ctx)
	stderr := &lineFilter{w: b.Stderr, drop: "warning: MIPS-I support is experimental"}
	defer stderr.Flush()

	cdir := cDir(w)
	if err := unpackC(cdir); err != nil {
		return err
	}
	rt, err := w.cRuntime(ctx, tool, stderr, cdir, opt)
	if err != nil {
		return err
	}

	start := time.Now()
	flags := append(cFlags(opt), "-I"+guestC+"/include")
	for _, d := range b.Defines {
		flags = append(flags, "-D"+d)
	}
	var objects []string
	for i, src := range b.Sources {
		obj := fmt.Sprintf("%s/c%d-%d.o", guestTmp, w.builds, i)
		w.builds++
		args := append([]string{"clang"}, flags...)
		if strings.EqualFold(filepath.Ext(src), ".s") {
			args = append(args, "-x", "assembler-with-cpp")
		} else {
			args = append(args, "-x", "c")
		}
		args = append(args, "-c", filepath.ToSlash(src), "-o", obj)
		if err := tool.run(ctx, stderr, args); err != nil {
			return fmt.Errorf("compiling %s: %w", filepath.Base(src), err)
		}
		objects = append(objects, obj)
	}
	b.logf("c: compiled %d file(s) in %s", len(objects), since(start))

	start = time.Now()
	script, err := w.cLinkScript()
	if err != nil {
		return err
	}
	out := fmt.Sprintf("%s/c%d.elf", guestTmp, w.builds)
	w.builds++
	// --defsym has to come before the script to be what the script's
	// DEFINED(__load) sees: lld applies them in order.
	args := []string{"ld.lld", "-m", "elf32ltsmip", fmt.Sprintf("--defsym=__load=0x%x", b.Addr), "-T", script,
		"--gc-sections", "--no-warn-mismatch", "-e", "_start", "-u", "main", "-o", out}
	args = append(args, rt...)
	args = append(args, objects...)
	if err := tool.run(ctx, stderr, args); err != nil {
		return fmt.Errorf("linking: %w", err)
	}
	b.logf("c: linked in %s", since(start))

	hostOut := filepath.Join(w.tmp, filepath.Base(out))
	elf, err := os.ReadFile(hostOut)
	if err != nil {
		return err
	}
	return os.WriteFile(b.Output, elf, 0o666)
}

// cDir is where the C files are unpacked, and what is compiled from them
// kept: beside the compiler, and named for the files, since a new yaroze may
// bring new headers or flags to the same compiler.
func cDir(w *Wasm) string {
	h := sha256.New()
	fmt.Fprintln(h, cFlags(""))
	fs.WalkDir(cFiles, "c", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := cFiles.ReadFile(path)
		fmt.Fprintf(h, "%s %d\n", path, len(data))
		h.Write(data)
		return err
	})
	return filepath.Join(w.dir, "c-"+hex.EncodeToString(h.Sum(nil))[:16])
}

// cLinkScript writes the target's link script with every variable word
// aligned, and returns its path as the compiler sees it. GCC aligned an
// array of bytes to a word, and programs count on that: a PACKET area is
// bytes in C and the library stores words into it. Clang has no option to do
// the same, but -fdata-sections gives each variable a section of its own,
// which SUBALIGN sets the alignment of.
func (w *Wasm) cLinkScript() (string, error) {
	data, err := os.ReadFile(filepath.Join(w.sysroot, w.env.TINYGOROOT, "targets", "playstation-yaroze.ld"))
	if err != nil {
		return "", err
	}
	script := string(data)
	for _, section := range []string{".rodata", ".data", ".bss"} {
		from := "  " + section + " : ALIGN(4)\n"
		if !strings.Contains(script, from) {
			return "", fmt.Errorf("the target's link script has no %q to align", strings.TrimSpace(from))
		}
		script = strings.Replace(script, from, "  "+section+" : ALIGN(4) SUBALIGN(4)\n", 1)
	}
	if err := os.WriteFile(filepath.Join(w.tmp, "c.ld"), []byte(script), 0o666); err != nil {
		return "", err
	}
	return guestTmp + "/c.ld", nil
}

// unpackC writes the embedded C files into dir, unless they are there.
func unpackC(dir string) error {
	if complete(dir) {
		return nil
	}
	err := fs.WalkDir(cFiles, "c", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, err := filepath.Rel("c", path)
		if err != nil {
			return err
		}
		data, err := cFiles.ReadFile(path)
		if err != nil {
			return err
		}
		at := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(at), 0o777); err != nil {
			return err
		}
		return os.WriteFile(at, data, 0o666)
	})
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, completeMark), nil, 0o666)
}

// cRuntime compiles the target's files and rt.c at opt, once, and returns
// the objects as the compiler sees them. They are compiled to this
// process's directory and moved into place together, so that one process
// never links another's half-written objects.
func (w *Wasm) cRuntime(ctx context.Context, tool cTool, stderr io.Writer, cdir, opt string) ([]string, error) {
	name := "rt-" + opt
	dir := filepath.Join(cdir, name)
	var objects []string
	sources := append([]string{}, cTargetFiles...)
	sources = append(sources, "rt.c")
	for _, s := range sources {
		objects = append(objects, guestC+"/"+name+"/"+s+".o")
	}
	if complete(dir) {
		return objects, nil
	}
	tmp, err := os.MkdirTemp(w.tmp, "rt-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmp)
	flags := append(cFlags(opt), "-I"+guestC+"/include")
	guest := guestTmp + "/" + filepath.Base(tmp)
	for _, s := range sources {
		src := guestSysroot + "/" + w.env.TINYGOROOT + "/targets/" + s
		if s == "rt.c" {
			src = guestC + "/rt.c"
		}
		args := append([]string{"clang"}, flags...)
		switch filepath.Ext(s) {
		case ".S":
			args = append(args, "-x", "assembler-with-cpp")
		default:
			args = append(args, "-x", "c")
		}
		args = append(args, "-c", src, "-o", guest+"/"+s+".o")
		if err := tool.run(ctx, stderr, args); err != nil {
			return nil, fmt.Errorf("compiling %s: %w", s, err)
		}
	}
	if err := os.WriteFile(filepath.Join(tmp, completeMark), nil, 0o666); err != nil {
		return nil, err
	}
	if err := os.Rename(tmp, dir); err != nil && !complete(dir) {
		return nil, err
	}
	return objects, nil
}

// A cTool runs the full compiler's tools.
type cTool interface {
	// run runs the tool args[0] with the rest as its arguments.
	run(ctx context.Context, stderr io.Writer, args []string) error
	close(ctx context.Context)
}

// fullTool finds the full compiler, downloading it if it has to, and has
// it run under the runtime the shipped compiler runs under.
func (w *Wasm) fullTool(ctx context.Context, b *CBuild) (cTool, error) {
	module, err := fullModule(ctx, b)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(cDir(w), 0o777); err != nil {
		return nil, err
	}
	if r, ok := w.rt.(*wasmtimeRuntime); ok {
		cwasm := strings.TrimSuffix(module, ".wasm") + "-wasmtime-" + r.version + ".cwasm"
		if err := r.precompile(b.Stderr, b.logf, module, cwasm); err != nil {
			return nil, err
		}
		return &wasmtimeTool{w: w, r: r, cwasm: cwasm}, nil
	}
	return newWazeroTool(ctx, w, module, b.logf)
}

// fullModule returns the path of the full compiler's module.
func fullModule(ctx context.Context, b *CBuild) (string, error) {
	if path := os.Getenv("YAROZE_CC"); path != "" {
		return filepath.Abs(path)
	}
	var m fullManifest
	if len(embeddedFull) > 0 {
		if err := json.Unmarshal(embeddedFull, &m); err != nil {
			return "", fmt.Errorf("the full compiler's manifest: %w", err)
		}
	}
	if m.URL == "" {
		return "", errors.New("building C needs the full compiler, and this yaroze does not say where to get it; set YAROZE_CC to a tinygo-full.wasm")
	}
	cache, err := cacheDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(cache, "wasm-full-"+m.SHA256[:16])
	module := filepath.Join(dir, "tinygo-full.wasm")
	if complete(dir) {
		markUsed(dir)
		return module, nil
	}
	fmt.Fprintf(b.Stderr, "yaroze: building C needs the full compiler; downloading %d MB, once\n", m.Size>>20)
	start := time.Now()
	data, err := download(ctx, m)
	if err != nil {
		return "", err
	}
	b.logf("c: downloaded the full compiler in %s", since(start))
	zr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("the full compiler: %w", err)
	}
	tmp, err := os.MkdirTemp(cache, "wasm-tmp-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmp)
	f, err := os.Create(filepath.Join(tmp, "tinygo-full.wasm"))
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(f, zr); err != nil {
		f.Close()
		return "", fmt.Errorf("the full compiler: %w", err)
	}
	if err := f.Close(); err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(tmp, completeMark), nil, 0o666); err != nil {
		return "", err
	}
	// As with unpack: the first of several processes to finish wins.
	if err := os.Rename(tmp, dir); err != nil && !complete(dir) {
		return "", err
	}
	markUsed(dir)
	return module, nil
}

// download fetches the gzipped module m names and checks it against m's
// hash, since it is about to be run.
func download(ctx context.Context, m fullManifest) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, m.URL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("downloading the full compiler: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("downloading the full compiler from %s: %s", m.URL, resp.Status)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("downloading the full compiler: %w", err)
	}
	sum := sha256.Sum256(data)
	if got := hex.EncodeToString(sum[:]); got != m.SHA256 {
		return nil, fmt.Errorf("the full compiler from %s has hash %s, not %s", m.URL, got, m.SHA256)
	}
	return data, nil
}

// cMounts are the directories the full compiler sees, by guest path: the
// host's file system for the sources, and on longer paths the sysroot, for
// the target's files, the C files and the process's own directory.
func (w *Wasm) cMounts() map[string]string {
	return map[string]string{
		guestSysroot: w.sysroot,
		guestTmp:     w.tmp,
		guestC:       cDir(w),
	}
}

// wasmtimeTool runs the full compiler under the wasmtime command, once per
// tool.
type wasmtimeTool struct {
	w     *Wasm
	r     *wasmtimeRuntime
	cwasm string
}

func (t *wasmtimeTool) run(ctx context.Context, stderr io.Writer, args []string) error {
	wd, err := os.Getwd()
	if err != nil {
		return err
	}
	cmdArgs := []string{"run", "--allow-precompiled", "--dir", "/::/"}
	for guest, host := range t.w.cMounts() {
		cmdArgs = append(cmdArgs, "--dir", host+"::"+guest)
	}
	for k, v := range t.w.environ(wd) {
		cmdArgs = append(cmdArgs, "--env", k+"="+v)
	}
	cmdArgs = append(cmdArgs, "--", t.cwasm)
	cmdArgs = append(cmdArgs, args...)
	return runCommand(ctx, stderr, t.r.path, cmdArgs)
}

func (t *wasmtimeTool) close(ctx context.Context) {}

// wazeroTool runs the full compiler under wazero, a new instance of the
// module for each tool, since it is a command that exits.
type wazeroTool struct {
	w        *Wasm
	rt       wazero.Runtime
	compiled wazero.CompiledModule
}

func newWazeroTool(ctx context.Context, w *Wasm, module string, logf func(string, ...any)) (*wazeroTool, error) {
	start := time.Now()
	data, err := os.ReadFile(module)
	if err != nil {
		return nil, err
	}
	ccache, err := wazero.NewCompilationCacheWithDir(filepath.Join(filepath.Dir(module), "wazero"))
	if err != nil {
		return nil, err
	}
	rt := wazero.NewRuntimeWithConfig(ctx, wazero.NewRuntimeConfig().
		WithCompilationCache(ccache).
		WithSkipValidationOnCacheHit(true))
	wasi_snapshot_preview1.MustInstantiate(ctx, rt)
	compiled, err := rt.CompileModule(compileWorkers(ctx), data)
	if err != nil {
		rt.Close(ctx)
		return nil, fmt.Errorf("compiling the full compiler: %w", err)
	}
	logf("c: compiled the full compiler in %s", since(start))
	return &wazeroTool{w: w, rt: rt, compiled: compiled}, nil
}

func (t *wazeroTool) run(ctx context.Context, stderr io.Writer, args []string) error {
	wd, err := os.Getwd()
	if err != nil {
		return err
	}
	fsc := wazero.NewFSConfig().WithReadOnlyDirMount("/", "/")
	for guest, host := range t.w.cMounts() {
		fsc = fsc.WithDirMount(host, guest)
	}
	cfg := wazero.NewModuleConfig().
		WithName("").
		WithArgs(append([]string{"tinygo"}, args...)...).
		WithFSConfig(fsc).
		WithStdout(stderr).
		WithStderr(stderr).
		WithSysWalltime().
		WithSysNanotime().
		WithSysNanosleep()
	for k, v := range t.w.environ(wd) {
		cfg = cfg.WithEnv(k, v)
	}
	var mod api.Module
	mod, err = t.rt.InstantiateModule(ctx, t.compiled, cfg)
	if mod != nil {
		mod.Close(ctx)
	}
	var exit *sys.ExitError
	if errors.As(err, &exit) {
		if exit.ExitCode() == 0 {
			return nil
		}
		return fmt.Errorf("exited with %d", exit.ExitCode())
	}
	return err
}

func (t *wazeroTool) close(ctx context.Context) { t.rt.Close(ctx) }

// A lineFilter passes on what is written to it, less the lines equal to
// drop.
type lineFilter struct {
	w    io.Writer
	drop string
	buf  bytes.Buffer
}

func (l *lineFilter) Write(p []byte) (int, error) {
	l.buf.Write(p)
	for {
		i := bytes.IndexByte(l.buf.Bytes(), '\n')
		if i < 0 {
			return len(p), nil
		}
		line := l.buf.Next(i + 1)
		if strings.TrimSpace(string(line)) != l.drop {
			if _, err := l.w.Write(line); err != nil {
				return len(p), err
			}
		}
	}
}

// Flush passes on an unfinished last line.
func (l *lineFilter) Flush() {
	if l.buf.Len() > 0 {
		l.w.Write(l.buf.Bytes())
		l.buf.Reset()
	}
}
