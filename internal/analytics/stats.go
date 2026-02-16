package analytics

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// Overview summarizes traffic and rate-limit decisions over a time window.
type Overview struct {
	WindowSeconds   int64   `json:"window_seconds"`
	TotalRequests   int64   `json:"total_requests"`
	AllowedRequests int64   `json:"allowed_requests"`
	BlockedRequests int64   `json:"blocked_requests"`
	UniqueClients   int64   `json:"unique_clients"`
	BlockRate       float64 `json:"block_rate"`
}

// TopBlockedClient represents a client with the highest blocked request count.
type TopBlockedClient struct {
	ClientID     string `json:"client_id"`
	BlockedCount int64  `json:"blocked_count"`
}

// RuleStats summarizes behavior for a specific rule over a time window.
type RuleStats struct {
	RuleID          string  `json:"rule_id"`
	WindowSeconds   int64   `json:"window_seconds"`
	TotalRequests   int64   `json:"total_requests"`
	AllowedRequests int64   `json:"allowed_requests"`
	BlockedRequests int64   `json:"blocked_requests"`
	BlockRate       float64 `json:"block_rate"`
	AvgResponseMS   float64 `json:"avg_response_ms"`
}

// TimelinePoint is a single bucket in an analytics timeline series.
type TimelinePoint struct {
	BucketStart time.Time `json:"bucket_start"`
	Allowed     int64     `json:"allowed"`
	Blocked     int64     `json:"blocked"`
	Total       int64     `json:"total"`
}

// QueryService provides read-only analytics queries backed by PostgreSQL/TimescaleDB.
type QueryService struct {
	db *sql.DB
}

// NewQueryService constructs an analytics query service.
func NewQueryService(db *sql.DB) (*QueryService, error) {
	if db == nil {
		return nil, fmt.Errorf("analytics: query service requires database connection")
	}

	return &QueryService{db: db}, nil
}

// GetOverview returns top-level traffic metrics for a time window.
func (s *QueryService) GetOverview(ctx context.Context, window time.Duration) (Overview, error) {
	if window <= 0 {
		return Overview{}, fmt.Errorf("analytics: window must be greater than zero")
	}

	since := time.Now().Add(-window)

	var out Overview
	out.WindowSeconds = int64(window.Seconds())

	err := s.db.QueryRowContext(ctx, `
		SELECT
			COUNT(*) AS total_requests,
			COALESCE(SUM(CASE WHEN allowed THEN 1 ELSE 0 END), 0) AS allowed_requests,
			COALESCE(SUM(CASE WHEN NOT allowed THEN 1 ELSE 0 END), 0) AS blocked_requests,
			COUNT(DISTINCT client_id) AS unique_clients
		FROM rate_limit_events
		WHERE timestamp >= $1
	`, since).Scan(
		&out.TotalRequests,
		&out.AllowedRequests,
		&out.BlockedRequests,
		&out.UniqueClients,
	)
	if err != nil {
		return Overview{}, fmt.Errorf("analytics: overview query failed: %w", err)
	}

	if out.TotalRequests > 0 {
		out.BlockRate = float64(out.BlockedRequests) / float64(out.TotalRequests)
	}

	return out, nil
}

// GetTopBlocked returns clients with highest blocked request counts.
func (s *QueryService) GetTopBlocked(ctx context.Context, window time.Duration, limit int) ([]TopBlockedClient, error) {
	if window <= 0 {
		return nil, fmt.Errorf("analytics: window must be greater than zero")
	}
	if limit <= 0 {
		return nil, fmt.Errorf("analytics: limit must be greater than zero")
	}

	since := time.Now().Add(-window)

	rows, err := s.db.QueryContext(ctx, `
		SELECT
			client_id,
			COUNT(*) AS blocked_count
		FROM rate_limit_events
		WHERE allowed = FALSE AND timestamp >= $1
		GROUP BY client_id
		ORDER BY blocked_count DESC, client_id ASC
		LIMIT $2
	`, since, limit)
	if err != nil {
		return nil, fmt.Errorf("analytics: top-blocked query failed: %w", err)
	}
	defer rows.Close()

	out := make([]TopBlockedClient, 0, limit)
	for rows.Next() {
		var item TopBlockedClient
		if scanErr := rows.Scan(&item.ClientID, &item.BlockedCount); scanErr != nil {
			return nil, fmt.Errorf("analytics: failed scanning top-blocked row: %w", scanErr)
		}
		out = append(out, item)
	}

	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, fmt.Errorf("analytics: top-blocked rows iteration failed: %w", rowsErr)
	}

	return out, nil
}

