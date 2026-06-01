package api

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/netip"
	"time"

	"github.com/jackc/pgx/v5"
	cachepkg "github.com/nouvadev/veritas/pkg/cache"
	eventsv1 "github.com/nouvadev/veritas/pkg/gen/proto/proto/events/v1"
	"github.com/nouvadev/veritas/pkg/utils"
	"google.golang.org/protobuf/proto"
)

type Cache interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key, value string, ttl time.Duration) error
}

type URLQuerier interface {
	GetURLByShortCode(ctx context.Context, shortCode string) (string, error)
}

type EventPublisher interface {
	Publish(subject string, data []byte) error
}

type URLHandler struct {
	Logger            *slog.Logger
	Querier           URLQuerier
	Cache             Cache
	Publisher         EventPublisher
	TrustedProxyCIDRs []netip.Prefix
}

func NewURLHandler(logger *slog.Logger, querier URLQuerier, cache Cache, publisher EventPublisher, trustedProxyCIDRs []netip.Prefix) *URLHandler {
	return &URLHandler{
		Logger:            logger,
		Querier:           querier,
		Cache:             cache,
		Publisher:         publisher,
		TrustedProxyCIDRs: trustedProxyCIDRs,
	}
}

func (h *URLHandler) RedirectToOriginalURL(w http.ResponseWriter, r *http.Request) {
	shortCode := r.PathValue("short_code")
	if shortCode == "" {
		utils.RespondWithError(w, http.StatusBadRequest, "Short code is required")
		return
	}

	// 1. Try to get from cache first
	originalURL, err := h.Cache.Get(r.Context(), shortCode)
	if err == nil {
		h.Logger.Info("cache hit", "short_code", shortCode)
		// Redirect and publish event
		h.publishRedirectEvent(shortCode, originalURL, r)
		http.Redirect(w, r, originalURL, http.StatusFound)
		return
	}

	if !errors.Is(err, cachepkg.ErrMiss) {
		h.Logger.Error("redis error", "err", err)
	} else {
		h.Logger.Info("cache miss", "short_code", shortCode)
	}

	// 2. If not in cache, get from DB
	originalURL, err = h.Querier.GetURLByShortCode(r.Context(), shortCode)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			utils.RespondWithError(w, http.StatusNotFound, "URL not found")
		} else {
			utils.RespondWithError(w, http.StatusInternalServerError, "Failed to get URL")
		}
		h.Logger.Error("db error", "err", err)
		return
	}

	// 3. Store in cache for future requests
	if err := h.Cache.Set(r.Context(), shortCode, originalURL, 1*time.Hour); err != nil {
		h.Logger.Error("failed to set cache", "err", err)
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
		IpAddress:   clientIPAddress(r, h.TrustedProxyCIDRs),
	}

	eventBytes, err := proto.Marshal(event)
	if err != nil {
		h.Logger.Error("failed to marshal redirect event", "err", err)
		return
	}

	subject := "veritas.redirect.success"
	if err := h.Publisher.Publish(subject, eventBytes); err != nil {
		h.Logger.Error("failed to publish nats event", "err", err)
	} else {
		h.Logger.Info("published nats event", "subject", subject)
	}
}
