package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.chrisrx.dev/x/assert"
)

func write(t *testing.T, root, name, content string) {
	t.Helper()
	path := filepath.Join(root, name)
	assert.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	assert.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}

// site builds a small tree of pages with every kind of link in it.
func site(t *testing.T) (*Site, string) {
	t.Helper()
	dir := t.TempDir()
	docs := filepath.Join(dir, "docs")
	write(t, docs, "index.md", "---\ntitle: Front\n---\n\nSee the [guide](guide/index.md) and [tools](tools/index.md), or the [player](/play/).\n")
	write(t, docs, "guide/index.md", "---\ntitle: How it fits\nsection: Guide\nweight: 1\n---\n\n## First\n\nA [page](getting-started.md), a [tool](../tools/mkpsxiso.md#streams), a [package](yaroze) and a picture: ![diagram](diagram.png)\n\n### Under first\n\nText.\n")
	write(t, docs, "guide/getting-started.md", "---\ntitle: Getting started\nweight: 2\n---\n\nBack to [the index](index.md) and [the front](../index.md).\n\n```go\nfunc main() {}\n```\n")
	write(t, docs, "guide/diagram.png", "not really a png")
	write(t, docs, "tools/index.md", "---\ntitle: Tools\nweight: 2\n---\n\nThe tools.\n")
	write(t, docs, "tools/listing.md", "---\ntitle: Listing\n---\n\nThe program:\n\n```go src=prog/main.go\n```\n\nTry it:\n\n<!-- player prog -->\n")
	write(t, docs, "tools/prog/main.go", "package main\n\nfunc main() {}\n")
	write(t, docs, "tools/mkpsxiso.md", "---\ntitle: mkpsxiso\n---\n\n## Streams\n\nRaw sectors.\n")
	write(t, docs, "examples/index.md", "---\ntitle: Examples\nweight: 3\norder: zebra\n---\n\nThe examples.\n")

	examples := filepath.Join(dir, "examples")
	write(t, examples, "demo/main.go", "// A demo that draws a\n// thing.\n//\n// A port of something. It draws a thing.\n//\n// # Controls\n//\n// Cross draws it.\n//\n// # Differences from the original\n//\n//   - It is in Go.\npackage main\n")
	write(t, examples, "apple/main.go", "// An apple.\npackage main\n")
	write(t, examples, "zebra/main.go", "// A zebra, first.\npackage main\n")
	write(t, examples, "internal/shared.go", "package shared\n")

	s, err := load(docs, examples)
	assert.NoError(t, err)
	return s, dir
}

func page(s *Site, url string) *Page {
	for _, p := range s.pages {
		if p.URL == url {
			return p
		}
	}
	return nil
}

func TestSections(t *testing.T) {
	s, _ := site(t)
	var names []string
	for _, sec := range s.Sections {
		names = append(names, sec.Title)
	}
	assert.Equal(t, []string{"Guide", "Tools", "Examples"}, names)

	guide := s.Sections[0]
	assert.Equal(t, "How it fits", guide.Index.Title)
	assert.Equal(t, []string{"How it fits", "Getting started"}, titles(guide.Pages))

	// The example's page joins the examples section, named by its directory,
	// with the package comment's first paragraph describing it and its
	// Controls section taken out of the text.
	ex := page(s, "examples/demo/")
	assert.Equal(t, "demo", ex.Title)
	assert.Equal(t, "A demo that draws a thing.", ex.Description)
	assert.Equal(t, "demo", ex.Embed)
	assert.Equal(t, "examples/demo", ex.Source)
	assert.Equal(t, s.Sections[2], ex.Section)
	assert.Equal(t, "<p>Cross draws it.\n", string(ex.Controls))
	assert.Equal(t, false, strings.Contains(string(ex.Content), "Cross"), string(ex.Content))
	assert.Equal(t, false, strings.Contains(string(ex.Content), "A demo that"), string(ex.Content))
	assert.Equal(t, true, strings.Contains(string(ex.Content), "<h2 id=\"hdr-Differences_from_the_original\">"))
	assert.Equal(t, []Heading{{2, "hdr-Differences_from_the_original", "Differences from the original"}}, ex.TOC)
	assert.Equal(t, true, strings.Contains(ex.Text, "Cross draws it."), ex.Text)

	// The examples are described, and ordered by the index's order line
	// first, then by name.
	assert.Equal(t, true, s.Sections[2].Described)
	assert.Equal(t, false, guide.Described)
	assert.Equal(t, []string{"Examples", "zebra", "apple", "demo"}, titles(s.Sections[2].Pages))
}

