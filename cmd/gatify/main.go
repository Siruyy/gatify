package main

import (
	"context"
	"crypto/subtle"
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/Siruyy/gatify/internal/analytics"
	"github.com/Siruyy/gatify/internal/api"
	"github.com/Siruyy/gatify/internal/config"
	"github.com/Siruyy/gatify/internal/limiter"
	"github.com/Siruyy/gatify/internal/proxy"
	"github.com/Siruyy/gatify/internal/rules"
	"github.com/Siruyy/gatify/internal/storage"
	_ "github.com/lib/pq"
)

var version = "dev"

func main() {
	fmt.Printf("🛡️  Gatify - Starting (version: %s)...\n", version)

	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load configuration", "error", err)
		os.Exit(1)
	}

	// Initialise structured logging based on LOG_LEVEL.
	initLogging(cfg.LogLevel)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	redisCfg := storage.DefaultRedisConfig()
	redisCfg.Addr = cfg.RedisAddr

	store, err := storage.NewRedisStorage(ctx, redisCfg)
	if err != nil {
		slog.Error("failed to initialize redis storage", "error", err)
		os.Exit(1)
	}
	defer func() {
		if closeErr := store.Close(); closeErr != nil {
			slog.Error("failed to close storage", "error", closeErr)
		}
	}()

	lim, err := limiter.New(store, limiter.Config{
		Limit:  cfg.RateLimitRequests,
		Window: cfg.RateLimitWindow,
	})
	if err != nil {
		slog.Error("failed to initialize limiter", "error", err)
		os.Exit(1)
	}

	statsStreamBroker := api.NewStatsStreamBroker(256)

	// Build an initial (empty) rules matcher; it will be updated when
	// rules are created/modified via the API.
	initialMatcher, _ := rules.New(nil)

	gatewayProxy, err := proxy.New(
		cfg.BackendURL,
		lim,
		proxy.WithTrustProxy(cfg.TrustProxy),
		proxy.WithStore(store),
		proxy.WithRulesMatcher(initialMatcher),
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
		slog.Error("failed to initialize gateway proxy", "error", err)
		os.Exit(1)
	}

	rulesRepo := api.NewInMemoryRepository()

	// Provide a callback that rebuilds and hot-swaps the rules matcher
	// whenever a rule is created, updated, or deleted via the API.
	rulesHandler := api.NewRulesHandler(rulesRepo, api.WithOnRulesChanged(func() {
		allRules, listErr := rulesRepo.List(context.Background())
		if listErr != nil {
			slog.Error("rules: failed to list rules after change", "error", listErr)
			return
		}

		engineRules := make([]rules.Rule, 0, len(allRules))
		for _, r := range allRules {
			if !r.Enabled {
				continue
			}
			engineRules = append(engineRules, rules.Rule{
				Name:       r.Name,
				Pattern:    r.Pattern,
				Methods:    r.Methods,
				Priority:   r.Priority,
				Limit:      r.Limit,
				Window:     time.Duration(r.WindowSeconds) * time.Second,
				IdentifyBy: rules.IdentifyBy(r.IdentifyBy),
				HeaderName: r.HeaderName,
			})
		}

		newMatcher, matcherErr := rules.New(engineRules)
		if matcherErr != nil {
			slog.Error("rules: failed to compile rules matcher", "error", matcherErr)
			return
		}

		gatewayProxy.SetMatcher(newMatcher)
		slog.Info("rules: reloaded active rules", "count", len(engineRules))
	}))

	adminToken := cfg.AdminAPIToken
	protectedRulesHandler := requireAdminToken(adminToken, rulesHandler)

	var statsProvider api.StatsProvider
	var analyticsLogger *analytics.Logger

	if cfg.DatabaseURL != "" {
		analyticsDB, openErr := sql.Open("postgres", cfg.DatabaseURL)
		if openErr != nil {
			slog.Error("failed to initialize analytics database connection", "error", openErr)
			os.Exit(1)
		}

		pingCtx, pingCancel := context.WithTimeout(context.Background(), 5*time.Second)
		if pingErr := analyticsDB.PingContext(pingCtx); pingErr != nil {
			pingCancel()
			_ = analyticsDB.Close()
			slog.Error("failed to connect analytics database", "error", pingErr)
			os.Exit(1)
		}
		pingCancel()

		queryService, serviceErr := analytics.NewQueryService(analyticsDB)
		if serviceErr != nil {
			_ = analyticsDB.Close()
			slog.Error("failed to initialize analytics query service", "error", serviceErr)
			os.Exit(1)
		}

		statsProvider = queryService

		// Instantiate the analytics logger (batch writer).
		analyticsLogger, err = analytics.New(analytics.Config{
			DB:            analyticsDB,
			BufferSize:    1000,
			BatchSize:     100,
			FlushInterval: 5 * time.Second,
		})
		if err != nil {
			_ = analyticsDB.Close()
			slog.Error("failed to initialize analytics logger", "error", err)
			os.Exit(1)
		}

		defer func() {
			shutCtx, shutCancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer shutCancel()
			if closeErr := analyticsLogger.Close(shutCtx); closeErr != nil {
				slog.Error("failed to close analytics logger", "error", closeErr)
			}
			if closeErr := analyticsDB.Close(); closeErr != nil {
				slog.Error("failed to close analytics database connection", "error", closeErr)
			}
		}()
	}

	// If analytics logger is available, hook it into the event sink.
	if analyticsLogger != nil {
		originalSink := gatewayProxy.EventSink()
		gatewayProxy.SetEventSink(func(event proxy.Event) {
			if originalSink != nil {
				originalSink(event)
			}
			analyticsLogger.Log(analytics.Event{
				Timestamp: event.Timestamp,
				ClientID:  event.ClientID,
				Method:    event.Method,
				Path:      event.Path,
				Allowed:   event.Allowed,
				Limit:     event.Limit,
				Remaining: event.Remaining,
			})
		})
	}

	statsHandler := api.NewStatsHandler(statsProvider)
	protectedStatsHandler := requireAdminToken(adminToken, statsHandler)
	statsStreamHandler := api.NewStatsStreamHandler(statsStreamBroker, cfg.AllowedOrigins)
	protectedStatsStreamHandler := requireAdminTokenWithQuery(adminToken, statsStreamHandler)

	// Build HTTP server mux with CORS middleware.
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

	var handler http.Handler = mux
	handler = corsMiddleware(handler, cfg.AllowedOrigins)

	server := &http.Server{
		Addr:         cfg.ListenAddr,
		Handler:      handler,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Start server in goroutine
	go func() {
		slog.Info("Gatify listening", "addr", server.Addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("Shutting down Gatify...")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		slog.Error("server shutdown error", "error", err)
	}
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte(`{"status":"ok","service":"gatify"}`)); err != nil {
		slog.Error("failed to write response", "error", err)
	}
}

func rootHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte("🛡️  Gatify API Gateway\n")); err != nil {
		slog.Error("failed to write response", "error", err)
	}
}

// corsMiddleware adds CORS headers based on the configured allowed origins.
func corsMiddleware(next http.Handler, allowedOrigins []string) http.Handler {
	if len(allowedOrigins) == 0 {
		return next
	}

	originSet := make(map[string]bool, len(allowedOrigins))
	for _, o := range allowedOrigins {
		originSet[strings.ToLower(o)] = true
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" && originSet[strings.ToLower(origin)] {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Admin-Token")
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Access-Control-Max-Age", "86400")
		}

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
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
				slog.Error("failed to write response", "error", err)
			}
			return
		}

		token := extractAdminToken(r, allowQueryToken)

		if token == "" {
			w.Header().Set("WWW-Authenticate", `Bearer realm="gatify-admin"`)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			if _, err := w.Write([]byte(`{"error":"missing admin token"}`)); err != nil {
				slog.Error("failed to write response", "error", err)
			}
			return
		}

		if subtle.ConstantTimeCompare([]byte(token), []byte(expectedToken)) != 1 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			if _, err := w.Write([]byte(`{"error":"invalid admin token"}`)); err != nil {
				slog.Error("failed to write response", "error", err)
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

// initLogging configures the global slog logger level based on LOG_LEVEL.
func initLogging(level string) {
	var slogLevel slog.Level
	switch strings.ToLower(level) {
	case "debug":
		slogLevel = slog.LevelDebug
	case "warn", "warning":
		slogLevel = slog.LevelWarn
	case "error":
		slogLevel = slog.LevelError
	default:
		slogLevel = slog.LevelInfo
	}

	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slogLevel,
	})
	slog.SetDefault(slog.New(handler))
}
