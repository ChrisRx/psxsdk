// genkeep writes a Go file whose closures call every //export function of
// exp/libps with zero arguments, so that a build keeps them all.
package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/build"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

var predeclared = map[string]bool{"bool": true, "byte": true, "rune": true, "int": true, "int8": true, "int16": true, "int32": true, "int64": true, "uint": true, "uint8": true, "uint16": true, "uint32": true, "uint64": true, "uintptr": true, "string": true, "error": true, "any": true, "float32": true, "float64": true}

func main() {
	dir := os.Args[1]
	ctx := build.Default
	ctx.BuildTags = []string{"yaroze", "golibps"}
	ctx.GOOS, ctx.GOARCH = "linux", "mipsle"
	pkg, err := ctx.ImportDir(dir, 0)
	if err != nil {
		panic(err)
	}
	fset := token.NewFileSet()
	var calls []string
	for _, name := range pkg.GoFiles {
		f, err := parser.ParseFile(fset, filepath.Join(dir, name), nil, parser.ParseComments)
		if err != nil {
			panic(err)
		}
		for _, d := range f.Decls {
			fn, ok := d.(*ast.FuncDecl)
			if !ok || fn.Recv != nil || fn.Doc == nil || !strings.Contains(fn.Doc.Text(), "") {
				continue
			}
			exported := false
			for _, c := range fn.Doc.List {
				if strings.HasPrefix(c.Text, "//export ") {
					exported = true
				}
			}
			if !exported || !ast.IsExported(fn.Name.Name) {
				continue
			}
			var args []string
			for _, p := range fn.Type.Params.List {
				t := qualify(p.Type)
				var b bytes.Buffer
				printer.Fprint(&b, fset, t)
				n := len(p.Names)
				if n == 0 {
					n = 1
				}
				for range n {
					args = append(args, "opaque["+b.String()+"]()")
				}
			}
			calls = append(calls, fmt.Sprintf("\tfunc() { libps.%s(%s) },", fn.Name.Name, strings.Join(args, ", ")))
		}
	}
	sort.Strings(calls)
	fmt.Println("//go:build yaroze\n\npackage main\n\nimport (\n\t\"unsafe\"\n\n\t\"go.chrisrx.dev/psx/exp/libps\"\n)\n\nvar _ unsafe.Pointer\n\nvar keep = [...]func(){")
	fmt.Println(strings.Join(calls, "\n"))
	fmt.Println("}")
}

func qualify(e ast.Expr) ast.Expr {
	switch t := e.(type) {
	case *ast.Ident:
		if !predeclared[t.Name] && ast.IsExported(t.Name) {
			return &ast.SelectorExpr{X: ast.NewIdent("libps"), Sel: t}
		}
	case *ast.StarExpr:
		t.X = qualify(t.X)
	case *ast.ArrayType:
		t.Elt = qualify(t.Elt)
		if t.Len != nil {
			t.Len = qualify(t.Len)
		}
	case *ast.Ellipsis:
		return &ast.ArrayType{Elt: qualify(t.Elt)}
	}
	return e
}
