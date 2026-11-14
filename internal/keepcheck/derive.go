package keepcheck

import (
	"fmt"
	"go/ast"
	"go/build"
	"go/format"
	"go/parser"
	"go/token"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// Keep is a parameter of a function of package yaroze that the library
// keeps after the call returns.
type Keep struct {
	Param int    // which parameter, from 0, or -1 for the receiver
	Name  string // its name
	Until string // the yaroze function after which it is not kept, or "" for good
}

// Table is what package yaroze's functions keep, by name: "LoadImage" for a
// function, "(*RECT).Load" or "(Bank).Open" for a method. Through is the
// functions that return what one of their parameters points to, by
// parameter, such as Packets, a conversion: a pointer passed through one is
// the pointer passed. Iterators is the functions and methods that return an
// iterator, whose loop body the compiler makes a function literal.
type Table struct {
	Keeps     map[string][]Keep
	Through   map[string]int
	Iterators map[string]bool
}

// Derive reads the yaroze package in dir, as the build that has the library
// sees it: the //go:keeps pragmas on its bindings, and then its own
// functions and methods that pass a parameter on to one, which keep it too.
func Derive(dir string) (*Table, error) {
	ctx := build.Default
	ctx.BuildTags = []string{"yaroze", "tinygo", "baremetal"}
	ctx.GOOS, ctx.GOARCH = "linux", "mipsle"
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	fset := token.NewFileSet()
	var decls []*ast.FuncDecl
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		if ok, err := ctx.MatchFile(dir, name); err != nil || !ok {
			if err != nil {
				return nil, err
			}
			continue
		}
		f, err := parser.ParseFile(fset, filepath.Join(dir, name), nil, parser.ParseComments)
		if err != nil {
			return nil, err
		}
		for _, d := range f.Decls {
			if fd, ok := d.(*ast.FuncDecl); ok {
				decls = append(decls, fd)
			}
		}
	}

	t := &Table{Keeps: make(map[string][]Keep), Through: make(map[string]int), Iterators: make(map[string]bool)}
	for _, fd := range decls {
		if returnsIterator(fd.Type) {
			t.Iterators[funcKey(fd)] = true
		}
	}
	for _, fd := range decls {
		if fd.Body != nil || fd.Doc == nil {
			continue
		}
		params := paramNames(fd)
		for _, c := range fd.Doc.List {
			f := strings.Fields(c.Text)
			if len(f) < 2 || f[0] != "//go:keeps" {
				continue
			}
			i := slices.Index(params, f[1])
			if i < 0 {
				return nil, fmt.Errorf("%s: //go:keeps names %s, which %s has not", fset.Position(c.Pos()), f[1], fd.Name.Name)
			}
			k := Keep{Param: i, Name: f[1]}
			if len(f) > 2 {
				k.Until = f[2]
			}
			t.Keeps[fd.Name.Name] = append(t.Keeps[fd.Name.Name], k)
		}
	}

	// The functions that return a parameter, or what it points to.
	for _, fd := range decls {
		if fd.Recv != nil || fd.Body == nil || len(fd.Body.List) != 1 {
			continue
		}
		ret, ok := fd.Body.List[0].(*ast.ReturnStmt)
		if !ok || len(ret.Results) != 1 {
			continue
		}
		if name := t.root(ret.Results[0]); name != "" {
			if i := slices.Index(paramNames(fd), name); i >= 0 {
				t.Through[fd.Name.Name] = i
			}
		}
	}

	// The package's own functions that pass a parameter on to one that
	// keeps it keep it too, until nothing more is found.
	for changed := true; changed; {
		changed = false
		for _, fd := range decls {
			if fd.Body == nil {
				continue
			}
			key := funcKey(fd)
			params := paramNames(fd)
			recv := recvName(fd)
			types := paramTypes(fd)
			ast.Inspect(fd.Body, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				callee, recvArg := t.callee(call, types)
				for _, k := range t.Keeps[callee] {
					var arg ast.Expr
					switch {
					case k.Param == -1:
						arg = recvArg
					case k.Param < len(call.Args):
						arg = call.Args[k.Param]
					}
					if arg == nil {
						continue
					}
					name := t.root(arg)
					idx := slices.Index(params, name)
					if name != "" && name == recv {
						idx = -1
					} else if idx < 0 {
						continue
					}
					if !slices.ContainsFunc(t.Keeps[key], func(o Keep) bool { return o.Param == idx }) {
						t.Keeps[key] = append(t.Keeps[key], Keep{Param: idx, Name: name, Until: k.Until})
						changed = true
					}
				}
				return true
			})
		}
	}
	for k := range t.Keeps {
		slices.SortFunc(t.Keeps[k], func(a, b Keep) int { return a.Param - b.Param })
	}
	return t, nil
}

