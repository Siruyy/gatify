package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/Siruyy/gatify/internal/limiter"
	"github.com/Siruyy/gatify/internal/proxy"
	"github.com/Siruyy/gatify/internal/storage"
)

func main() {
	fmt.Println("🛡️  Gatify - Starting...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	redisCfg := storage.DefaultRedisConfig()
	redisCfg.Addr = getEnv("REDIS_ADDR", redisCfg.Addr)

	store, err := storage.NewRedisStorage(ctx, redisCfg)
	if err != nil {
		log.Fatalf("failed to initialize redis storage: %v", err)
	}
	defer func() {
		if closeErr := store.Close(); closeErr != nil {
			log.Printf("failed to close storage: %v", closeErr)
		}
	}()

	lim, err := limiter.New(store, limiter.Config{
		Limit:  getEnvInt64("RATE_LIMIT_REQUESTS", 100),
		Window: time.Duration(getEnvInt("RATE_LIMIT_WINDOW_SECONDS", 60)) * time.Second,
	})
	if err != nil {
		log.Fatalf("failed to initialize limiter: %v", err)
	}

	targetURLRaw := getEnv("BACKEND_URL", "http://localhost:8080")
	targetURL, err := url.Parse(targetURLRaw)
	if err != nil {
		log.Fatalf("invalid BACKEND_URL %q: %v", targetURLRaw, err)
	}

	gatewayProxy, err := proxy.New(targetURL, lim)
	if err != nil {
		log.Fatalf("failed to initialize gateway proxy: %v", err)
	}

	// Temporary HTTP server for testing
	mux := http.NewServeMux()
	mux.HandleFunc("/health", healthHandler)
	mux.HandleFunc("/", rootHandler)
	mux.Handle("/proxy/", http.StripPrefix("/proxy", gatewayProxy))
	mux.HandleFunc("/proxy", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/proxy/", http.StatusMovedPermanently)
	})

	server := &http.Server{
		Addr:         ":3000",
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	// Start server in goroutine
	go func() {
		log.Printf("✅ Gatify listening on %s", server.Addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("🛑 Shutting down Gatify...")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("server shutdown error: %v", err)
	}
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte(`{"status":"ok","service":"gatify"}`)); err != nil {
		log.Printf("Failed to write response: %v", err)
	}
}

func rootHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte("🛡️  Gatify API Gateway\n")); err != nil {
		log.Printf("Failed to write response: %v", err)
	}
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}

	return fallback
}

func getEnvInt(key string, fallback int) int {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}

	parsed, err := strconv.Atoi(raw)
	if err != nil {
		log.Printf("invalid %s=%q, using fallback %d", key, raw, fallback)
		return fallback
	}

	return parsed
}

func getEnvInt64(key string, fallback int64) int64 {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}

	parsed, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		log.Printf("invalid %s=%q, using fallback %d", key, raw, fallback)
		return fallback
	}

	return parsed
}
