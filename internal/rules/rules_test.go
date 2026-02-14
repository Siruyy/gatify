package rules

import (
	"testing"
	"time"
)

func defaultRule(name, pattern string, priority int, methods ...string) Rule {
	return Rule{
		Name:       name,
		Pattern:    pattern,
		Methods:    methods,
		Priority:   priority,
		Limit:      100,
		Window:     time.Minute,
		IdentifyBy: IdentifyByIP,
	}
}

func TestNew_EmptyRules(t *testing.T) {
	m, err := New(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result := m.Match("GET", "/anything"); result != nil {
		t.Fatal("expected nil match for empty rules")
	}
}

func TestNew_InvalidPattern(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
	}{
		{"empty pattern", ""},
		{"no leading slash", "no-slash"},
		{"empty param name", "/foo/:"},
		{"non-terminal wildcard", "/api/*/foo"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := New([]Rule{{Name: "bad", Pattern: tt.pattern}})
			if err == nil {
				t.Fatalf("expected error for pattern %q", tt.pattern)
			}
		})
	}
}

func TestNew_HeaderWithoutName(t *testing.T) {
	_, err := New([]Rule{{
		Name:       "bad-header",
		Pattern:    "/api/*",
		IdentifyBy: IdentifyByHeader,
		HeaderName: "",
	}})
	if err == nil {
		t.Fatal("expected error when IdentifyBy is header but HeaderName is empty")
	}
}

