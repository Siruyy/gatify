package api

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Siruyy/gatify/internal/analytics"
	gatifyhttp "github.com/Siruyy/gatify/internal/httputil"
)

const (
	defaultWindow = 24 * time.Hour
	defaultLimit  = 10
	maxLimit      = 100
	defaultBucket = 5 * time.Minute
	minBucket     = 1 * time.Minute
	maxBucket     = 24 * time.Hour
)

// StatsProvider exposes analytics read models required by the stats API.
type StatsProvider interface {
	GetOverview(ctx context.Context, window time.Duration) (analytics.Overview, error)
	GetTopBlocked(ctx context.Context, window time.Duration, limit int) ([]analytics.TopBlockedClient, error)
	GetRuleStats(ctx context.Context, ruleID string, window time.Duration) (analytics.RuleStats, error)
	GetTimeline(ctx context.Context, window, bucket time.Duration) ([]analytics.TimelinePoint, error)
}

// StatsHandler serves analytics/statistics endpoints.
type StatsHandler struct {
	provider StatsProvider
}

// NewStatsHandler creates a stats API handler.
func NewStatsHandler(provider StatsProvider) *StatsHandler {
	return &StatsHandler{provider: provider}
}

// ServeHTTP handles:
// - GET /api/stats/overview
// - GET /api/stats/top-blocked
// - GET /api/stats/rules/{id}
// - GET /api/stats/timeline
func (h *StatsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	if path == "/api/stats" || path == "/api/stats/" {
		http.NotFound(w, r)
		return
	}

	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		gatifyhttp.WriteJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	if h.provider == nil {
		gatifyhttp.WriteJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "analytics service unavailable"})
		return
	}

	window, err := parseDurationQuery(r, "window", defaultWindow)
	if err != nil {
		gatifyhttp.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	switch {
	case path == "/api/stats/overview":
		h.handleOverview(w, r, window)
	case path == "/api/stats/top-blocked":
		h.handleTopBlocked(w, r, window)
	case path == "/api/stats/timeline":
		h.handleTimeline(w, r, window)
	case strings.HasPrefix(path, "/api/stats/rules/"):
		ruleID := strings.TrimPrefix(path, "/api/stats/rules/")
		if ruleID == "" || strings.Contains(ruleID, "/") {
			http.NotFound(w, r)
			return
		}
		h.handleRuleStats(w, r, ruleID, window)
	default:
		http.NotFound(w, r)
	}
}

func (h *StatsHandler) handleOverview(w http.ResponseWriter, r *http.Request, window time.Duration) {
	result, err := h.provider.GetOverview(r.Context(), window)
	if err != nil {
		gatifyhttp.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to fetch overview stats"})
		return
	}

	gatifyhttp.WriteJSON(w, http.StatusOK, map[string]any{"data": result})
}

func (h *StatsHandler) handleTopBlocked(w http.ResponseWriter, r *http.Request, window time.Duration) {
	limit, err := parseLimitQuery(r, defaultLimit, maxLimit)
	if err != nil {
		gatifyhttp.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	result, queryErr := h.provider.GetTopBlocked(r.Context(), window, limit)
	if queryErr != nil {
		gatifyhttp.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to fetch top blocked clients"})
		return
	}

	gatifyhttp.WriteJSON(w, http.StatusOK, map[string]any{"data": result})
}

func (h *StatsHandler) handleRuleStats(w http.ResponseWriter, r *http.Request, ruleID string, window time.Duration) {
	result, err := h.provider.GetRuleStats(r.Context(), ruleID, window)
	if err != nil {
		gatifyhttp.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to fetch rule stats"})
		return
	}

	gatifyhttp.WriteJSON(w, http.StatusOK, map[string]any{"data": result})
}

func (h *StatsHandler) handleTimeline(w http.ResponseWriter, r *http.Request, window time.Duration) {
	bucket, err := parseDurationQuery(r, "bucket", defaultBucket)
	if err != nil {
		gatifyhttp.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	if bucket < minBucket || bucket > maxBucket {
		gatifyhttp.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "bucket must be between 1m and 24h"})
		return
	}

	result, queryErr := h.provider.GetTimeline(r.Context(), window, bucket)
	if queryErr != nil {
		gatifyhttp.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to fetch timeline stats"})
		return
	}

	gatifyhttp.WriteJSON(w, http.StatusOK, map[string]any{"data": result})
}

func parseLimitQuery(r *http.Request, fallback, max int) (int, error) {
	raw := strings.TrimSpace(r.URL.Query().Get("limit"))
	if raw == "" {
		return fallback, nil
	}

	parsed, err := strconv.Atoi(raw)
	if err != nil || parsed <= 0 {
		return 0, errBadQuery("limit must be a positive integer")
	}

	if parsed > max {
		return max, nil
	}

	return parsed, nil
}

func parseDurationQuery(r *http.Request, key string, fallback time.Duration) (time.Duration, error) {
	raw := strings.TrimSpace(r.URL.Query().Get(key))
	if raw == "" {
		return fallback, nil
	}

	parsed, err := parseFlexibleDuration(raw)
	if err != nil || parsed <= 0 {
		return 0, errBadQuery(key + " must be a valid positive duration (for example: 15m, 1h, 7d)")
	}

	return parsed, nil
}

func parseFlexibleDuration(raw string) (time.Duration, error) {
	raw = strings.TrimSpace(strings.ToLower(raw))
	if strings.HasSuffix(raw, "d") {
		daysRaw := strings.TrimSuffix(raw, "d")
		days, err := strconv.Atoi(daysRaw)
		if err != nil {
			return 0, err
		}
		return time.Duration(days) * 24 * time.Hour, nil
	}

	return time.ParseDuration(raw)
}

type badQueryError struct {
	message string
}

func (e badQueryError) Error() string {
	return e.message
}

func errBadQuery(message string) error {
	return badQueryError{message: message}
}