func TestGroups(t *testing.T) {
	dir := t.TempDir()
	docs := filepath.Join(dir, "docs")
	write(t, docs, "guide/index.md", "---\ntitle: How it fits\nsection: Guide\ngroup: Docs\nweight: 1\ndescription: The guide.\n---\n")
	write(t, docs, "guide/getting-started.md", "---\ntitle: Getting started\n---\n")
	write(t, docs, "examples/index.md", "---\ntitle: Examples\nweight: 2\n---\n")
	write(t, docs, "tools/index.md", "---\ntitle: Tools\ngroup: Docs\nweight: 3\ndescription: The tools.\n---\n")
	s, err := load(docs, "")
	assert.NoError(t, err)

	// The group's link goes where its first section's would, and it has a
	// page of its own.
	var nav []string
	for _, tab := range s.Nav {
		nav = append(nav, tab.Title+" "+tab.URL)
	}
	assert.Equal(t, []string{"Docs docs/", "Examples examples/"}, nav)
	docsPage := page(s, "docs/")
	assert.Equal(t, s.Groups[0], docsPage.Group)
	assert.Equal(t, []*Section{s.Sections[0], s.Sections[2]}, s.Groups[0].Sections)
	assert.Equal(t, s.Groups[0], page(s, "guide/getting-started/").Docs())
	assert.Equal(t, true, page(s, "tools/").Under(s.Nav[0]))
	assert.Equal(t, false, page(s, "examples/").Under(s.Nav[0]))

	out := filepath.Join(dir, "out")
	assert.NoError(t, s.write(out, ""))
	html, _ := os.ReadFile(filepath.Join(out, "guide/getting-started/index.html"))
	// The side lists the group's sections, and the pages of the page's own.
	got := string(html)
	assert.Equal(t, true, strings.Contains(got, `<a class="head" href="../../guide/" aria-current="true">Guide</a>`), got)
	assert.Equal(t, true, strings.Contains(got, `<a class="head" href="../../tools/">Tools</a>`), got)
	assert.Equal(t, true, strings.Contains(got, `aria-current="page">Getting started</a>`), got)
	assert.Equal(t, true, strings.Contains(got, `<a href="../../docs/" aria-current="true">Docs</a>`), got)
	html, _ = os.ReadFile(filepath.Join(out, "docs/index.html"))
	got = string(html)
	assert.Equal(t, true, strings.Contains(got, `<li><a href="../tools/"><b>Tools</b><span>The tools.</span></a></li>`), got)

	// A group's page may not take a page's place.
	write(t, docs, "docs/index.md", "---\ntitle: Docs\n---\n")
	_, err = load(docs, "")
	assert.Error(t, "where docs/docs/index.md is", err)
}

func TestOrderUnknown(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "docs/examples/index.md", "---\ntitle: Examples\norder: missing\n---\n")
	write(t, dir, "examples/demo/main.go", "// A demo.\npackage main\n")
	_, err := load(filepath.Join(dir, "docs"), filepath.Join(dir, "examples"))
	assert.Error(t, `orders "missing"`, err)
}

func titles(pages []*Page) []string {
	var out []string
	for _, p := range pages {
		out = append(out, p.Title)
	}
	return out
}

