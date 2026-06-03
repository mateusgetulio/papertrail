package agents

import (
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mateusgetulio/papertrail/internal/config"
)

// Context carries the shared dependencies passed to every agent in a run.
type Context struct {
	DB         *pgxpool.Pool
	Cfg        *config.Config
	RunID      string
	Log        *slog.Logger
	ReportsDir string
}
