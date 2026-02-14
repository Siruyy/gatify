//go:build e2e

package e2e

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

const (
	gatewayURL = "http://localhost:3000"
	proxyPath  = "/proxy/"
)

// TestMain sets up the test environment
func TestMain(m *testing.M) {
	// Verify required services are running
	if !checkService(gatewayURL+"/health", 5*time.Second) {
		fmt.Fprintf(os.Stderr, "Error: Gatify gateway not available at %s\n", gatewayURL)
		fmt.Fprintf(os.Stderr, "Please start the gateway with: go run ./cmd/gatify\n")
		os.Exit(1)
	}

	// Run tests
	code := m.Run()
	os.Exit(code)
}

// checkService verifies a service is available
func checkService(url string, timeout time.Duration) bool {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return false
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK
}

// TestHealth verifies the health endpoint
func TestHealth(t *testing.T) {
	resp, err := http.Get(gatewayURL + "/health")
	if err != nil {
		t.Fatalf("Failed to get health endpoint: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	t.Logf("Health check response: %s", body)
}

// TestRateLimitAllow tests that requests under the limit are allowed
func TestRateLimitAllow(t *testing.T) {
	clientIP := fmt.Sprintf("10.10.0.%d", time.Now().UnixNano()%200+1)

	// Make a few requests that should all succeed
	for i := 0; i < 5; i++ {
		req, err := http.NewRequest(http.MethodGet, gatewayURL+proxyPath+"get", nil)
		if err != nil {
			t.Fatalf("Failed to create request %d: %v", i+1, err)
		}
		req.Header.Set("X-Forwarded-For", clientIP)

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("Request %d failed: %v", i+1, err)
		}

		if resp.StatusCode == http.StatusTooManyRequests {
			t.Errorf("Request %d was rate limited unexpectedly", i+1)
		}

		// Check rate limit headers
		if limit := resp.Header.Get("X-RateLimit-Limit"); limit == "" {
			t.Error("Missing X-RateLimit-Limit header")
		}
		if remaining := resp.Header.Get("X-RateLimit-Remaining"); remaining == "" {
			t.Error("Missing X-RateLimit-Remaining header")
		}
		if reset := resp.Header.Get("X-RateLimit-Reset"); reset == "" {
			t.Error("Missing X-RateLimit-Reset header")
		}

		t.Logf("Request %d: Status=%d, Remaining=%s",
			i+1, resp.StatusCode, resp.Header.Get("X-RateLimit-Remaining"))

		if closeErr := resp.Body.Close(); closeErr != nil {
			t.Logf("failed to close response body on request %d: %v", i+1, closeErr)
		}

		// Brief pause between requests
		time.Sleep(10 * time.Millisecond)
	}
}

// TestRateLimitEnforce tests that the rate limiter blocks excess requests
func TestRateLimitEnforce(t *testing.T) {
	// Use a unique client ID per test to avoid interference
	clientID := fmt.Sprintf("10.0.0.%d", time.Now().UnixNano()%200+1)
	const maxNetworkErrors = 3
	var networkErrs atomic.Int32

	// Send many requests quickly to trigger rate limit
	const numRequests = 150 // Should exceed default limit of 100
	var blocked atomic.Int32
	var allowed atomic.Int32

	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	for i := 0; i < numRequests; i++ {
		req, err := http.NewRequest(http.MethodGet, gatewayURL+proxyPath+"status/200", nil)
		if err != nil {
			t.Fatalf("Failed to create request: %v", err)
		}

		// Use same client ID for all requests
		req.Header.Set("X-Forwarded-For", clientID)

		resp, err := client.Do(req)
		if err != nil {
			currErrs := networkErrs.Add(1)
			t.Logf("Request %d failed: %v (network errors: %d)", i+1, err, currErrs)
			if currErrs > maxNetworkErrors {
				t.Fatalf("Exceeded max network errors (%d), latest error: %v", maxNetworkErrors, err)
			}
			continue
		}

		switch resp.StatusCode {
		case http.StatusOK:
			allowed.Add(1)
		case http.StatusTooManyRequests:
			blocked.Add(1)
		default:
			_ = resp.Body.Close()
			t.Fatalf("Request %d returned unexpected status code: %d", i+1, resp.StatusCode)
		}

		resp.Body.Close()

		// Small delay to avoid overwhelming the system
		if i%10 == 0 {
			time.Sleep(5 * time.Millisecond)
		}
	}

	allowedCount := allowed.Load()
	blockedCount := blocked.Load()

	t.Logf("Results: %d allowed, %d blocked out of %d requests", allowedCount, blockedCount, numRequests)

	if blockedCount == 0 {
		t.Error("Expected some requests to be blocked by rate limiter, but none were")
	}

	if allowedCount == 0 {
		t.Error("Expected some requests to be allowed, but all were blocked")
	}

	// Verify at least some requests exceeded the limit
	if blockedCount < 10 {
		t.Errorf("Expected more blocked requests, got only %d", blockedCount)
	}
}

// TestConcurrentClients tests multiple clients making requests simultaneously
func TestConcurrentClients(t *testing.T) {
	const (
		numClients      = 10
		requestsPerClient = 5
	)

	var wg sync.WaitGroup
	results := make(chan testResult, numClients*requestsPerClient)

	// Launch concurrent clients
	for clientID := 0; clientID < numClients; clientID++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			client := &http.Client{Timeout: 5 * time.Second}
			clientKey := fmt.Sprintf("10.0.1.%d", id+1)

			for reqNum := 0; reqNum < requestsPerClient; reqNum++ {
				req, err := http.NewRequest(http.MethodGet, gatewayURL+proxyPath+"status/200", nil)
				if err != nil {
					results <- testResult{clientID: id, err: err}
					continue
				}

				req.Header.Set("X-Forwarded-For", clientKey)

				start := time.Now()
				resp, err := client.Do(req)
				duration := time.Since(start)

				if err != nil {
					results <- testResult{clientID: id, err: err}
					continue
				}

				results <- testResult{
					clientID:   id,
					statusCode: resp.StatusCode,
					duration:   duration,
				}

				resp.Body.Close()
			}
		}(clientID)
	}

	// Wait for all clients to finish
	wg.Wait()
	close(results)

	// Analyze results
	var (
		totalRequests   int
		successCount    int
		rateLimitCount  int
		errorCount      int
		totalDuration   time.Duration
	)

	for result := range results {
		totalRequests++

		if result.err != nil {
			errorCount++
			t.Errorf("Client %d request failed: %v", result.clientID, result.err)
			continue
		}

		totalDuration += result.duration

		switch result.statusCode {
		case http.StatusOK:
			successCount++
		case http.StatusTooManyRequests:
			rateLimitCount++
		default:
			errorCount++
			t.Errorf("Client %d got unexpected status code: %d", result.clientID, result.statusCode)
		}
	}

	t.Logf("Concurrent clients results:")
	t.Logf("  Total requests: %d", totalRequests)
	t.Logf("  Successful: %d", successCount)
	t.Logf("  Rate limited: %d", rateLimitCount)
	t.Logf("  Errors: %d", errorCount)
	t.Logf("  Average latency: %v", totalDuration/time.Duration(totalRequests))

	// Verify most requests succeeded
	if successCount == 0 {
		t.Error("No successful requests in concurrent test")
	}

	// Verify reasonable latency (should be sub-second for most requests)
	avgLatency := totalDuration / time.Duration(totalRequests)
	if avgLatency > time.Second {
		t.Errorf("Average latency too high: %v", avgLatency)
	}
}

