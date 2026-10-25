package main

import (
	"fmt"
	"strings"

	"github.com/gopikchr/gopikchr"
	gast "github.com/yuin/goldmark/ast"
	mdparser "github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// A diagram is a fenced block in the pikchr language, drawn as an SVG when
// the page is built. Its lines and text take the page's color, so it
// follows the theme, and it is never drawn wider than pikchr made it. A
// phone shows it shrunk to the column, and assets/diagram.js opens it at
// its own size when it is tapped.
type diagram struct {
	gast.BaseBlock
	svg   string
	width int
}

var diagramKind = gast.NewNodeKind("Diagram")

func (d *diagram) Kind() gast.NodeKind { return diagramKind }

func (d *diagram) Dump(source []byte, level int) { gast.DumpHelper(d, source, level, nil, nil) }

// diagrams replaces each pikchr block in a page with its diagram, and
// keeps the first error, which fails the build.
type diagrams struct {
	err error
}

func (t *diagrams) Transform(doc *gast.Document, reader text.Reader, pc mdparser.Context) {
	var blocks []*gast.FencedCodeBlock
	gast.Walk(doc, func(n gast.Node, entering bool) (gast.WalkStatus, error) {
		if b, ok := n.(*gast.FencedCodeBlock); ok && entering && string(b.Language(reader.Source())) == "pikchr" {
			blocks = append(blocks, b)
		}
		return gast.WalkContinue, nil
	})
	for _, b := range blocks {
		var src strings.Builder
		lines := b.Lines()
		for i := range lines.Len() {
			seg := lines.At(i)
			src.Write(seg.Value(reader.Source()))
		}
		svg, width, _, err := gopikchr.Convert(src.String(), gopikchr.WithPlaintextErrors())
		if err != nil {
			if t.err == nil {
				t.err = fmt.Errorf("pikchr: %s", svg)
			}
			continue
		}
		d := &diagram{svg: strings.ReplaceAll(svg, "rgb(0,0,0)", "currentColor"), width: width}
		b.Parent().ReplaceChild(b.Parent(), b, d)
	}
}

// diagramRenderer writes a diagram.
type diagramRenderer struct{}

func (diagramRenderer) RegisterFuncs(r renderer.NodeRendererFuncRegisterer) {
	r.Register(diagramKind, func(w util.BufWriter, source []byte, n gast.Node, entering bool) (gast.WalkStatus, error) {
		if entering {
			d := n.(*diagram)
			fmt.Fprintf(w, "<figure class=\"diagram\" style=\"max-width: %dpx\">%s</figure>\n", d.width, d.svg)
		}
		return gast.WalkSkipChildren, nil
	})
}
