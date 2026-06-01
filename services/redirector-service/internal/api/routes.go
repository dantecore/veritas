package api

import (
	"log/slog"
	"net/http"

	commonhandlers "github.com/nouvadev/veritas/pkg/api/handlers"
	database "github.com/nouvadev/veritas/pkg/database/sqlc"
	"github.com/redis/go-redis/v9"
)

func Routes(logger *slog.Logger, querier database.Querier, cache *redis.Client, publisher EventPublisher) http.Handler {
	mux := http.NewServeMux()
	u := NewURLHandler(logger, querier, cache, publisher)

	mux.HandleFunc("GET /healthcheck", commonhandlers.Healthcheck)
	mux.HandleFunc("GET /{short_code}", u.RedirectToOriginalURL)

	return mux
}
