// Command site builds the documentation site: the pages under docs/, a page
// for each example from its own package comment, and the playground and the
// player, which are web/static, under play/: the playground is its index,
// and the player, which the example pages frame, is play/player.html.
//
// An example's package comment is all there is to say about it, and its
// page is laid out from the comment's parts: the first paragraph, one
// sentence, is what the example is, shown under its name in the list of
// examples and under the heading on its page; a "# Controls" heading's
// section is what the controller does in it, shown under the player; and
// the rest is the page's text. The examples' index page orders them, by an
// order line in its front matter naming the ones that come first.
//
// There is no framework behind it. A page is a Markdown file with a short
// front matter, rendered with goldmark and chroma into one template, and the
// site's sections are the directories under docs/. Every link the output
// holds is relative, so the same build serves from the root of a host and
// from the directory a pull request preview is published under.
//
// Links in a page are written as paths and rewritten on the way out:
//
//   - to another page, by its source file: "../tools/mkpsxiso.md"
//   - to somewhere on the site outside the pages, from the root: "/play/"
//   - to a file beside the page, such as a picture, which is copied through
//   - to anything else in the repository, by its path from the root there:
//     "examples/check", the same on every page
//
// The playground starts a project from one of the examples, square or
// scene, whose sources are copied under play/starters and listed in
// play/starters.json, so that the program a newcomer first sees is one that
// builds as an example does and cannot drift from the package; the list
// names each example, for the playground to add its data files from the
// example's beside it.
//
// task site runs it after building the examples' discs and the emulator,
// then puts those under play/ beside the player.
package main

