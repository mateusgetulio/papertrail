package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/joho/godotenv"
	"github.com/mateusgetulio/papertrail/internal/analysis"
	"github.com/mateusgetulio/papertrail/internal/api"
	"github.com/mateusgetulio/papertrail/internal/chunk"
	"github.com/mateusgetulio/papertrail/internal/compliance"
	"github.com/mateusgetulio/papertrail/internal/config"
	"github.com/mateusgetulio/papertrail/internal/discovery"
	"github.com/mateusgetulio/papertrail/internal/extract"
	"github.com/mateusgetulio/papertrail/internal/fetch"
	"github.com/mateusgetulio/papertrail/internal/ingest"
	"github.com/mateusgetulio/papertrail/internal/llm"
	pipelinepkg "github.com/mateusgetulio/papertrail/internal/pipeline"
	"github.com/mateusgetulio/papertrail/internal/scoring"
	"github.com/mateusgetulio/papertrail/internal/search"
	bravepkg "github.com/mateusgetulio/papertrail/internal/search/brave"
	"github.com/mateusgetulio/papertrail/internal/store"
)

func main() {
	_ = godotenv.Load()

	spike := flag.NewFlagSet("spike", flag.ExitOnError)
	pdfPath := spike.String("pdf", "", "Path to a local PDF file to analyse")

	discover := flag.NewFlagSet("discover", flag.ExitOnError)
	discoverLimit := discover.Int("limit", 5, "Results per query")
	discoverDryRun := discover.Bool("dry-run", false, "Print candidates without saving")

	ingestCmd := flag.NewFlagSet("ingest", flag.ExitOnError)
	ingestLimit := ingestCmd.Int("limit", 3, "Max documents to ingest per run")

	analyseCmd := flag.NewFlagSet("analyse", flag.ExitOnError)
	analyseLimit := analyseCmd.Int("limit", 10, "Max documents to analyse per run")

	serveCmd := flag.NewFlagSet("serve", flag.ExitOnError)
	serveAddr := serveCmd.String("addr", ":8080", "Address to listen on")

	pipelineCmd := flag.NewFlagSet("pipeline", flag.ExitOnError)
	pipelineLimit := pipelineCmd.Int("limit", 10, "Per-stage cap (documents / candidates)")
	pipelineIngest := pipelineCmd.Bool("ingest", false, "Run live discovery + ingest in Agent 1 (uses Brave + OpenAI)")
	pipelineAnalyse := pipelineCmd.Bool("analyse", false, "Run LLM analysis in Agent 2 over freshly-extracted docs (uses OpenAI)")

	if len(os.Args) < 2 {
		fmt.Println("Usage: papertrail <spike|discover|ingest|analyse|serve|pipeline> [flags]")
		os.Exit(1)
	}

	switch os.Args[1] {
	case "spike":
		_ = spike.Parse(os.Args[2:])
		if *pdfPath == "" {
			fmt.Println("--pdf is required")
			os.Exit(1)
		}
		if err := runSpike(*pdfPath); err != nil {
			slog.Error("spike failed", "err", err)
			os.Exit(1)
		}
	case "discover":
		_ = discover.Parse(os.Args[2:])
		if err := runDiscover(*discoverLimit, *discoverDryRun); err != nil {
			slog.Error("discover failed", "err", err)
			os.Exit(1)
		}
	case "ingest":
		_ = ingestCmd.Parse(os.Args[2:])
		if err := runIngest(*ingestLimit); err != nil {
			slog.Error("ingest failed", "err", err)
			os.Exit(1)
		}
	case "analyse":
		_ = analyseCmd.Parse(os.Args[2:])
		if err := runAnalyse(*analyseLimit); err != nil {
			slog.Error("analyse failed", "err", err)
			os.Exit(1)
		}
	case "serve":
		_ = serveCmd.Parse(os.Args[2:])
		if err := runServe(*serveAddr); err != nil {
			slog.Error("serve failed", "err", err)
			os.Exit(1)
		}
	case "pipeline":
		_ = pipelineCmd.Parse(os.Args[2:])
		if err := runPipeline(*pipelineIngest, *pipelineAnalyse, *pipelineLimit); err != nil {
			slog.Error("pipeline failed", "err", err)
			os.Exit(1)
		}
	default:
		fmt.Printf("unknown command: %s\n", os.Args[1])
		os.Exit(1)
	}
}

