// Package api implements the Phase 4 review dashboard: a server-rendered
// HTMX UI over the scored SaaS idea candidates, with reviewer actions and a
// CSV export of approved ideas.
package api

import (
	"embed"
	"html/template"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed templates/*.html
var templatesFS embed.FS

// Server holds dependencies for the dashboard HTTP handlers.
type Server struct {
	db          *pgxpool.Pool
	queueTmpl   *template.Template
	detailTmpl  *template.Template
	partialTmpl *template.Template
}

// NewServer parses the embedded templates and returns a ready Server.
func NewServer(db *pgxpool.Pool) (*Server, error) {
	parse := func(files ...string) (*template.Template, error) {
		return template.New("").Funcs(funcMap).ParseFS(templatesFS, files...)
	}
	queue, err := parse("templates/layout.html", "templates/partials.html", "templates/queue.html")
	if err != nil {
		return nil, err
	}
	detail, err := parse("templates/layout.html", "templates/partials.html", "templates/detail.html")
	if err != nil {
		return nil, err
	}
	partial, err := parse("templates/partials.html")
	if err != nil {
		return nil, err
	}
	return &Server{db: db, queueTmpl: queue, detailTmpl: detail, partialTmpl: partial}, nil
}

// Routes returns the HTTP handler for the dashboard.
func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", s.handleQueue)
	mux.HandleFunc("GET /ideas/{id}", s.handleDetail)
	mux.HandleFunc("POST /ideas/{id}/review", s.handleReview)
	mux.HandleFunc("GET /export/csv", s.handleExportCSV)
	return logRequests(mux)
}

func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		slog.Info("http", "method", r.Method, "path", r.URL.Path, "dur", time.Since(start).String())
	})
}

// funcMap holds template helpers shared across all pages.
var funcMap = template.FuncMap{
	"join": func(items []string) string { return strings.Join(items, ", ") },
	// barWidth turns an adjusted 1..10 score into a CSS width percentage.
	"barWidth": func(adj float64) string {
		pct := adj * 10
		if pct < 0 {
			pct = 0
		}
		if pct > 100 {
			pct = 100
		}
		return formatPct(pct)
	},
	// pct formats a 0..1 weight as a percentage label.
	"pct": func(w float64) string { return formatPct(w * 100) },
	// labelize turns enum_values like "needs_more_evidence" into "Needs more evidence".
	"labelize": labelize,
	// stateColor returns Tailwind classes for a review state badge.
	"stateColor": func(state string) string {
		switch state {
		case "approved":
			return "bg-green-100 text-green-800"
		case "rejected":
			return "bg-red-100 text-red-800"
		case "needs_more_evidence":
			return "bg-amber-100 text-amber-800"
		case "merged":
			return "bg-purple-100 text-purple-800"
		default:
			return "bg-gray-100 text-gray-700"
		}
	},
	"reviewStates": func() []string { return reviewStateOptions },
}