import (
	"bytes"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"go/ast"
	"go/doc/comment"
	"go/format"
	"go/parser"
	"go/token"
	"html/template"
	"io"
	"io/fs"
	"log"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"

	chromahtml "github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/alecthomas/chroma/v2/styles"
	"github.com/yuin/goldmark"
	highlighting "github.com/yuin/goldmark-highlighting/v2"
	gast "github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	mdparser "github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

//go:embed templates assets
var files embed.FS

var opts = struct {
	Docs     string
	Examples string
	Static   string
	Starters string
	API      string
	Out      string
	Repo     string
	Name     string
}{
	Docs:     "docs",
	Examples: "examples",
	Static:   "web/static",
	Starters: "square,scene",
	API:      "yaroze,scene,disc,arc",
	Out:      ".build/site",
	Repo:     "https://github.com/ChrisRx/psxsdk",
	Name:     "psxsdk",
}

func main() {
	flag.StringVar(&opts.Docs, "docs", opts.Docs, "directory of pages")
	flag.StringVar(&opts.Examples, "examples", opts.Examples, "directory of example programs, one page each")
	flag.StringVar(&opts.Static, "static", opts.Static, "the playground and the player, copied under play/")
	flag.StringVar(&opts.Starters, "starters", opts.Starters, "the examples the playground starts a project from, copied under play/starters")
	flag.StringVar(&opts.API, "api", opts.API, "packages whose exported names the playground completes and explains, written to play/api.json")
	flag.StringVar(&opts.Out, "out", opts.Out, "directory to write the site into")
	flag.StringVar(&opts.Repo, "repo", opts.Repo, "repository that paths in links refer to")
	flag.StringVar(&opts.Name, "name", opts.Name, "the site's name")
	flag.Parse()

	s, err := load(opts.Docs, opts.Examples)
	if err != nil {
		log.Fatal(err)
	}
	if err := s.write(opts.Out, opts.Static); err != nil {
		log.Fatal(err)
	}
	if opts.Static != "" && opts.Starters != "" {
		if err := writeStarters(opts.Examples, strings.Split(opts.Starters, ","), filepath.Join(opts.Out, "play")); err != nil {
			log.Fatal(err)
		}
	}
	if opts.Static != "" && opts.API != "" {
		if err := writeAPI(strings.Split(opts.API, ","), filepath.Join(opts.Out, "play", "api.json")); err != nil {
			log.Fatal(err)
		}
	}
	fmt.Printf("wrote %d pages to %s\n", len(s.pages), opts.Out)
}

// A Site is the pages and the sections they fall into.
type Site struct {
	Name     string
	Repo     string
	Sections []*Section
	Groups   []*Group
	Nav      []Tab // the header's links, to sections and groups, in order
	docs     string
	pages    []*Page
	assets   map[string]string // files beside pages to copy, output path to source
}

// A Section is a directory of pages under docs/, named by its index page.
type Section struct {
	Name   string // the directory
	Title  string
	URL    string // from the root, with a trailing slash
	Index  *Page
	Pages  []*Page // the index first, then by its order, weight and title
	Group  *Group  // the group the section is in, if any
	weight int

	// Described lists the section's pages with their descriptions, in the
	// sidebar and on the index, as the examples are.
	Described bool
}

// A Group is sections that share one link in the header, named by the
// group key of their indexes, and a page of its own listing them, at the
// group's name in lower case. A page in one has the group's sections down
// its side, with its own section's pages under it.
type Group struct {
	Title    string
	URL      string
	Sections []*Section
	Page     *Page
}

// A Tab is a link in the header: a section's, or a group's.
type Tab struct {
	Title   string
	URL     string
	Section *Section
	Group   *Group
}

// A Page is one Markdown file, or one example, rendered.
type Page struct {
	Title       string
	Description string
	Weight      int
	Section     *Section
	Group       *Group // for a group's page, the group it lists
	URL         string // from the root, with a trailing slash; "" for the front page
	Content     template.HTML
	TOC         []Heading
	Text        string        // the body as plain text, for search
	Embed       string        // an example to show the player for
	Controls    template.HTML // what the controller does, for an example
	Source      string        // a path in the repository the page documents
	section     string        // for an index, the section's name when not the title
	group       string        // for an index, the group its section is in
	order       []string      // for an index, the pages that come first, by name

	src   string // the Markdown file, relative to docs/, or "" when generated
	index bool
}

// A Heading is an entry in a page's table of contents.
type Heading struct {
	Level int
	ID    string
	Text  string
}

// Docs is the group the page is in, as a page of one of its sections or
// as the group's own page, or nil.
func (p *Page) Docs() *Group {
	if p.Group != nil {
		return p.Group
	}
	if p.Section != nil {
		return p.Section.Group
	}
	return nil
}

// Under is whether the page is under the header's link t.
func (p *Page) Under(t Tab) bool {
	if t.Group != nil {
		return p.Docs() == t.Group
	}
	return p.Section != nil && p.Section == t.Section
}

// Root is the path from the page's directory back to the site's root, which
// every link in the template is written from.
func (p *Page) Root() string {
	return strings.Repeat("../", strings.Count(p.URL, "/"))
}

// load reads the pages under docs and makes one for each example.
func load(docs, examples string) (*Site, error) {
	s := &Site{Name: opts.Name, Repo: opts.Repo, docs: docs, assets: map[string]string{}}
	sections := map[string]*Section{}

	err := filepath.WalkDir(docs, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(p, ".md") {
			return nil
		}
		rel, _ := filepath.Rel(docs, p)
		rel = filepath.ToSlash(rel)
		page, err := s.readPage(rel)
		if err != nil {
			return fmt.Errorf("%s: %w", p, err)
		}
		if dir := path.Dir(rel); dir != "." {
			name := strings.SplitN(dir, "/", 2)[0]
			sec := sections[name]
			if sec == nil {
				sec = &Section{Name: name, URL: name + "/"}
				sections[name] = sec
				s.Sections = append(s.Sections, sec)
			}
			page.Section = sec
			if page.index && dir == name {
				sec.Index = page
				sec.Title = page.Title
				if page.section != "" {
					sec.Title = page.section
				}
				sec.weight = page.Weight
			}
		}
		s.pages = append(s.pages, page)
		return nil
	})
	if err != nil {
		return nil, err
	}

	if err := s.addExamples(examples, sections); err != nil {
		return nil, err
	}

	for _, sec := range s.Sections {
		if sec.Index == nil {
			return nil, fmt.Errorf("docs/%s has no index.md to name the section", sec.Name)
		}
	}
	sort.SliceStable(s.Sections, func(i, j int) bool {
		a, b := s.Sections[i], s.Sections[j]
		if a.weight != b.weight {
			return a.weight < b.weight
		}
		return a.Title < b.Title
	})
	if err := s.group(); err != nil {
		return nil, err
	}
	for _, p := range s.pages {
		if p.Section != nil {
			p.Section.Pages = append(p.Section.Pages, p)
		}
	}
	for _, sec := range s.Sections {
		// The pages the index's order names come first, in that order.
		rank := map[string]int{}
		for i, name := range sec.Index.order {
			if !slices.ContainsFunc(sec.Pages, func(p *Page) bool { return !p.index && path.Base(p.URL) == name }) {
				return nil, fmt.Errorf("docs/%s/index.md orders %q, which is not one of its pages", sec.Name, name)
			}
			rank[name] = i + 1
		}
		first := func(p *Page) int {
			if r, ok := rank[path.Base(p.URL)]; ok {
				return r
			}
			return len(rank) + 1
		}
		sort.SliceStable(sec.Pages, func(i, j int) bool {
			a, b := sec.Pages[i], sec.Pages[j]
			if a.index != b.index {
				return a.index
			}
			if ra, rb := first(a), first(b); ra != rb {
				return ra < rb
			}
			if a.Weight != b.Weight {
				return a.Weight < b.Weight
			}
			return a.Title < b.Title
		})
	}
	return s, nil
}

