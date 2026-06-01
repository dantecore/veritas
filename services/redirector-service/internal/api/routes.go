package api

import (
	"log/slog"
	"net/http"
	"net/netip"

	commonhandlers "github.com/nouvadev/veritas/pkg/api/handlers"
)

func Routes(logger *slog.Logger, querier URLQuerier, cache Cache, publisher EventPublisher, trustedProxyCIDRs []netip.Prefix) http.Handler {
	mux := http.NewServeMux()
	u := NewURLHandler(logger, querier, cache, publisher, trustedProxyCIDRs)

	mux.HandleFunc("GET /healthcheck", commonhandlers.Healthcheck)
	mux.HandleFunc("GET /{short_code}", u.RedirectToOriginalURL)

	return mux
}
