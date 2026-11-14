// Package keepcheck finds, in a program's source, the variables it passes
// to the Net Yaroze library that the library goes on using after they are
// gone: what yaroze vet reports.
//
// TinyGo takes every function the library binds to keep none of the
// pointers it is given, as cgo has C do, so a variable whose address is
// passed stays on the stack. A few of the library's parameters are kept
// all the same, which their bindings say with //go:keeps, and the compiler
// refuses a program that would lose one (transform/keeps.go in
// internal/toolchain/_tinygo). This checks the same on the source, where it
// can name the variable, before a build: a variable of a function other
// than main passed to a parameter kept until a later call, such as
// LoadImage's pixels until DrawSync, when the function does not make that
// call after it; and a variable made in a loop or in a function literal,
// such as the frame loop's body, passed to a parameter kept for good, which
// would be allocated every time round and never freed.
//
// It reads the syntax alone, and follows a variable's address to the call
// through &, its fields and elements, slicing and conversions, but not
// through another variable, so what it finds is certain and what it misses
// the compiler still refuses. A function literal that runs once is taken as
// one that may run every frame, and so is the body of a range over an
// iterator, such as Screen.Frames, which the compiler makes one.
package keepcheck

//go:generate go run ./gen

import (
	"go/ast"
	"go/build"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// Path is the import path of the package whose functions keep.
const Path = "go.chrisrx.dev/psx/yaroze"

// A Problem is a variable passed to the library that it would lose.
type Problem struct {
	Pos token.Position
	Msg string
}

// CheckDir checks the package in dir, as the PlayStation build sees it.
func CheckDir(dir string) ([]Problem, error) {
	ctx := build.Default
	ctx.BuildTags = []string{"yaroze", "tinygo", "baremetal"}
	ctx.GOOS, ctx.GOARCH = "linux", "mipsle"
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	fset := token.NewFileSet()
	var files []*ast.File
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		if ok, err := ctx.MatchFile(dir, name); err != nil {
			return nil, err
		} else if !ok {
			continue
		}
		f, err := parser.ParseFile(fset, filepath.Join(dir, name), nil, 0)
		if err != nil {
			return nil, err
		}
		files = append(files, f)
	}
	return Check(fset, files), nil
}

// Check checks the files of one package against the generated table.
func Check(fset *token.FileSet, files []*ast.File) []Problem {
	return check(&generated, fset, files)
}

func check(t *Table, fset *token.FileSet, files []*ast.File) []Problem {
	c := &checker{t: t, fset: fset, iterators: make(map[string]bool)}
	// The package's own iterators, by name, which a range over a call of
	// one makes its body a function literal.
	for _, f := range files {
		for _, d := range f.Decls {
			if fd, ok := d.(*ast.FuncDecl); ok && returnsIterator(fd.Type) {
				c.iterators[fd.Name.Name] = true
			}
		}
	}
	for _, f := range files {
		c.yaroze = ""
		for _, imp := range f.Imports {
			if strings.Trim(imp.Path.Value, `"`) != Path {
				continue
			}
			c.yaroze = "yaroze"
			if imp.Name != nil {
				c.yaroze = imp.Name.Name
			}
		}
		if c.yaroze == "" || c.yaroze == "_" {
			continue
		}
		isMain := f.Name.Name == "main"
		// The package's variables, for their types: none is a local.
		c.scopes = nil
		c.push()
		for _, f := range files {
			for _, d := range f.Decls {
				gd, ok := d.(*ast.GenDecl)
				if !ok || gd.Tok != token.VAR {
					continue
				}
				for _, s := range gd.Specs {
					vs := s.(*ast.ValueSpec)
					for i, id := range vs.Names {
						var v ast.Expr
						if i < len(vs.Values) {
							v = vs.Values[i]
						}
						c.declare(id, vs.Type, v)
					}
				}
			}
		}
		for _, d := range f.Decls {
			fd, ok := d.(*ast.FuncDecl)
			if !ok || fd.Body == nil {
				continue
			}
			fn := &function{name: fd.Name.Name, main: isMain && fd.Recv == nil && fd.Name.Name == "main"}
			c.enter(fn, fd.Recv, fd.Type, fd.Body)
		}
	}
	slices.SortFunc(c.problems, func(a, b Problem) int {
		if a.Pos.Filename != b.Pos.Filename {
			return strings.Compare(a.Pos.Filename, b.Pos.Filename)
		}
		return a.Pos.Offset - b.Pos.Offset
	})
	return c.problems
}

