package tests

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"go.chrisrx.dev/x/assert"
)

// needCC skips a test unless YAROZE_CC names the full compiler, so that a
// test run does not download it, and returns a yaroze built to use it.
func needCC(t *testing.T) string {
	t.Helper()
	if os.Getenv("YAROZE_CC") == "" {
		t.Skip("YAROZE_CC is not set to a tinygo-full.wasm")
	}
	yaroze := filepath.Join(t.TempDir(), "yaroze")
	out, err := exec.Command("go", "build", "-o", yaroze, "go.chrisrx.dev/psx/cmd/yaroze").CombinedOutput()
	assert.NoError(t, err, string(out))
	return yaroze
}

// Our libps.h has to lay out every struct as the library does, or a program
// silently reads the wrong fields. The yaroze package's generated bindings
// assert the sizes clang gave for the SDK's header, so a C program asserting
// the same of ours fails to compile if one differs.
func TestCHeaderSizes(t *testing.T) {
	yaroze := needCC(t)
	data, err := os.ReadFile("../yaroze/zz_generated_yaroze.go")
	assert.NoError(t, err)
	var c strings.Builder
	c.WriteString("#include <libps.h>\n")
	n := 0
	for _, m := range regexp.MustCompile(`(?m)^var _ \[unsafe\.Sizeof\((\w+)\{\}\) - (\d+)\]byte$`).FindAllSubmatch(data, -1) {
		name := string(m[1])
		switch name {
		case "DIRENTRY", "EXEC":
			name = "struct " + name
		case "_GsCOORDINATE2":
			name = "GsCOORDINATE2"
		}
		fmt.Fprintf(&c, "typedef char size%d[sizeof(%s) == %s ? 1 : -1];\n", n, name, m[2])
		n++
	}
	assert.Equal(t, true, n > 20, "the bindings' assertions should have been found")
	c.WriteString("int main(void) { return 0; }\n")
	dir := t.TempDir()
	assert.NoError(t, os.WriteFile(filepath.Join(dir, "main.c"), []byte(c.String()), 0o644))
	out, err := exec.Command(yaroze, "build", "-o", filepath.Join(dir, "main.exe"), dir).CombinedOutput()
	assert.NoError(t, err, string(out))
}

// What a program's compiler calls for it has to do what it is called for, and
// not call itself: written as C, a helper is a pattern the optimizer knows.
// And an array of bytes has to be word aligned, as GCC had it, since the
// library stores words into a PACKET area.
func TestCRuntimeHelpers(t *testing.T) {
	yaroze := needCC(t)
	dir := t.TempDir()
	assert.NoError(t, os.WriteFile(filepath.Join(dir, "main.c"), []byte(`#include <stdio.h>
#include <stdlib.h>
volatile unsigned int small = 7, big = 0x80000006u, top = 0xfffffffeu;
volatile int negative = -7;
volatile float half = 0.5f;
char one = 1;
char area[64];
char other = 2;
char rodata_one[1] = { 3 };
char rodata_area[64] = { 4 };
/* Reads through a pointer it never set, as old programs do by accident: null,
   which on this machine is memory like any other. */
int *unset(void)
{
	int *p;
	*(volatile int *)&p;
	return p;
}
int main(void)
{
	printf("G %d\n", unset() == 0);
	printf("F %d\n", ((unsigned long)area | (unsigned long)rodata_area) & 3);
	double d = big;
	float f = small;
	printf("A %d\n", (int)(d - 2147483648.0));
	printf("B %d\n", (int)(f + half + half));
	printf("C %u\n", (unsigned int)(double)top);
	printf("D %d\n", abs(negative));
	printf("E %d\n", (int)(float)big == -2147483648 || (float)big > 0.0f);
	printf("DONE\n");
	for (;;)
		VSync(0);
}
`), 0o644))
	exe := filepath.Join(dir, "main.exe")
	out, err := exec.Command(yaroze, "build", "-o", exe, dir).CombinedOutput()
	assert.NoError(t, err, string(out))
	psxemu := filepath.Join(dir, "psxemu")
	out, err = exec.Command("go", "build", "-o", psxemu, "go.chrisrx.dev/psx/tools/psxemu").CombinedOutput()
	assert.NoError(t, err, string(out))
	out, err = exec.Command(psxemu, "--bios", "../emu/openbios/openbios.bin", "--exe", exe, "--script", "1200").CombinedOutput()
	assert.NoError(t, err, string(out))
	for _, want := range []string{"A 6", "B 8", "C 4294967294", "D 7", "E 1", "F 0", "G 1", "DONE"} {
		assert.Equal(t, true, strings.Contains(string(out), want), want+" in "+string(out))
	}
}

// Projects from the Yaroze websites, built from their source by yaroze, have
// to draw what the executables that came with them draw: the same video
// memory, byte for byte, after the same number of frames.
// PSX_WEBARCHIVE is the archive's site directory.
func TestCArchiveProjects(t *testing.T) {
	root := os.Getenv("PSX_WEBARCHIVE")
	if root == "" {
		t.Skip("PSX_WEBARCHIVE is not set")
	}
	yaroze := needCC(t)
	psxemu := filepath.Join(t.TempDir(), "psxemu")
	out, err := exec.Command("go", "build", "-o", psxemu, "go.chrisrx.dev/psx/tools/psxemu").CombinedOutput()
	assert.NoError(t, err, string(out))
	eco2exe := filepath.Join(t.TempDir(), "eco2exe")
	out, err = exec.Command("go", "build", "-o", eco2exe, "go.chrisrx.dev/psx/tools/eco2exe").CombinedOutput()
	assert.NoError(t, err, string(out))

	vram := func(t *testing.T, exe string) []byte {
		path := exe + ".vram"
		out, err := exec.Command(psxemu, "--quiet", "--bios", "../emu/openbios/openbios.bin", "--exe", exe, "--script", "1500", "--vram", path).CombinedOutput()
		assert.NoError(t, err, string(out))
		data, err := os.ReadFile(path)
		assert.NoError(t, err)
		return data
	}
	tutor := filepath.Join(root, "yaroze", "ftp", "scee", "tutor")
	for _, tt := range []struct {
		name  string
		dir   string
		loads []string // what the auto file loads, for the executable that came with it
	}{
		{"sprite", filepath.Join(tutor, "chap1.zip.x", "3"), []string{"../data/yar8bit.tim@80090000"}},
		{"24bit", filepath.Join(tutor, "chap2.zip.x", "7"), []string{"../data/YAR24BIT.TIM@80090000"}},
		{"game", filepath.Join(tutor, "chap3.zip.x", "2"), nil},
	} {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			built := filepath.Join(dir, "built.exe")
			out, err := exec.Command(yaroze, "build", "-o", built, tt.dir).CombinedOutput()
			assert.NoError(t, err, string(out))

			shipped := filepath.Join(dir, "shipped.exe")
			args := []string{}
			for _, l := range tt.loads {
				args = append(args, "--load", filepath.Join(tt.dir, l))
			}
			args = append(args, filepath.Join(tt.dir, "MAIN"), shipped)
			out, err = exec.Command(eco2exe, args...).CombinedOutput()
			assert.NoError(t, err, string(out))

			got, want := vram(t, built), vram(t, shipped)
			assert.Equal(t, true, bytes.Equal(got, want), "video memory should match what the shipped executable draws")
			assert.Equal(t, true, bytes.ContainsFunc(want, func(r rune) bool { return r != 0 }), "the program should have drawn something")
		})
	}
}