func runSpike(pdfPath string) error {
	ctx := context.Background()

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}

	slog.Info("extracting text", "path", pdfPath)
	text, err := extract.PDF(ctx, pdfPath)
	if err != nil {
		return fmt.Errorf("extract: %w", err)
	}
	slog.Info("extracted", "chars", len(text))

	chunks := chunk.Chunk(text, chunk.DefaultMaxChars, chunk.DefaultOverlap)
	slog.Info("chunked", "count", len(chunks))

	client := llm.NewOpenAI(cfg.OpenAIKey, cfg.OpenAIModel)

	// Step 1: triage
	sampleSize := 3
	if len(chunks) < sampleSize {
		sampleSize = len(chunks)
	}
	triage, err := client.Triage(ctx, fmt.Sprintf("file: %s", pdfPath), chunks[:sampleSize])
	if err != nil {
		return fmt.Errorf("triage: %w", err)
	}
	slog.Info("triage", "relevant", triage.Relevant, "topics", triage.Topics)
	if !triage.Relevant {
		fmt.Println("Document not relevant:", triage.Reason)
		return nil
	}

	// Step 2: extract signals
	signals, err := client.ExtractSignals(ctx, chunks, pdfPath)
	if err != nil {
		return fmt.Errorf("signals: %w", err)
	}
	slog.Info("signals extracted", "count", len(signals))
	if len(signals) == 0 {
		fmt.Println("No disruption signals found.")
		return nil
	}

	// Step 3: generate candidate ideas
	ideas, err := client.GenerateIdeas(ctx, signals, nil, pdfPath)
	if err != nil {
		return fmt.Errorf("ideas: %w", err)
	}
	slog.Info("ideas generated", "count", len(ideas))

	// Step 4: score each surviving idea
	for i := range ideas {
		sub, err := client.Score(ctx, ideas[i], chunks)
		if err != nil {
			slog.Warn("scoring failed", "idea", ideas[i].IdeaName, "err", err)
			continue
		}
		sub.AvgSourceTrustWeight = 0.80 // default; real value comes from source registry
		sub.DistinctSourceDocs = 1
		ideas[i].OverallScore = scoring.Compute(sub)
	}

	// Output
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(ideas)
}

func runDiscover(limit int, dryRun bool) error {
	ctx := context.Background()

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}

	var provider search.Provider
	if cfg.BraveAPIKey != "" {
		provider = bravepkg.New(cfg.BraveAPIKey)
		slog.Info("search provider", "name", "brave")
	} else {
		return fmt.Errorf("BRAVE_API_KEY not set — no search provider available")
	}

	robots := compliance.NewRobotsCache()
	rates := compliance.NewRateLimiterRegistry()
	policies := compliance.NewPolicyRegistry()
	gate := compliance.NewGate(robots, rates, policies)

	runner := discovery.NewRunner(provider, gate)

	queries := search.DefaultTemplates()
	var expanded []string
	for _, t := range queries {
		expanded = append(expanded, t.Expand()...)
	}
	slog.Info("discover", "total_queries", len(expanded), "limit_per_query", limit)

	out := make(chan discovery.Candidate, 64)
	var candidates []discovery.Candidate

	go func() {
		defer close(out)
		if err := runner.Run(ctx, expanded, limit, out); err != nil {
			slog.Warn("runner error", "err", err)
		}
	}()

	for c := range out {
		candidates = append(candidates, c)
		slog.Info("candidate", "url", c.CanonicalURL, "type", c.ContentTypeGuess, "domain", c.Domain)
	}

	slog.Info("discover complete", "allowed", len(candidates))

	if dryRun {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(candidates)
	}
	return nil
}

