package tests

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"go.chrisrx.dev/x/assert"
)

// tortureHarness reports how a test ended over the TTY. The test's own main,
// abort and exit are renamed to reach it, because abort and exit in the resident
// library do not say which way a program went.
const tortureHarness = `
extern int printf(), t_main(), VSync();
static void hang(void) { for (;;) VSync(0); }
void t_abort(void) { printf("TORTURE abort\n"); hang(); }
void t_exit(int c) { printf("TORTURE exit %d\n", c); hang(); }
int main(void) { t_main(); printf("TORTURE exit 0\n"); hang(); return 0; }
`

// tortureFailure is how a test is expected not to pass, and why. It is listed
// by the test's name, or by the name and level, as in conversion/-O0, if it
// fails only there.
type tortureFailure struct{ outcome, why string }

// tortureExpected is what does not pass under GCC 2.95.3. None of it is a fault in how the compiler is built.
var tortureExpected = map[string]tortureFailure{
	"conversion":     {"no result", "the resident __floatsidf never returns for INT_MIN, and an unsigned of 0x80000000 is converted through it"},
	"strct-stdarg-1": {"abort", "va_arg in GCC's va-mips.h misreads a struct whose size is not a multiple of 4, on little-endian MIPS"},
	"920501-8":       {"abort", "the resident sprintf has no %f"},
	"930513-1":       {"abort", "the resident sprintf has no %f"},
	"memcpy-bi":      {"abort", "the resident memcmp compares a byte when its length is 0"},
	"921215-1":       {"link", "needs _flush_cache for a nested function's trampoline, which the SDK does not have"},
	"931002-1":       {"link", "needs _flush_cache for a nested function's trampoline, which the SDK does not have"},
	"nestfunc-1":     {"link", "needs _flush_cache for a nested function's trampoline, which the SDK does not have"},
	"980707-1":       {"link", "needs _ctype_, which the SDK does not have"},
	"980709-1":       {"compile", "needs math.h, which the SDK does not have"},
	"990413-2":       {"compile", "x86 assembly, for x86 targets only"},
	"gofast":         {"compile", "tests the GOFAST library, which this toolchain does not use because the resident library has the standard names"},
}

// tortureYarozeExpected is the same for yaroze. Most of it is what clang does
// not have or does not accept.
var tortureYarozeExpected = map[string]tortureFailure{
	"920415-1":       {"build", "a nested function, which clang does not have"},
	"920428-2":       {"build", "a nested function, which clang does not have"},
	"920501-7":       {"build", "a nested function, which clang does not have"},
	"920612-2":       {"build", "a nested function, which clang does not have"},
	"920721-4":       {"build", "a nested function, which clang does not have"},
	"921017-1":       {"build", "a nested function, which clang does not have"},
	"921215-1":       {"build", "a nested function, which clang does not have"},
	"931002-1":       {"build", "a nested function, which clang does not have"},
	"nestfunc-1":     {"build", "a nested function, which clang does not have"},
	"nest-stdar-1":   {"build", "a nested function, which clang does not have"},
	"strct-varg-1":   {"build", "needs varargs.h, the stdarg.h from before ANSI C, which clang does not have"},
	"va-arg-3":       {"build", "needs varargs.h, the stdarg.h from before ANSI C, which clang does not have"},
	"931018-1":       {"build", "declares an extern variable and then defines it const, which clang rejects and GCC accepted"},
	"990413-2":       {"build", "x86 assembly, for x86 targets only"},
	"gofast":         {"build", "tests the GOFAST library, and uses stderr, which the SDK does not have"},
	"920501-8":       {"abort", "the resident sprintf has no %f"},
	"930513-1":       {"abort", "the resident sprintf has no %f"},
	"memcpy-bi":      {"abort", "the resident memcmp compares a byte when its length is 0"},
	"conversion/-O0": {"no result", "libps.a's __floatsisf never returns for INT_MIN, as the resident __floatsidf does not; optimized, the conversions are of constants and folded"},
	"990208-1/-O1":   {"abort", "expects a function whose label's address is taken to be inlined into two others, which is how well GCC optimizes and not whether the code is right"},
	"990208-1/-O2":   {"abort", "expects a function whose label's address is taken to be inlined into two others, which is how well GCC optimizes and not whether the code is right"},
}

// GCC's own c-torture execute tests, compiled by the GCC 2.95.3 from the
// mipsel-ecoff-toolchain repository and run in the emulator. Each program checks
// its own results and aborts on a wrong one, so unlike comparing compilers this
// holds the compiler to the right answer, and it can find what a whole family of
// builds gets wrong: a misaligned _end and a broken conversion in libgcc were
// both found this way.
//
// PSX_GCC295 is that repository's nix build .#legacy-sdk. PSX_TORTURE is the
// suite's execute directory, which no release tarball has, but the tag does:
//
//	git clone --depth 1 -b releases/gcc-2.95.0 https://github.com/gcc-mirror/gcc
//	PSX_TORTURE=gcc/gcc/testsuite/gcc.c-torture/execute
func TestCTorture(t *testing.T) {
	root := tortureRoot(t)
	prefix := os.Getenv("PSX_GCC295")
	if prefix == "" {
		t.Skip("PSX_GCC295 is not set to an SDK for GCC 2.95.3")
	}
	gcc := filepath.Join(prefix, "bin", "mipsel-ecoff-gcc")
	bin := t.TempDir()
	psxemu := goBuild(t, bin, "psxemu")
	eco2exe := goBuild(t, bin, "eco2exe")
	harness := filepath.Join(bin, "harness.c")
	assert.NoError(t, os.WriteFile(harness, []byte(tortureHarness), 0o644))
	object := filepath.Join(bin, "harness.o")
	out, err := exec.Command(gcc, "-O1", "-msoft-float", "-w", "-c", harness, "-o", object).CombinedOutput()
	assert.NoError(t, err, string(out))

	tortureSuite(t, root, psxemu, tortureExpected, func(src, level, dir string) (string, string) {
		obj := filepath.Join(dir, "test.o")
		program := filepath.Join(dir, "test")
		exe := filepath.Join(dir, "test.exe")
		if out, err := exec.Command(gcc, "-O"+level, "-msoft-float", "-w", "-Dmain=t_main", "-Dabort=t_abort", "-Dexit=t_exit", "-c", src, "-o", obj).CombinedOutput(); err != nil {
			return "compile", string(out)
		}
		if out, err := exec.Command(gcc, "-Wl,-Ttext,0x80140000,-e,_start", "-msoft-float", "-o", program, object, obj).CombinedOutput(); err != nil {
			return "link", string(out)
		}
		if out, err := exec.Command(eco2exe, program, exe).CombinedOutput(); err != nil {
			return "eco2exe", string(out)
		}
		return "", exe
	})
}

