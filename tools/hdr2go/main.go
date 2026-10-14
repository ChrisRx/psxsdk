// Command hdr2go generates Go bindings from a C header.
//
// It exists because binding libps by hand does not scale: the library has
// 172 platform entry points and a few dozen structs, and every struct has to
// match the C layout byte for byte or the library silently reads the wrong
// fields. A mistake there does not fail to compile, it draws nothing.
//
// Two clang dumps are the input, so the types and the layout come from the
// same compiler that will build the code rather than from a reading of the
// header:
//
//	clang -Xclang -ast-dump=json -fsyntax-only      structure and types
//	clang -Xclang -fdump-record-layouts -fsyntax-only   offsets and sizes
//
// Layouts are taken from the second, for the actual target, so padding the
// C compiler inserted implicitly is written out explicitly in the Go struct.
// Every size is then asserted against what clang reported, as an array
// length rather than as a test: a test could not run, since the host
// toolchain will not compile the bodyless function declarations and the
// target has nowhere to run one, whereas an assertion that is part of the
// file fails during the build that matters.
//
// Anything the type mapping does not recognise is an error rather than a
// guess. A binding that is quietly wrong is worse than one that is missing.
package main

import (
	"encoding/json"
	"fmt"
	"go/format"
	"log"
	"maps"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

// astNode is the part of clang's JSON AST this needs.
type astNode struct {
	Kind  string    `json:"kind"`
	Name  string    `json:"name"`
	Type  *astType  `json:"type"`
	Inner []astNode `json:"inner"`
	Loc   *astLoc   `json:"loc"`
	Decl  *astNode  `json:"ownedTagDecl"`
	Compl bool      `json:"completeDefinition"`
	Vari  bool      `json:"variadic"`
}

type astType struct {
	QualType  string `json:"qualType"`
	Desugared string `json:"desugaredQualType"`
}

type astLoc struct {
	File         string `json:"file"`
	Line         int    `json:"line"`
	Col          int    `json:"col"`
	IncludedFrom *struct {
		File string `json:"file"`
	} `json:"includedFrom"`
}

// A record is a C struct with the layout clang computed for the target.
type record struct {
	Name   string
	Size   int
	Fields []field
}

type field struct {
	Name   string
	Type   string // Go type
	Offset int
}

// A function is a C function to bind.
type function struct {
	Name    string
	Params  []param
	Ret     string // Go type, empty for void
	Comment string
}

type param struct {
	Name string
	Type string
}

// baseTypes maps C spellings to Go, for the o32 ABI where int and long are
// both 32 bits.
var baseTypes = map[string]string{
	"void": "",
	// char is byte rather than int8 so that strings, which is nearly all
	// this header uses char for, work with Go string handling.
	"char":               "byte",
	"signed char":        "int8",
	"unsigned char":      "uint8",
	"u_char":             "uint8",
	"short":              "int16",
	"short int":          "int16",
	"unsigned short":     "uint16",
	"unsigned short int": "uint16",
	"u_short":            "uint16",
	"int":                "int32",
	"unsigned int":       "uint32",
	"u_int":              "uint32",
	"long":               "int32",
	"long int":           "int32",
	"unsigned long":      "uint32",
	"unsigned long int":  "uint32",
	"u_long":             "uint32",
	"float":              "float32",
	"double":             "float64",
	"size_t":             "uint32",
}

// typeAliases holds the typedefs that are not struct definitions in their
// own right: a scalar name like PACKET, or a second name for a tag that is
// defined elsewhere, such as GsCOORDINATE2 for struct _GsCOORDINATE2. Each
// becomes a Go alias declaration, so the generated API spells types the way
// the C header does.
var typeAliases = map[string]string{}

// goScalars is the set of Go types baseTypes can produce, used to tell an
// alias of a scalar from an alias of a struct.
var goScalars = map[string]bool{
	"int8": true, "uint8": true, "byte": true, "int16": true, "uint16": true,
	"int32": true, "uint32": true, "float32": true, "float64": true,
}

var (
	arrayRe   = regexp.MustCompile(`^(.*?)\s*\[(\d+)\]$`)
	funcPtrRe = regexp.MustCompile(`\(\s*\*\s*\)\s*\(`)
)

// goType converts a C type spelling to Go. known holds the struct typedefs
// that have been generated, so references to them resolve.
func goType(c string, known map[string]bool) (string, error) {
	c = strings.TrimSpace(c)
	c = strings.TrimPrefix(c, "const ")
	c = strings.TrimPrefix(c, "volatile ")
	c = strings.TrimSpace(c)

	// Function pointers cannot be expressed as a Go func here, because the
	// callee is C. Pass the address instead.
	if funcPtrRe.MatchString(c) {
		return "uintptr", nil
	}

	if m := arrayRe.FindStringSubmatch(c); m != nil {
		elem, err := goType(m[1], known)
		if err != nil {
			return "", err
		}
		return "[" + m[2] + "]" + elem, nil
	}

	if strings.HasSuffix(c, "*") {
		inner := strings.TrimSpace(strings.TrimSuffix(c, "*"))
		inner = strings.TrimPrefix(inner, "const ")
		inner = strings.TrimSpace(inner)
		if inner == "void" {
			return "unsafe.Pointer", nil
		}
		elem, err := goType(inner, known)
		if err != nil {
			return "", err
		}
		if elem == "" {
			return "unsafe.Pointer", nil
		}
		return "*" + elem, nil
	}

	if g, ok := baseTypes[c]; ok {
		return g, nil
	}
	// struct tags appear as "struct Foo"
	c = strings.TrimPrefix(c, "struct ")
	if known[c] {
		return c, nil
	}
	return "", fmt.Errorf("unmapped C type %q", c)
}

// parseLayouts reads clang's record layout dump into name -> size and
// name -> field offsets, along with the set of records that use bitfields.
func parseLayouts(text string) (map[string]int, map[string]map[string]int, map[string]bool) {
	sizes := make(map[string]int)
	offsets := make(map[string]map[string]int)
	bits := make(map[string]bool)

	var cur string
	indent := -1 // indentation of this record's own fields
	sizeRe := regexp.MustCompile(`\[sizeof=(\d+)`)
	// A record opens with its name, which for the anonymous structs this
	// header is full of is a source location instead:
	//   0 | struct RECT
	//   0 | struct (unnamed at /path/libps.h:20:9)
	namedRe := regexp.MustCompile(`^\s*0 \| (?:struct|union) (\w+)\s*$`)
	anonRe := regexp.MustCompile(`^\s*0 \| (?:struct|union) \(unnamed at .*?:(\d+):(\d+)\)\s*$`)
	// "        4 |   unsigned long flg". Nested structs are expanded inline
	// at deeper indentation, so the indent is what distinguishes a record's
	// own fields from those of a struct embedded in it.
	fieldRe := regexp.MustCompile(`^\s*(\d+) \|(\s+)\S.*?(\w+)\s*$`)
	// A bitfield carries its bit range after the byte offset:
	//   "   0:0-23 |   unsigned int p"
	// Nothing below can place one, so the record it belongs to is noted
	// and reported rather than written out wrongly.
	bitRe := regexp.MustCompile(`^\s*\d+:\d+-\d+ \|`)

	for _, line := range strings.Split(text, "\n") {
		if m := anonRe.FindStringSubmatch(line); m != nil {
			cur = m[1] + ":" + m[2]
			offsets[cur] = make(map[string]int)
			indent = -1
			continue
		}
		if m := namedRe.FindStringSubmatch(line); m != nil {
			cur = m[1]
			offsets[cur] = make(map[string]int)
			indent = -1
			continue
		}
		if cur == "" {
			continue
		}
		if m := sizeRe.FindStringSubmatch(line); m != nil {
			if _, seen := sizes[cur]; !seen {
				sizes[cur], _ = strconv.Atoi(m[1])
			}
			cur = ""
			continue
		}
		if bitRe.MatchString(line) {
			bits[cur] = true
			continue
		}
		if m := fieldRe.FindStringSubmatch(line); m != nil {
			if indent < 0 {
				indent = len(m[2])
			}
			if len(m[2]) != indent {
				continue // a field of an embedded struct
			}
			off, _ := strconv.Atoi(m[1])
			offsets[cur][m[3]] = off
		}
	}
	return sizes, offsets, bits
}

// layoutKey is how a record is named in the layout dump: its tag if it has
// one, and otherwise the source position clang reports for the anonymous
// struct, which is the only thing tying it back to the AST.
func layoutKey(n *astNode) string {
	if n.Name != "" {
		return n.Name
	}
	if n.Loc != nil && n.Loc.Line != 0 {
		return strconv.Itoa(n.Loc.Line) + ":" + strconv.Itoa(n.Loc.Col)
	}
	return ""
}

// fileTracker resolves which file a declaration came from.
//
// clang only emits "file" in a location when it differs from the previous
// node's, so most declarations carry no filename at all and inherit the one
// before them. Reading each node in isolation finds almost nothing.
type fileTracker struct{ cur string }

// Only "file" is consulted. "includedFrom" names the file doing the
// including, not the one the declaration is in, so treating it as the
// location silently reattributes every declaration after the first to the
// wrong file.
func (t *fileTracker) file(n *astNode) string {
	if n.Loc != nil && n.Loc.File != "" {
		t.cur = n.Loc.File
	}
	return t.cur
}

var opts struct {
	Layouts string
	Header  string
	Package string
	Out     string
}

func run(*cobra.Command, []string) error {
	raw, err := os.ReadFile(opts.Layouts)
	if err != nil {
		return err
	}
	sizes, offsets, bitRecords := parseLayouts(string(raw))

	var root astNode
	if err := json.NewDecoder(os.Stdin).Decode(&root); err != nil {
		return fmt.Errorf("reading AST: %v", err)
	}

	// Which top-level declarations came from the header being bound. clang
	// names the file only when it changes from the previous node, so this
	// has to be resolved in order and once, before anything else looks at
	// a node.
	inHeader := make([]bool, len(root.Inner))
	var tracker fileTracker
	for i := range root.Inner {
		inHeader[i] = strings.Contains(tracker.file(&root.Inner[i]), opts.Header)
	}

	// Pair anonymous records with the typedef that names them.
	//
	// Almost every struct in this header is written "typedef struct { ... }
	// RECT;", which clang emits as a RecordDecl with no name followed by the
	// TypedefDecl that names it. Looking only at named RecordDecls finds
	// just the three declared with a tag. The layout dump keys on the
	// typedef name, so that is the name to adopt.
	named := make(map[int]string) // index of RecordDecl -> name to use
	known := make(map[string]bool)
	aliases := make(map[string]string) // scalar typedef -> Go type
	pending := -1
	for i, n := range root.Inner {
		if !inHeader[i] {
			continue
		}
		switch n.Kind {
		case "RecordDecl":
			if !n.Compl {
				continue
			}
			if n.Name != "" {
				named[i] = n.Name
				known[n.Name] = true
				pending = -1
			} else {
				pending = i
			}
		case "TypedefDecl":
			if n.Name == "" {
				continue
			}
			if pending >= 0 {
				named[pending] = n.Name
				known[n.Name] = true
				pending = -1
				continue
			}
			if n.Type == nil {
				continue
			}
			d := n.Type.Desugared
			if d == "" {
				d = n.Type.QualType
			}
			// A typedef of an already-named struct. Where the two names
			// agree ("typedef struct EXEC EXEC;") the record declaration
			// has already produced the Go type; where they differ
			// ("typedef struct _GsCOORDINATE2 GsCOORDINATE2;") the second
			// name needs an alias of its own, or every declaration using
			// it refers to a type that was never written out.
			if strings.HasPrefix(d, "struct ") && !strings.Contains(d, "*") &&
				!strings.Contains(d, "(") {
				known[n.Name] = true
				if tag := strings.TrimPrefix(d, "struct "); tag != n.Name {
					aliases[n.Name] = tag
				}
				continue
			}
			// A scalar typedef such as "typedef unsigned char PACKET".
			// Without these, every declaration mentioning one is dropped.
			if g, ok := baseTypes[d]; ok && g != "" && n.Name != d {
				known[n.Name] = true
				aliases[n.Name] = g
			}
		}
	}

	typeAliases = aliases

	var records []record
	var funcs []function
	bound := make(map[string]bool)
	var skipped, unmapped []string

	for i, n := range root.Inner {
		if !inHeader[i] {
			continue
		}
		switch n.Kind {
		case "RecordDecl":
			name, ok := named[i]
			if !ok {
				continue
			}
			key := layoutKey(&n)
			r := record{Name: name, Size: sizes[key]}
			if r.Size == 0 {
				unmapped = append(unmapped, name+": no layout for key "+key)
				continue
			}
			// Go has no bitfields, so this cannot write the struct out
			// field by field. Report it and let it be bound by hand;
			// references to it still resolve, because the hand-written
			// type keeps the C name.
			if bitRecords[key] {
				unmapped = append(unmapped, name+": uses bitfields, bind by hand")
				continue
			}
			bad := false
			for _, f := range n.Inner {
				if f.Kind != "FieldDecl" || f.Type == nil {
					continue
				}
				g, err := goType(f.Type.QualType, known)
				if err != nil || g == "" {
					unmapped = append(unmapped, r.Name+"."+f.Name+": "+f.Type.QualType)
					bad = true
					break
				}
				r.Fields = append(r.Fields, field{
					Name:   f.Name,
					Type:   g,
					Offset: offsets[key][f.Name],
				})
			}
			if !bad && len(r.Fields) > 0 {
				records = append(records, r)
			}

		case "FunctionDecl":
			if n.Name == "" || n.Type == nil {
				continue
			}
			// An empty parameter list in this vintage of C means
			// "unspecified", not "none". Several of these are variadic in
			// practice (FntPrint), and a generated zero-argument binding
			// would be actively misleading.
			if strings.Contains(n.Type.QualType, "()") || n.Vari {
				skipped = append(skipped, n.Name+" "+n.Type.QualType)
				continue
			}
			fn := function{Name: n.Name}
			retC, _, _ := strings.Cut(n.Type.QualType, "(")
			ret, err := goType(retC, known)
			if err != nil {
				unmapped = append(unmapped, n.Name+" returns "+retC)
				continue
			}
			fn.Ret = ret
			bad := false
			i := 0
			for _, p := range n.Inner {
				if p.Kind != "ParmVarDecl" || p.Type == nil {
					continue
				}
				g, err := goType(p.Type.QualType, known)
				if err != nil || g == "" {
					unmapped = append(unmapped, n.Name+" param "+p.Type.QualType)
					bad = true
					break
				}
				name := p.Name
				if name == "" {
					name = fmt.Sprintf("a%d", i)
				}
				fn.Params = append(fn.Params, param{Name: goIdent(name), Type: g})
				i++
			}
			// The header declares some of the math entry points
			// twice. Take the first of a name and ignore the rest,
			// keyed on the Go name so that two C names differing only
			// in case would also be caught.
			if !bad && !bound[exported(fn.Name)] {
				bound[exported(fn.Name)] = true
				funcs = append(funcs, fn)
			}
		}
	}

	// An alias is only worth writing if what it stands for was written.
	// One left pointing at a record that was skipped would not compile.
	for name, target := range aliases {
		if goScalars[target] {
			continue
		}
		if !slices.ContainsFunc(records, func(r record) bool { return r.Name == target }) {
			unmapped = append(unmapped, name+": alias of missing type "+target)
			delete(aliases, name)
		}
	}

	slices.SortFunc(records, func(a, b record) int { return strings.Compare(a.Name, b.Name) })
	slices.SortFunc(funcs, func(a, b function) int { return strings.Compare(a.Name, b.Name) })

	if err := writeBindings(records, aliases, funcs); err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "%d structs, %d aliases, %d functions (named=%d known=%d sizes=%d)\n", len(records), len(aliases), len(funcs), len(named), len(known), len(sizes))
	if len(skipped) > 0 {
		fmt.Fprintf(os.Stderr, "%d unprototyped or variadic, bind by hand:\n", len(skipped))
		for _, s := range skipped {
			fmt.Fprintf(os.Stderr, "  %s\n", s)
		}
	}
	if len(unmapped) > 0 {
		fmt.Fprintf(os.Stderr, "%d declarations with unmapped types, skipped:\n", len(unmapped))
		for _, s := range unmapped {
			fmt.Fprintf(os.Stderr, "  %s\n", s)
		}
	}
	return nil
}