// root returns the variable whose memory e points into, through &, the
// indexes and fields it takes the address of, slicing, parentheses,
// conversions to a pointer and the functions that return a parameter, or ""
// if it is anything else. A field or element read without & is a value held
// in the variable, not the variable's memory.
func (t *Table) root(e ast.Expr) string {
	addressed := false
	for {
		switch x := e.(type) {
		case *ast.ParenExpr:
			e = x.X
		case *ast.UnaryExpr:
			if x.Op != token.AND {
				return ""
			}
			addressed = true
			e = x.X
		case *ast.IndexExpr:
			if !addressed {
				return ""
			}
			e = x.X
		case *ast.SliceExpr:
			e = x.X
		case *ast.SelectorExpr:
			if !addressed {
				return ""
			}
			e = x.X
		case *ast.CallExpr:
			arg, ok := t.through(x)
			if !ok {
				return ""
			}
			e = arg
		case *ast.Ident:
			return x.Name
		default:
			return ""
		}
	}
}

// through returns the argument a call passes back, if it is a conversion
// to a pointer type or a call of a function that returns its parameter.
func (t *Table) through(call *ast.CallExpr) (ast.Expr, bool) {
	fun := call.Fun
	for {
		p, ok := fun.(*ast.ParenExpr)
		if !ok {
			break
		}
		fun = p.X
	}
	if len(call.Args) == 1 {
		if _, ok := fun.(*ast.StarExpr); ok {
			return call.Args[0], true
		}
		if sel, ok := fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "Pointer" {
			if id, ok := sel.X.(*ast.Ident); ok && id.Name == "unsafe" {
				return call.Args[0], true
			}
		}
	}
	name := ""
	switch f := fun.(type) {
	case *ast.Ident:
		name = f.Name
	case *ast.SelectorExpr:
		name = f.Sel.Name // the yaroze package's, seen from outside
	}
	if i, ok := t.Through[name]; ok && i < len(call.Args) {
		return call.Args[i], true
	}
	return nil, false
}

// callee returns the table's name for what call calls, inside the yaroze
// package, and the receiver it is called on, if it is a method: types gives
// the named type of each parameter and the receiver.
func (t *Table) callee(call *ast.CallExpr, types map[string]string) (string, ast.Expr) {
	switch f := call.Fun.(type) {
	case *ast.Ident:
		return f.Name, nil
	case *ast.SelectorExpr:
		if id, ok := f.X.(*ast.Ident); ok {
			if typ, ok := types[id.Name]; ok {
				for _, key := range []string{"(*" + typ + ")." + f.Sel.Name, "(" + typ + ")." + f.Sel.Name} {
					if _, ok := t.Keeps[key]; ok {
						return key, f.X
					}
				}
			}
		}
	}
	return "", nil
}

