package api

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
	database "github.com/nouvadev/veritas/pkg/database/sqlc"
	"github.com/nouvadev/veritas/pkg/utils"
)

type URLHandler struct {
	Logger *slog.Logger
	DB     *pgxpool.Pool
}

type URLRequest struct {
	OriginalURL string `json:"original_url"`
}

type URLResponse struct {
	ShortURL string `json:"short_url"`
}

func NewURLHandler(logger *slog.Logger, db *pgxpool.Pool) *URLHandler {
	return &URLHandler{
		Logger: logger,
		DB:     db,
	}
}

func (h *URLHandler) CreateShortURL(w http.ResponseWriter, r *http.Request) {
	// get original url from request
	var req URLRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.RespondWithError(w, http.StatusBadRequest, "Invalid request body")
		h.Logger.Error("Invalid request body", "error", err)
		return
	}

	if !utils.ValidateURL(req.OriginalURL) {
		utils.RespondWithError(w, http.StatusBadRequest, "Invalid URL")
		h.Logger.Error("Invalid URL", "url", req.OriginalURL)
		return
	}

	tx, err := h.DB.Begin(r.Context())
	if err != nil {
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to create URL")
		h.Logger.Error("Failed to begin URL creation transaction", "error", err)
		return
	}
	defer tx.Rollback(r.Context())

	queries := database.New(tx)
	insertedID, err := queries.CreateURL(r.Context(), req.OriginalURL)
	if err != nil {
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to create URL")
		h.Logger.Error("Failed to create URL", "error", err)
		return
	}

	shortCode := utils.ToBase62(uint64(insertedID))

	err = queries.UpdateShortCode(r.Context(), database.UpdateShortCodeParams{
		ShortCode: shortCode,
		ID:        insertedID,
	})
	if err != nil {
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to update short code")
		h.Logger.Error("Failed to update short code", "error", err)
		return
	}

	if err := tx.Commit(r.Context()); err != nil {
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to create URL")
		h.Logger.Error("Failed to commit URL creation transaction", "error", err)
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