type function struct {
	name    string
	main    bool // main.main, which does not return while the program runs
	literal bool // a function literal, which may run every frame
	syncs   map[string][]token.Pos
}

// A local is a variable of a function, or a value a composite literal makes.
type local struct {
	name  string
	fn    *function // nil for a package-level variable
	loop  bool      // made in a loop of fn
	typ   string    // its type's name in the yaroze package, if it is one
	ptr   bool      // a pointer to a value a composite literal made
	array bool      // an array, whose elements are its own memory
}

type checker struct {
	t         *Table
	fset      *token.FileSet
	yaroze    string
	iterators map[string]bool
	problems  []Problem

	fn     *function
	loop   int
	scopes []map[string]*local
}

func (c *checker) push() { c.scopes = append(c.scopes, map[string]*local{}) }
func (c *checker) pop()  { c.scopes = c.scopes[:len(c.scopes)-1] }

func (c *checker) lookup(name string) *local {
	for i := len(c.scopes) - 1; i >= 0; i-- {
		if l, ok := c.scopes[i][name]; ok {
			return l
		}
	}
	return nil
}

func (c *checker) declare(id *ast.Ident, typ ast.Expr, value ast.Expr) {
	if id == nil || id.Name == "_" {
		return
	}
	l := &local{name: id.Name, fn: c.fn, loop: c.loop > 0}
	l.typ, l.ptr = c.typeOf(typ, value)
	if typ == nil {
		if lit, ok := value.(*ast.CompositeLit); ok {
			typ = lit.Type
		}
	}
	if at, ok := typ.(*ast.ArrayType); ok && at.Len != nil {
		l.array = true
	}
	c.scopes[len(c.scopes)-1][id.Name] = l
}

// typeOf returns the name of a yaroze type that a declared type or a value
// has, and whether the value is the address of a composite literal.
func (c *checker) typeOf(typ, value ast.Expr) (string, bool) {
	name := func(e ast.Expr) string {
		if sel, ok := e.(*ast.SelectorExpr); ok {
			if id, ok := sel.X.(*ast.Ident); ok && id.Name == c.yaroze {
				return sel.Sel.Name
			}
		}
		return ""
	}
	if typ != nil {
		return name(typ), false
	}
	switch v := value.(type) {
	case *ast.CompositeLit:
		return name(v.Type), false
	case *ast.UnaryExpr:
		if lit, ok := v.X.(*ast.CompositeLit); ok && v.Op == token.AND {
			return name(lit.Type), true
		}
	}
	return "", false
}

// enter checks the body of a function, its parameters declared.
func (c *checker) enter(fn *function, recv *ast.FieldList, typ *ast.FuncType, body *ast.BlockStmt) {
	outer, outerLoop := c.fn, c.loop
	c.fn, c.loop = fn, 0
	fn.syncs = c.syncCalls(body)
	c.push()
	for _, fl := range []*ast.FieldList{recv, typ.Params, typ.Results} {
		if fl == nil {
			continue
		}
		for _, f := range fl.List {
			for _, n := range f.Names {
				c.declare(n, f.Type, nil)
			}
		}
	}
	c.walk(body)
	c.pop()
	c.fn, c.loop = outer, outerLoop
}

// syncCalls returns where body calls each yaroze function, outside the
// function literals in it.
func (c *checker) syncCalls(body *ast.BlockStmt) map[string][]token.Pos {
	calls := make(map[string][]token.Pos)
	ast.Inspect(body, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.FuncLit:
			return false
		case *ast.CallExpr:
			if name := c.yarozeFunc(x); name != "" {
				calls[name] = append(calls[name], x.Pos())
			}
		}
		return true
	})
	return calls
}

// yarozeFunc returns the name of the yaroze function call calls, if it does.
func (c *checker) yarozeFunc(call *ast.CallExpr) string {
	if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
		if id, ok := sel.X.(*ast.Ident); ok && id.Name == c.yaroze && c.lookup(id.Name) == nil {
			return sel.Sel.Name
		}
	}
	if id, ok := call.Fun.(*ast.Ident); ok && c.yaroze == "." && c.lookup(id.Name) == nil {
		return id.Name
	}
	return ""
}

