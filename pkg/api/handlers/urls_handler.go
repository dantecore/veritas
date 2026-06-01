package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/nouvadev/veritas/pkg/config"
	database "github.com/nouvadev/veritas/pkg/database/sqlc"
	eventsv1 "github.com/nouvadev/veritas/pkg/gen/proto/proto/events/v1"
	"github.com/nouvadev/veritas/pkg/utils"
	"github.com/redis/go-redis/v9"
	"google.golang.org/protobuf/proto"
)

// URLHandler handles all URL-related HTTP requests
type URLHandler struct {
	App *config.AppConfig
}

type URLRequest struct {
	OriginalURL string `json:"original_url"`
}

type URLResponse struct {
	ShortURL string `json:"short_url"`
}

func NewURLHandler(app *config.AppConfig) *URLHandler {
	return &URLHandler{App: app}
}

func (h *URLHandler) CreateShortURL(w http.ResponseWriter, r *http.Request) {
	// get original url from request
	var req URLRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.RespondWithError(w, http.StatusBadRequest, "Invalid request body")
		h.App.Logger.Error("Invalid request body", "error", err)
		return
	}

	if !utils.ValidateURL(req.OriginalURL) {
		utils.RespondWithError(w, http.StatusBadRequest, "Invalid URL")
		h.App.Logger.Error("Invalid URL", "url", req.OriginalURL)
		return
	}

	tx, err := h.App.DB.Begin(r.Context())
	if err != nil {
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to create URL")
		h.App.Logger.Error("Failed to begin URL creation transaction", "error", err)
		return
	}
	defer tx.Rollback(r.Context())

	queries := database.New(tx)
	insertedID, err := queries.CreateURL(r.Context(), req.OriginalURL)
	if err != nil {
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to create URL")
		h.App.Logger.Error("Failed to create URL", "error", err)
		return
	}

	shortCode := utils.ToBase62(uint64(insertedID))

	err = queries.UpdateShortCode(r.Context(), database.UpdateShortCodeParams{
		ShortCode: shortCode,
		ID:        insertedID,
	})
	if err != nil {
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to update short code")
		h.App.Logger.Error("Failed to update short code", "error", err)
		return
	}

	if err := tx.Commit(r.Context()); err != nil {
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to create URL")
		h.App.Logger.Error("Failed to commit URL creation transaction", "error", err)
		return
	}

	// Build complete URL in backend (RESTful best practice)
	baseURL := os.Getenv("BASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8080" // fallback for development
	}
	shortURL := fmt.Sprintf("%s/%s", baseURL, shortCode)

	// Respond to the user with complete URL (single source of truth)
	utils.RespondWithJSON(w, http.StatusCreated, URLResponse{ShortURL: shortURL})
}

func (h *URLHandler) RedirectToOriginalURL(w http.ResponseWriter, r *http.Request) {
	shortCode := r.PathValue("short_code")
	if shortCode == "" {
		utils.RespondWithError(w, http.StatusBadRequest, "Short code is required")
		return
	}

	// 1. Try to get from cache first
	originalURL, err := h.App.Cache.Get(r.Context(), shortCode).Result()
	if err == nil {
		h.App.Logger.Info("cache hit", "short_code", shortCode)
		// Redirect and publish event
		h.publishRedirectEvent(shortCode, originalURL, r)
		http.Redirect(w, r, originalURL, http.StatusFound)
		return
	}

	if !errors.Is(err, redis.Nil) {
		h.App.Logger.Error("redis error", "err", err)
	} else {
		h.App.Logger.Info("cache miss", "short_code", shortCode)
	}

	// 2. If not in cache, get from DB
	originalURL, err = h.App.Querier.GetURLByShortCode(r.Context(), shortCode)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			utils.RespondWithError(w, http.StatusNotFound, "URL not found")
		} else {
			utils.RespondWithError(w, http.StatusInternalServerError, "Failed to get URL")
		}
		h.App.Logger.Error("db error", "err", err)
		return
	}

	// 3. Store in cache for future requests
	if err := h.App.Cache.Set(r.Context(), shortCode, originalURL, 1*time.Hour).Err(); err != nil {
		h.App.Logger.Error("failed to set cache", "err", err)
	}

	// Redirect and publish event
	h.publishRedirectEvent(shortCode, originalURL, r)
	http.Redirect(w, r, originalURL, http.StatusFound)
}

func (h *URLHandler) publishRedirectEvent(shortCode, originalURL string, r *http.Request) {
	event := &eventsv1.RedirectEvent{
		ShortCode:   shortCode,
		OriginalUrl: originalURL,
		UserAgent:   r.UserAgent(),
		IpAddress:   r.RemoteAddr,
	}

	eventBytes, err := proto.Marshal(event)
	if err != nil {
		h.App.Logger.Error("failed to marshal redirect event", "err", err)
		return
	}

	subject := "veritas.redirect.success"
	if err := h.App.NATS.Publish(subject, eventBytes); err != nil {
		h.App.Logger.Error("failed to publish nats event", "err", err)
	} else {
		h.App.Logger.Info("published nats event", "subject", subject)
	}
}
