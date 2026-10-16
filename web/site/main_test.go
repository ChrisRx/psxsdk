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
	write(t, docs, "examples/index.md", "---\ntitle: Examples\nweight: 3\n---\n\nThe examples.\n")

	examples := filepath.Join(dir, "examples")
	write(t, examples, "demo/main.go", "// The demo, in Go.\n//\n// A port of something. It draws a thing.\n//\n// # Differences from the original\n//\n//   - It is in Go.\npackage main\n")
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
	// with the package comment's first sentence describing it.
	ex := page(s, "examples/demo/")
	assert.Equal(t, "demo", ex.Title)
	assert.Equal(t, "The demo, in Go.", ex.Description)
	assert.Equal(t, "demo", ex.Embed)
	assert.Equal(t, "examples/demo", ex.Source)
	assert.Equal(t, s.Sections[2], ex.Section)
	assert.Equal(t, true, strings.Contains(string(ex.Content), "<h2 id=\"hdr-Differences_from_the_original\">"))
	assert.Equal(t, []Heading{{2, "hdr-Differences_from_the_original", "Differences from the original"}}, ex.TOC)
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
	write(t, static, "index.html", "<p>player</p>")
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

	html, _ := os.ReadFile(filepath.Join(out, "guide/getting-started/index.html"))
	page := string(html)
	// Everything the template links is relative to the page.
	assert.Equal(t, true, strings.Contains(page, `href="../../assets/site.css"`), page)
	assert.Equal(t, true, strings.Contains(page, `<html lang="en" data-root="../../">`), page)
	assert.Equal(t, true, strings.Contains(page, `aria-current="page">Getting started<`), page)
	assert.Equal(t, true, strings.Contains(page, `<small>Previous</small>How it fits<`), page)

	ex, _ := os.ReadFile(filepath.Join(out, "examples/demo/index.html"))
	assert.Equal(t, true, strings.Contains(string(ex), `src="../../play/?embed#demo"`), string(ex))

	css, _ := os.ReadFile(filepath.Join(out, "assets/chroma.css"))
	assert.Equal(t, true, strings.HasPrefix(string(css), "@media (prefers-color-scheme: light) {"))
	assert.Equal(t, true, strings.Contains(string(css), "@media (prefers-color-scheme: dark) {"))
	// No rule applies in both schemes.
	assert.Equal(t, 0, strings.Count(strings.SplitN(string(css), "@media", 2)[0], ".chroma"))
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
	assert.Equal(t, true, strings.Contains(content, `<iframe class="player" src="../../play/?embed#prog"`), content)
	assert.Equal(t, true, strings.Contains(content, `href="../../play/#prog"`), content)
	// What was pulled in is searchable like the rest.
	assert.Equal(t, true, strings.Contains(p.Text, "func main() {}"), p.Text)

	dir := t.TempDir()
	write(t, dir, "x/index.md", "---\ntitle: X\n---\n\n```go src=missing.go\n```\n")
	_, err := load(dir, "")
	assert.Error(t, "including missing.go", err)
}
