package ingest

import (
	"context"
	"crypto/sha256"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mateusgetulio/papertrail/internal/chunk"
	"github.com/mateusgetulio/papertrail/internal/compliance"
	"github.com/mateusgetulio/papertrail/internal/discovery"
	"github.com/mateusgetulio/papertrail/internal/extract"
	"github.com/mateusgetulio/papertrail/internal/fetch"
	"github.com/mateusgetulio/papertrail/internal/llm"
	"github.com/mateusgetulio/papertrail/internal/metadata"
	"github.com/mateusgetulio/papertrail/internal/store"
)

// Pipeline orchestrates the full ingestion flow for a single candidate URL:
//
//	fetch → gate → detect → metadata → extract text → chunk → embed → store → purge
type Pipeline struct {
	fetcher  *fetch.Fetcher
	gate     compliance.Gate
	policies *compliance.PolicyRegistry
	llm      llm.Client
	db       *pgxpool.Pool
}

func New(
	fetcher *fetch.Fetcher,
	gate compliance.Gate,
	policies *compliance.PolicyRegistry,
	llmClient llm.Client,
	db *pgxpool.Pool,
) *Pipeline {
	return &Pipeline{
		fetcher:  fetcher,
		gate:     gate,
		policies: policies,
		llm:      llmClient,
		db:       db,
	}
}