func TestLinks(t *testing.T) {
	s, _ := site(t)
	front := string(page(s, "").Content)
	assert.Equal(t, true, strings.Contains(front, `href="guide/"`), front)
	assert.Equal(t, true, strings.Contains(front, `href="tools/"`), front)
	assert.Equal(t, true, strings.Contains(front, `href="play/"`), front)

	guide := string(page(s, "guide/").Content)
	assert.Equal(t, true, strings.Contains(guide, `href="../guide/getting-started/"`), guide)
	assert.Equal(t, true, strings.Contains(guide, `href="../tools/mkpsxiso/#streams"`), guide)
	assert.Equal(t, true, strings.Contains(guide, `href="https://github.com/ChrisRx/psxsdk/tree/main/yaroze"`), guide)
	assert.Equal(t, true, strings.Contains(guide, `src="../guide/diagram.png"`), guide)
	assert.Equal(t, 1, len(s.assets))

	started := string(page(s, "guide/getting-started/").Content)
	assert.Equal(t, true, strings.Contains(started, `href="../../guide/"`), started)
	assert.Equal(t, true, strings.Contains(started, `href="../../"`), started)
	// Code is highlighted into classes, so that one stylesheet per color
	// scheme can style it.
	assert.Equal(t, true, strings.Contains(started, `<span class="kd">func</span>`), started)
}

func TestTOCAndText(t *testing.T) {
	s, _ := site(t)
	p := page(s, "guide/")
	assert.Equal(t, []Heading{{2, "first", "First"}, {3, "under-first", "Under first"}}, p.TOC)
	assert.Equal(t, "First A page, a tool, a package and a picture: diagram Under first Text.", p.Text)

	// Code counts as text.
	assert.Equal(t, true, strings.Contains(page(s, "guide/getting-started/").Text, "func main() {}"))
}

func TestWrite(t *testing.T) {
	s, dir := site(t)
	static := filepath.Join(dir, "static")
	write(t, static, "index.html", `<link rel="stylesheet" href="style.css"><a href="../docs/">Docs</a><script src="app.js"></script><script src="https://cdn.example/x.js"></script>`)
	write(t, static, "compile-worker.js", `import { x } from "./wasi.js";`+"\n"+`const w = new Worker("worker.js");`)
	write(t, static, "fonts/a.woff2", "font")
	out := filepath.Join(dir, "out")
	assert.NoError(t, s.write(out, static))

	for _, name := range []string{
		"index.html", "guide/index.html", "guide/getting-started/index.html", "tools/mkpsxiso/index.html",
		"examples/demo/index.html", "guide/diagram.png", "search.json",
		"assets/site.css", "assets/chroma.css", "assets/search.js",
		"play/index.html", "play/fonts/a.woff2",
	} {
		_, err := os.Stat(filepath.Join(out, name))
		assert.NoError(t, err, name)
	}

	// The player's pages and modules name the files beside them with a
	// stamp, the same in each, and nothing else.
	v, err := stamp(os.DirFS(static))
	assert.NoError(t, err)
	assert.Equal(t, 8, len(v))
	player, _ := os.ReadFile(filepath.Join(out, "play/index.html"))
	assert.Equal(t, `<link rel="stylesheet" href="style.css?v=`+v+`"><a href="../docs/">Docs</a><script src="app.js?v=`+v+`"></script><script src="https://cdn.example/x.js"></script>`, string(player))
	module, _ := os.ReadFile(filepath.Join(out, "play/compile-worker.js"))
	assert.Equal(t, `import { x } from "./wasi.js?v=`+v+`";`+"\n"+`const w = new Worker("worker.js");`, string(module))

	html, _ := os.ReadFile(filepath.Join(out, "guide/getting-started/index.html"))
	page := string(html)
	// Everything the template links is relative to the page.
	assert.Equal(t, true, strings.Contains(page, `href="../../assets/site.css"`), page)
	assert.Equal(t, true, strings.Contains(page, `<html lang="en" data-root="../../">`), page)
	assert.Equal(t, true, strings.Contains(page, `aria-current="page">Getting started<`), page)
	assert.Equal(t, true, strings.Contains(page, `<small>Previous</small>How it fits<`), page)

	ex, _ := os.ReadFile(filepath.Join(out, "examples/demo/index.html"))
	assert.Equal(t, true, strings.Contains(string(ex), `src="../../play/player.html#demo"`), string(ex))
	// The list of examples says what each is, and so does the page.
	assert.Equal(t, true, strings.Contains(string(ex), `aria-current="page"><b>demo</b><small>A demo that draws a thing.</small>`), string(ex))
	assert.Equal(t, true, strings.Contains(string(ex), `<p class="lede">A demo that draws a thing.</p>`), string(ex))
	assert.Equal(t, true, strings.Contains(string(ex), `<section class="controls" aria-label="Controls">`), string(ex))
	index, _ := os.ReadFile(filepath.Join(out, "examples/index.html"))
	assert.Equal(t, true, strings.Contains(string(index), `<li><a href="../examples/zebra/"><b>zebra</b><span>A zebra, first.</span></a></li>`), string(index))

	css, _ := os.ReadFile(filepath.Join(out, "assets/chroma.css"))
	// Each scheme's rules apply when the visitor chose it, or else when
	// the system's is it.
	assert.Equal(t, true, strings.HasPrefix(string(css), `:root[data-theme="light"] {`))
	assert.Equal(t, true, strings.Contains(string(css), `:root[data-theme="dark"] {`))
	assert.Equal(t, true, strings.Contains(string(css), "@media (prefers-color-scheme: light) {\n:root:not([data-theme=\"dark\"]) {"))
	assert.Equal(t, true, strings.Contains(string(css), "@media (prefers-color-scheme: dark) {\n:root:not([data-theme=\"light\"]) {"))
	// No rule applies in both schemes.
	assert.Equal(t, 0, strings.Count(strings.SplitN(string(css), ":root", 2)[0], ".chroma"))
}

