package api

import (
	"net/http"

	"github.com/nouvadev/veritas/pkg/api/handlers"
	"github.com/nouvadev/veritas/pkg/config"
)

func CreateURLRoutes(app *config.AppConfig) http.Handler {
	mux := http.NewServeMux()

	h := handlers.NewHealthcheckHandler(app)
	u := handlers.NewURLHandler(app)

	mux.HandleFunc("GET /api/healthcheck", h.HealthcheckHandler)
	mux.HandleFunc("POST /api/create", u.CreateShortURL)

	return mux
}

func RedirectRoutes(app *config.AppConfig) http.Handler {
	mux := http.NewServeMux()

	h := handlers.NewHealthcheckHandler(app)
	u := handlers.NewURLHandler(app)

	mux.HandleFunc("GET /healthcheck", h.HealthcheckHandler)
	mux.HandleFunc("GET /{short_code}", u.RedirectToOriginalURL)

	return mux
}
