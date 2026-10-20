package main

import (
	"bytes"
	"encoding/json"
	"go/ast"
	"go/build"
	"go/doc"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// An APIEntry is one exported name of a package, as the playground's editor
// offers it for completion: what it is, how it is declared, and its doc
// comment as plain text.
type APIEntry struct {
	Name    string     `json:"name"`
	Kind    string     `json:"kind"` // func, type, const or var
	Sig     string     `json:"sig"`
	Doc     string     `json:"doc,omitempty"`
	Fields  []APIField `json:"fields,omitempty"`  // of a struct type
	Methods []APIEntry `json:"methods,omitempty"` // of a type
}

// An APIField is a field of a struct type.
type APIField struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// writeAPI writes the exported names of the packages in dirs, as built for
// the Net Yaroze, to out as JSON, keyed by import path, for the
// playground's completion and its hover.
func writeAPI(dirs []string, out string) error {
	api := map[string][]APIEntry{}
	for _, dir := range dirs {
		path, entries, err := packageAPI(dir)
		if err != nil {
			return err
		}
		api[path] = entries
	}
	data, err := json.Marshal(api)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		return err
	}
	return os.WriteFile(out, data, 0o644)
}

// packageAPI reads the package in dir and returns its import path and its
// exported names.
func packageAPI(dir string) (string, []APIEntry, error) {
	ctx := build.Default
	ctx.BuildTags = []string{"yaroze"}
	bp, err := ctx.ImportDir(dir, 0)
	if err != nil {
		return "", nil, err
	}
	fset := token.NewFileSet()
	var files []*ast.File
	for _, name := range bp.GoFiles {
		f, err := parser.ParseFile(fset, filepath.Join(dir, name), nil, parser.ParseComments)
		if err != nil {
			return "", nil, err
		}
		files = append(files, f)
	}
	path := importPath(dir, bp.ImportPath)
	pkg, err := doc.NewFromFiles(fset, files, path)
	if err != nil {
		return "", nil, err
	}
	return path, apiEntries(fset, pkg), nil
}

// importPath is the import path of the package in dir by the go.mod above
// it, which go/build does not read, or fallback when there is none.
func importPath(dir, fallback string) string {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return fallback
	}
	for root := abs; ; root = filepath.Dir(root) {
		data, err := os.ReadFile(filepath.Join(root, "go.mod"))
		if err == nil {
			for _, line := range strings.Split(string(data), "\n") {
				if mod, ok := strings.CutPrefix(strings.TrimSpace(line), "module "); ok {
					rel, err := filepath.Rel(root, abs)
					if err != nil {
						return fallback
					}
					return path.Join(strings.TrimSpace(mod), filepath.ToSlash(rel))
				}
			}
		}
		if filepath.Dir(root) == root {
			return fallback
		}
	}
}

// apiEntries lists a package's exported names in the order go/doc sorts
// them, with each type's constants, variables and constructors at the top
// level, where they are written in a program, and its methods under it.
func apiEntries(fset *token.FileSet, pkg *doc.Package) []APIEntry {
	var entries []APIEntry
	values := func(vs []*doc.Value) {
		for _, v := range vs {
			entries = append(entries, valueEntries(fset, v)...)
		}
	}
	funcs := func(fs []*doc.Func) {
		for _, f := range fs {
			entries = append(entries, funcEntry(fset, f))
		}
	}
	values(pkg.Consts)
	values(pkg.Vars)
	funcs(pkg.Funcs)
	for _, t := range pkg.Types {
		if !ast.IsExported(t.Name) {
			continue
		}
		e := APIEntry{Name: t.Name, Kind: "type", Doc: t.Doc}
		for _, spec := range t.Decl.Specs {
			ts, ok := spec.(*ast.TypeSpec)
			if !ok || ts.Name.Name != t.Name {
				continue
			}
			e.Sig = "type " + t.Name + " " + typeSummary(fset, ts.Type)
			if st, ok := ts.Type.(*ast.StructType); ok {
				for _, f := range st.Fields.List {
					typ := nodeString(fset, f.Type)
					for _, n := range f.Names {
						if n.IsExported() {
							e.Fields = append(e.Fields, APIField{n.Name, typ})
						}
					}
				}
			}
		}
		for _, m := range t.Methods {
			if ast.IsExported(m.Name) {
				e.Methods = append(e.Methods, funcEntry(fset, m))
			}
		}
		entries = append(entries, e)
		values(t.Consts)
		values(t.Vars)
		funcs(t.Funcs)
	}
	return entries
}

func funcEntry(fset *token.FileSet, f *doc.Func) APIEntry {
	decl := *f.Decl
	decl.Doc, decl.Body = nil, nil
	return APIEntry{Name: f.Name, Kind: "func", Sig: nodeString(fset, &decl), Doc: f.Doc}
}

// valueEntries makes an entry of each exported name in a const or var
// declaration, documented by its own comment or else the group's.
func valueEntries(fset *token.FileSet, v *doc.Value) []APIEntry {
	kind := v.Decl.Tok.String()
	var entries []APIEntry
	for _, spec := range v.Decl.Specs {
		vs := spec.(*ast.ValueSpec)
		docText := v.Doc
		if vs.Doc != nil {
			docText = vs.Doc.Text()
		} else if vs.Comment != nil && len(v.Decl.Specs) > 1 {
			docText = vs.Comment.Text()
		}
		for i, n := range vs.Names {
			if !n.IsExported() {
				continue
			}
			sig := kind + " " + n.Name
			if vs.Type != nil {
				sig += " " + nodeString(fset, vs.Type)
			}
			if i < len(vs.Values) {
				sig += " = " + nodeString(fset, vs.Values[i])
			}
			entries = append(entries, APIEntry{Name: n.Name, Kind: kind, Sig: sig, Doc: docText})
		}
	}
	return entries
}

// typeSummary is a type's underlying type as declared, with a struct or
// interface's body left out, since its fields are listed on their own.
func typeSummary(fset *token.FileSet, t ast.Expr) string {
	switch t.(type) {
	case *ast.StructType:
		return "struct"
	case *ast.InterfaceType:
		return "interface"
	}
	return nodeString(fset, t)
}

func nodeString(fset *token.FileSet, n any) string {
	var buf bytes.Buffer
	if err := printer.Fprint(&buf, fset, n); err != nil {
		return ""
	}
	return strings.Join(strings.Fields(buf.String()), " ")
}
