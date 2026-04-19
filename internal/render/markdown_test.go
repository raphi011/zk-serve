package render_test

import (
	"strings"
	"testing"

	"github.com/raphaelgruber/zk-serve/internal/render"
)

func TestGFMTable(t *testing.T) {
	md := "| A | B |\n|---|---|\n| 1 | 2 |\n"
	result, err := render.Markdown([]byte(md), nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.HTML, "<table>") {
		t.Errorf("expected <table>, got: %s", result.HTML)
	}
	if !strings.Contains(result.HTML, "<th>") {
		t.Errorf("expected <th>, got: %s", result.HTML)
	}
}

func TestGFMTaskList(t *testing.T) {
	md := "- [x] done\n- [ ] todo\n"
	result, err := render.Markdown([]byte(md), nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.HTML, `type="checkbox"`) {
		t.Errorf("expected checkbox input, got: %s", result.HTML)
	}
	if !strings.Contains(result.HTML, "checked") {
		t.Errorf("expected checked attribute, got: %s", result.HTML)
	}
}

func TestGFMStrikethrough(t *testing.T) {
	md := "~~deleted~~\n"
	result, err := render.Markdown([]byte(md), nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.HTML, "<del>") {
		t.Errorf("expected <del>, got: %s", result.HTML)
	}
}

func TestSyntaxHighlighting(t *testing.T) {
	md := "```go\nfunc main() {}\n```\n"
	result, err := render.Markdown([]byte(md), nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.HTML, "chroma") {
		t.Errorf("expected chroma CSS class, got: %s", result.HTML)
	}
}

func TestWikiLinkTransformer(t *testing.T) {
	md := "See [[notes/go-concurrency]] for details.\n"
	result, err := render.Markdown([]byte(md), nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.HTML, `href="/note/`) {
		t.Errorf("expected wiki link href, got: %s", result.HTML)
	}
}

func TestMermaidPassthrough(t *testing.T) {
	md := "```mermaid\ngraph TD\n  A-->B\n```\n"
	result, err := render.Markdown([]byte(md), nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.HTML, `class="mermaid"`) {
		t.Errorf("expected mermaid class, got: %s", result.HTML)
	}
	if !strings.Contains(result.HTML, "graph TD") {
		t.Errorf("expected mermaid source preserved, got: %s", result.HTML)
	}
	if strings.Contains(result.HTML, "chroma") {
		t.Errorf("mermaid must not be chroma-highlighted, got: %s", result.HTML)
	}
}

func TestRawHTMLAllowed(t *testing.T) {
	md := "<strong>bold</strong>\n"
	result, err := render.Markdown([]byte(md), nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.HTML, "<strong>bold</strong>") {
		t.Errorf("expected raw HTML passthrough, got: %s", result.HTML)
	}
}

func TestFrontmatterStripped(t *testing.T) {
	md := "---\ntitle: My Note\ntags: [go]\n---\n\nActual content.\n"
	result, err := render.Markdown([]byte(md), nil)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(result.HTML, "title:") || strings.Contains(result.HTML, "tags:") {
		t.Errorf("frontmatter should be stripped, got: %s", result.HTML)
	}
	if !strings.Contains(result.HTML, "Actual content") {
		t.Errorf("body content should remain, got: %s", result.HTML)
	}
}

func TestHeadingExtraction(t *testing.T) {
	md := "## Architecture\n\nSome text.\n\n### File Naming\n\nMore text.\n\n## Workflows\n"
	result, err := render.Markdown([]byte(md), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Headings) != 3 {
		t.Fatalf("got %d headings, want 3", len(result.Headings))
	}
	h := result.Headings[0]
	if h.Text != "Architecture" {
		t.Errorf("headings[0].Text = %q, want Architecture", h.Text)
	}
	if h.Level != 2 {
		t.Errorf("headings[0].Level = %d, want 2", h.Level)
	}
	if h.ID == "" {
		t.Error("headings[0].ID should not be empty")
	}
	if result.Headings[1].Level != 3 {
		t.Errorf("headings[1].Level = %d, want 3", result.Headings[1].Level)
	}
	if result.Headings[1].Text != "File Naming" {
		t.Errorf("headings[1].Text = %q", result.Headings[1].Text)
	}
	if result.Headings[2].Text != "Workflows" {
		t.Errorf("headings[2].Text = %q", result.Headings[2].Text)
	}
}

func TestHeadingExtractionH1Excluded(t *testing.T) {
	md := "# Title\n\n## Section\n"
	result, err := render.Markdown([]byte(md), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Headings) != 1 {
		t.Fatalf("got %d headings, want 1 (h1 should be excluded)", len(result.Headings))
	}
	if result.Headings[0].Text != "Section" {
		t.Errorf("headings[0].Text = %q", result.Headings[0].Text)
	}
}

func TestHeadingExtractionEmpty(t *testing.T) {
	md := "Just a paragraph with no headings.\n"
	result, err := render.Markdown([]byte(md), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Headings) != 0 {
		t.Fatalf("got %d headings, want 0", len(result.Headings))
	}
}
