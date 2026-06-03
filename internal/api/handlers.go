package api

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"net/url"
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

const pageSize = 25

type queueView struct {
	Ideas      []store.IdeaRow
	Filter     store.IdeaFilter
	Labels     []string
	States     []string
	Sorts      []struct{ Key, Label string }
	Page       int
	TotalPages int
	Total      int
	PrevURL    string
	NextURL    string
}

type detailView struct {
	Idea         *store.IdeaDetail
	Breakdown    []scoring.CriterionRow
	States       []string
	MergeTargets []store.IdeaRef
}

func (s *Server) handleQueue(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	sort := q.Get("sort")
	if sort == "" {
		sort = "score"
	}
	page := atoiDefault(q.Get("page"), 1)
	if page < 1 {
		page = 1
	}

	f := store.IdeaFilter{
		Label:    q.Get("label"),
		State:    q.Get("state"),
		MinScore: atoiDefault(q.Get("min"), 0),
		MaxScore: atoiDefault(q.Get("max"), 0),
		Query:    strings.TrimSpace(q.Get("q")),
		Sort:     sort,
		Limit:    pageSize,
	}

	total, err := store.CountIdeas(r.Context(), s.db, f)
	if err != nil {
		s.fail(w, "count ideas", err)
		return
	}
	totalPages := (total + pageSize - 1) / pageSize
	if totalPages < 1 {
		totalPages = 1
	}
	if page > totalPages {
		page = totalPages
	}
	f.Offset = (page - 1) * pageSize

	ideas, err := store.ListIdeas(r.Context(), s.db, f)
	if err != nil {
		s.fail(w, "list ideas", err)
		return
	}

	view := queueView{
		Ideas: ideas, Filter: f, Labels: ideaLabels, States: reviewStateOptions,
		Sorts:      store.SortOptions,
		Page:       page,
		TotalPages: totalPages,
		Total:      total,
		PrevURL:    pageURL(r.URL, page-1, page > 1),
		NextURL:    pageURL(r.URL, page+1, page < totalPages),
	}
	s.render(w, s.queueTmpl, "layout", view)
}

