package yaroze

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// goAPI is the import path of the Go library's own API, exp/ps. A
// program that imports it cannot run on the resident library, so the
// import chooses the Go library, as --libps go does.
const goAPI = "go.chrisrx.dev/psx/exp/ps"

// importsGoAPI reports whether any Go file of the package directory pkg,
// test files aside, imports goAPI. It reads the imports alone, without
// resolving them, so a package that cannot be parsed reports false and
// is left to the compiler to complain about.
func importsGoAPI(pkg string) bool {
	entries, err := os.ReadDir(pkg)
	if err != nil {
		return false
	}
	fset := token.NewFileSet()
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, filepath.Join(pkg, name), nil, parser.ImportsOnly)
		if err != nil {
			continue
		}
		for _, imp := range f.Imports {
			if path, err := strconv.Unquote(imp.Path.Value); err == nil && path == goAPI {
				return true
			}
		}
	}
	return false
}