// group puts the sections whose indexes name a group into it, makes each
// group's page, and lays out the header: a group's link goes where its first
// section's would.
func (s *Site) group() error {
	groups := map[string]*Group{}
	for _, sec := range s.Sections {
		name := sec.Index.group
		if name == "" {
			s.Nav = append(s.Nav, Tab{Title: sec.Title, URL: sec.URL, Section: sec})
			continue
		}
		g := groups[name]
		if g == nil {
			g = &Group{Title: name, URL: strings.ToLower(name) + "/"}
			if i := slices.IndexFunc(s.pages, func(p *Page) bool { return p.URL == g.URL }); i >= 0 {
				return fmt.Errorf("group %s's page would be at %s, where docs/%s is", name, g.URL, s.pages[i].src)
			}
			g.Page = &Page{Title: name, URL: g.URL, Group: g}
			groups[name] = g
			s.Groups = append(s.Groups, g)
			s.pages = append(s.pages, g.Page)
			s.Nav = append(s.Nav, Tab{Title: name, URL: g.URL, Group: g})
		}
		g.Sections = append(g.Sections, sec)
		sec.Group = g
	}
	for _, g := range s.Groups {
		var text strings.Builder
		for _, sec := range g.Sections {
			fmt.Fprintf(&text, "%s %s\n", sec.Title, sec.Index.Description)
		}
		g.Page.Text = text.String()
	}
	return nil
}

// splitList splits a front matter value of names separated by commas.
func splitList(v string) []string {
	var out []string
	for name := range strings.SplitSeq(v, ",") {
		if name = strings.TrimSpace(name); name != "" {
			out = append(out, name)
		}
	}
	return out
}

// readPage renders the Markdown file rel, under docs/.
func (s *Site) readPage(rel string) (*Page, error) {
	src, err := os.ReadFile(filepath.Join(s.docs, filepath.FromSlash(rel)))
	if err != nil {
		return nil, err
	}
	meta, body, err := frontMatter(src)
	if err != nil {
		return nil, err
	}
	p := &Page{
		Title:       meta["title"],
		Description: meta["description"],
		Embed:       meta["embed"],
		section:     meta["section"],
		group:       meta["group"],
		order:       splitList(meta["order"]),
		src:         rel,
		index:       path.Base(rel) == "index.md",
	}
	if p.Title == "" {
		return nil, errors.New("no title in the front matter")
	}
	if w := meta["weight"]; w != "" {
		if p.Weight, err = strconv.Atoi(w); err != nil {
			return nil, fmt.Errorf("weight %q is not a number", w)
		}
	}
	if p.index {
		p.URL = path.Dir(rel) + "/"
		if p.URL == "./" {
			p.URL = ""
		}
	} else {
		p.URL = strings.TrimSuffix(rel, ".md") + "/"
	}
	if err := s.render(p, body); err != nil {
		return nil, err
	}
	return p, nil
}

// frontMatter splits the "key: value" lines between the leading "---" lines
// from the body. The keys are title, which every page needs, weight, which
// orders pages in a section and sections in the header, description, embed,
// which names an example to show the player for, and on an index, section,
// which names the section when its page's title would not, group, which
// puts the section in a group of that name, and order, the names of the
// section's pages to put first, separated by commas.
func frontMatter(src []byte) (map[string]string, []byte, error) {
	meta := map[string]string{}
	if !bytes.HasPrefix(src, []byte("---\n")) {
		return meta, src, nil
	}
	rest := src[4:]
	end := bytes.Index(rest, []byte("\n---\n"))
	if end < 0 {
		return nil, nil, errors.New("front matter is not closed")
	}
	for line := range strings.SplitSeq(string(rest[:end]), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		k, v, ok := strings.Cut(line, ":")
		if !ok {
			return nil, nil, fmt.Errorf("front matter line %q is not key: value", line)
		}
		meta[strings.TrimSpace(k)] = strings.TrimSpace(v)
	}
	return meta, rest[end+5:], nil
}