// Run processes a single allowed discovery candidate end-to-end.
// Returns (true, nil) when the document was stored, (false, nil) when gracefully
// skipped (gated, wrong type, empty text), and (false, err) on hard failures.
func (p *Pipeline) Run(ctx context.Context, c discovery.Candidate) (bool, error) {
	log := slog.With("url", c.CanonicalURL)

	// 1. Upsert source row
	policy := p.policies.Policy(c.Domain)
	sourceID, err := store.EnsureSource(ctx, p.db, c.Domain, c.Domain, policy.TrustTier)
	if err != nil {
		return false, fmt.Errorf("ensure source: %w", err)
	}

	// 2. HEAD pre-check: skip files that are too large before downloading
	if ok, reason := p.fetcher.HeadOK(ctx, c.CanonicalURL); !ok {
		_ = store.InsertFetchAudit(ctx, p.db, nil, c.CanonicalURL, "skipped", reason, true, false)
		log.Warn("HEAD check failed, skipping", "reason", reason)
		return false, nil
	}

	// 3. Fetch
	log.Info("fetching")
	result, err := p.fetcher.Fetch(ctx, c.CanonicalURL)
	if err != nil {
		_ = store.InsertFetchAudit(ctx, p.db, nil, c.CanonicalURL, "failed", err.Error(), true, false)
		return false, fmt.Errorf("fetch: %w", err)
	}

	// 3. Post-fetch gate: detect gated/bot-wall content
	if fetch.IsGated(result) {
		_ = store.InsertFetchAudit(ctx, p.db, nil, c.CanonicalURL, "skipped", "post-fetch gating detected", true, true)
		log.Warn("post-fetch gate: skipping gated document")
		return false, nil
	}
	if result.StatusCode != 200 {
		_ = store.InsertFetchAudit(ctx, p.db, nil, c.CanonicalURL, "skipped",
			fmt.Sprintf("http status %d", result.StatusCode), true, false)
		return false, nil
	}

	// 4. Detect content kind
	kind := fetch.Detect(result)
	if kind == fetch.KindUnknown {
		_ = store.InsertFetchAudit(ctx, p.db, nil, c.CanonicalURL, "skipped", "unknown content type", true, false)
		log.Warn("unknown content type, skipping")
		return false, nil
	}
	log.Info("detected", "kind", kind)

	// 5. Extract metadata
	var meta metadata.DocMeta
	var rawText string

	switch kind {
	case fetch.KindPDF:
		tmpPath, err := metadata.WriteTempFile(result.Body)
		if err != nil {
			return false, fmt.Errorf("write temp: %w", err)
		}
		defer os.Remove(tmpPath) // compliance: no full-text retention

		meta, err = metadata.FromPDF(ctx, tmpPath)
		if err != nil {
			log.Warn("pdf metadata failed", "err", err)
		}
		rawText, err = extract.PDF(ctx, tmpPath)
		if err != nil {
			return false, fmt.Errorf("pdf extract: %w", err)
		}

	case fetch.KindHTML:
		htmlStr := string(result.Body)
		meta, err = metadata.FromHTML(htmlStr)
		if err != nil {
			log.Warn("html metadata failed", "err", err)
		}
		rawText, err = metadata.ExtractMainText(htmlStr)
		if err != nil {
			return false, fmt.Errorf("html extract: %w", err)
		}
	}

	if strings.TrimSpace(rawText) == "" {
		_ = store.InsertFetchAudit(ctx, p.db, nil, c.CanonicalURL, "skipped", "empty text after extraction", true, false)
		log.Warn("empty text, skipping")
		return false, nil
	}
	// Sanitize to valid UTF-8 (pdftotext can emit invalid byte sequences)
	rawText = strings.ToValidUTF8(rawText, "")
	log.Info("extracted", "chars", len(rawText), "pages", meta.PageCount)

	// 6. Insert document row
	contentType := "html"
	if kind == fetch.KindPDF {
		contentType = "pdf"
	}
	docID, err := store.InsertDocument(ctx, p.db, store.InsertDocumentParams{
		SourceID:      sourceID,
		CanonicalURL:  c.CanonicalURL,
		URLHash:       c.URLHash,
		ContentHash:   result.ContentHash,
		ContentType:   contentType,
		Title:         meta.Title,
		Authors:       meta.Authors,
		Publisher:     meta.Publisher,
		PublishedAt:   meta.PublishedAt,
		Language:      meta.Language,
		PageCount:     meta.PageCount,
		DiscoveredVia: c.DiscoveredVia,
		State:         "fetched",
	})
	if err != nil {
		return false, fmt.Errorf("insert document: %w", err)
	}

	_ = store.InsertFetchAudit(ctx, p.db, &docID, c.CanonicalURL, "allowed", "ingested", true, false)

	// 7. Chunk
	chunks := chunk.Chunk(rawText, chunk.DefaultMaxChars, chunk.DefaultOverlap)
	log.Info("chunked", "count", len(chunks))

	// Build short excerpts (≤25 words each, compliance docs/03 §6)
	excerpts := make([]string, len(chunks))
	for i, ch := range chunks {
		excerpts[i] = excerpt25(ch)
	}

	// 8. Embed all chunks in one batch call
	embeddings, err := p.llm.Embed(ctx, excerpts)
	if err != nil {
		log.Warn("embedding failed", "err", err)
		embeddings = make([][]float32, len(chunks))
	}

	// 9. Store chunks + embeddings; rawText is NOT persisted (compliance)
	charPos := 0
	for i, ch := range chunks {
		var emb []float32
		if i < len(embeddings) {
			emb = embeddings[i]
		}
		tokens := len(strings.Fields(ch))
		if err := store.InsertChunk(ctx, p.db,
			docID, i, charPos, charPos+len(ch), tokens,
			excerpts[i], emb,
		); err != nil {
			log.Warn("insert chunk failed", "idx", i, "err", err)
		}
		charPos += len(ch)
	}

	// 10. Transition state to "extracted" (full text already discarded — never persisted)
	if err := store.UpdateDocumentState(ctx, p.db, docID, "extracted"); err != nil {
		log.Warn("state update failed", "err", err)
	}

	log.Info("ingested", "doc_id", docID, "chunks", len(chunks))
	return true, nil
}

// excerpt25 returns up to the first 25 words of a chunk for evidence storage.
func excerpt25(text string) string {
	words := strings.Fields(text)
	if len(words) > 25 {
		words = words[:25]
	}
	return strings.Join(words, " ")
}

// queryHash returns a SHA-256 of the query string for dedup in search_query_log.
func QueryHash(query string) []byte {
	h := sha256.Sum256([]byte(query))
	return h[:]
}