// goIdent avoids colliding with Go keywords.
var keywords = map[string]bool{
	"type": true, "func": true, "range": true, "len": true, "cap": true,
	"map": true, "chan": true, "go": true, "select": true, "var": true,
	"const": true, "interface": true, "struct": true, "package": true,
	"import": true, "return": true, "defer": true, "switch": true,
	"case": true, "default": true, "for": true, "if": true, "else": true,
	"new": true, "copy": true, "close": true, "delete": true, "append": true,
}

func goIdent(s string) string {
	if keywords[s] {
		return s + "_"
	}
	return s
}

func writeBindings(records []record, aliases map[string]string, funcs []function) error {
	var b strings.Builder
	fmt.Fprintf(&b, "// Code generated by hdr2go from %s. DO NOT EDIT.\n", opts.Header)
	fmt.Fprintf(&b, "//\n")
	fmt.Fprintf(&b, "// Struct layouts come from clang's record layout dump for the target, so\n")
	fmt.Fprintf(&b, "// the padding C inserted implicitly is written out here explicitly, and\n")
	fmt.Fprintf(&b, "// every size is asserted at the bottom of the file against the value\n")
	fmt.Fprintf(&b, "// clang reported.\n")
	fmt.Fprintf(&b, "//\n")
	fmt.Fprintf(&b, "// Functions are bodyless declarations carrying //export, which in TinyGo\n")
	fmt.Fprintf(&b, "// imports a C symbol rather than exporting a Go one.\n\n")
	fmt.Fprintf(&b, "package %s\n\n", opts.Package)
	fmt.Fprintf(&b, "import \"unsafe\"\n\n")
	fmt.Fprintf(&b, "var _ unsafe.Pointer\n\n")

	for _, r := range records {
		fmt.Fprintf(&b, "// %s is the C struct of the same name, %d bytes.\n", r.Name, r.Size)
		fmt.Fprintf(&b, "type %s struct {\n", r.Name)
		pos := 0
		pad := 0
		for _, f := range r.Fields {
			if f.Offset > pos {
				fmt.Fprintf(&b, "\t_pad%d [%d]byte // implicit padding in C\n", pad, f.Offset-pos)
				pad++
				pos = f.Offset
			}
			fmt.Fprintf(&b, "\t%s %s\n", exported(f.Name), f.Type)
			pos = f.Offset + sizeOfGo(f.Type, records)
		}
		if r.Size > pos {
			fmt.Fprintf(&b, "\t_pad%d [%d]byte // tail padding in C\n", pad, r.Size-pos)
		}
		fmt.Fprintf(&b, "}\n\n")
	}

	for _, name := range slices.Sorted(maps.Keys(aliases)) {
		fmt.Fprintf(&b, "// %s is the C typedef of the same name.\n", name)
		fmt.Fprintf(&b, "type %s = %s\n\n", name, aliases[name])
	}

	for _, f := range funcs {
		var ps []string
		for _, p := range f.Params {
			ps = append(ps, p.Name+" "+p.Type)
		}
		// The Go name is capitalised and the C name stays on the
		// //export line, which is what actually names the symbol. Most
		// of the library already starts with a capital; the exceptions
		// are the libc and math entry points, and leaving those as
		// written would put them out of reach of any other package and
		// would shadow the builtins named close and delete.
		fmt.Fprintf(&b, "//export %s\n", f.Name)
		fmt.Fprintf(&b, "func %s(%s)", exported(f.Name), strings.Join(ps, ", "))
		if f.Ret != "" {
			fmt.Fprintf(&b, " %s", f.Ret)
		}
		fmt.Fprintf(&b, "\n\n")
	}

	// Size assertions, as array lengths rather than as a test, because a
	// test cannot run: the host toolchain will not compile bodyless
	// functions, and the target has nowhere to run one. An array of
	// negative length is a compile error, so the pair below fails unless
	// the Go struct is exactly the size clang computed, and it fails
	// during the build that matters rather than in a separate step.
	fmt.Fprintf(&b, "// Layout assertions. Each pair fails to compile unless the Go struct is\n")
	fmt.Fprintf(&b, "// the size clang computed for the target.\n")
	for _, r := range records {
		fmt.Fprintf(&b, "var _ [unsafe.Sizeof(%s{}) - %d]byte\n", r.Name, r.Size)
		fmt.Fprintf(&b, "var _ [%d - unsafe.Sizeof(%s{})]byte\n", r.Size, r.Name)
	}

	src, err := format.Source([]byte(b.String()))
	if err != nil {
		// Write it out anyway so the mistake can be read.
		os.WriteFile(opts.Out, []byte(b.String()), 0o644)
		return fmt.Errorf("generated code does not parse: %v", err)
	}
	return os.WriteFile(opts.Out, src, 0o644)
}