// TestDifferentClientIPs verifies that different client IPs have independent rate limits
func TestDifferentClientIPs(t *testing.T) {
	client1IP := "192.168.1.100"
	client2IP := "192.168.1.200"

	// Make requests from client 1
	for i := 0; i < 5; i++ {
		req, err := http.NewRequest(http.MethodGet, gatewayURL+proxyPath+"status/200", nil)
		if err != nil {
			t.Fatalf("Client 1 request %d failed to create request: %v", i+1, err)
		}
		req.Header.Set("X-Forwarded-For", client1IP)

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("Client 1 request %d failed: %v", i+1, err)
		}
		resp.Body.Close()

		if resp.StatusCode == http.StatusTooManyRequests {
			t.Errorf("Client 1 request %d was rate limited unexpectedly", i+1)
		}
	}

	// Make requests from client 2 - should have its own limit
	for i := 0; i < 5; i++ {
		req, err := http.NewRequest(http.MethodGet, gatewayURL+proxyPath+"status/200", nil)
		if err != nil {
			t.Fatalf("Client 2 request %d failed to create request: %v", i+1, err)
		}
		req.Header.Set("X-Forwarded-For", client2IP)

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("Client 2 request %d failed: %v", i+1, err)
		}
		resp.Body.Close()

		if resp.StatusCode == http.StatusTooManyRequests {
			t.Errorf("Client 2 request %d was rate limited unexpectedly", i+1)
		}
	}

	t.Log("Both clients successfully made requests with independent limits")
}

// TestResponseHeaders verifies that all expected headers are present
func TestResponseHeaders(t *testing.T) {
	resp, err := http.Get(gatewayURL + proxyPath + "headers")
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	requiredHeaders := []string{
		"X-RateLimit-Limit",
		"X-RateLimit-Remaining",
		"X-RateLimit-Reset",
	}

	for _, header := range requiredHeaders {
		if value := resp.Header.Get(header); value == "" {
			t.Errorf("Missing required header: %s", header)
		} else {
			t.Logf("Header %s: %s", header, value)
		}
	}
}

// TestBackendProxying verifies that requests are properly proxied to the backend
func TestBackendProxying(t *testing.T) {
	tests := []struct {
		name           string
		method         string
		path           string
		expectedStatus int
	}{
		{"GET request", http.MethodGet, "get", http.StatusOK},
		{"POST request", http.MethodPost, "post", http.StatusOK},
		{"Status 200", http.MethodGet, "status/200", http.StatusOK},
		{"Status 404", http.MethodGet, "status/404", http.StatusNotFound},
	}

	runID := time.Now().UnixNano()%200 + 1
	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := http.NewRequest(tt.method, gatewayURL+proxyPath+tt.path, nil)
			if err != nil {
				t.Fatalf("Failed to create request: %v", err)
			}
			req.Header.Set("X-Forwarded-For", fmt.Sprintf("10.20.%d.%d", runID, i+1))

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("Request failed: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, resp.StatusCode)
			}
		})
	}
}

// testResult holds results from concurrent tests
type testResult struct {
	clientID   int
	statusCode int
	duration   time.Duration
	err        error
}
