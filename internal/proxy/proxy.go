// Package proxy provides HTTP reverse proxy functionality.
package proxy

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
	"time"

	gatifyhttp "github.com/Siruyy/gatify/internal/httputil"
	"github.com/Siruyy/gatify/internal/rules"
	"github.com/Siruyy/gatify/internal/storage"
)

// RateLimiter defines the limiter behavior required by the proxy.
type RateLimiter interface {
	Allow(ctx context.Context, key string) (storage.Result, error)
}

// SlidingWindowStore defines storage capabilities for per-rule rate limiting.
type SlidingWindowStore interface {
	CheckSlidingWindow(ctx context.Context, key string, limit int64, window time.Duration) (storage.Result, error)
}

// GatewayProxy is an HTTP reverse proxy with optional rate limiting.
type GatewayProxy struct {
	proxy      *httputil.ReverseProxy
	limiter    RateLimiter
	store      SlidingWindowStore
	matcher    *rules.Matcher
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

// WithRulesMatcher configures a rules matcher for per-route rate limiting.
// Requires WithStore to also be set for per-rule sliding window checks.
func WithRulesMatcher(matcher *rules.Matcher) Option {
	return func(gp *GatewayProxy) {
		gp.matcher = matcher
	}
}

// WithStore configures a sliding window store for per-rule rate limiting.
func WithStore(store SlidingWindowStore) Option {
	return func(gp *GatewayProxy) {
		gp.store = store
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
		slog.Error("proxy: backend error", "error", err)
		gatifyhttp.WriteJSON(w, http.StatusBadGateway, map[string]string{
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

	// Check per-route rules first if the rules engine is wired in.
	if gp.matcher != nil && gp.store != nil {
		match := gp.matcher.Match(r.Method, r.URL.Path)
		if match != nil {
			rule := match.Rule

			// Determine rate limit key based on the rule's IdentifyBy setting.
			ruleClientID := clientID
			if rule.IdentifyBy == rules.IdentifyByHeader && rule.HeaderName != "" {
				headerVal := strings.TrimSpace(r.Header.Get(rule.HeaderName))
				if headerVal != "" {
					ruleClientID = headerVal
				}
			}

			ruleKey := fmt.Sprintf("rule:%s:%s", rule.Name, ruleClientID)
			var err error
			result, err = gp.store.CheckSlidingWindow(r.Context(), ruleKey, rule.Limit, rule.Window)
			if err != nil {
				// Graceful degradation: allow request through when Redis is down.
				slog.Warn("proxy: per-rule limiter error, allowing request", "rule", rule.Name, "error", err)
				gp.publishEvent(Event{
					Timestamp: time.Now().UTC(),
					ClientID:  clientID,
					Method:    r.Method,
					Path:      r.URL.Path,
					Allowed:   true,
					Status:    http.StatusOK,
				})
				gp.proxy.ServeHTTP(w, r)
				return
			}

			w.Header().Set("X-RateLimit-Limit", strconv.FormatInt(result.Limit, 10))
			w.Header().Set("X-RateLimit-Remaining", strconv.FormatInt(result.Remaining, 10))
			w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(result.ResetAt.Unix(), 10))
			w.Header().Set("X-RateLimit-Rule", rule.Name)

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

				gatifyhttp.WriteJSON(w, http.StatusTooManyRequests, map[string]any{
					"error":     "rate limit exceeded",
					"rule":      rule.Name,
					"limit":     result.Limit,
					"remaining": result.Remaining,
					"reset_at":  result.ResetAt.UTC(),
				})
				return
			}

			// Rule matched and allowed — publish event and proxy to backend.
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
			return
		}
	}

	// Fallback to global rate limiter when no rule matched.
	if gp.limiter != nil {
		var err error
		result, err = gp.limiter.Allow(r.Context(), clientID)
		if err != nil {
			// Graceful degradation: if the rate limiter is unavailable (e.g. Redis
			// is down), allow the request through rather than blocking all traffic.
			slog.Warn("proxy: limiter error, allowing request", "error", err)
			gp.publishEvent(Event{
				Timestamp: time.Now().UTC(),
				ClientID:  clientID,
				Method:    r.Method,
				Path:      r.URL.Path,
				Allowed:   true,
				Status:    http.StatusOK,
			})
			gp.proxy.ServeHTTP(w, r)
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

			gatifyhttp.WriteJSON(w, http.StatusTooManyRequests, map[string]any{
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

// SetMatcher atomically replaces the current rules matcher.
// This allows hot-reloading rules without restarting the proxy.
func (gp *GatewayProxy) SetMatcher(m *rules.Matcher) {
	gp.matcher = m
}

// EventSink returns the current event sink callback (may be nil).
func (gp *GatewayProxy) EventSink() func(Event) {
	return gp.eventSink
}

// SetEventSink replaces the event sink callback.
func (gp *GatewayProxy) SetEventSink(sink func(Event)) {
	gp.eventSink = sink
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
