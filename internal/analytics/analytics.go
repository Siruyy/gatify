// Package analytics provides asynchronous event logging for rate limit analytics.
package analytics

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"sync"
	"time"
)

// Event represents a single rate limit event to be logged.
type Event struct {
	Timestamp   time.Time
	ClientID    string
	Method      string
	Path        string
	Allowed     bool
	RuleID      string
	Limit       int64
	Remaining   int64
	ResponseMS  int64 // Response time in milliseconds
}

// Logger is an asynchronous event logger that batches writes to reduce
// database load and ensure zero latency impact on the request path.
type Logger struct {
	db       *sql.DB
	events   chan Event
	done     chan struct{}
	wg       sync.WaitGroup
	
	// Configuration
	batchSize     int
	flushInterval time.Duration
	
	// Metrics (optional, for monitoring)
	mu            sync.RWMutex
	eventsLogged  int64
	eventsDropped int64
}

// Config holds configuration for the analytics logger.
type Config struct {
	DB            *sql.DB
	BufferSize    int           // Size of event channel buffer (default: 100)
	BatchSize     int           // Number of events to batch before writing (default: 100)
	FlushInterval time.Duration // Maximum time before flushing (default: 5s)
}

// New creates a new analytics logger and starts the background worker.
func New(cfg Config) (*Logger, error) {
	if cfg.DB == nil {
		return nil, fmt.Errorf("analytics: database connection is required")
	}
	
	// Set defaults
	if cfg.BufferSize <= 0 {
		cfg.BufferSize = 100
	}
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = 100
	}
	if cfg.FlushInterval <= 0 {
		cfg.FlushInterval = 5 * time.Second
	}
	
	logger := &Logger{
		db:            cfg.DB,
		events:        make(chan Event, cfg.BufferSize),
		done:          make(chan struct{}),
		batchSize:     cfg.BatchSize,
		flushInterval: cfg.FlushInterval,
	}
	
	// Test DB connection before starting worker
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := cfg.DB.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("analytics: database not available: %w", err)
	}
	
	// Start background worker
	logger.wg.Add(1)
	go logger.worker()
	
	return logger, nil
}

// Log queues an event for asynchronous logging. This method is non-blocking
// and will drop events if the buffer is full to avoid impacting request latency.
func (l *Logger) Log(event Event) {
	select {
	case l.events <- event:
		// Event queued successfully
	default:
		// Buffer full, drop event and increment counter
		l.mu.Lock()
		l.eventsDropped++
		l.mu.Unlock()
		log.Printf("analytics: event buffer full, dropping event")
	}
}

// Close gracefully shuts down the logger, flushing all pending events.
func (l *Logger) Close(ctx context.Context) error {
	// Signal worker to stop
	close(l.done)
	
	// Wait for worker to finish with context timeout
	doneCh := make(chan struct{})
	go func() {
		l.wg.Wait()
		close(doneCh)
	}()
	
	select {
	case <-doneCh:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("analytics: shutdown timeout exceeded")
	}
}

// Stats returns current logger statistics.
func (l *Logger) Stats() (logged, dropped int64) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.eventsLogged, l.eventsDropped
}

// worker is the background goroutine that batches and writes events to the database.
func (l *Logger) worker() {
	defer l.wg.Done()
	
	batch := make([]Event, 0, l.batchSize)
	ticker := time.NewTicker(l.flushInterval)
	defer ticker.Stop()
	
	for {
		select {
		case event := <-l.events:
			batch = append(batch, event)
			
			// Flush if batch is full
			if len(batch) >= l.batchSize {
				l.flush(batch)
				batch = batch[:0] // Reset slice
			}
			
		case <-ticker.C:
			// Periodic flush
			if len(batch) > 0 {
				l.flush(batch)
				batch = batch[:0]
			}
			
		case <-l.done:
			// Drain remaining events
			l.drainAndFlush(batch)
			return
		}
	}
}

// flush writes a batch of events to the database.
func (l *Logger) flush(events []Event) {
	if len(events) == 0 {
		return
	}
	
	// Skip flush if DB is not properly initialized (e.g., in tests)
	if l.db == nil {
		return
	}
	
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	// Begin transaction
	tx, err := l.db.BeginTx(ctx, nil)
	if err != nil {
		log.Printf("analytics: failed to begin transaction: %v", err)
		return
	}
	defer tx.Rollback() // Rollback if not committed
	
	// Prepare insert statement
	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO rate_limit_events (
			timestamp, client_id, method, path, allowed, 
			rule_id, limit_value, remaining, response_ms
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`)
	if err != nil {
		log.Printf("analytics: failed to prepare statement: %v", err)
		return
	}
	defer stmt.Close()
	
	// Execute batch insert
	for _, event := range events {
		_, err := stmt.ExecContext(ctx,
			event.Timestamp,
			event.ClientID,
			event.Method,
			event.Path,
			event.Allowed,
			event.RuleID,
			event.Limit,
			event.Remaining,
			event.ResponseMS,
		)
		if err != nil {
			log.Printf("analytics: failed to insert event: %v", err)
			// Continue with other events
		}
	}
	
	// Commit transaction
	if err := tx.Commit(); err != nil {
		log.Printf("analytics: failed to commit transaction: %v", err)
		return
	}
	
	// Update metrics
	l.mu.Lock()
	l.eventsLogged += int64(len(events))
	l.mu.Unlock()
	
	log.Printf("analytics: flushed %d events", len(events))
}

// drainAndFlush drains remaining events from the channel and flushes them.
func (l *Logger) drainAndFlush(batch []Event) {
	// Drain channel
	for {
		select {
		case event := <-l.events:
			batch = append(batch, event)
			
			// Flush if batch is full
			if len(batch) >= l.batchSize {
				l.flush(batch)
				batch = batch[:0]
			}
		default:
			// Channel is empty, flush remaining and return
			if len(batch) > 0 {
				l.flush(batch)
			}
			return
		}
	}
}