// expand resolves what a page pulls in from outside its own text, before it
// is parsed as Markdown:
//
//   - an empty fenced code block whose info string names a file relative to
//     the page, "```go src=square/main.go", is filled with that file, so
//     that a program a page shows is the one that is built and tested; with
//     names after the path, "src=square/main.go#main,setUp", it is filled
//     with those declarations of the Go file alone, each with its comment,
//     a function or method, or a var, const or type, out of its group if it
//     is in one
//   - a line "<!-- player NAME -->" becomes the player, running the disc
//     built for NAME, as an example's page has above its text
func (s *Site) expand(p *Page, body []byte) ([]byte, error) {
	var out bytes.Buffer
	lines := strings.Split(string(body), "\n")
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		if name, ok := strings.CutPrefix(line, "<!-- player "); ok && strings.HasSuffix(name, " -->") {
			out.WriteString(playerHTML(p.Root(), strings.TrimSuffix(name, " -->")))
			out.WriteByte('\n')
			continue
		}
		if lang, src, ok := strings.Cut(line, " src="); ok && strings.HasPrefix(lang, "```") && i+1 < len(lines) && lines[i+1] == "```" {
			if p.src == "" {
				return nil, fmt.Errorf("src= in a page that is not a file")
			}
			file, names, _ := strings.Cut(src, "#")
			rel := path.Join(path.Dir(p.src), file)
			data, err := os.ReadFile(filepath.Join(s.docs, filepath.FromSlash(rel)))
			if err == nil && names != "" {
				data, err = declarations(data, splitList(names))
			}
			if err != nil {
				return nil, fmt.Errorf("including %s: %w", src, err)
			}
			out.WriteString(lang + "\n")
			out.Write(bytes.TrimRight(data, "\n"))
			out.WriteString("\n```\n")
			i++
			continue
		}
		out.WriteString(line)
		out.WriteByte('\n')
	}
	return out.Bytes(), nil
}

// declarations returns the declarations of names in the Go source src, in
// the order named, separated by blank lines. A function is named by its
// name and a method by its receiver's type and its name, "Screen.Open". A
// var, const or type in a parenthesized group is taken out of the group,
// with the keyword put in front of it.
func declarations(src []byte, names []string) ([]byte, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "", src, parser.ParseComments|parser.SkipObjectResolution)
	if err != nil {
		return nil, err
	}
	text := func(from, to token.Pos) string {
		return string(src[fset.Position(from).Offset:fset.Position(to).Offset])
	}
	// start is where a node's text begins, its comment if it has one.
	start := func(doc *ast.CommentGroup, n ast.Node) token.Pos {
		if doc != nil {
			return doc.Pos()
		}
		return n.Pos()
	}
	find := func(name string) (string, bool) {
		for _, d := range f.Decls {
			switch d := d.(type) {
			case *ast.FuncDecl:
				if funcName(d) == name {
					return text(start(d.Doc, d), d.End()), true
				}
			case *ast.GenDecl:
				for _, spec := range d.Specs {
					var doc *ast.CommentGroup
					var idents []*ast.Ident
					switch spec := spec.(type) {
					case *ast.ValueSpec:
						doc, idents = spec.Doc, spec.Names
					case *ast.TypeSpec:
						doc, idents = spec.Doc, []*ast.Ident{spec.Name}
					}
					if !slices.ContainsFunc(idents, func(id *ast.Ident) bool { return id.Name == name }) {
						continue
					}
					if !d.Lparen.IsValid() {
						return text(start(d.Doc, d), d.End()), true
					}
					// Out of its group: one tab less indented, with its
					// keyword between its comment and itself, and
					// formatted again, since the group aligned it with
					// its neighbors.
					var b strings.Builder
					if doc != nil {
						b.WriteString(text(doc.Pos(), spec.Pos()))
					}
					b.WriteString(d.Tok.String() + " " + text(spec.Pos(), spec.End()))
					decl := strings.ReplaceAll(b.String(), "\n\t", "\n")
					if formatted, err := format.Source([]byte(decl)); err == nil {
						decl = string(formatted)
					}
					return decl, true
				}
			}
		}
		return "", false
	}
	var out []string
	for _, name := range names {
		decl, ok := find(name)
		if !ok {
			return nil, fmt.Errorf("no declaration of %s", name)
		}
		out = append(out, decl)
	}
	return []byte(strings.Join(out, "\n\n")), nil
}