// The same suite, built by yaroze as a C project of the test and the harness,
// so that its compiler is held to the right answers too. It needs YAROZE_CC,
// as the other C tests do, and PSX_TORTURE.
func TestCTortureYaroze(t *testing.T) {
	root := tortureRoot(t)
	yaroze := needCC(t)
	psxemu := goBuild(t, t.TempDir(), "psxemu")

	// The renames come first in the test's own file, since in a project the
	// defines would reach the harness too.
	const rename = "#define main t_main\n#define abort t_abort\n#define exit t_exit\n#line 1 \"%s\"\n"
	tortureSuite(t, root, psxemu, tortureYarozeExpected, func(src, level, dir string) (string, string) {
		data, err := os.ReadFile(src)
		if err != nil {
			return "setup", err.Error()
		}
		project := filepath.Join(dir, "project")
		exe := filepath.Join(dir, "test.exe")
		if err := errors.Join(
			os.Mkdir(project, 0o755),
			os.WriteFile(filepath.Join(project, "harness.c"), []byte(tortureHarness), 0o644),
			os.WriteFile(filepath.Join(project, "test.c"), append(fmt.Appendf(nil, rename, filepath.Base(src)), data...), 0o644),
		); err != nil {
			return "setup", err.Error()
		}
		if out, err := exec.Command(yaroze, "build", "--opt", level, "-o", exe, project).CombinedOutput(); err != nil {
			return "build", string(out)
		}
		return "", exe
	})
}

func tortureRoot(t *testing.T) string {
	t.Helper()
	root := os.Getenv("PSX_TORTURE")
	if root == "" {
		t.Skip("PSX_TORTURE is not set to c-torture's execute directory")
	}
	return root
}

// goBuild builds one of the repository's tools into dir.
func goBuild(t *testing.T, dir, tool string) string {
	t.Helper()
	path := filepath.Join(dir, tool)
	out, err := exec.Command("go", "build", "-o", path, "go.chrisrx.dev/psx/tools/"+tool).CombinedOutput()
	assert.NoError(t, err, string(out))
	return path
}

// tortureSuite runs every test in root at -O0, -O1 and -O2. build returns
// the executable, or the step at which it failed and why.
func tortureSuite(t *testing.T, root, psxemu string, expected map[string]tortureFailure, build func(src, level, dir string) (failed, detail string)) {
	sources, err := filepath.Glob(filepath.Join(root, "*.c"))
	assert.NoError(t, err)
	assert.Equal(t, true, len(sources) > 300, "the suite's tests should have been found")
	names := make(map[string]bool)
	for _, src := range sources {
		name := strings.TrimSuffix(filepath.Base(src), ".c")
		names[name] = true
		for _, level := range []string{"0", "1", "2"} {
			t.Run(name+"/-O"+level, func(t *testing.T) {
				parallel(t)
				outcome, detail := build(src, level, t.TempDir())
				if outcome == "" {
					outcome, detail = tortureRun(psxemu, detail)
				}
				want := "pass"
				if e, ok := expected[name+"/-O"+level]; ok {
					want = e.outcome
				} else if e, ok := expected[name]; ok {
					want = e.outcome
				}
				if outcome != want {
					t.Errorf("got %s, want %s\n%s", outcome, want, detail)
				}
			})
		}
	}
	for key := range expected {
		if name, _, _ := strings.Cut(key, "/"); !names[name] {
			t.Errorf("%s is expected not to pass and is not in the suite", key)
		}
	}
}

var tortureResult = regexp.MustCompile(`TORTURE (abort|exit (-?\d+))`)

// tortureRun runs exe and returns how it went: pass, or how it did not.
func tortureRun(psxemu, exe string) (outcome, detail string) {
	out, err := exec.Command(psxemu, "--bios", "../emu/openbios/openbios.bin", "--exe", exe, "--script", "1100").CombinedOutput()
	if err != nil {
		var exit *exec.ExitError
		if !errors.As(err, &exit) {
			return "psxemu", err.Error()
		}
		return "psxemu", fmt.Sprintf("%v: %s", err, out)
	}
	m := tortureResult.FindSubmatch(out)
	switch {
	case m == nil && strings.Contains(string(out), "halting"):
		return "halt", string(out)
	case m == nil:
		return "no result", string(out)
	case string(m[1]) == "abort":
		return "abort", string(out)
	case string(m[2]) != "0":
		return "nonzero exit", string(out)
	}
	return "pass", ""
}