// GetRuleStats returns summary statistics for a specific rule id.
func (s *QueryService) GetRuleStats(ctx context.Context, ruleID string, window time.Duration) (RuleStats, error) {
	if ruleID == "" {
		return RuleStats{}, fmt.Errorf("analytics: rule id is required")
	}
	if window <= 0 {
		return RuleStats{}, fmt.Errorf("analytics: window must be greater than zero")
	}

	since := time.Now().Add(-window)

	out := RuleStats{
		RuleID:        ruleID,
		WindowSeconds: int64(window.Seconds()),
	}

	err := s.db.QueryRowContext(ctx, `
		SELECT
			COUNT(*) AS total_requests,
			COALESCE(SUM(CASE WHEN allowed THEN 1 ELSE 0 END), 0) AS allowed_requests,
			COALESCE(SUM(CASE WHEN NOT allowed THEN 1 ELSE 0 END), 0) AS blocked_requests,
			COALESCE(AVG(response_ms), 0) AS avg_response_ms
		FROM rate_limit_events
		WHERE rule_id = $1 AND timestamp >= $2
	`, ruleID, since).Scan(
		&out.TotalRequests,
		&out.AllowedRequests,
		&out.BlockedRequests,
		&out.AvgResponseMS,
	)
	if err != nil {
		return RuleStats{}, fmt.Errorf("analytics: rule stats query failed: %w", err)
	}

	if out.TotalRequests > 0 {
		out.BlockRate = float64(out.BlockedRequests) / float64(out.TotalRequests)
	}

	return out, nil
}

// GetTimeline returns allowed/blocked request counts bucketed by time interval.
func (s *QueryService) GetTimeline(ctx context.Context, window, bucket time.Duration) ([]TimelinePoint, error) {
	if window <= 0 {
		return nil, fmt.Errorf("analytics: window must be greater than zero")
	}
	if bucket <= 0 {
		return nil, fmt.Errorf("analytics: bucket must be greater than zero")
	}

	since := time.Now().Add(-window)
	bucketSeconds := int64(bucket.Seconds())

	rows, err := s.db.QueryContext(ctx, `
		SELECT
			to_timestamp(FLOOR(EXTRACT(EPOCH FROM timestamp) / $1) * $1)::timestamptz AS bucket_start,
			COALESCE(SUM(CASE WHEN allowed THEN 1 ELSE 0 END), 0) AS allowed_count,
			COALESCE(SUM(CASE WHEN NOT allowed THEN 1 ELSE 0 END), 0) AS blocked_count
		FROM rate_limit_events
		WHERE timestamp >= $2
		GROUP BY bucket_start
		ORDER BY bucket_start ASC
	`, bucketSeconds, since)
	if err != nil {
		return nil, fmt.Errorf("analytics: timeline query failed: %w", err)
	}
	defer rows.Close()

	out := make([]TimelinePoint, 0)
	for rows.Next() {
		var point TimelinePoint
		if scanErr := rows.Scan(&point.BucketStart, &point.Allowed, &point.Blocked); scanErr != nil {
			return nil, fmt.Errorf("analytics: failed scanning timeline row: %w", scanErr)
		}
		point.Total = point.Allowed + point.Blocked
		out = append(out, point)
	}

	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, fmt.Errorf("analytics: timeline rows iteration failed: %w", rowsErr)
	}

	return out, nil
}
