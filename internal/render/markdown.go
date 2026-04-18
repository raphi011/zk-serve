package render

import (
	"bytes"
	"fmt"

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

// — goldmark ————————————————————————————————————————————————————————————————

func newMD(lookup map[string]string) goldmark.Markdown {
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
			parser.WithASTTransformers(
				util.Prioritized(&h1Stripper{}, 101),
				util.Prioritized(&mermaidTransformer{}, 100),
			),
		),
		goldmark.WithRendererOptions(
			html.WithUnsafe(),
			renderer.WithNodeRenderers(
				util.Prioritized(&mermaidRenderer{}, 100),
			),
		),
	)
}

// Markdown renders src (Markdown bytes with optional YAML frontmatter) to HTML.
// lookup maps wiki link targets (stem or path-without-extension) to note paths.
// Pass nil to fall back to /note/<target> for all wiki links.
func Markdown(src []byte, lookup map[string]string) (string, error) {
	var buf bytes.Buffer
	ctx := parser.NewContext()
	if err := newMD(lookup).Convert(src, &buf, parser.WithContext(ctx)); err != nil {
		return "", fmt.Errorf("render markdown: %w", err)
	}
	return buf.String(), nil
}
