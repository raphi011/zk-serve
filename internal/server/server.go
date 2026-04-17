package server

import (
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"time"

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
	mux      *http.ServeMux
	tmpl     *template.Template
	zkClient ClientInterface
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
	s := &Server{
		mux:      http.NewServeMux(),
		tmpl:     tmpl,
		zkClient: client,
	}
	s.registerRoutes()
	return s, nil
}

func (s *Server) registerRoutes() {
	staticSub, _ := fs.Sub(staticFS, "static")
	s.mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticSub))))
	s.mux.HandleFunc("GET /", s.handleIndex)
	s.mux.HandleFunc("GET /search", s.handleSearch)
	s.mux.HandleFunc("GET /tags", s.handleTags)
	s.mux.HandleFunc("GET /note/{path...}", s.handleNote)
}

// ServeHTTP implements http.Handler.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

// ListenAndServe starts the HTTP server on addr.
func (s *Server) ListenAndServe(addr string) error {
	return http.ListenAndServe(addr, s)
}