func (c *checker) children(n ast.Node) {
	ast.Inspect(n, func(m ast.Node) bool {
		if m == n {
			return true
		}
		if m != nil {
			c.walk(m)
		}
		return false
	})
}

func (c *checker) walk(n ast.Node) {
	switch x := n.(type) {
	case *ast.FuncLit:
		name := "the function literal in " + strings.TrimPrefix(c.fn.name, "the function literal in ")
		c.enter(&function{name: name, literal: true}, nil, x.Type, x.Body)
	case *ast.BlockStmt, *ast.IfStmt, *ast.SwitchStmt, *ast.TypeSwitchStmt, *ast.SelectStmt, *ast.CaseClause, *ast.CommClause:
		c.push()
		c.children(n)
		c.pop()
	case *ast.ForStmt:
		c.push()
		c.loop++
		c.children(n)
		c.loop--
		c.pop()
	case *ast.RangeStmt:
		c.walk(x.X)
		if c.isIterator(x.X) {
			// The compiler makes the body a function literal, which the
			// iterator calls each time round, and which returns each time.
			outer, outerLoop := c.fn, c.loop
			c.fn = &function{name: "the body of the loop over " + types.ExprString(x.X), literal: true}
			c.fn.syncs = c.syncCalls(x.Body)
			c.loop = 0
			c.push()
			if x.Tok == token.DEFINE {
				if id, ok := x.Key.(*ast.Ident); ok {
					c.declare(id, nil, nil)
				}
				if id, ok := x.Value.(*ast.Ident); ok {
					c.declare(id, nil, nil)
				}
			}
			c.walk(x.Body)
			c.pop()
			c.fn, c.loop = outer, outerLoop
			return
		}
		c.push()
		c.loop++
		if x.Tok == token.DEFINE {
			if id, ok := x.Key.(*ast.Ident); ok {
				c.declare(id, nil, nil)
			}
			if id, ok := x.Value.(*ast.Ident); ok {
				c.declare(id, nil, nil)
			}
		}
		c.walk(x.Body)
		c.loop--
		c.pop()
	case *ast.DeclStmt:
		c.children(n)
		if gd, ok := x.Decl.(*ast.GenDecl); ok && gd.Tok == token.VAR {
			for _, s := range gd.Specs {
				vs := s.(*ast.ValueSpec)
				for i, id := range vs.Names {
					var v ast.Expr
					if i < len(vs.Values) {
						v = vs.Values[i]
					}
					c.declare(id, vs.Type, v)
				}
			}
		}
	case *ast.AssignStmt:
		for _, r := range x.Rhs {
			c.walk(r)
		}
		for i, l := range x.Lhs {
			id, ok := l.(*ast.Ident)
			if x.Tok != token.DEFINE || !ok {
				c.walk(l)
				continue
			}
			if _, here := c.scopes[len(c.scopes)-1][id.Name]; here {
				continue // assigned, not declared
			}
			var v ast.Expr
			if len(x.Rhs) == len(x.Lhs) {
				v = x.Rhs[i]
			}
			c.declare(id, nil, v)
		}
	case *ast.CallExpr:
		c.call(x)
		c.children(n)
	default:
		c.children(n)
	}
}

// isIterator reports whether e calls a function or method that returns an
// iterator: one of the yaroze package's, or one of the package's own.
func (c *checker) isIterator(e ast.Expr) bool {
	call, ok := e.(*ast.CallExpr)
	if !ok {
		return false
	}
	if name := c.yarozeFunc(call); name != "" {
		return c.t.Iterators[name]
	}
	switch f := call.Fun.(type) {
	case *ast.Ident:
		return c.lookup(f.Name) == nil && c.iterators[f.Name]
	case *ast.SelectorExpr:
		if l := c.local(f.X); l != nil && l.typ != "" {
			return c.t.Iterators["(*"+l.typ+")."+f.Sel.Name] || c.t.Iterators["("+l.typ+")."+f.Sel.Name]
		}
		return c.iterators[f.Sel.Name]
	}
	return false
}