// returnsIterator reports whether a function of type ft returns an
// iterator: an iter.Seq or iter.Seq2, or a function taking a yield function.
func returnsIterator(ft *ast.FuncType) bool {
	if ft.Results == nil || len(ft.Results.List) != 1 || len(ft.Results.List[0].Names) > 1 {
		return false
	}
	typ := ft.Results.List[0].Type
	switch x := typ.(type) {
	case *ast.IndexExpr:
		typ = x.X
	case *ast.IndexListExpr:
		typ = x.X
	}
	if sel, ok := typ.(*ast.SelectorExpr); ok {
		if id, ok := sel.X.(*ast.Ident); ok && id.Name == "iter" {
			return sel.Sel.Name == "Seq" || sel.Sel.Name == "Seq2"
		}
	}
	if f, ok := typ.(*ast.FuncType); ok && f.Params != nil && len(f.Params.List) == 1 {
		_, ok := f.Params.List[0].Type.(*ast.FuncType)
		return ok
	}
	return false
}

func funcKey(fd *ast.FuncDecl) string {
	if fd.Recv == nil || len(fd.Recv.List) == 0 {
		return fd.Name.Name
	}
	typ := fd.Recv.List[0].Type
	if star, ok := typ.(*ast.StarExpr); ok {
		if id, ok := star.X.(*ast.Ident); ok {
			return "(*" + id.Name + ")." + fd.Name.Name
		}
	}
	if id, ok := typ.(*ast.Ident); ok {
		return "(" + id.Name + ")." + fd.Name.Name
	}
	return fd.Name.Name
}

func paramNames(fd *ast.FuncDecl) []string {
	var names []string
	for _, f := range fd.Type.Params.List {
		if len(f.Names) == 0 {
			names = append(names, "_")
		}
		for _, n := range f.Names {
			names = append(names, n.Name)
		}
	}
	return names
}

func recvName(fd *ast.FuncDecl) string {
	if fd.Recv == nil || len(fd.Recv.List) == 0 || len(fd.Recv.List[0].Names) == 0 {
		return ""
	}
	return fd.Recv.List[0].Names[0].Name
}

// paramTypes returns the named type, without the pointer, of each
// parameter and the receiver of fd that has one.
func paramTypes(fd *ast.FuncDecl) map[string]string {
	types := make(map[string]string)
	add := func(fl *ast.FieldList) {
		if fl == nil {
			return
		}
		for _, f := range fl.List {
			typ := f.Type
			if star, ok := typ.(*ast.StarExpr); ok {
				typ = star.X
			}
			id, ok := typ.(*ast.Ident)
			if !ok {
				continue
			}
			for _, n := range f.Names {
				types[n.Name] = id.Name
			}
		}
	}
	add(fd.Recv)
	add(fd.Type.Params)
	return types
}

// Source returns the table as the Go source of zz_generated_keeps.go.
func (t *Table) Source() ([]byte, error) {
	var b strings.Builder
	b.WriteString("// Code generated by go run ./gen from package yaroze. DO NOT EDIT.\n\npackage keepcheck\n\n")
	b.WriteString("// generated is what package yaroze's functions keep, from the //go:keeps\n// pragmas on its bindings and its own functions that pass on to them.\n")
	b.WriteString("var generated = Table{\n\tKeeps: map[string][]Keep{\n")
	for _, k := range slices.Sorted(maps.Keys(t.Keeps)) {
		fmt.Fprintf(&b, "\t\t%q: {", k)
		for i, keep := range t.Keeps[k] {
			if i > 0 {
				b.WriteString(", ")
			}
			fmt.Fprintf(&b, "{Param: %d, Name: %q, Until: %q}", keep.Param, keep.Name, keep.Until)
		}
		b.WriteString("},\n")
	}
	b.WriteString("\t},\n\tThrough: map[string]int{\n")
	for _, k := range slices.Sorted(maps.Keys(t.Through)) {
		fmt.Fprintf(&b, "\t\t%q: %d,\n", k, t.Through[k])
	}
	b.WriteString("\t},\n\tIterators: map[string]bool{\n")
	for _, k := range slices.Sorted(maps.Keys(t.Iterators)) {
		fmt.Fprintf(&b, "\t\t%q: true,\n", k)
	}
	b.WriteString("\t},\n}\n")
	return format.Source([]byte(b.String()))
}
