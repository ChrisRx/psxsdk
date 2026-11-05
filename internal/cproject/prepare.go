package cproject

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Prepare writes a copy of the project's sources, and everything they
// include, under dst, and returns the sources' new paths, in order. The
// copies differ from the originals in what a DOS compiler took for granted
// and a current one does not:
//
//   - The 0x1A that ends a DOS text file is gone, and line ends are LF.
//   - Includes name files as they are spelled on disk, with forward
//     slashes, since DOS did not care about either. A system include that
//     differs only in case from one in system becomes that one.
//   - The second byte of a Shift-JIS character can be a backslash. In a
//     string it is written so that it stays that byte, and in a comment it
//     is dropped: a // comment ending in one would otherwise take the next
//     line of code with it.
//   - A string may run over several lines, as GCC then let it, each line
//     end being part of the string.
//
// system reports whether a header of that name, in lower case, is one the
// compiler's own include directory has.
func (p *Project) Prepare(dst string, system func(string) bool) ([]string, error) {
	pr := preparer{p: p, dst: dst, system: system, done: make(map[string]string)}
	sources := make([]string, len(p.Sources))
	for i, src := range p.Sources {
		out, err := pr.file(src)
		if err != nil {
			return nil, err
		}
		sources[i] = out
	}
	return sources, nil
}

type preparer struct {
	p      *Project
	dst    string
	system func(string) bool
	done   map[string]string // from an original to its copy
}

// mirror is where the copy of the file at path goes. Files outside the
// project, reached with .., go under directories named for that.
func (pr *preparer) mirror(path string) string {
	rel, err := filepath.Rel(pr.p.Dir, path)
	if err != nil {
		rel = filepath.Base(path)
	}
	rel = strings.ReplaceAll(rel, "..", "up")
	return filepath.Join(pr.dst, rel)
}

var includeLine = regexp.MustCompile(`^(\s*#\s*include\s*)(<[^>\n]+>|"[^"\n]+")(.*)$`)

func (pr *preparer) file(path string) (string, error) {
	if out, ok := pr.done[path]; ok {
		return out, nil
	}
	out := pr.mirror(path)
	pr.done[path] = out
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(out), 0o777); err != nil {
		return "", err
	}
	lines := strings.Split(string(clean(data)), "\n")
	for i, line := range lines {
		m := includeLine.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		spec, err := pr.include(path, out, m[2])
		if err != nil {
			return "", err
		}
		lines[i] = m[1] + spec + m[3]
	}
	return out, os.WriteFile(out, []byte(strings.Join(lines, "\n")), 0o666)
}

// include resolves an include in the file at from, whose copy is at out,
// copying what it names, and returns how the copy should spell it. One it
// cannot find it leaves for the compiler to report, except for its slashes.
func (pr *preparer) include(from, out, spec string) (string, error) {
	quoted := spec[0] == '"'
	name := strings.ReplaceAll(spec[1:len(spec)-1], `\`, "/")

	dirs := append([]string{filepath.Dir(from)}, pr.p.Includes...)
	if !quoted {
		// A system include is the compiler's own unless the project has it
		// and the compiler does not: <pad.h>, with -I. in a Makefile.
		if pr.system(strings.ToLower(name)) {
			return "<" + strings.ToLower(name) + ">", nil
		}
	}
	for _, dir := range dirs {
		path, ok := find(dir, name)
		if !ok {
			continue
		}
		if fi, err := os.Stat(path); err != nil || fi.IsDir() {
			continue
		}
		copied, err := pr.file(path)
		if err != nil {
			return "", err
		}
		rel, err := filepath.Rel(filepath.Dir(out), copied)
		if err != nil {
			return "", fmt.Errorf("%s: %w", from, err)
		}
		return `"` + filepath.ToSlash(rel) + `"`, nil
	}
	if pr.system(strings.ToLower(name)) {
		return "<" + strings.ToLower(name) + ">", nil
	}
	return string(spec[0]) + name + string(spec[len(spec)-1]), nil
}

// clean does what can be done to a source file without knowing what it
// includes: line ends, the DOS end of file, and Shift-JIS backslashes.
func clean(data []byte) []byte {
	const (
		code = iota
		quote
		lineComment
		blockComment
	)
	out := make([]byte, 0, len(data))
	state := code
	var delim byte
	directive, blank := false, true // on a line of #, and nothing yet on this one
	for i := 0; i < len(data); i++ {
		c := data[i]
		switch c {
		case 0x1a:
			continue
		case '\r':
			if i+1 < len(data) && data[i+1] == '\n' {
				continue
			}
		}
		next := byte(0)
		if i+1 < len(data) {
			next = data[i+1]
		}
		switch state {
		case code:
			switch c {
			case '\n':
				directive, blank = false, true
			case ' ', '\t':
			case '#':
				directive = directive || blank
				blank = false
			default:
				blank = false
			}
			switch {
			case c == '"' || c == '\'':
				state, delim = quote, c
			case c == '/' && next == '/':
				state = lineComment
			case c == '/' && next == '*':
				state = blockComment
				out = append(out, c, next)
				i++
				continue
			}
		case quote:
			switch {
			case sjisLead(c) && sjisTrail(next):
				out = append(out, c)
				if next == '\\' {
					out = append(out, '\\')
				}
				out = append(out, next)
				i++
				continue
			case c == '\\':
				out = append(out, c, next)
				i++
				continue
			case c == '\n' && delim == '"' && !directive:
				out = append(out, '\\', 'n', '\\')
			case c == delim || c == '\n':
				// An apostrophe in a #error or between #if 0 ends at its line.
				state = code
				if c == '\n' {
					directive, blank = false, true
				}
			}
		case lineComment, blockComment:
			switch {
			case sjisLead(c) && sjisTrail(next):
				out = append(out, c)
				if next == '\\' {
					next = ' '
				}
				out = append(out, next)
				i++
				continue
			case state == lineComment && c == '\\' && (next == '\n' || next == '\r'):
				// Continues the comment, as written: left alone.
			case state == lineComment && c == '\n':
				state = code
				directive, blank = false, true
			case state == blockComment && c == '*' && next == '/':
				out = append(out, c, next)
				i++
				state = code
				continue
			}
		}
		out = append(out, c)
	}
	return out
}

func sjisLead(c byte) bool { return c >= 0x81 && c <= 0x9f || c >= 0xe0 && c <= 0xfc }

func sjisTrail(c byte) bool { return c >= 0x40 && c <= 0xfc && c != 0x7f }