// funcName is a function's name, or a method's receiver type and name.
func funcName(d *ast.FuncDecl) string {
	if d.Recv == nil || len(d.Recv.List) == 0 {
		return d.Name.Name
	}
	t := d.Recv.List[0].Type
	if star, ok := t.(*ast.StarExpr); ok {
		t = star.X
	}
	if idx, ok := t.(*ast.IndexExpr); ok {
		t = idx.X
	}
	if id, ok := t.(*ast.Ident); ok {
		return id.Name + "." + d.Name.Name
	}
	return d.Name.Name
}

// playerHTML is the player framed in a page, booted into the disc built
// for name, which the player takes from its hash.
func playerHTML(root, name string) string {
	return fmt.Sprintf(`<iframe class="player" src="%[1]splay/player.html#%[2]s" title="%[2]s, running in the emulator" allow="fullscreen; gamepad" allowfullscreen scrolling="no" loading="lazy"></iframe>`, root, name)
}

// render turns a page's Markdown into its content, table of contents and
// search text.
func (s *Site) render(p *Page, body []byte) error {
	body, err := s.expand(p, body)
	if err != nil {
		return err
	}
	links := &linker{site: s, page: p}
	md := goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,
			highlighting.NewHighlighting(
				highlighting.WithFormatOptions(chromahtml.WithClasses(true)),
			),
		),
		goldmark.WithParserOptions(
			mdparser.WithAutoHeadingID(),
			mdparser.WithASTTransformers(util.Prioritized(links, 100)),
		),
		goldmark.WithRendererOptions(html.WithUnsafe()),
	)
	doc := md.Parser().Parse(text.NewReader(body))
	if links.err != nil {
		return links.err
	}

	var plain strings.Builder
	err = gast.Walk(doc, func(n gast.Node, entering bool) (gast.WalkStatus, error) {
		if !entering {
			return gast.WalkContinue, nil
		}
		switch n := n.(type) {
		case *gast.Heading:
			if n.Level == 2 || n.Level == 3 {
				id, _ := n.AttributeString("id")
				p.TOC = append(p.TOC, Heading{Level: n.Level, ID: string(id.([]byte)), Text: string(n.Text(body))})
			}
			plain.WriteByte(' ')
		case *gast.Text:
			plain.Write(n.Segment.Value(body))
			if n.SoftLineBreak() || n.HardLineBreak() {
				plain.WriteByte(' ')
			}
		case *gast.FencedCodeBlock, *gast.CodeBlock:
			// Code is searchable too: a function's name is what someone
			// looking for how it is used types.
			lines := n.Lines()
			for i := 0; i < lines.Len(); i++ {
				seg := lines.At(i)
				plain.Write(seg.Value(body))
				plain.WriteByte(' ')
			}
		case *gast.Paragraph, *gast.ListItem, *gast.CodeSpan:
			plain.WriteByte(' ')
		}
		return gast.WalkContinue, nil
	})
	if err != nil {
		return err
	}
	p.Text = strings.Join(strings.Fields(plain.String()), " ")

	var out bytes.Buffer
	if err := md.Renderer().Render(&out, body, doc); err != nil {
		return err
	}
	p.Content = anchored(out.String())
	return nil
}

// headingID matches a heading with an id, as goldmark and the comment
// printer both write them, with its text.
var headingID = regexp.MustCompile(`(?s)<h([2-4]) id="([^"]+)">(.*?)</h[2-4]>`)

// anchored gives every section heading in rendered HTML a link to itself,
// shown when the heading is hovered, so that a section's address can be
// taken from the page.
func anchored(html string) template.HTML {
	return template.HTML(headingID.ReplaceAllString(html,
		`<h$1 id="$2">$3<a class="anchor" href="#$2" aria-label="Link to this section">#</a></h$1>`))
}

// A linker rewrites the links in a page as the package comment describes.
type linker struct {
	site *Site
	page *Page
	err  error
}

func (l *linker) Transform(doc *gast.Document, reader text.Reader, pc mdparser.Context) {
	gast.Walk(doc, func(n gast.Node, entering bool) (gast.WalkStatus, error) {
		if !entering {
			return gast.WalkContinue, nil
		}
		switch n := n.(type) {
		case *gast.Link:
			n.Destination = []byte(l.rewrite(string(n.Destination)))
		case *gast.Image:
			n.Destination = []byte(l.rewrite(string(n.Destination)))
		}
		return gast.WalkContinue, nil
	})
}

