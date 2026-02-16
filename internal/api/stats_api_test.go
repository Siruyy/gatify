package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Siruyy/gatify/internal/analytics"
)

type fakeStatsProvider struct {
	overviewResult   analytics.Overview
	topBlockedResult []analytics.TopBlockedClient
	ruleStatsResult  analytics.RuleStats
	timelineResult   []analytics.TimelinePoint

	err error

	lastOverviewWindow time.Duration
	lastTopWindow      time.Duration
	lastTopLimit       int
	lastRuleID         string
	lastRuleWindow     time.Duration
	lastTimelineWindow time.Duration
	lastTimelineBucket time.Duration
}

func (f *fakeStatsProvider) GetOverview(_ context.Context, window time.Duration) (analytics.Overview, error) {
	f.lastOverviewWindow = window
	if f.err != nil {
		return analytics.Overview{}, f.err
	}

	return f.overviewResult, nil
}

func (f *fakeStatsProvider) GetTopBlocked(_ context.Context, window time.Duration, limit int) ([]analytics.TopBlockedClient, error) {
	f.lastTopWindow = window
	f.lastTopLimit = limit
	if f.err != nil {
		return nil, f.err
	}

	return f.topBlockedResult, nil
}

func (f *fakeStatsProvider) GetRuleStats(_ context.Context, ruleID string, window time.Duration) (analytics.RuleStats, error) {
	f.lastRuleID = ruleID
	f.lastRuleWindow = window
	if f.err != nil {
		return analytics.RuleStats{}, f.err
	}

	return f.ruleStatsResult, nil
}

func (f *fakeStatsProvider) GetTimeline(_ context.Context, window, bucket time.Duration) ([]analytics.TimelinePoint, error) {
	f.lastTimelineWindow = window
	f.lastTimelineBucket = bucket
	if f.err != nil {
		return nil, f.err
	}

	return f.timelineResult, nil
}

func TestStatsAPI_Overview(t *testing.T) {
	fake := &fakeStatsProvider{
		overviewResult: analytics.Overview{
			WindowSeconds:   3600,
			TotalRequests:   100,
			AllowedRequests: 90,
			BlockedRequests: 10,
			UniqueClients:   25,
			BlockRate:       0.10,
		},
	}

	h := NewStatsHandler(fake)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/stats/overview?window=1h", nil)

	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, w.Code, w.Body.String())
	}

	if fake.lastOverviewWindow != time.Hour {
		t.Fatalf("expected window %v, got %v", time.Hour, fake.lastOverviewWindow)
	}

	var payload struct {
		Data analytics.Overview `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if payload.Data.TotalRequests != 100 {
		t.Fatalf("expected total_requests=100, got %d", payload.Data.TotalRequests)
	}
}

func TestStatsAPI_TopBlocked(t *testing.T) {
	fake := &fakeStatsProvider{
		topBlockedResult: []analytics.TopBlockedClient{{ClientID: "10.0.0.1", BlockedCount: 42}},
	}

	h := NewStatsHandler(fake)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/stats/top-blocked?window=2h&limit=15", nil)

	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, w.Code, w.Body.String())
	}

	if fake.lastTopWindow != 2*time.Hour {
		t.Fatalf("expected window %v, got %v", 2*time.Hour, fake.lastTopWindow)
	}
	if fake.lastTopLimit != 15 {
		t.Fatalf("expected limit 15, got %d", fake.lastTopLimit)
	}
}

func TestStatsAPI_RuleStats(t *testing.T) {
	fake := &fakeStatsProvider{
		ruleStatsResult: analytics.RuleStats{RuleID: "rule-123", TotalRequests: 5},
	}

	h := NewStatsHandler(fake)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/stats/rules/rule-123?window=30m", nil)

	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, w.Code, w.Body.String())
	}

	if fake.lastRuleID != "rule-123" {
		t.Fatalf("expected rule id rule-123, got %q", fake.lastRuleID)
	}
	if fake.lastRuleWindow != 30*time.Minute {
		t.Fatalf("expected window 30m, got %v", fake.lastRuleWindow)
	}
}

func TestStatsAPI_Timeline(t *testing.T) {
	fake := &fakeStatsProvider{
		timelineResult: []analytics.TimelinePoint{{Allowed: 10, Blocked: 2, Total: 12}},
	}

	h := NewStatsHandler(fake)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/stats/timeline?window=24h&bucket=1h", nil)

	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, w.Code, w.Body.String())
	}

	if fake.lastTimelineWindow != 24*time.Hour {
		t.Fatalf("expected window 24h, got %v", fake.lastTimelineWindow)
	}
	if fake.lastTimelineBucket != time.Hour {
		t.Fatalf("expected bucket 1h, got %v", fake.lastTimelineBucket)
	}
}

func TestStatsAPI_ServiceUnavailable(t *testing.T) {
	h := NewStatsHandler(nil)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/stats/overview", nil)

	h.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusServiceUnavailable, w.Code, w.Body.String())
	}
}

func TestStatsAPI_BadDuration(t *testing.T) {
	h := NewStatsHandler(&fakeStatsProvider{})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/stats/overview?window=banana", nil)

	h.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestStatsAPI_BadLimit(t *testing.T) {
	h := NewStatsHandler(&fakeStatsProvider{})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/stats/top-blocked?limit=0", nil)

	h.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestStatsAPI_MethodNotAllowed(t *testing.T) {
	h := NewStatsHandler(&fakeStatsProvider{})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/stats/overview", nil)

	h.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, w.Code)
	}
}

func TestStatsAPI_ProviderError(t *testing.T) {
	h := NewStatsHandler(&fakeStatsProvider{err: errors.New("query failed")})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/stats/overview", nil)

	h.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, w.Code)
	}
}
