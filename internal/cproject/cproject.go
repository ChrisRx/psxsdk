// Package cproject reads a Net Yaroze C project as the SDK left it: C and
// assembly sources, a Makefile that names them and where they are linked,
// and the batch file called auto that told siocons what to load.
//
// The projects are DOS programs built by a GCC of 1996. Prepare writes a
// copy of the sources that a current clang on any system reads the same
// way: see prepare.go.
package cproject

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// DefaultAddr is where a program is linked when its Makefile does not say,
// the SDK's own default.
const DefaultAddr = 0x80140000

// A Load is a file siocons put in memory before the program ran.
type Load struct {
	File string
	Addr uint32
}

// A Project is what a project's files say about how to build it.
type Project struct {
	Dir      string   // the project's directory, absolute
	Sources  []string // the C and assembly files, absolute
	Defines  []string // from the Makefile's -D flags, without the -D
	Includes []string // from its -I flags, absolute
	Addr     uint32   // where the program is linked
	Loads    []Load   // from auto's local dload lines

	// Missing names the files auto loads that are not there, which a
	// program built without them is unlikely to get far without.
	Missing []string

	program string // the program auto loads, if it says
}

// Is reports whether dir holds a C project rather than a Go package: C
// sources and no Go ones.
func Is(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	var c bool
	for _, e := range entries {
		switch strings.ToLower(filepath.Ext(e.Name())) {
		case ".go":
			return false
		case ".c":
			c = true
		}
	}
	return c
}

// Read reads the project in dir.
func Read(dir string) (*Project, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	p := &Project{Dir: abs, Addr: DefaultAddr}
	if err := p.readAuto(); err != nil {
		return nil, err
	}
	objects, err := p.readMakefile()
	if err != nil {
		return nil, err
	}
	for _, o := range objects {
		src, err := source(abs, strings.TrimSuffix(o, filepath.Ext(o)))
		if err != nil {
			return nil, err
		}
		p.Sources = append(p.Sources, src)
	}
	if len(p.Sources) == 0 {
		entries, err := os.ReadDir(abs)
		if err != nil {
			return nil, err
		}
		for _, e := range entries {
			if strings.EqualFold(filepath.Ext(e.Name()), ".c") {
				p.Sources = append(p.Sources, filepath.Join(abs, e.Name()))
			}
		}
	}
	if len(p.Sources) == 0 {
		return nil, fmt.Errorf("%s has no C sources", abs)
	}
	return p, nil
}

// source finds the C or assembly file an object was made from.
func source(dir, base string) (string, error) {
	for _, ext := range []string{".c", ".s"} {
		if path, ok := find(dir, base+ext); ok {
			return path, nil
		}
	}
	return "", fmt.Errorf("the Makefile in %s names %s.o, which has no C or assembly source", dir, base)
}

var (
	makeVar   = regexp.MustCompile(`^\s*([A-Za-z_][A-Za-z0-9_]*)\s*[:+]?=\s*(.*)$`)
	makeRef   = regexp.MustCompile(`\$[({]([A-Za-z_][A-Za-z0-9_]*)[)}]`)
	linkAddr  = regexp.MustCompile(`-Ttext[\s=]+(?:-Xlinker\s+)?(?:0x)?([0-9a-fA-F]{8})`)
	objectVar = []string{"OBJS", "OBJ", "OBJECTS", "OBJFILES", "OBJ_FILES"}
)

// readMakefile takes the link address, flags and objects from the
// project's Makefile, if it has one, and returns the objects. A Makefile is
// only read, not run: the projects' build the same few ways, and their
// rules run DOS commands.
func (p *Project) readMakefile() ([]string, error) {
	var name string
	entries, err := os.ReadDir(p.Dir)
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if strings.EqualFold(e.Name(), "makefile") {
			name = e.Name()
			break
		}
		if name == "" && strings.EqualFold(filepath.Ext(e.Name()), ".mak") {
			name = e.Name()
		}
	}
	if name == "" {
		return nil, nil
	}
	data, err := os.ReadFile(filepath.Join(p.Dir, name))
	if err != nil {
		return nil, err
	}
	lines, vars := makeLines(data)
	text := strings.Join(lines, "\n")

	if m := linkAddr.FindStringSubmatch(text); m != nil {
		addr, err := strconv.ParseUint(m[1], 16, 32)
		if err != nil {
			return nil, err
		}
		p.Addr = uint32(addr)
	}
	for _, f := range strings.Fields(vars["CFLAGS"] + " " + vars["CCFLAGS"]) {
		switch {
		case strings.HasPrefix(f, "-D") && len(f) > 2:
			p.Defines = append(p.Defines, f[2:])
		case strings.HasPrefix(f, "-I") && len(f) > 2:
			if dir, ok := find(p.Dir, f[2:]); ok {
				p.Includes = append(p.Includes, dir)
			}
		}
	}

	if objects := ruleObjects(lines, p.program); objects != nil {
		return objects, nil
	}
	for _, name := range objectVar {
		if objects := objectsIn(vars[name]); objects != nil {
			return objects, nil
		}
	}
	return nil, nil
}