// sizeOfGo is the size of a generated Go type, used only to find the gaps
// that need explicit padding.
func sizeOfGo(t string, records []record) int {
	if m := arrayRe.FindStringSubmatch(t); m != nil {
		n, _ := strconv.Atoi(m[2])
		return n * sizeOfGo(m[1], records)
	}
	if strings.HasPrefix(t, "[") {
		// [N]elem
		i := strings.Index(t, "]")
		n, _ := strconv.Atoi(t[1:i])
		return n * sizeOfGo(t[i+1:], records)
	}
	if a, ok := typeAliases[t]; ok {
		return sizeOfGo(a, records)
	}
	switch t {
	case "int8", "uint8", "byte":
		return 1
	case "int16", "uint16":
		return 2
	case "int32", "uint32", "float32", "uintptr", "unsafe.Pointer":
		return 4
	case "float64":
		return 8
	}
	if strings.HasPrefix(t, "*") {
		return 4
	}
	for _, r := range records {
		if r.Name == t {
			return r.Size
		}
	}
	return 4
}

func exported(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

func main() {
	cmd := &cobra.Command{
		Use:          "hdr2go [flags] < ast.json",
		Short:        "generate Go bindings from a C header",
		Args:         cobra.NoArgs,
		RunE:         run,
		SilenceUsage: true,
	}
	f := cmd.Flags()
	f.StringVar(&opts.Layouts, "layouts", "", "clang -fdump-record-layouts output")
	f.StringVar(&opts.Header, "header", "libps.h", "only bind declarations from this header")
	f.StringVar(&opts.Package, "package", "libps", "package name to generate")
	f.StringVar(&opts.Out, "out", "zz_generated.go", "file to write")
	cmd.MarkFlagRequired("layouts")

	if err := cmd.Execute(); err != nil {
		log.Fatal(err)
	}
}
