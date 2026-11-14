package keepcheck

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.chrisrx.dev/x/assert"
)

// The table is what the yaroze package's source says now.
func TestGeneratedTable(t *testing.T) {
	derived, err := Derive("../../yaroze")
	assert.NoError(t, err)
	want, err := derived.Source()
	assert.NoError(t, err)
	got, err := os.ReadFile("zz_generated_keeps.go")
	assert.NoError(t, err)
	assert.Equal(t, string(want), string(got), "zz_generated_keeps.go is out of date: go generate ./internal/keepcheck")
}

// checkSource checks a program's main.go, and returns each problem as its
// line and the start of its message.
func checkSource(t *testing.T, src string) []string {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "main.go", src, 0)
	assert.NoError(t, err)
	var got []string
	for _, p := range Check(fset, []*ast.File{f}) {
		got = append(got, fmt.Sprintf("%d: %s", p.Pos.Line, p.Msg))
	}
	return got
}

func TestCheck(t *testing.T) {
	got := checkSource(t, `package main

import "go.chrisrx.dev/psx/yaroze"

var packets [2][256]uint32

func setup() {
	var area [256]uint32
	yaroze.GsSetWorkBase(yaroze.Packets(&area[0])) // made once: the compiler sees to it
	var synced [64]uint32
	r := yaroze.RECT{W: 16, H: 8}
	r.Load(&synced[0])
	yaroze.DrawSync(0)
	var c yaroze.GsCOORDINATE2
	c.Init(nil) // made once
}

func unsynced() {
	var pixels [64]uint32
	r := yaroze.RECT{W: 16, H: 8}
	yaroze.LoadImage(&r, &pixels[0]) // 21: unsynced returns before DrawSync
}

func main() {
	var own [64]uint32
	r := yaroze.RECT{W: 16, H: 8}
	yaroze.LoadImage(&r, &own[0]) // main does not return
	yaroze.GsSetWorkBase(yaroze.Packets(&packets[0][0]))
	setup()
	for i := 0; i < 2; i++ {
		var each [256]uint32
		yaroze.GsSetWorkBase(yaroze.Packets(&each[0])) // 32: made each time round
		s := []uint32{1, 2}
		yaroze.GsSetWorkBase(yaroze.Packets(&s[0])) // a slice's elements: not its memory
	}
	screen := yaroze.Screen{}
	for ot := range screen.Frames() {
		var buf [64]uint32
		yaroze.LoadImage(&r, &buf[0]) // 39: the frame loop's body returns first
		c := &yaroze.GsCOORDINATE2{}
		c.Init(nil) // 41: made every frame
		yaroze.GsSortClear(0, 0, 0, ot)
	}
}
`)
	want := []string{
		"21: yaroze.LoadImage goes on with p until DrawSync, but pixels is a variable of unsynced",
		"32: yaroze.GsSetWorkBase keeps outpacketp, so each, made in a loop, is allocated each time round",
		"39: yaroze.LoadImage goes on with p until DrawSync, but buf is a variable of the body of the loop over screen.Frames()",
		"41: (*GsCOORDINATE2).Init keeps c, so c, made in the body of the loop over screen.Frames(), which may run every frame",
	}
	assert.Equal(t, len(want), len(got), strings.Join(got, "\n"))
	for i := range min(len(want), len(got)) {
		assert.Equal(t, true, strings.HasPrefix(got[i], want[i]), got[i])
	}
}

// What a program names yaroze, and what shadows it.
func TestCheckImportNames(t *testing.T) {
	got := checkSource(t, `package main

import y "go.chrisrx.dev/psx/yaroze"

func load() {
	var buf [64]uint32
	r := y.RECT{}
	y.LoadImage(&r, &buf[0])
}

func other() {
	y := struct{ LoadImage func(*int, *int) }{}
	var a, b int
	y.LoadImage(&a, &b)
}

func main() {}
`)
	assert.Equal(t, 1, len(got), strings.Join(got, "\n"))
}

// The programs in the repository keep to the rules.
func TestCheckRepository(t *testing.T) {
	var dirs []string
	for _, root := range []string{"../../examples", "../../docs"} {
		err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if err != nil || !d.IsDir() {
				return err
			}
			if strings.HasPrefix(d.Name(), ".") || d.Name() == "testdata" {
				return filepath.SkipDir
			}
			dirs = append(dirs, path)
			return nil
		})
		assert.NoError(t, err)
	}
	assert.Equal(t, true, len(dirs) > 20, "the examples should have been found")
	for _, dir := range dirs {
		problems, err := CheckDir(dir)
		assert.NoError(t, err)
		for _, p := range problems {
			t.Errorf("%s: %s", p.Pos, p.Msg)
		}
	}
}
