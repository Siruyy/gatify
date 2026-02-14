package limiter

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Siruyy/gatify/internal/storage"
)

type fakeStore struct {
	result      storage.Result
	err         error
	lastKey     string
	lastLimit   int64
	lastWindow  time.Duration
	callCounter int
}

func (f *fakeStore) CheckSlidingWindow(_ context.Context, key string, limit int64, window time.Duration) (storage.Result, error) {
	f.callCounter++
	f.lastKey = key
	f.lastLimit = limit
	f.lastWindow = window
	return f.result, f.err
}

func TestNewLimiterValidation(t *testing.T) {
	store := &fakeStore{}

	if _, err := New(nil, Config{Limit: 10, Window: time.Second}); err == nil {
		t.Fatal("expected error when store is nil")
	}

	if _, err := New(store, Config{Limit: 0, Window: time.Second}); err == nil {
		t.Fatal("expected error when limit is zero")
	}

	if _, err := New(store, Config{Limit: 10, Window: 0}); err == nil {
		t.Fatal("expected error when window is zero")
	}
}

func TestAllowValidation(t *testing.T) {
	l, err := New(&fakeStore{}, Config{Limit: 10, Window: time.Second})
	if err != nil {
		t.Fatalf("failed to create limiter: %v", err)
	}

	if _, err := l.Allow(context.Background(), ""); err == nil {
		t.Fatal("expected error for empty key")
	}
}

func TestAllowDelegatesToStore(t *testing.T) {
	expected := storage.Result{
		Count:     3,
		Limit:     10,
		Remaining: 7,
		Allowed:   true,
	}
	store := &fakeStore{result: expected}

	l, err := New(store, Config{Limit: 10, Window: time.Minute})
	if err != nil {
		t.Fatalf("failed to create limiter: %v", err)
	}

	result, err := l.Allow(context.Background(), "client-1")
	if err != nil {
		t.Fatalf("Allow returned error: %v", err)
	}

	if store.callCounter != 1 {
		t.Fatalf("expected store to be called once, got %d", store.callCounter)
	}
	if store.lastKey != "client-1" {
		t.Fatalf("expected key client-1, got %s", store.lastKey)
	}
	if store.lastLimit != 10 {
		t.Fatalf("expected limit 10, got %d", store.lastLimit)
	}
	if store.lastWindow != time.Minute {
		t.Fatalf("expected window 1m, got %v", store.lastWindow)
	}
	if result != expected {
		t.Fatalf("unexpected result: got %+v want %+v", result, expected)
	}
}

func TestAllowStoreError(t *testing.T) {
	store := &fakeStore{err: errors.New("boom")}

	l, err := New(store, Config{Limit: 10, Window: time.Minute})
	if err != nil {
		t.Fatalf("failed to create limiter: %v", err)
	}

	if _, err := l.Allow(context.Background(), "client-1"); err == nil {
		t.Fatal("expected error from store")
	}
}
