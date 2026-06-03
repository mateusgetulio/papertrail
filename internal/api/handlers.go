package api

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/mateusgetulio/papertrail/internal/scoring"
	"github.com/mateusgetulio/papertrail/internal/store"
)

// ideaLabels mirrors the idea_label enum (docs/09 notes / migration 001).
var ideaLabels = []string{
	"enterprise_high_ticket", "smb_saas", "vertical_saas", "developer_tool",
	"compliance_regtech", "ai_workflow_automation", "marketplace",
	"consumer_mass_market", "not_suitable",
}

var reviewStateOptions = store.ReviewStates

type queueView struct {
	Ideas  []store.IdeaRow
	Filter store.IdeaFilter
	Labels []string
	States []string
}

type detailView struct {
	Idea      *store.IdeaDetail
	Breakdown []scoring.CriterionRow
	States    []string
}

func (s *Server) handleQueue(w http.ResponseWriter, r *http.Request) {
	f := store.IdeaFilter{
		Label:    r.URL.Query().Get("label"),
		State:    r.URL.Query().Get("state"),
		MinScore: atoiDefault(r.URL.Query().Get("min"), 0),
		MaxScore: atoiDefault(r.URL.Query().Get("max"), 0),
	}
	ideas, err := store.ListIdeas(r.Context(), s.db, f)
	if err != nil {
		s.fail(w, "list ideas", err)
		return
	}
	view := queueView{Ideas: ideas, Filter: f, Labels: ideaLabels, States: reviewStateOptions}
	s.render(w, s.queueTmpl, "layout", view)
}

func (s *Server) handleDetail(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	idea, err := store.GetIdeaDetail(r.Context(), s.db, id)
	if err != nil {
		s.fail(w, "get idea", err)
		return
	}
	if idea == nil {
		http.NotFound(w, r)
		return
	}
	view := detailView{Idea: idea, Breakdown: scoring.Breakdown(idea.Criteria), States: reviewStateOptions}
	s.render(w, s.detailTmpl, "layout", view)
}

func (s *Server) handleReview(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	state := r.PostForm.Get("state")
	if !validState(state) {
		http.Error(w, "invalid review state", http.StatusBadRequest)
		return
	}
	reviewer := strings.TrimSpace(r.PostForm.Get("reviewer"))
	notes := strings.TrimSpace(r.PostForm.Get("notes"))

	var mergedInto *int64
	if state == "merged" {
		raw := strings.TrimSpace(r.PostForm.Get("merged_into"))
		if raw == "" {
			http.Error(w, "merged_into is required when merging", http.StatusBadRequest)
			return
		}
		target, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || target == id {
			http.Error(w, "merged_into must be a different idea id", http.StatusBadRequest)
			return
		}
		mergedInto = &target
	}

	if err := store.UpdateReviewStatus(r.Context(), s.db, id, state, reviewer, notes, mergedInto); err != nil {
		s.fail(w, "update review", err)
		return
	}

	// Re-fetch and return the review panel partial for HTMX to swap in.
	idea, err := store.GetIdeaDetail(r.Context(), s.db, id)
	if err != nil {
		s.fail(w, "reload idea", err)
		return
	}
	view := detailView{Idea: idea, Breakdown: scoring.Breakdown(idea.Criteria), States: reviewStateOptions}
	s.render(w, s.partialTmpl, "reviewpanel", view)
}

func (s *Server) handleExportCSV(w http.ResponseWriter, r *http.Request) {
	ideas, err := store.ListIdeas(r.Context(), s.db, store.IdeaFilter{State: "approved"})
	if err != nil {
		s.fail(w, "export", err)
		return
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition",
		fmt.Sprintf(`attachment; filename="papertrail-approved-%s.csv"`, time.Now().Format("20060102")))

	cw := csv.NewWriter(w)
	defer cw.Flush()
	_ = cw.Write([]string{"id", "idea_name", "label", "sales_motion", "score", "industries", "pain_point", "pitch"})
	for _, i := range ideas {
		_ = cw.Write([]string{
			strconv.FormatInt(i.ID, 10), i.IdeaName, i.Label, i.SalesMotion,
			strconv.Itoa(i.OverallScore), strings.Join(i.Industries, "; "), i.PainPoint, i.Pitch,
		})
	}
}

// --- helpers ---

func (s *Server) render(w http.ResponseWriter, t *template.Template, name string, data any) {
	var buf bytes.Buffer
	if err := t.ExecuteTemplate(&buf, name, data); err != nil {
		s.fail(w, "render "+name, err)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = buf.WriteTo(w)
}

func validState(s string) bool {
	for _, v := range reviewStateOptions {
		if v == s {
			return true
		}
	}
	return false
}

func pathID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return 0, false
	}
	return id, true
}

func atoiDefault(s string, def int) int {
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return n
}

func (s *Server) fail(w http.ResponseWriter, what string, err error) {
	slog.Error("dashboard error", "op", what, "err", err)
	http.Error(w, "internal error", http.StatusInternalServerError)
}

func formatPct(p float64) string {
	return strconv.FormatFloat(p, 'f', 0, 64) + "%"
}

// labelize converts an enum value like "needs_more_evidence" into a readable
// "Needs more evidence".
func labelize(s string) string {
	if s == "" {
		return ""
	}
	s = strings.ReplaceAll(s, "_", " ")
	return strings.ToUpper(s[:1]) + s[1:]
}
