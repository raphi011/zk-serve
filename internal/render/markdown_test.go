package render_test

import (
	"strings"
	"testing"

	"github.com/raphaelgruber/zk-serve/internal/render"
)

func TestGFMTable(t *testing.T) {
	md := "| A | B |\n|---|---|\n| 1 | 2 |\n"
	got, err := render.Markdown([]byte(md))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "<table>") {
		t.Errorf("expected <table>, got: %s", got)
	}
	if !strings.Contains(got, "<th>") {
		t.Errorf("expected <th>, got: %s", got)
	}
}

func TestGFMTaskList(t *testing.T) {
	md := "- [x] done\n- [ ] todo\n"
	got, err := render.Markdown([]byte(md))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, `type="checkbox"`) {
		t.Errorf("expected checkbox input, got: %s", got)
	}
	if !strings.Contains(got, "checked") {
		t.Errorf("expected checked attribute, got: %s", got)
	}
}

func TestGFMStrikethrough(t *testing.T) {
	md := "~~deleted~~\n"
	got, err := render.Markdown([]byte(md))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "<del>") {
		t.Errorf("expected <del>, got: %s", got)
	}
}

func TestSyntaxHighlighting(t *testing.T) {
	md := "```go\nfunc main() {}\n```\n"
	got, err := render.Markdown([]byte(md))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "chroma") {
		t.Errorf("expected chroma CSS class, got: %s", got)
	}
}

func TestWikiLinkTransformer(t *testing.T) {
	md := "See [[notes/go-concurrency]] for details.\n"
	got, err := render.Markdown([]byte(md))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, `href="/note/notes/go-concurrency"`) {
		t.Errorf("expected wiki link href, got: %s", got)
	}
}

func TestMermaidPassthrough(t *testing.T) {
	md := "```mermaid\ngraph TD\n  A-->B\n```\n"
	got, err := render.Markdown([]byte(md))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, `class="mermaid"`) {
		t.Errorf("expected mermaid class, got: %s", got)
	}
	if !strings.Contains(got, "graph TD") {
		t.Errorf("expected mermaid source preserved, got: %s", got)
	}
	if strings.Contains(got, "chroma") {
		t.Errorf("mermaid must not be chroma-highlighted, got: %s", got)
	}
}

func TestRawHTMLAllowed(t *testing.T) {
	md := "<strong>bold</strong>\n"
	got, err := render.Markdown([]byte(md))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "<strong>bold</strong>") {
		t.Errorf("expected raw HTML passthrough, got: %s", got)
	}
}