func runIngest(limit int) error {
	ctx := context.Background()

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}

	db, err := store.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("db: %w", err)
	}
	defer db.Close()

	llmClient := llm.NewOpenAI(cfg.OpenAIKey, cfg.OpenAIModel)

	var provider search.Provider
	if cfg.BraveAPIKey != "" {
		provider = bravepkg.New(cfg.BraveAPIKey)
	} else {
		return fmt.Errorf("BRAVE_API_KEY not set")
	}

	robots := compliance.NewRobotsCache()
	rates := compliance.NewRateLimiterRegistry()
	policies := compliance.NewPolicyRegistry()
	gate := compliance.NewGate(robots, rates, policies)

	runner := discovery.NewRunner(provider, gate)

	queries := search.DefaultTemplates()
	var expanded []string
	for _, t := range queries {
		expanded = append(expanded, t.Expand()...)
	}

	// Use a child context so we can cancel discovery once we have enough docs.
	discCtx, cancelDisc := context.WithCancel(ctx)
	defer cancelDisc()

	out := make(chan discovery.Candidate, 16)
	go func() {
		defer close(out)
		_ = runner.Run(discCtx, expanded, 3, out)
	}()

	fetcher := fetch.New(fetch.NewMemCache())
	pipeline := ingest.New(fetcher, gate, policies, llmClient, db)

	ingested := 0
	for c := range out {
		if ingested >= limit {
			cancelDisc() // stop discovery goroutine
			// drain so the goroutine can exit
			go func() {
				for range out {
				}
			}()
			break
		}
		slog.Info("ingesting", "url", c.CanonicalURL)
		stored, err := pipeline.Run(ctx, c)
		if err != nil {
			slog.Warn("ingest failed", "url", c.CanonicalURL, "err", err)
			continue
		}
		if stored {
			ingested++
		}
	}

	slog.Info("ingest complete", "ingested", ingested)
	return nil
}

func runAnalyse(limit int) error {
	ctx := context.Background()

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}

	db, err := store.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("db: %w", err)
	}
	defer db.Close()

	llmClient := llm.NewOpenAI(cfg.OpenAIKey, cfg.OpenAIModel)
	agent := analysis.NewAgent(llmClient, db)

	docs, err := store.GetExtractedDocuments(ctx, db, limit)
	if err != nil {
		return fmt.Errorf("get documents: %w", err)
	}
	slog.Info("documents to analyse", "count", len(docs))

	totalIdeas := 0
	for _, doc := range docs {
		n, err := agent.Analyse(ctx, doc)
		if err != nil {
			slog.Warn("analyse failed", "doc_id", doc.ID, "err", err)
			continue
		}
		totalIdeas += n
	}

	slog.Info("analyse complete", "docs", len(docs), "ideas_stored", totalIdeas)
	return nil
}

func runServe(addr string) error {
	ctx := context.Background()

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}

	db, err := store.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("db: %w", err)
	}
	defer db.Close()

	srv, err := api.NewServer(db)
	if err != nil {
		return fmt.Errorf("server: %w", err)
	}

	slog.Info("review dashboard listening", "addr", addr)
	httpSrv := &http.Server{
		Addr:              addr,
		Handler:           srv.Routes(),
		ReadHeaderTimeout: 10 * time.Second,
	}
	return httpSrv.ListenAndServe()
}

func runPipeline(ingestLive, analyseLive bool, limit int) error {
	ctx := context.Background()

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}

	db, err := store.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("db: %w", err)
	}
	defer db.Close()

	res, err := pipelinepkg.Run(ctx, db, cfg, slog.Default(), pipelinepkg.Options{
		Ingest:  ingestLive,
		Analyse: analyseLive,
		Limit:   limit,
	})
	if err != nil {
		return err
	}

	fmt.Printf("\nRun %s — decision: %s\n", res.RunID, res.Gate.Decision)
	fmt.Printf("  documents prepared: %d\n", len(res.Operator.DocumentsPrepared))
	fmt.Printf("  ranked candidates:  %d\n", len(res.Analyst.RankedCandidates))
	if res.Report.ReportMarkdownLocation != "" {
		fmt.Printf("  report:             %s\n", res.Report.ReportMarkdownLocation)
	}
	fmt.Printf("  blockers: %d · warnings: %d · backlog: %d\n",
		len(res.Gate.Blockers), len(res.Gate.Warnings), len(res.Gate.NiceToHaveBacklog))
	if res.Stopped {
		fmt.Println("  ⚠️  chain stopped early for a blocker — see blockers above.")
		for _, b := range res.Gate.Blockers {
			fmt.Printf("    - [%s] %s → %s\n", b.SourceAgent, b.Issue, b.RecommendedFix)
		}
	}
	return nil
}
