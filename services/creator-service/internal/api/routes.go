package api

import (
	"log/slog"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	commonhandlers "github.com/nouvadev/veritas/pkg/api/handlers"
)

func Routes(logger *slog.Logger, db *pgxpool.Pool) http.Handler {
	mux := http.NewServeMux()
	u := NewURLHandler(logger, db)

	mux.HandleFunc("GET /api/healthcheck", commonhandlers.Healthcheck)
	mux.HandleFunc("POST /api/create", u.CreateShortURL)

	return mux
}
