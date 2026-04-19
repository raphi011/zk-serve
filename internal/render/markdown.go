package render

import (
	"bytes"
	"fmt"
	"strings"

	chromahtml "github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	highlighting "github.com/yuin/goldmark-highlighting/v2"
	meta "github.com/yuin/goldmark-meta"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
	"go.abhg.dev/goldmark/wikilink"
)

// noteResolver converts [[target]] to /note/<resolved-path>.
// lookup maps stem and path-without-extension → note path (e.g. "foo" → "notes/foo.md").
// If the target isn't found in lookup, it falls back to /note/<target>.
type noteResolver struct {
	lookup map[string]string
}

func (r noteResolver) ResolveWikilink(n *wikilink.Node) ([]byte, error) {
	target := string(n.Target)
	if r.lookup != nil {
		if path, ok := r.lookup[target]; ok {
			return []byte("/note/" + path), nil
		}
	}
	return append([]byte("/note/"), n.Target...), nil
}

// — Mermaid custom node ———————————————————————————————————————————————————————

var mermaidKind = ast.NewNodeKind("Mermaid")

type mermaidNode struct {
	ast.BaseBlock
	content []byte
}

func (n *mermaidNode) Kind() ast.NodeKind                       { return mermaidKind }
func (n *mermaidNode) IsRaw() bool                              { return true }
func (n *mermaidNode) Dump(src []byte, level int)               { ast.DumpHelper(n, src, level, nil, nil) }

// h1Stripper removes the first h1 heading (already shown in the note header).
type h1Stripper struct{}

func (t *h1Stripper) Transform(doc *ast.Document, _ text.Reader, _ parser.Context) {
	ast.Walk(doc, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		h, ok := node.(*ast.Heading)
		if ok && h.Level == 1 {
			h.Parent().RemoveChild(h.Parent(), h)
			return ast.WalkStop, nil
		}
		return ast.WalkContinue, nil
	})
}

// mermaidTransformer replaces fenced mermaid blocks with mermaidNode before rendering.
type mermaidTransformer struct{}

func (t *mermaidTransformer) Transform(doc *ast.Document, reader text.Reader, pc parser.Context) {
	src := reader.Source()
	var targets []*ast.FencedCodeBlock

	ast.Walk(doc, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		cb, ok := node.(*ast.FencedCodeBlock)
		if ok && string(cb.Language(src)) == "mermaid" {
			targets = append(targets, cb)
		}
		return ast.WalkContinue, nil
	})

	for _, cb := range targets {
		var buf bytes.Buffer
		for i := 0; i < cb.Lines().Len(); i++ {
			line := cb.Lines().At(i)
			buf.Write(line.Value(src))
		}
		mn := &mermaidNode{content: buf.Bytes()}
		cb.Parent().ReplaceChild(cb.Parent(), cb, mn)
	}
}

// mermaidRenderer emits <pre class="mermaid">…</pre> for mermaidNode.
type mermaidRenderer struct{}

func (r *mermaidRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(mermaidKind, r.render)
}

func (r *mermaidRenderer) render(w util.BufWriter, _ []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}
	mn := node.(*mermaidNode)
	_, _ = fmt.Fprintf(w, "<pre class=\"mermaid\">%s</pre>\n", mn.content)
	return ast.WalkContinue, nil
}

// Heading represents a heading extracted from the Goldmark AST.
type Heading struct {
	ID    string
	Text  string
	Level int
}

// Result is the output of rendering markdown.
type Result struct {
	HTML     string
	Headings []Heading
}

// headingCollector is an AST transformer that collects h2/h3 headings.
type headingCollector struct {
	headings []Heading
}

func (hc *headingCollector) Transform(doc *ast.Document, reader text.Reader, pc parser.Context) {
	src := reader.Source()
	ast.Walk(doc, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		h, ok := node.(*ast.Heading)
		if !ok || h.Level < 2 || h.Level > 3 {
			return ast.WalkContinue, nil
		}
		// Extract text from heading children.
		var textBuf bytes.Buffer
		for c := h.FirstChild(); c != nil; c = c.NextSibling() {
			if t, ok := c.(*ast.Text); ok {
				textBuf.Write(t.Segment.Value(src))
			} else {
				ast.Walk(c, func(inner ast.Node, entering bool) (ast.WalkStatus, error) {
					if entering {
						if t, ok := inner.(*ast.Text); ok {
							textBuf.Write(t.Segment.Value(src))
						}
					}
					return ast.WalkContinue, nil
				})
			}
		}
		heading := Heading{Text: textBuf.String(), Level: h.Level}
		if id, ok := h.AttributeString("id"); ok {
			heading.ID = string(id.([]byte))
		}
		hc.headings = append(hc.headings, heading)
		return ast.WalkContinue, nil
	})
}

// externalLinkRenderer adds target="_blank" rel="noopener" to external links.
type externalLinkRenderer struct{}

func (r *externalLinkRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(ast.KindLink, r.renderLink)
}

func (r *externalLinkRenderer) renderLink(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	n := node.(*ast.Link)
	dest := string(n.Destination)
	isExternal := strings.HasPrefix(dest, "http://") || strings.HasPrefix(dest, "https://")

	if entering {
		_, _ = w.WriteString(`<a href="`)
		_, _ = w.Write(util.EscapeHTML(n.Destination))
		_, _ = w.WriteString(`"`)
		if isExternal {
			_, _ = w.WriteString(` target="_blank" rel="noopener"`)
		}
		if n.Title != nil {
			_, _ = w.WriteString(` title="`)
			_, _ = w.Write(util.EscapeHTML(n.Title))
			_, _ = w.WriteString(`"`)
		}
		_, _ = w.WriteString(`>`)
	} else {
		_, _ = w.WriteString(`</a>`)
	}
	return ast.WalkContinue, nil
}

// — goldmark ————————————————————————————————————————————————————————————————

func newMD(lookup map[string]string, hc *headingCollector) goldmark.Markdown {
	return goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,
			meta.Meta,
			&wikilink.Extender{Resolver: noteResolver{lookup: lookup}},
			highlighting.NewHighlighting(
				highlighting.WithStyle("dracula"),
				highlighting.WithFormatOptions(chromahtml.WithClasses(true)),
			),
		),
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(),
			parser.WithASTTransformers(
				util.Prioritized(&h1Stripper{}, 101),
				util.Prioritized(hc, 102),
				util.Prioritized(&mermaidTransformer{}, 100),
			),
		),
		goldmark.WithRendererOptions(
			html.WithUnsafe(),
			renderer.WithNodeRenderers(
				util.Prioritized(&mermaidRenderer{}, 100),
				util.Prioritized(&externalLinkRenderer{}, 50),
			),
		),
	)
}

// Markdown renders src (Markdown bytes with optional YAML frontmatter) to HTML.
// lookup maps wiki link targets (stem or path-without-extension) to note paths.
// Pass nil to fall back to /note/<target> for all wiki links.
func Markdown(src []byte, lookup map[string]string) (Result, error) {
	hc := &headingCollector{}
	var buf bytes.Buffer
	ctx := parser.NewContext()
	if err := newMD(lookup, hc).Convert(src, &buf, parser.WithContext(ctx)); err != nil {
		return Result{}, fmt.Errorf("render markdown: %w", err)
	}
	return Result{HTML: buf.String(), Headings: hc.headings}, nil
}