func TestFrontMatter(t *testing.T) {
	meta, body, err := frontMatter([]byte("---\ntitle: A: B\nweight: 3\n---\n\nBody\n"))
	assert.NoError(t, err)
	assert.Equal(t, map[string]string{"title": "A: B", "weight": "3"}, meta)
	assert.Equal(t, "\nBody\n", string(body))

	meta, body, err = frontMatter([]byte("Just body\n"))
	assert.NoError(t, err)
	assert.Equal(t, 0, len(meta))
	assert.Equal(t, "Just body\n", string(body))

	_, _, err = frontMatter([]byte("---\ntitle: open\n"))
	assert.Error(t, "not closed", err)

	dir := t.TempDir()
	write(t, dir, "index.md", "---\nweight: 1\n---\n\nNo title.\n")
	_, err = load(dir, "")
	assert.Error(t, "no title", err)
}

func TestExpand(t *testing.T) {
	s, _ := site(t)
	p := page(s, "tools/listing/")
	content := string(p.Content)
	assert.Equal(t, true, strings.Contains(content, `<span class="kn">package</span>`), content)
	assert.Equal(t, true, strings.Contains(content, `<iframe class="player" src="../../play/player.html#prog"`), content)
	// What was pulled in is searchable like the rest.
	assert.Equal(t, true, strings.Contains(p.Text, "func main() {}"), p.Text)

	dir := t.TempDir()
	write(t, dir, "x/index.md", "---\ntitle: X\n---\n\n```go src=missing.go\n```\n")
	_, err := load(dir, "")
	assert.Error(t, "including missing.go", err)
}

func TestDeclarations(t *testing.T) {
	src := []byte(`package main

// speed is how fast.
const speed = 2

var (
	// The screen.
	screen = Screen{
		Depth: 4,
	}
	pad  Pad
)

type T struct{}

// Open opens.
func (t *T) Open() {}

// main runs.
func main() {
	screen.Open()
}
`)
	got, err := declarations(src, []string{"main", "screen", "speed", "pad", "T.Open"})
	assert.NoError(t, err)
	assert.Equal(t, `// main runs.
func main() {
	screen.Open()
}

// The screen.
var screen = Screen{
	Depth: 4,
}

// speed is how fast.
const speed = 2

var pad Pad

// Open opens.
func (t *T) Open() {}`, string(got))

	_, err = declarations(src, []string{"nothing"})
	assert.Error(t, "no declaration of nothing", err)

	dir := t.TempDir()
	write(t, dir, "x/index.md", "---\ntitle: X\n---\n\n```go src=prog.go#main\n```\n")
	write(t, dir, "x/prog.go", string(src))
	s, err := load(dir, "")
	assert.NoError(t, err)
	assert.Equal(t, true, strings.Contains(page(s, "x/").Text, "main runs. func main() { screen.Open() }"), page(s, "x/").Text)
	assert.Equal(t, false, strings.Contains(page(s, "x/").Text, "speed"), page(s, "x/").Text)
}
