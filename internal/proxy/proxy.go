// Package proxy provides HTTP reverse proxy functionality.
package proxy

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/Siruyy/gatify/internal/storage"
)

// RateLimiter defines the limiter behavior required by the proxy.
type RateLimiter interface {
	Allow(ctx context.Context, key string) (storage.Result, error)
}

// GatewayProxy is an HTTP reverse proxy with optional rate limiting.
type GatewayProxy struct {
	proxy      *httputil.ReverseProxy
	limiter    RateLimiter
	trustProxy bool
	eventSink  func(Event)
}

// Event represents a single gateway request outcome for live streaming.
type Event struct {
	Timestamp time.Time `json:"timestamp"`
	ClientID  string    `json:"client_id"`
	Method    string    `json:"method"`
	Path      string    `json:"path"`
	Allowed   bool      `json:"allowed"`
	Limit     int64     `json:"limit,omitempty"`
	Remaining int64     `json:"remaining,omitempty"`
	Status    int       `json:"status"`
}

// Option configures optional GatewayProxy behavior.
type Option func(*GatewayProxy)

// WithTrustProxy enables trusting X-Forwarded-For headers for client
// identification. Only enable this when the gateway sits behind a
// trusted reverse proxy that sets the header.
func WithTrustProxy(trust bool) Option {
	return func(gp *GatewayProxy) {
		gp.trustProxy = trust
	}
}

// WithEventSink configures a callback for request outcome events.
func WithEventSink(sink func(Event)) Option {
	return func(gp *GatewayProxy) {
		gp.eventSink = sink
	}
}

// New creates a new GatewayProxy targeting the provided backend URL.
func New(target *url.URL, limiter RateLimiter, opts ...Option) (*GatewayProxy, error) {
	if target == nil {
		return nil, fmt.Errorf("proxy: target URL is required")
	}
	if target.Scheme != "http" && target.Scheme != "https" {
		return nil, fmt.Errorf("proxy: target URL scheme must be http or https, got %q", target.Scheme)
	}
	if target.Host == "" {
		return nil, fmt.Errorf("proxy: target URL must include a host")
	}

	rp := httputil.NewSingleHostReverseProxy(target)
	rp.ErrorHandler = func(w http.ResponseWriter, _ *http.Request, err error) {
		log.Printf("proxy: backend error: %v", err)
		writeJSON(w, http.StatusBadGateway, map[string]string{
			"error": "bad gateway",
		})
	}

	gp := &GatewayProxy{
		proxy:   rp,
		limiter: limiter,
	}
	for _, opt := range opts {
		opt(gp)
	}

	return gp, nil
}

// ServeHTTP applies rate limiting and proxies allowed requests to backend.
func (gp *GatewayProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	clientID := gp.clientKey(r)
	result := storage.Result{}

	if gp.limiter != nil {
		var err error
		result, err = gp.limiter.Allow(r.Context(), clientID)
		if err != nil {
			log.Printf("proxy: limiter error: %v", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{
				"error": "rate limiter unavailable",
			})
			return
		}

		w.Header().Set("X-RateLimit-Limit", strconv.FormatInt(result.Limit, 10))
		w.Header().Set("X-RateLimit-Remaining", strconv.FormatInt(result.Remaining, 10))
		w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(result.ResetAt.Unix(), 10))

		if !result.Allowed {
			gp.publishEvent(Event{
				Timestamp: time.Now().UTC(),
				ClientID:  clientID,
				Method:    r.Method,
				Path:      r.URL.Path,
				Allowed:   false,
				Limit:     result.Limit,
				Remaining: result.Remaining,
				Status:    http.StatusTooManyRequests,
			})

			writeJSON(w, http.StatusTooManyRequests, map[string]any{
				"error":     "rate limit exceeded",
				"limit":     result.Limit,
				"remaining": result.Remaining,
				"reset_at":  result.ResetAt.UTC(),
			})
			return
		}
	}

	gp.publishEvent(Event{
		Timestamp: time.Now().UTC(),
		ClientID:  clientID,
		Method:    r.Method,
		Path:      r.URL.Path,
		Allowed:   true,
		Limit:     result.Limit,
		Remaining: result.Remaining,
		Status:    http.StatusOK,
	})

	gp.proxy.ServeHTTP(w, r)
}

func (gp *GatewayProxy) publishEvent(event Event) {
	if gp.eventSink != nil {
		gp.eventSink(event)
	}
}

func (gp *GatewayProxy) clientKey(r *http.Request) string {
	if gp.trustProxy {
		xff := strings.TrimSpace(r.Header.Get("X-Forwarded-For"))
		if xff != "" {
			parts := strings.Split(xff, ",")
			if len(parts) > 0 {
				candidate := strings.TrimSpace(parts[0])
				if candidate != "" {
					return candidate
				}
			}
		}
	}

	host, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	if err == nil && host != "" {
		return host
	}

	if trimmed := strings.TrimSpace(r.RemoteAddr); trimmed != "" {
		return trimmed
	}

	return "unknown"
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(body); err != nil {
		http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
	}
}
