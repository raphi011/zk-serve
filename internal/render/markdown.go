package render

import (
	"bytes"
	"fmt"
	"regexp"

	chromahtml "github.com/alecthomas/chroma/v2/formatters/html"
	highlighting "github.com/yuin/goldmark-highlighting/v2"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/renderer/html"
)

var wikiLinkRe = regexp.MustCompile(`\[\[([^\]]+)\]\]`)

// mermaidFenceRe matches a full mermaid fenced code block.
var mermaidFenceRe = regexp.MustCompile("(?m)^```mermaid\n((?:[^`]|`[^`]|``[^`])*?)```")

// preprocess converts wiki links and mermaid blocks before goldmark parsing.
func preprocess(src []byte) []byte {
	// Convert mermaid fences to raw <pre class="mermaid"> blocks.
	src = mermaidFenceRe.ReplaceAllFunc(src, func(match []byte) []byte {
		sub := mermaidFenceRe.FindSubmatch(match)
		content := bytes.TrimRight(sub[1], "\n")
		return []byte(fmt.Sprintf("<pre class=\"mermaid\">%s</pre>", content))
	})

	// Convert [[target]] to [target](/note/target).
	src = wikiLinkRe.ReplaceAllFunc(src, func(match []byte) []byte {
		sub := wikiLinkRe.FindSubmatch(match)
		target := sub[1]
		return []byte(fmt.Sprintf("[%s](/note/%s)", target, target))
	})

	return src
}

var md = goldmark.New(
	goldmark.WithExtensions(
		extension.GFM,
		highlighting.NewHighlighting(
			highlighting.WithStyle("dracula"),
			highlighting.WithFormatOptions(chromahtml.WithClasses(true)),
		),
	),
	goldmark.WithRendererOptions(html.WithUnsafe()),
)

func Markdown(src []byte) (string, error) {
	src = preprocess(src)
	var buf bytes.Buffer
	if err := md.Convert(src, &buf); err != nil {
		return "", fmt.Errorf("render markdown: %w", err)
	}
	return buf.String(), nil
}
