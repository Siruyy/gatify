package proxy

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/Siruyy/gatify/internal/storage"
)

type fakeLimiter struct {
	result storage.Result
	err    error
	calls  int
	key    string
}

func (f *fakeLimiter) Allow(_ context.Context, key string) (storage.Result, error) {
	f.calls++
	f.key = key
	if f.err != nil {
		return storage.Result{}, f.err
	}
	return f.result, nil
}

func TestNew_Validation(t *testing.T) {
	if _, err := New(nil, nil); err == nil {
		t.Fatal("expected error when target is nil")
	}

	badScheme, _ := url.Parse("ftp://example.com")
	if _, err := New(badScheme, nil); err == nil {
		t.Fatal("expected error for non-http scheme")
	}

	noHost, _ := url.Parse("http://")
	if _, err := New(noHost, nil); err == nil {
		t.Fatal("expected error for missing host")
	}
}

func TestServeHTTP_AllowsAndProxies(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/anything" {
			t.Fatalf("expected backend path /anything, got %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("backend-ok"))
	}))
	defer backend.Close()

	targetURL, _ := url.Parse(backend.URL)
	lim := &fakeLimiter{
		result: storage.Result{
			Allowed:   true,
			Limit:     10,
			Remaining: 9,
			ResetAt:   time.Now().Add(30 * time.Second),
		},
	}

	gp, err := New(targetURL, lim)
	if err != nil {
		t.Fatalf("failed to create proxy: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "http://gatify.local/anything", nil)
	req.RemoteAddr = "1.2.3.4:12345"
	res := httptest.NewRecorder()

	gp.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", res.Code)
	}
	if strings.TrimSpace(res.Body.String()) != "backend-ok" {
		t.Fatalf("unexpected backend response: %q", res.Body.String())
	}
	if lim.calls != 1 {
		t.Fatalf("expected limiter call count 1, got %d", lim.calls)
	}
	if lim.key != "1.2.3.4" {
		t.Fatalf("expected limiter key 1.2.3.4, got %q", lim.key)
	}
}

func TestServeHTTP_RateLimited(t *testing.T) {
	backendCalled := false
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		backendCalled = true
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	targetURL, _ := url.Parse(backend.URL)
	lim := &fakeLimiter{
		result: storage.Result{
			Allowed:   false,
			Limit:     3,
			Remaining: 0,
			ResetAt:   time.Now().Add(30 * time.Second),
		},
	}

	gp, err := New(targetURL, lim)
	if err != nil {
		t.Fatalf("failed to create proxy: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "http://gatify.local/blocked", nil)
	req.RemoteAddr = "9.9.9.9:4321"
	res := httptest.NewRecorder()

	gp.ServeHTTP(res, req)

	if res.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", res.Code)
	}
	if !strings.Contains(res.Body.String(), "rate limit exceeded") {
		t.Fatalf("expected rate limit message, got %q", res.Body.String())
	}
	if got := res.Header().Get("X-RateLimit-Limit"); got != "3" {
		t.Fatalf("expected X-RateLimit-Limit=3, got %q", got)
	}
	if got := res.Header().Get("X-RateLimit-Remaining"); got != "0" {
		t.Fatalf("expected X-RateLimit-Remaining=0, got %q", got)
	}
	if backendCalled {
		t.Fatal("backend should not be called when rate limited")
	}
}

func TestServeHTTP_LimiterError(t *testing.T) {
	backendCalled := false
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		backendCalled = true
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	targetURL, _ := url.Parse(backend.URL)
	lim := &fakeLimiter{err: errors.New("limiter down")}

	gp, err := New(targetURL, lim)
	if err != nil {
		t.Fatalf("failed to create proxy: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "http://gatify.local/fail", nil)
	res := httptest.NewRecorder()

	gp.ServeHTTP(res, req)

	if res.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", res.Code)
	}
	if !strings.Contains(res.Body.String(), "rate limiter unavailable") {
		t.Fatalf("expected limiter unavailable message, got %q", res.Body.String())
	}
	if backendCalled {
		t.Fatal("backend should not be called when limiter errors")
	}
}

func TestClientKey_TrustProxy(t *testing.T) {
	targetURL, _ := url.Parse("http://127.0.0.1:9999")
	gp, _ := New(targetURL, nil, WithTrustProxy(true))

	req := httptest.NewRequest(http.MethodGet, "http://gatify.local/x", nil)
	req.Header.Set("X-Forwarded-For", "8.8.8.8, 10.0.0.1")
	req.RemoteAddr = "127.0.0.1:5555"

	if got := gp.clientKey(req); got != "8.8.8.8" {
		t.Fatalf("expected forwarded client key with trust proxy, got %q", got)
	}
}

func TestClientKey_NoTrustProxy(t *testing.T) {
	targetURL, _ := url.Parse("http://127.0.0.1:9999")
	gp, _ := New(targetURL, nil)

	req := httptest.NewRequest(http.MethodGet, "http://gatify.local/x", nil)
	req.Header.Set("X-Forwarded-For", "8.8.8.8, 10.0.0.1")
	req.RemoteAddr = "127.0.0.1:5555"

	if got := gp.clientKey(req); got != "127.0.0.1" {
		t.Fatalf("expected RemoteAddr key without trust proxy, got %q", got)
	}
}