func (l *linker) rewrite(dest string) string {
	if dest == "" || strings.Contains(dest, "://") || strings.HasPrefix(dest, "mailto:") || strings.HasPrefix(dest, "#") {
		return dest
	}
	target, frag, _ := strings.Cut(dest, "#")
	if frag != "" {
		frag = "#" + frag
	}
	if strings.HasPrefix(target, "/") {
		// Somewhere on the site outside the pages.
		return l.page.Root() + strings.TrimPrefix(target, "/") + frag
	}
	// Relative to the page's source file.
	from := "."
	if l.page.src != "" {
		from = path.Dir(l.page.src)
	}
	resolved := path.Join(from, target)
	if strings.HasPrefix(resolved, "../") {
		// Climbed out of docs/: a path in the repository.
		return l.site.Repo + "/tree/main/" + strings.TrimPrefix(resolved, "../") + frag
	}
	switch {
	case strings.HasSuffix(resolved, ".md"):
		url := strings.TrimSuffix(resolved, ".md") + "/"
		if path.Base(resolved) == "index.md" {
			url = path.Dir(resolved) + "/"
			if url == "./" {
				url = ""
			}
		}
		return l.page.Root() + url + frag
	case l.site.docs != "" && exists(filepath.Join(l.site.docs, filepath.FromSlash(resolved))):
		// A file beside the page, copied through at the same path.
		l.site.assets[resolved] = filepath.Join(l.site.docs, filepath.FromSlash(resolved))
		return l.page.Root() + resolved + frag
	default:
		// Nothing of that name beside the page: a path in the repository,
		// as written, so that "examples/check" means the same on every page.
		return l.site.Repo + "/tree/main/" + path.Clean(target) + frag
	}
}

func exists(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && !fi.IsDir()
}

// addExamples makes a page for every program under examples/ from its
// package comment, in the examples section, which docs/examples/index.md
// introduces.
func (s *Site) addExamples(dir string, sections map[string]*Section) error {
	if dir == "" {
		return nil
	}
	sec := sections["examples"]
	if sec == nil {
		return fmt.Errorf("no docs/examples/index.md to put the example pages under")
	}
	sec.Described = true
	ents, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, e := range ents {
		main := filepath.Join(dir, e.Name(), "main.go")
		if !e.IsDir() || !exists(main) {
			continue
		}
		p, err := s.examplePage(e.Name(), main)
		if err != nil {
			return fmt.Errorf("%s: %w", main, err)
		}
		p.Section = sec
		s.pages = append(s.pages, p)
	}
	return nil
}

// examplePage makes a program's page from its package comment, as the
// package comment of this command describes, with the player showing the
// program above the text.
func (s *Site) examplePage(name, main string) (*Page, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, main, nil, parser.ParseComments|parser.PackageClauseOnly)
	if err != nil {
		return nil, err
	}
	if f.Doc == nil {
		return nil, errors.New("no package comment to make a page from")
	}
	var cp comment.Parser
	d := cp.Parse(f.Doc.Text())
	pr := &comment.Printer{HeadingLevel: 2}
	text := func(blocks ...comment.Block) string {
		return strings.Join(strings.Fields(string(pr.Text(&comment.Doc{Content: blocks, Links: d.Links}))), " ")
	}

	p := &Page{
		Title:  name,
		URL:    "examples/" + name + "/",
		Embed:  name,
		Source: path.Join(filepath.Base(filepath.Dir(filepath.Dir(main))), name),
	}

	// The first paragraph says what the program is.
	lead, ok := d.Content[0].(*comment.Paragraph)
	if !ok {
		return nil, errors.New("the package comment does not start with a paragraph saying what the program is")
	}
	p.Description = text(lead)

	// The Controls section goes under the player, and the rest is the text.
	var body, controls []comment.Block
	inControls := false
	for _, b := range d.Content[1:] {
		if h, ok := b.(*comment.Heading); ok {
			if inControls = headingText(h) == "Controls"; inControls {
				continue
			}
		}
		if inControls {
			controls = append(controls, b)
		} else {
			body = append(body, b)
		}
	}
	if len(controls) > 0 {
		p.Controls = template.HTML(pr.HTML(&comment.Doc{Content: controls, Links: d.Links}))
	}
	p.Content = anchored(string(pr.HTML(&comment.Doc{Content: body, Links: d.Links})))
	p.Text = p.Description + " " + text(append(controls, body...)...)
	for _, b := range body {
		if h, ok := b.(*comment.Heading); ok {
			p.TOC = append(p.TOC, Heading{Level: 2, ID: h.DefaultID(), Text: headingText(h)})
		}
	}
	return p, nil
}

