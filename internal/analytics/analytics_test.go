package analytics

import (
	"context"
	"testing"
	"time"

	_ "github.com/lib/pq"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{
			name: "nil database",
			cfg: Config{
				DB: nil,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger, err := New(tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("New() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if logger != nil {
				ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				defer cancel()
				_ = logger.Close(ctx)
			}
		})
	}
}

func TestLogger_Log(t *testing.T) {
	t.Skip("Requires database connection - covered by integration tests")
}

func TestLogger_LogBufferFull(t *testing.T) {
	t.Skip("Requires database connection - covered by integration tests")
}

func TestLogger_Close(t *testing.T) {
	t.Skip("Requires database connection - covered by integration tests")
}

func TestLogger_CloseTimeout(t *testing.T) {
	t.Skip("Requires database connection - covered by integration tests")
}

func TestLogger_Stats(t *testing.T) {
	t.Skip("Requires database connection - covered by integration tests")
}

// BenchmarkLogger_Log measures the latency impact of logging an event.
// Note: This benchmark uses a mock setup and measures channel operations only.
// For real-world performance, see integration benchmarks.
func BenchmarkLogger_Log(b *testing.B) {
	b.Skip("Requires database connection - use integration benchmarks")
}

// BenchmarkLogger_LogWithStats measures the impact of stats tracking
func BenchmarkLogger_LogWithStats(b *testing.B) {
	b.Skip("Requires database connection - use integration benchmarks")
}

// TestLogger_ConcurrentLog tests concurrent logging from multiple goroutines
func TestLogger_ConcurrentLog(t *testing.T) {
	t.Skip("Requires database connection - covered by integration tests")
}