// pageURL returns a query string for the given page preserving current filters,
// or "" when the link should be disabled.
func pageURL(u *url.URL, page int, enabled bool) string {
	if !enabled {
		return ""
	}
	q := u.Query()
	q.Set("page", strconv.Itoa(page))
	return "?" + q.Encode()
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
	refs, err := store.ListIdeaRefs(r.Context(), s.db, id)
	if err != nil {
		s.fail(w, "list refs", err)
		return
	}
	view := detailView{Idea: idea, Breakdown: scoring.Breakdown(idea.Criteria), States: reviewStateOptions, MergeTargets: refs}
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
	refs, err := store.ListIdeaRefs(r.Context(), s.db, id)
	if err != nil {
		s.fail(w, "list refs", err)
		return
	}
	view := detailView{Idea: idea, Breakdown: scoring.Breakdown(idea.Criteria), States: reviewStateOptions, MergeTargets: refs}
	// reviewresult = the swapped review panel + an out-of-band update of the
	// header status badge, so the top of the page stays consistent.
	s.render(w, s.partialTmpl, "reviewresult", view)
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

func (s *Server) handleInsights(w http.ResponseWriter, r *http.Request) {
	ins, err := store.GetInsights(r.Context(), s.db)
	if err != nil {
		s.fail(w, "insights", err)
		return
	}
	// maxBucket drives the histogram bar widths in the template.
	maxBucket := 1
	for _, b := range ins.ScoreBuckets {
		if b.Count > maxBucket {
			maxBucket = b.Count
		}
	}
	s.render(w, s.insightsTmpl, "layout", struct {
		Insights  *store.Insights
		MaxBucket int
	}{ins, maxBucket})
}

type compareCritRow struct {
	Label  string
	Values []int
}

func (s *Server) handleCompare(w http.ResponseWriter, r *http.Request) {
	ids := parseIDs(r.URL.Query()["ids"])
	var ideas []*store.IdeaDetail
	var breakdowns [][]scoring.CriterionRow
	for _, id := range ids {
		idea, err := store.GetIdeaDetail(r.Context(), s.db, id)
		if err != nil {
			s.fail(w, "compare get", err)
			return
		}
		if idea != nil {
			ideas = append(ideas, idea)
			breakdowns = append(breakdowns, scoring.Breakdown(idea.Criteria))
		}
	}

	// Build one row per criterion with each idea's raw score aligned by column.
	var rows []compareCritRow
	for i, label := range scoring.Labels() {
		row := compareCritRow{Label: label, Values: make([]int, len(breakdowns))}
		for j, bd := range breakdowns {
			if i < len(bd) {
				row.Values[j] = bd[i].Raw
			}
		}
		rows = append(rows, row)
	}

	s.render(w, s.compareTmpl, "layout", struct {
		Ideas []*store.IdeaDetail
		Rows  []compareCritRow
	}{ideas, rows})
}

type editView struct {
	Idea          *store.IdeaDetail
	IndustriesCSV string
	CountriesCSV  string
}

func (s *Server) handleEditForm(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	idea, err := store.GetIdeaDetail(r.Context(), s.db, id)
	if err != nil {
		s.fail(w, "edit get", err)
		return
	}
	if idea == nil {
		http.NotFound(w, r)
		return
	}
	s.render(w, s.editTmpl, "layout", editView{
		Idea:          idea,
		IndustriesCSV: strings.Join(idea.Industries, ", "),
		CountriesCSV:  strings.Join(idea.Countries, ", "),
	})
}

func (s *Server) handleEditSave(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	name := strings.TrimSpace(r.PostForm.Get("idea_name"))
	if name == "" {
		http.Error(w, "idea name is required", http.StatusBadRequest)
		return
	}
	edit := store.IdeaEdit{
		IdeaName:       name,
		Pitch:          strings.TrimSpace(r.PostForm.Get("pitch")),
		PainPoint:      strings.TrimSpace(r.PostForm.Get("pain_point")),
		TargetCustomer: strings.TrimSpace(r.PostForm.Get("target_customer")),
		BuyerPersona:   strings.TrimSpace(r.PostForm.Get("buyer_persona")),
		WhyWork:        strings.TrimSpace(r.PostForm.Get("why_work")),
		WhyFail:        strings.TrimSpace(r.PostForm.Get("why_fail")),
		PossibleMVP:    strings.TrimSpace(r.PostForm.Get("possible_mvp")),
		First10:        strings.TrimSpace(r.PostForm.Get("first_10")),
		Industries:     splitCSV(r.PostForm.Get("industries")),
		Countries:      splitCSV(r.PostForm.Get("countries")),
	}
	if err := store.UpdateIdeaFields(r.Context(), s.db, id, edit); err != nil {
		s.fail(w, "edit save", err)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/ideas/%d", id), http.StatusSeeOther)
}

// --- helpers ---

// parseIDs collects positive int64 ids from query values, accepting both
// repeated (?ids=1&ids=2) and comma-separated (?ids=1,2) forms.
func parseIDs(vals []string) []int64 {
	var out []int64
	seen := map[int64]bool{}
	for _, v := range vals {
		for _, part := range strings.Split(v, ",") {
			n, err := strconv.ParseInt(strings.TrimSpace(part), 10, 64)
			if err == nil && n > 0 && !seen[n] {
				seen[n] = true
				out = append(out, n)
			}
		}
	}
	return out
}

// splitCSV turns "a, b ,c" into ["a","b","c"], dropping blanks.
func splitCSV(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

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

// acronyms are rendered upper/camel-cased instead of title-cased so enum and
// driver values read naturally ("smb_saas" -> "SMB SaaS", not "Smb Saas").
var acronyms = map[string]string{
	"saas": "SaaS", "smb": "SMB", "ai": "AI", "mvp": "MVP", "regtech": "RegTech",
	"api": "API", "ui": "UI", "ux": "UX", "b2b": "B2B", "b2c": "B2C",
	"crm": "CRM", "erp": "ERP", "hr": "HR", "it": "IT", "kpi": "KPI",
	"roi": "ROI", "saa": "SaaS",
}

// labelize converts an enum/driver value like "needs_more_evidence" or
// "smb_saas" into readable text, preserving known acronyms.
func labelize(s string) string {
	if s == "" {
		return ""
	}
	words := strings.Fields(strings.ReplaceAll(s, "_", " "))
	for i, wlc := 0, ""; i < len(words); i++ {
		wlc = strings.ToLower(words[i])
		if ac, ok := acronyms[wlc]; ok {
			words[i] = ac
		} else if i == 0 {
			words[i] = strings.ToUpper(words[i][:1]) + words[i][1:]
		} else {
			words[i] = wlc
		}
	}
	return strings.Join(words, " ")
}