func headingText(h *comment.Heading) string {
	var b strings.Builder
	for _, t := range h.Text {
		if p, ok := t.(comment.Plain); ok {
			b.WriteString(string(p))
		}
	}
	return b.String()
}

// A Starter is an example the playground starts a project from: its name,
// whose data the site publishes under play/examples, and its files, by the
// path each takes in the project, as copied under play/starters.
type Starter struct {
	Name  string            `json:"name"`
	Files map[string]string `json:"files"`
}

// writeStarters copies each named example's main.go, its scenes under
// scenes/, the sources under its assets/ directory and the files under its
// pack/ directories, under play/starters, and writes play/starters.json
// listing them.
func writeStarters(examples string, names []string, play string) error {
	dir := filepath.Join(play, "starters")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	var starters []Starter
	for _, name := range names {
		st := Starter{Name: name, Files: map[string]string{}}
		files := []struct{ src, dst string }{{"main.go", name + ".go.txt"}}
		for _, dir := range []string{"scenes", "assets"} {
			entries, err := os.ReadDir(filepath.Join(examples, name, dir))
			if err != nil {
				continue
			}
			for _, e := range entries {
				if !e.IsDir() {
					files = append(files, struct{ src, dst string }{dir + "/" + e.Name(), name + "." + dir + "." + e.Name()})
				}
			}
		}
		// The files packed into archives, a directory each under pack/.
		if packs, err := os.ReadDir(filepath.Join(examples, name, "pack")); err == nil {
			for _, d := range packs {
				if !d.IsDir() {
					continue
				}
				entries, err := os.ReadDir(filepath.Join(examples, name, "pack", d.Name()))
				if err != nil {
					continue
				}
				for _, e := range entries {
					if !e.IsDir() {
						files = append(files, struct{ src, dst string }{"pack/" + d.Name() + "/" + e.Name(), name + ".pack." + d.Name() + "." + e.Name()})
					}
				}
			}
		}
		for _, f := range files {
			src := filepath.Join(examples, name, f.src)
			if _, err := os.Stat(src); err != nil {
				if f.src == "main.go" {
					return err
				}
				continue
			}
			if err := copyFile(src, filepath.Join(dir, f.dst)); err != nil {
				return err
			}
			st.Files[f.src] = "starters/" + f.dst
		}
		starters = append(starters, st)
	}
	data, err := json.MarshalIndent(starters, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(play, "starters.json"), append(data, '\n'), 0o644)
}

// write renders every page into out, and copies the assets and the player.
func (s *Site) write(out, static string) error {
	tmpl, err := template.New("").Funcs(template.FuncMap{
		"prevnext": prevNext,
		"player":   func(root, name string) template.HTML { return template.HTML(playerHTML(root, name)) },
	}).ParseFS(files, "templates/*.html")
	if err != nil {
		return err
	}

	for _, p := range s.pages {
		dir := filepath.Join(out, filepath.FromSlash(p.URL))
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
		var buf bytes.Buffer
		if err := tmpl.ExecuteTemplate(&buf, "page.html", map[string]any{
			"Site": s,
			"Page": p,
		}); err != nil {
			return fmt.Errorf("%s: %w", p.URL, err)
		}
		if err := os.WriteFile(filepath.Join(dir, "index.html"), buf.Bytes(), 0o644); err != nil {
			return err
		}
	}

	// What the search box looks through.
	type entry struct {
		Title   string `json:"t"`
		Section string `json:"s"`
		URL     string `json:"u"`
		Text    string `json:"x"`
	}
	var index []entry
	for _, p := range s.pages {
		sec := ""
		if p.Section != nil {
			sec = p.Section.Title
		}
		index = append(index, entry{p.Title, sec, p.URL, p.Text})
	}
	data, err := json.Marshal(index)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(out, "search.json"), data, 0o644); err != nil {
		return err
	}

	// The stylesheet and script, and the highlighting styles for both
	// color schemes, which chroma writes from its style definitions. Each
	// is confined to its scheme: a style only colors the token types it
	// has an entry for, so a rule from the light style left in force under
	// the dark one would paint whatever the dark style leaves alone in a
	// color meant for a light background, and it would vanish.
	// A scheme is the one the visitor chose with the header's switch,
	// data-theme, or else the system's. The rules are nested in the
	// selector for each, which is CSS nesting.
	if err := copyFS(files, "assets", filepath.Join(out, "assets")); err != nil {
		return err
	}
	var css bytes.Buffer
	formatter := chromahtml.New(chromahtml.WithClasses(true))
	for _, scheme := range []struct{ name, other, style string }{{"light", "dark", "github"}, {"dark", "light", "github-dark"}} {
		var rules bytes.Buffer
		if err := formatter.WriteCSS(&rules, styles.Get(scheme.style)); err != nil {
			return err
		}
		fmt.Fprintf(&css, ":root[data-theme=%q] {\n%s}\n", scheme.name, rules.Bytes())
		fmt.Fprintf(&css, "@media (prefers-color-scheme: %s) {\n:root:not([data-theme=%q]) {\n%s}\n}\n", scheme.name, scheme.other, rules.Bytes())
	}
	if err := os.WriteFile(filepath.Join(out, "assets", "chroma.css"), css.Bytes(), 0o644); err != nil {
		return err
	}

	for rel, src := range s.assets {
		if err := copyFile(src, filepath.Join(out, filepath.FromSlash(rel))); err != nil {
			return err
		}
	}
	if static != "" {
		if err := copyPlayer(os.DirFS(static), filepath.Join(out, "play")); err != nil {
			return fmt.Errorf("copying the player: %w", err)
		}
	}
	return nil
}

