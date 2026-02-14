// Package proxy provides HTTP reverse proxy functionality.
package proxy

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"

	"github.com/Siruyy/gatify/internal/storage"
)

// RateLimiter defines the limiter behavior required by the proxy.
type RateLimiter interface {
	Allow(ctx context.Context, key string) (storage.Result, error)
}

// GatewayProxy is an HTTP reverse proxy with optional rate limiting.
type GatewayProxy struct {
	proxy   *httputil.ReverseProxy
	limiter RateLimiter
}

// New creates a new GatewayProxy targeting the provided backend URL.
func New(target *url.URL, limiter RateLimiter) (*GatewayProxy, error) {
	if target == nil {
		return nil, fmt.Errorf("proxy: target URL is required")
	}

	rp := httputil.NewSingleHostReverseProxy(target)
	rp.ErrorHandler = func(w http.ResponseWriter, _ *http.Request, err error) {
		writeJSON(w, http.StatusBadGateway, map[string]string{
			"error":   "bad gateway",
			"message": err.Error(),
		})
	}

	return &GatewayProxy{
		proxy:   rp,
		limiter: limiter,
	}, nil
}

// ServeHTTP applies rate limiting and proxies allowed requests to backend.
func (gp *GatewayProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if gp.limiter != nil {
		result, err := gp.limiter.Allow(r.Context(), clientKey(r))
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{
				"error":   "rate limiter unavailable",
				"message": err.Error(),
			})
			return
		}

		w.Header().Set("X-RateLimit-Limit", strconv.FormatInt(result.Limit, 10))
		w.Header().Set("X-RateLimit-Remaining", strconv.FormatInt(result.Remaining, 10))
		w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(result.ResetAt.Unix(), 10))

		if !result.Allowed {
			writeJSON(w, http.StatusTooManyRequests, map[string]any{
				"error":     "rate limit exceeded",
				"limit":     result.Limit,
				"remaining": result.Remaining,
				"reset_at":  result.ResetAt.UTC(),
			})
			return
		}
	}

	gp.proxy.ServeHTTP(w, r)
}

func clientKey(r *http.Request) string {
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
