package cproject

import (
	"os"
	"path/filepath"
	"testing"

	"go.chrisrx.dev/x/assert"
)

func write(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for name, data := range files {
		path := filepath.Join(dir, filepath.FromSlash(name))
		assert.NoError(t, os.MkdirAll(filepath.Dir(path), 0o777))
		assert.NoError(t, os.WriteFile(path, []byte(data), 0o666))
	}
}

func TestIs(t *testing.T) {
	dir := t.TempDir()
	assert.Equal(t, false, Is(dir))
	write(t, dir, map[string]string{"MAIN.C": ""})
	assert.Equal(t, true, Is(dir))
	write(t, dir, map[string]string{"main.go": ""})
	assert.Equal(t, false, Is(dir))
}

func names(p *Project) []string {
	var out []string
	for _, s := range p.Sources {
		out = append(out, filepath.Base(s))
	}
	return out
}

func TestReadMakefileAndAuto(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, map[string]string{
		"Makefile": "# built by hand\r\n" +
			"CFLAGS = -O2 -DFAST -DLEVEL=2 -Iinc\r\n" +
			"LINKER = -Xlinker -Map -Xlinker mapfile -Xlinker -Ttext -Xlinker 80100000\r\n" +
			"OBJS = main.o \\\r\n\tGAME.o\r\n" +
			"main: $(OBJS)\r\n\t$(CC) $(OBJS) $(LINKER) -o main\r\n",
		"auto":     "local dload ..\\DATA\\Sprite.TIM 80090000\r\nlocal dload gone.tim 800a0000\r\nlocal load main\r\ngo\r\n",
		"MAIN.C":   "",
		"game.c":   "",
		"unused.c": "",
		"inc/x.h":  "",
	})
	write(t, filepath.Dir(dir), map[string]string{"DATA/sprite.tim": "tim"})
	p, err := Read(dir)
	assert.NoError(t, err)
	assert.Equal(t, uint32(0x80100000), p.Addr)
	assert.Equal(t, []string{"MAIN.C", "game.c"}, names(p))
	assert.Equal(t, []string{"FAST", "LEVEL=2"}, p.Defines)
	assert.Equal(t, []string{filepath.Join(dir, "inc")}, p.Includes)
	assert.Equal(t, 1, len(p.Loads))
	assert.Equal(t, uint32(0x80090000), p.Loads[0].Addr)
	assert.Equal(t, []string{"gone.tim"}, p.Missing)
}

// A Makefile's variable is often only what its programs share: the rule for
// the program auto loads adds its own main.
func TestReadRuleOfProgram(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, map[string]string{
		"makefile": "OBJECTS = pad.o\nall: main demo\n" +
			"main: $(OBJECTS) main.o\n\t$(CC) $(OBJECTS) main.o -o main\n" +
			"demo: $(OBJECTS) store.o demo.o\n\t$(CC) $(OBJECTS) store.o demo.o -o demo\n" +
			"main.o: main.c pad.h\n",
		"auto":  "local load demo\ngo\n",
		"pad.c": "", "main.c": "", "demo.c": "", "store.c": "",
	})
	p, err := Read(dir)
	assert.NoError(t, err)
	assert.Equal(t, []string{"pad.c", "store.c", "demo.c"}, names(p))

	// With no auto to say, it is the first thing all makes.
	assert.NoError(t, os.Remove(filepath.Join(dir, "auto")))
	p, err = Read(dir)
	assert.NoError(t, err)
	assert.Equal(t, []string{"pad.c", "main.c"}, names(p))
}

func TestReadDefaults(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, map[string]string{"b.c": "", "A.C": ""})
	p, err := Read(dir)
	assert.NoError(t, err)
	assert.Equal(t, uint32(DefaultAddr), p.Addr)
	assert.Equal(t, 2, len(p.Sources))
}

func TestReadObjectWithoutSource(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, map[string]string{"Makefile": "OBJS = main.o lib.o\n", "main.c": ""})
	_, err := Read(dir)
	assert.Error(t, "which has no C or assembly source", err)
}

func TestClean(t *testing.T) {
	for _, tt := range []struct{ name, in, want string }{
		{"line ends and the DOS end of file", "int a;\r\nint b;\r\n\x1a", "int a;\nint b;\n"},
		{"Shift-JIS backslash in a string", "char *s = \"\x95\x5c\";\n", "char *s = \"\x95\\\x5c\";\n"},
		{"Shift-JIS backslash ending a line comment", "// \x95\x5c\nint a;\n", "// \x95 \nint a;\n"},
		{"Shift-JIS backslash in a block comment", "/* \x95\x5c */\n", "/* \x95  */\n"},
		{"an escape in a string is left alone", `char *s = "a\"b\\";` + "\n", `char *s = "a\"b\\";` + "\n"},
		{"a backslash in code is left alone", "#define A 1 \\\n  + 2\n", "#define A 1 \\\n  + 2\n"},
		{"a string over two lines", "char *s = \"a\nb\";\n", "char *s = \"a\\n\\\nb\";\n"},
		{"an apostrophe in a #error ends at its line", "#error don't\nint a;\n", "#error don't\nint a;\n"},
		{"a quote in a #error ends at its line", "#error \"no\nchar *s = \"\x95\x5c\";\n", "#error \"no\nchar *s = \"\x95\\\x5c\";\n"},
		{"an apostrophe in a comment", "// don't\nchar *s = \"a\";\n", "// don't\nchar *s = \"a\";\n"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, string(clean([]byte(tt.in))))
		})
	}
}

func TestPrepareIncludes(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "game")
	write(t, root, map[string]string{
		"game/MAIN.C": "#include <LIBPS.H>\r\n" +
			"#include \"Sub\\Pad.h\"\r\n" +
			"#include <local.h>\r\n" +
			"#include \"../shared/util.h\"\r\n" +
			"#include \"nowhere\\x.h\"\r\n\x1a",
		"game/sub/PAD.H":   "#include \"other.h\"\n",
		"game/sub/other.h": "int other;\n",
		"game/local.h":     "int local;\n",
		"shared/util.h":    "int util;\n",
	})
	p := &Project{Dir: dir, Sources: []string{filepath.Join(dir, "MAIN.C")}}
	dst := filepath.Join(t.TempDir(), "src")
	sources, err := p.Prepare(dst, func(name string) bool { return name == "libps.h" })
	assert.NoError(t, err)
	data, err := os.ReadFile(sources[0])
	assert.NoError(t, err)
	assert.Equal(t, "#include <libps.h>\n"+
		"#include \"sub/PAD.H\"\n"+
		"#include \"local.h\"\n"+
		"#include \"up/shared/util.h\"\n"+
		"#include \"nowhere/x.h\"\n", string(data))
	for _, name := range []string{"sub/PAD.H", "sub/other.h", "local.h", "up/shared/util.h"} {
		_, err := os.Stat(filepath.Join(dst, name))
		assert.NoError(t, err)
	}
}
