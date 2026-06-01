package api

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	cachepkg "github.com/nouvadev/veritas/pkg/cache"
	eventsv1 "github.com/nouvadev/veritas/pkg/gen/proto/proto/events/v1"
	"google.golang.org/protobuf/proto"
)

type cacheSetCall struct {
	key   string
	value string
	ttl   time.Duration
}

type fakeCache struct {
	getValue string
	getErr   error
	getCalls []string
	setErr   error
	setCalls []cacheSetCall
}

func (c *fakeCache) Get(_ context.Context, key string) (string, error) {
	c.getCalls = append(c.getCalls, key)
	return c.getValue, c.getErr
}

func (c *fakeCache) Set(_ context.Context, key, value string, ttl time.Duration) error {
	c.setCalls = append(c.setCalls, cacheSetCall{key: key, value: value, ttl: ttl})
	return c.setErr
}

type fakeQuerier struct {
	value string
	err   error
	calls []string
}

func (q *fakeQuerier) GetURLByShortCode(_ context.Context, shortCode string) (string, error) {
	q.calls = append(q.calls, shortCode)
	return q.value, q.err
}

type publishCall struct {
	subject string
	data    []byte
}

type fakePublisher struct {
	err   error
	calls []publishCall
}

func (p *fakePublisher) Publish(subject string, data []byte) error {
	p.calls = append(p.calls, publishCall{subject: subject, data: data})
	return p.err
}

func TestRedirectToOriginalURLCacheHit(t *testing.T) {
	cache := &fakeCache{getValue: "https://example.com/from-cache"}
	querier := &fakeQuerier{}
	publisher := &fakePublisher{}

	rec := serveRedirect(cache, querier, publisher)

	assertRedirect(t, rec, cache.getValue)
	if len(cache.getCalls) != 1 || cache.getCalls[0] != "abc123" {
		t.Fatalf("expected one cache read for abc123, got %#v", cache.getCalls)
	}
	if len(querier.calls) != 0 {
		t.Fatalf("expected no DB calls, got %d", len(querier.calls))
	}
	if len(cache.setCalls) != 0 {
		t.Fatalf("expected no cache writes, got %d", len(cache.setCalls))
	}
	assertPublishedRedirectEvent(t, publisher, cache.getValue)
}

func TestRedirectToOriginalURLCacheMiss(t *testing.T) {
	cache := &fakeCache{getErr: cachepkg.ErrMiss}
	querier := &fakeQuerier{value: "https://example.com/from-db"}
	publisher := &fakePublisher{}

	rec := serveRedirect(cache, querier, publisher)

	assertRedirect(t, rec, querier.value)
	if len(querier.calls) != 1 || querier.calls[0] != "abc123" {
		t.Fatalf("expected one DB call for abc123, got %#v", querier.calls)
	}
	if len(cache.setCalls) != 1 {
		t.Fatalf("expected one cache write, got %d", len(cache.setCalls))
	}
	setCall := cache.setCalls[0]
	if setCall.key != "abc123" || setCall.value != querier.value || setCall.ttl != time.Hour {
		t.Fatalf("unexpected cache write: %#v", setCall)
	}
	assertPublishedRedirectEvent(t, publisher, querier.value)
}

func TestRedirectToOriginalURLNotFound(t *testing.T) {
	cache := &fakeCache{getErr: cachepkg.ErrMiss}
	querier := &fakeQuerier{err: pgx.ErrNoRows}
	publisher := &fakePublisher{}

	rec := serveRedirect(cache, querier, publisher)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, rec.Code)
	}
	if len(cache.setCalls) != 0 {
		t.Fatalf("expected no cache writes, got %d", len(cache.setCalls))
	}
	if len(publisher.calls) != 0 {
		t.Fatalf("expected no published events, got %d", len(publisher.calls))
	}
}

func TestRedirectToOriginalURLFallsBackWhenCacheGetFails(t *testing.T) {
	cache := &fakeCache{getErr: errors.New("redis unavailable")}
	querier := &fakeQuerier{value: "https://example.com/from-db"}
	publisher := &fakePublisher{}

	rec := serveRedirect(cache, querier, publisher)

	assertRedirect(t, rec, querier.value)
	if len(querier.calls) != 1 {
		t.Fatalf("expected one DB call, got %d", len(querier.calls))
	}
}

func TestRedirectToOriginalURLReturnsInternalServerErrorWhenDBFails(t *testing.T) {
	cache := &fakeCache{getErr: cachepkg.ErrMiss}
	querier := &fakeQuerier{err: errors.New("database unavailable")}
	publisher := &fakePublisher{}

	rec := serveRedirect(cache, querier, publisher)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, rec.Code)
	}
	if len(cache.setCalls) != 0 {
		t.Fatalf("expected no cache writes, got %d", len(cache.setCalls))
	}
	if len(publisher.calls) != 0 {
		t.Fatalf("expected no published events, got %d", len(publisher.calls))
	}
}

func TestRedirectToOriginalURLContinuesWhenCacheSetFails(t *testing.T) {
	cache := &fakeCache{
		getErr: cachepkg.ErrMiss,
		setErr: errors.New("redis unavailable"),
	}
	querier := &fakeQuerier{value: "https://example.com/from-db"}
	publisher := &fakePublisher{}

	rec := serveRedirect(cache, querier, publisher)

	assertRedirect(t, rec, querier.value)
}

func TestRedirectToOriginalURLContinuesWhenPublishFails(t *testing.T) {
	cache := &fakeCache{getValue: "https://example.com/from-cache"}
	querier := &fakeQuerier{}
	publisher := &fakePublisher{err: errors.New("nats unavailable")}

	rec := serveRedirect(cache, querier, publisher)

	assertRedirect(t, rec, cache.getValue)
	if len(publisher.calls) != 1 {
		t.Fatalf("expected one publish attempt, got %d", len(publisher.calls))
	}
}

func serveRedirect(cache Cache, querier URLQuerier, publisher EventPublisher) *httptest.ResponseRecorder {
	router := Routes(testLogger(), querier, cache, publisher)
	req := httptest.NewRequest(http.MethodGet, "/abc123", nil)
	req.Header.Set("User-Agent", "test-agent")
	req.RemoteAddr = "192.0.2.1:1234"
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	return rec
}

func assertRedirect(t *testing.T, rec *httptest.ResponseRecorder, wantLocation string) {
	t.Helper()
	if rec.Code != http.StatusFound {
		t.Fatalf("expected status %d, got %d", http.StatusFound, rec.Code)
	}
	if location := rec.Header().Get("Location"); location != wantLocation {
		t.Fatalf("expected Location %q, got %q", wantLocation, location)
	}
}

func assertPublishedRedirectEvent(t *testing.T, publisher *fakePublisher, wantOriginalURL string) {
	t.Helper()
	if len(publisher.calls) != 1 {
		t.Fatalf("expected one published event, got %d", len(publisher.calls))
	}

	call := publisher.calls[0]
	if call.subject != "veritas.redirect.success" {
		t.Fatalf("expected redirect success subject, got %q", call.subject)
	}

	var event eventsv1.RedirectEvent
	if err := proto.Unmarshal(call.data, &event); err != nil {
		t.Fatalf("unmarshal redirect event: %v", err)
	}
	if event.ShortCode != "abc123" || event.OriginalUrl != wantOriginalURL {
		t.Fatalf("unexpected redirect event: %#v", event)
	}
	if event.UserAgent != "test-agent" || event.IpAddress != "192.0.2.1:1234" {
		t.Fatalf("unexpected request metadata: %#v", event)
	}
}
