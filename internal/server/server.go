package server

import (
	"bytes"
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"time"

	chromahtml "github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/alecthomas/chroma/v2/styles"

	"github.com/raphaelgruber/zk-serve/internal/zk"
)

//go:embed static
var staticFS embed.FS

//go:embed templates
var templateFS embed.FS

var templateFuncs = template.FuncMap{
	"fmtDate": func(t time.Time) string {
		if t.IsZero() {
			return ""
		}
		return t.Format("2006-01-02")
	},
	"mul": func(a, b int) int {
		return a * b
	},
}

// ClientInterface is the subset of zk.Client the server needs.
type ClientInterface interface {
	List(query string, tags []string) ([]zk.Note, error)
	TagList() ([]zk.Tag, error)
}

// Server wires routes and holds shared state.
type Server struct {
	mux       *http.ServeMux
	tmpl      *template.Template
	zkClient  ClientInterface
	chromaDark  []byte
	chromaLight []byte
}

// New creates a Server using a *zk.Client (production path).
func New(zkClient *zk.Client) (*Server, error) {
	return NewWithClient(zkClient)
}

// NewWithClient creates a Server with any ClientInterface (enables testing).
func NewWithClient(client ClientInterface) (*Server, error) {
	tmpl, err := template.New("").
		Funcs(templateFuncs).
		ParseFS(templateFS, "templates/*.html")
	if err != nil {
		return nil, fmt.Errorf("parse templates: %w", err)
	}
	dark, err := buildChromaCSS("dracula")
	if err != nil {
		return nil, fmt.Errorf("chroma dark css: %w", err)
	}
	light, err := buildChromaCSS("github")
	if err != nil {
		return nil, fmt.Errorf("chroma light css: %w", err)
	}
	s := &Server{
		mux:         http.NewServeMux(),
		tmpl:        tmpl,
		zkClient:    client,
		chromaDark:  dark,
		chromaLight: light,
	}
	s.registerRoutes()
	return s, nil
}

func buildChromaCSS(styleName string) ([]byte, error) {
	style := styles.Get(styleName)
	if style == nil {
		style = styles.Fallback
	}
	var buf bytes.Buffer
	if err := chromahtml.New(chromahtml.WithClasses(true)).WriteCSS(&buf, style); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (s *Server) registerRoutes() {
	staticSub, _ := fs.Sub(staticFS, "static")
	s.mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticSub))))
	s.mux.HandleFunc("GET /static/chroma.css", s.handleChromaCSS)
	s.mux.HandleFunc("GET /", s.handleIndex)
	s.mux.HandleFunc("GET /search", s.handleSearch)
	s.mux.HandleFunc("GET /tags", s.handleTags)
	s.mux.HandleFunc("GET /note/{path...}", s.handleNote)
}

func (s *Server) handleChromaCSS(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/css; charset=utf-8")
	w.Write(scopeChromaCSS(s.chromaDark, `html:not([data-theme="light"]) `))
	w.Write(scopeChromaCSS(s.chromaLight, `[data-theme="light"] `))
}

// scopeChromaCSS inserts scope before every ".chroma" occurrence in each line.
// Chroma CSS lines look like: /* Keyword */ .chroma .k { color: ... }
// so a prefix check on ".chroma" would miss all of them.
func scopeChromaCSS(css []byte, scope string) []byte {
	var out bytes.Buffer
	for _, line := range bytes.Split(css, []byte("\n")) {
		if idx := bytes.Index(line, []byte(".chroma")); idx >= 0 {
			out.Write(line[:idx])
			out.WriteString(scope)
			out.Write(line[idx:])
		} else {
			out.Write(line)
		}
		out.WriteByte('\n')
	}
	return out.Bytes()
}

// ServeHTTP implements http.Handler.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

// ListenAndServe starts the HTTP server on addr.
func (s *Server) ListenAndServe(addr string) error {
	return http.ListenAndServe(addr, s)
}