func TestMatch_ExactPath(t *testing.T) {
	m, err := New([]Rule{
		defaultRule("health", "/api/health", 1),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	result := m.Match("GET", "/api/health")
	if result == nil {
		t.Fatal("expected match for /api/health")
	}
	if result.Rule.Name != "health" {
		t.Fatalf("expected rule 'health', got %q", result.Rule.Name)
	}
	if len(result.Params) != 0 {
		t.Fatalf("expected no params, got %v", result.Params)
	}

	if m.Match("GET", "/api/healthz") != nil {
		t.Fatal("expected no match for /api/healthz")
	}
	if m.Match("GET", "/api") != nil {
		t.Fatal("expected no match for /api")
	}
}

func TestMatch_NamedParameters(t *testing.T) {
	m, err := New([]Rule{
		defaultRule("user", "/users/:id", 1),
		defaultRule("user-posts", "/users/:userId/posts/:postId", 1),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	result := m.Match("GET", "/users/42")
	if result == nil {
		t.Fatal("expected match for /users/42")
	}
	if result.Params["id"] != "42" {
		t.Fatalf("expected id=42, got %q", result.Params["id"])
	}

	result = m.Match("GET", "/users/abc/posts/99")
	if result == nil {
		t.Fatal("expected match for /users/abc/posts/99")
	}
	if result.Params["userId"] != "abc" {
		t.Fatalf("expected userId=abc, got %q", result.Params["userId"])
	}
	if result.Params["postId"] != "99" {
		t.Fatalf("expected postId=99, got %q", result.Params["postId"])
	}

	if m.Match("GET", "/users") != nil {
		t.Fatal("expected no match for /users")
	}
	if m.Match("GET", "/users/") != nil {
		t.Fatal("expected no match for /users/")
	}
}

func TestMatch_Wildcard(t *testing.T) {
	m, err := New([]Rule{
		defaultRule("api-all", "/api/*", 1),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	tests := []struct {
		path     string
		matches  bool
		wildcard string
	}{
		{"/api/health", true, "health"},
		{"/api/users/42", true, "users/42"},
		{"/api/", true, ""},
		{"/api", false, ""},
		{"/other", false, ""},
	}

	for _, tt := range tests {
		result := m.Match("GET", tt.path)
		if tt.matches && result == nil {
			t.Fatalf("expected match for %s", tt.path)
		}
		if !tt.matches && result != nil {
			t.Fatalf("expected no match for %s", tt.path)
		}
		if tt.matches && result.Params["*"] != tt.wildcard {
			t.Fatalf("path %s: expected wildcard=%q, got %q", tt.path, tt.wildcard, result.Params["*"])
		}
	}
}

func TestMatch_MethodFiltering(t *testing.T) {
	m, err := New([]Rule{
		defaultRule("read-only", "/api/data", 1, "GET", "HEAD"),
		defaultRule("write", "/api/data", 2, "POST", "PUT"),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// POST should match "write" (higher priority checked first)
	result := m.Match("POST", "/api/data")
	if result == nil {
		t.Fatal("expected match for POST /api/data")
	}
	if result.Rule.Name != "write" {
		t.Fatalf("expected rule 'write', got %q", result.Rule.Name)
	}

	// GET should match "read-only"
	result = m.Match("GET", "/api/data")
	if result == nil {
		t.Fatal("expected match for GET /api/data")
	}
	if result.Rule.Name != "read-only" {
		t.Fatalf("expected rule 'read-only', got %q", result.Rule.Name)
	}

	// DELETE should not match either
	if m.Match("DELETE", "/api/data") != nil {
		t.Fatal("expected no match for DELETE /api/data")
	}
}

func TestMatch_AllMethods(t *testing.T) {
	m, err := New([]Rule{
		defaultRule("catch-all", "/api/*", 1),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, method := range []string{"GET", "POST", "PUT", "DELETE", "PATCH", "OPTIONS"} {
		if m.Match(method, "/api/test") == nil {
			t.Fatalf("expected match for %s /api/test", method)
		}
	}
}

func TestMatch_PriorityOrder(t *testing.T) {
	m, err := New([]Rule{
		defaultRule("low-priority", "/api/*", 1),
		defaultRule("high-priority", "/api/health", 10),
		defaultRule("mid-priority", "/api/:resource", 5),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// /api/health should match high-priority (priority 10)
	result := m.Match("GET", "/api/health")
	if result == nil {
		t.Fatal("expected match for /api/health")
	}
	if result.Rule.Name != "high-priority" {
		t.Fatalf("expected 'high-priority', got %q", result.Rule.Name)
	}

	// /api/users should match mid-priority (priority 5)
	result = m.Match("GET", "/api/users")
	if result == nil {
		t.Fatal("expected match for /api/users")
	}
	if result.Rule.Name != "mid-priority" {
		t.Fatalf("expected 'mid-priority', got %q", result.Rule.Name)
	}

	// /api/users/42 should match low-priority (wildcard only)
	result = m.Match("GET", "/api/users/42")
	if result == nil {
		t.Fatal("expected match for /api/users/42")
	}
	if result.Rule.Name != "low-priority" {
		t.Fatalf("expected 'low-priority', got %q", result.Rule.Name)
	}
}

func TestMatch_CaseInsensitiveMethods(t *testing.T) {
	m, err := New([]Rule{
		defaultRule("r", "/test", 1, "get"),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if m.Match("GET", "/test") == nil {
		t.Fatal("expected match for uppercase GET")
	}
	if m.Match("get", "/test") == nil {
		t.Fatal("expected match for lowercase get")
	}
}

func TestMatch_IdentifierConfig(t *testing.T) {
	m, err := New([]Rule{
		{
			Name:       "api-key-rule",
			Pattern:    "/api/*",
			Priority:   1,
			Limit:      50,
			Window:     time.Minute,
			IdentifyBy: IdentifyByHeader,
			HeaderName: "X-API-Key",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	result := m.Match("GET", "/api/test")
	if result == nil {
		t.Fatal("expected match")
	}
	if result.Rule.IdentifyBy != IdentifyByHeader {
		t.Fatalf("expected IdentifyByHeader, got %v", result.Rule.IdentifyBy)
	}
	if result.Rule.HeaderName != "X-API-Key" {
		t.Fatalf("expected X-API-Key header, got %q", result.Rule.HeaderName)
	}
}

func TestMatch_NoMatch(t *testing.T) {
	m, err := New([]Rule{
		defaultRule("api", "/api/*", 1),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if m.Match("GET", "/other/path") != nil {
		t.Fatal("expected no match for /other/path")
	}
	if m.Match("GET", "/") != nil {
		t.Fatal("expected no match for /")
	}
}

func TestMatch_SpecialCharsInPath(t *testing.T) {
	m, err := New([]Rule{
		defaultRule("dots", "/api/v1.0/health", 1),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if m.Match("GET", "/api/v1.0/health") == nil {
		t.Fatal("expected match for path with dots")
	}
	// Ensure the dot is literal, not regex wildcard
	if m.Match("GET", "/api/v1X0/health") != nil {
		t.Fatal("expected no match: dot should be literal")
	}
}

// BenchmarkMatch validates the <0.1ms lookup performance requirement.
func BenchmarkMatch(b *testing.B) {
	rules := []Rule{
		defaultRule("exact", "/api/health", 10),
		defaultRule("param", "/api/users/:id", 5),
		defaultRule("param-nested", "/api/users/:id/posts/:postId", 5),
		defaultRule("wildcard", "/api/*", 1),
		defaultRule("admin", "/admin/*", 1, "GET"),
		defaultRule("write", "/api/data", 8, "POST", "PUT"),
	}

	m, err := New(rules)
	if err != nil {
		b.Fatalf("unexpected error: %v", err)
	}

	paths := []struct {
		method string
		path   string
	}{
		{"GET", "/api/health"},
		{"GET", "/api/users/42"},
		{"GET", "/api/users/42/posts/99"},
		{"POST", "/api/data"},
		{"GET", "/api/unknown/deep/path"},
		{"GET", "/no-match"},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p := paths[i%len(paths)]
		m.Match(p.method, p.path)
	}
}