// ruleObjects returns the objects of the program being built: those in the
// rule for it and the command that links it. A Makefile's variable of objects
// is often only what its programs share, each rule adding its own main. The
// program is the one auto loads, or else the first thing all makes, or the
// first rule's target.
func ruleObjects(lines []string, program string) []string {
	type rule struct {
		target string
		prereq []string
		lines  []string
	}
	var rules []*rule
	for _, line := range lines {
		if strings.HasPrefix(line, "\t") {
			if len(rules) > 0 {
				rules[len(rules)-1].lines = append(rules[len(rules)-1].lines, line)
			}
			continue
		}
		target, prereq, ok := strings.Cut(line, ":")
		target = strings.TrimSpace(target)
		if !ok || target == "" || strings.ContainsAny(target, " \t") {
			continue
		}
		rules = append(rules, &rule{target: target, prereq: strings.Fields(prereq)})
	}
	byName := func(name string) *rule {
		name = strings.TrimSuffix(strings.ToLower(name), ".exe")
		for _, r := range rules {
			if strings.TrimSuffix(strings.ToLower(r.target), ".exe") == name {
				return r
			}
		}
		return nil
	}
	r := byName(program)
	if r == nil {
		if all := byName("all"); all != nil && len(all.prereq) > 0 {
			r = byName(all.prereq[0])
		}
	}
	if r == nil {
		for _, c := range rules {
			if !strings.HasPrefix(c.target, ".") && !strings.EqualFold(filepath.Ext(c.target), ".o") {
				r = c
				break
			}
		}
	}
	if r == nil {
		return nil
	}
	objects := objectsIn(strings.Join(r.prereq, " "))
	for _, line := range r.lines {
		for _, o := range objectsIn(line) {
			if !slices.Contains(objects, o) {
				objects = append(objects, o)
			}
		}
	}
	return objects
}

// makeLines splits a Makefile into its logical lines, with continuations
// joined, comments gone and variables expanded, and returns its variables
// too.
func makeLines(data []byte) ([]string, map[string]string) {
	text := strings.ReplaceAll(string(data), "\r", "")
	text = strings.ReplaceAll(text, "\\\n", " ")
	vars := make(map[string]string)
	var lines []string
	for line := range strings.SplitSeq(text, "\n") {
		line, _, _ = strings.Cut(line, "#")
		if m := makeVar.FindStringSubmatch(line); m != nil {
			if strings.Contains(line[:strings.Index(line, "=")], "+") {
				vars[m[1]] += " " + m[2]
			} else {
				vars[m[1]] = m[2]
			}
			continue
		}
		lines = append(lines, line)
	}
	expand := func(s string) string {
		for range 8 {
			next := makeRef.ReplaceAllStringFunc(s, func(ref string) string {
				return vars[makeRef.FindStringSubmatch(ref)[1]]
			})
			if next == s {
				break
			}
			s = next
		}
		return s
	}
	for k, v := range vars {
		vars[k] = expand(v)
	}
	for i, line := range lines {
		lines[i] = expand(line)
	}
	return lines, vars
}

// objectsIn returns the object files named in s.
func objectsIn(s string) []string {
	var objects []string
	for _, f := range strings.Fields(s) {
		if strings.EqualFold(filepath.Ext(f), ".o") {
			objects = append(objects, f)
		}
	}
	return objects
}

// readAuto takes the program and the files to load from the project's auto
// file, if it has one. Its lines are siocons commands, of which local load
// names the program and local dload its data.
func (p *Project) readAuto() error {
	path, ok := find(p.Dir, "auto")
	if !ok {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	scan := bufio.NewScanner(bytes.NewReader(data))
	for scan.Scan() {
		f := strings.Fields(scan.Text())
		if len(f) >= 3 && strings.EqualFold(f[0], "local") && strings.EqualFold(f[1], "load") {
			p.program = filepath.Base(strings.ReplaceAll(f[2], `\`, "/"))
			continue
		}
		if len(f) < 4 || !strings.EqualFold(f[0], "local") || !strings.EqualFold(f[1], "dload") {
			continue
		}
		addr, err := strconv.ParseUint(strings.TrimPrefix(strings.ToLower(f[3]), "0x"), 16, 32)
		if err != nil {
			return fmt.Errorf("%s: %q: %w", path, scan.Text(), err)
		}
		file, ok := find(p.Dir, f[2])
		if !ok {
			p.Missing = append(p.Missing, f[2])
			continue
		}
		p.Loads = append(p.Loads, Load{File: file, Addr: uint32(addr)})
	}
	return scan.Err()
}

// find resolves a path as DOS would, with either slash and in any case,
// relative to dir unless it is absolute. An exact match wins over one that
// differs in case.
func find(dir, path string) (string, bool) {
	path = strings.ReplaceAll(path, `\`, "/")
	cur := dir
	if strings.HasPrefix(path, "/") {
		cur = "/"
	}
	for part := range strings.SplitSeq(path, "/") {
		switch part {
		case "", ".":
			continue
		case "..":
			cur = filepath.Dir(cur)
			continue
		}
		entries, err := os.ReadDir(cur)
		if err != nil {
			return "", false
		}
		var match string
		for _, e := range entries {
			if e.Name() == part {
				match = part
				break
			}
			if match == "" && strings.EqualFold(e.Name(), part) {
				match = e.Name()
			}
		}
		if match == "" {
			return "", false
		}
		cur = filepath.Join(cur, match)
	}
	return cur, true
}