// call checks the arguments of a call of a yaroze function or method that
// keeps one.
func (c *checker) call(call *ast.CallExpr) {
	key, recv := "", ast.Expr(nil)
	if name := c.yarozeFunc(call); name != "" {
		key = name
	} else if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
		if l := c.local(sel.X); l != nil && l.typ != "" {
			for _, k := range []string{"(*" + l.typ + ")." + sel.Sel.Name, "(" + l.typ + ")." + sel.Sel.Name} {
				if _, ok := c.t.Keeps[k]; ok {
					key, recv = k, sel.X
					break
				}
			}
		}
	}
	for _, k := range c.t.Keeps[key] {
		var arg ast.Expr
		if k.Param < 0 {
			if l := c.local(recv); l != nil && !l.ptr {
				arg = &ast.UnaryExpr{OpPos: recv.Pos(), Op: token.AND, X: recv}
			} else {
				arg = recv
			}
		} else if k.Param < len(call.Args) {
			arg = call.Args[k.Param]
		}
		if arg == nil {
			continue
		}
		v := c.variable(arg)
		if v == nil {
			continue
		}
		what := v.name
		if what == "" {
			what = "the value made here"
		}
		if k.Until != "" {
			if v.fn != c.fn || v.fn.main || c.syncsAfter(k.Until, call.Pos()) {
				continue
			}
			c.report(call, displayName(key)+" goes on with "+k.Name+" until "+k.Until+", but "+what+" is a variable of "+v.fn.name+", which can return first: call "+k.Until+" before it does, or make "+what+" a package-level variable")
			continue
		}
		switch {
		case v.loop:
			c.report(call, displayName(key)+" keeps "+k.Name+", so "+what+", made in a loop, is allocated each time round and never freed: make it a package-level variable")
		case v.fn.literal:
			c.report(call, displayName(key)+" keeps "+k.Name+", so "+what+", made in "+v.fn.name+", which may run every frame, is allocated each time it runs and never freed: make it a package-level variable")
		}
	}
}

func (c *checker) syncsAfter(name string, pos token.Pos) bool {
	return slices.ContainsFunc(c.fn.syncs[name], func(p token.Pos) bool { return p > pos })
}

// local returns the local variable e names, if it names one.
func (c *checker) local(e ast.Expr) *local {
	for {
		p, ok := e.(*ast.ParenExpr)
		if !ok {
			break
		}
		e = p.X
	}
	if id, ok := e.(*ast.Ident); ok {
		return c.lookup(id.Name)
	}
	return nil
}

// variable returns the local variable, or the composite literal's value,
// whose memory arg points into, or nil if it is not one or cannot tell.
func (c *checker) variable(arg ast.Expr) *local {
	addressed, element := false, false
	e := arg
	for {
		switch x := e.(type) {
		case *ast.ParenExpr:
			e = x.X
			continue
		case *ast.CallExpr:
			inner, ok := c.t.through(x)
			if !ok {
				return nil
			}
			e = inner
			continue
		case *ast.UnaryExpr:
			if x.Op != token.AND {
				return nil
			}
			if _, ok := x.X.(*ast.CompositeLit); ok {
				return &local{fn: c.fn, loop: c.loop > 0}
			}
			addressed = true
			e = x.X
			continue
		case *ast.IndexExpr:
			if !addressed {
				return nil
			}
			element = true
			e = x.X
			continue
		case *ast.SelectorExpr:
			if !addressed {
				return nil
			}
			element = false
			e = x.X
			continue
		case *ast.SliceExpr:
			addressed, element = true, true
			e = x.X
			continue
		case *ast.Ident:
			l := c.lookup(x.Name)
			switch {
			case l == nil, l.fn == nil, !addressed && !l.ptr:
				return nil
			case element && !l.array && !l.ptr:
				return nil // a slice's elements are not its own memory
			}
			return l
		}
		return nil
	}
}

func (c *checker) report(n ast.Node, msg string) {
	c.problems = append(c.problems, Problem{Pos: c.fset.Position(n.Pos()), Msg: msg})
}

// displayName names a table entry as a program writes it.
func displayName(key string) string {
	if strings.HasPrefix(key, "(") {
		return key
	}
	return "yaroze." + key
}
