package api

import (
	"log/slog"
	"net/http"

	commonhandlers "github.com/nouvadev/veritas/pkg/api/handlers"
)

func Routes(logger *slog.Logger, querier URLQuerier, cache Cache, publisher EventPublisher) http.Handler {
	mux := http.NewServeMux()
	u := NewURLHandler(logger, querier, cache, publisher)

	mux.HandleFunc("GET /healthcheck", commonhandlers.Healthcheck)
	mux.HandleFunc("GET /{short_code}", u.RedirectToOriginalURL)

	return mux
}