// copyPlayer copies the player's files under dst, stamping the pages and
// the modules with a version: a short hash of all the files, put after the
// name of each script, style and module they load from beside them, as
// app.js?v=1a2b3c4d. GitHub Pages lets a browser keep a file for ten
// minutes without asking again, so without the stamp a visitor who reloads
// across a deploy gets the new page running the old scripts, and sees a
// page with nothing working. The scripts carry the stamp on to the files
// they load themselves, the workers and the data beside them, by
// versioned() in app.js and worker.js.
func copyPlayer(fsys fs.FS, dst string) error {
	v, err := stamp(fsys)
	if err != nil {
		return err
	}
	return fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		target := filepath.Join(dst, filepath.FromSlash(p))
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		if ext := path.Ext(p); ext != ".html" && ext != ".js" {
			in, err := fsys.Open(p)
			if err != nil {
				return err
			}
			defer in.Close()
			return writeFrom(in, target)
		}
		data, err := fs.ReadFile(fsys, p)
		if err != nil {
			return err
		}
		return writeFrom(bytes.NewReader(stampRefs(data, v)), target)
	})
}

// stamp returns eight hex digits of a hash over the files under fsys, their
// paths and contents, the same for the same files.
func stamp(fsys fs.FS) (string, error) {
	h := sha256.New()
	err := fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := fs.ReadFile(fsys, p)
		if err != nil {
			return err
		}
		fmt.Fprintf(h, "%s %d\n", p, len(data))
		h.Write(data)
		return nil
	})
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil))[:8], nil
}

// The references a page or module makes to a file beside it: a script's
// src or a style's href naming a file with no directory, and a module
// import from ./NAME.js. Anything with a directory, a scheme or a query is
// left as it is.
var (
	htmlRef = regexp.MustCompile(`\b(src|href)="([\w.-]+\.(?:js|css))"`)
	jsRef   = regexp.MustCompile(`\bfrom "\./([\w.-]+\.js)"`)
)

// stampRefs puts ?v=v after each such reference in data.
func stampRefs(data []byte, v string) []byte {
	data = htmlRef.ReplaceAll(data, []byte(`$1="$2?v=`+v+`"`))
	return jsRef.ReplaceAll(data, []byte(`from "./$1?v=`+v+`"`))
}

// prevNext returns the pages either side of p in its section.
func prevNext(p *Page) map[string]*Page {
	out := map[string]*Page{}
	if p.Section == nil {
		return out
	}
	pages := p.Section.Pages
	for i, q := range pages {
		if q == p {
			if i > 0 {
				out["Prev"] = pages[i-1]
			}
			if i+1 < len(pages) {
				out["Next"] = pages[i+1]
			}
		}
	}
	return out
}

func copyFS(fsys fs.FS, root, dst string) error {
	return fs.WalkDir(fsys, root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, p)
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		in, err := fsys.Open(p)
		if err != nil {
			return err
		}
		defer in.Close()
		return writeFrom(in, target)
	})
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	return writeFrom(in, dst)
}

func writeFrom(in io.Reader, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
