package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/Siruyy/gatify/internal/analytics"
	"github.com/Siruyy/gatify/internal/api"
	"github.com/Siruyy/gatify/internal/limiter"
	"github.com/Siruyy/gatify/internal/proxy"
	"github.com/Siruyy/gatify/internal/storage"
	_ "github.com/lib/pq"
)

var version = "dev"

func main() {
	fmt.Printf("🛡️  Gatify - Starting (version: %s)...\n", version)

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

	// Enable trust proxy to use X-Forwarded-For headers
	trustProxy := getEnv("TRUST_PROXY", "false") == "true"
	statsStreamBroker := api.NewStatsStreamBroker(256)
	gatewayProxy, err := proxy.New(
		targetURL,
		lim,
		proxy.WithTrustProxy(trustProxy),
		proxy.WithEventSink(func(event proxy.Event) {
			statsStreamBroker.Publish(api.StatsStreamEvent{
				Timestamp: event.Timestamp,
				ClientID:  event.ClientID,
				Method:    event.Method,
				Path:      event.Path,
				Allowed:   event.Allowed,
				Limit:     event.Limit,
				Remaining: event.Remaining,
				Status:    event.Status,
			})
		}),
	)
	if err != nil {
		log.Fatalf("failed to initialize gateway proxy: %v", err)
	}

	rulesRepo := api.NewInMemoryRepository()
	rulesHandler := api.NewRulesHandler(rulesRepo)
	adminToken := strings.TrimSpace(getEnv("ADMIN_API_TOKEN", ""))
	protectedRulesHandler := requireAdminToken(adminToken, rulesHandler)

	var statsProvider api.StatsProvider
	databaseURL := strings.TrimSpace(getEnv("DATABASE_URL", ""))
	if databaseURL != "" {
		analyticsDB, openErr := sql.Open("postgres", databaseURL)
		if openErr != nil {
			log.Fatalf("failed to initialize analytics database connection: %v", openErr)
		}

		pingCtx, pingCancel := context.WithTimeout(context.Background(), 5*time.Second)
		if pingErr := analyticsDB.PingContext(pingCtx); pingErr != nil {
			pingCancel()
			_ = analyticsDB.Close()
			log.Fatalf("failed to connect analytics database: %v", pingErr)
		}
		pingCancel()

		queryService, serviceErr := analytics.NewQueryService(analyticsDB)
		if serviceErr != nil {
			_ = analyticsDB.Close()
			log.Fatalf("failed to initialize analytics query service: %v", serviceErr)
		}

		statsProvider = queryService

		defer func() {
			if closeErr := analyticsDB.Close(); closeErr != nil {
				log.Printf("failed to close analytics database connection: %v", closeErr)
			}
		}()
	}

	statsHandler := api.NewStatsHandler(statsProvider)
	protectedStatsHandler := requireAdminToken(adminToken, statsHandler)
	statsStreamHandler := api.NewStatsStreamHandler(statsStreamBroker)
	protectedStatsStreamHandler := requireAdminTokenWithQuery(adminToken, statsStreamHandler)

	// Temporary HTTP server for testing
	mux := http.NewServeMux()
	mux.HandleFunc("/health", healthHandler)
	mux.HandleFunc("/", rootHandler)
	mux.Handle("/api/rules", protectedRulesHandler)
	mux.Handle("/api/rules/", protectedRulesHandler)
	mux.Handle("/api/stats", protectedStatsHandler)
	mux.Handle("/api/stats/", protectedStatsHandler)
	mux.Handle("/api/stats/stream", protectedStatsStreamHandler)
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

func requireAdminToken(expectedToken string, next http.Handler) http.Handler {
	return requireAdminTokenInternal(expectedToken, next, false)
}

func requireAdminTokenWithQuery(expectedToken string, next http.Handler) http.Handler {
	return requireAdminTokenInternal(expectedToken, next, true)
}

func requireAdminTokenInternal(expectedToken string, next http.Handler, allowQueryToken bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.TrimSpace(expectedToken) == "" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			if _, err := w.Write([]byte(`{"error":"admin API token not configured"}`)); err != nil {
				log.Printf("Failed to write response: %v", err)
			}
			return
		}

		token := extractAdminToken(r, allowQueryToken)

		if token == "" {
			w.Header().Set("WWW-Authenticate", `Bearer realm="gatify-admin"`)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			if _, err := w.Write([]byte(`{"error":"missing admin token"}`)); err != nil {
				log.Printf("Failed to write response: %v", err)
			}
			return
		}

		if token != expectedToken {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			if _, err := w.Write([]byte(`{"error":"invalid admin token"}`)); err != nil {
				log.Printf("Failed to write response: %v", err)
			}
			return
		}

		next.ServeHTTP(w, r)
	})
}

func extractAdminToken(r *http.Request, allowQueryToken bool) string {
	token := strings.TrimSpace(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
	if token == "" {
		token = strings.TrimSpace(r.Header.Get("X-Admin-Token"))
	}
	if token == "" && allowQueryToken {
		token = strings.TrimSpace(r.URL.Query().Get("token"))
	}

	return token
}
