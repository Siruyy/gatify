package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRulesAPI_CRUD(t *testing.T) {
	h := NewRulesHandler(NewInMemoryRepository())

	createBody := `{
		"name": "users-limit",
		"pattern": "/api/users/:id",
		"methods": ["get", "post"],
		"priority": 10,
		"limit": 100,
		"window_seconds": 60,
		"identify_by": "ip",
		"enabled": true
	}`

	createResp := performRequest(t, h, http.MethodPost, "/api/rules", createBody)
	if createResp.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusCreated, createResp.Code, createResp.Body.String())
	}

	created := decodeDataRule(t, createResp.Body.Bytes())
	if created.ID == "" {
		t.Fatal("expected created rule id")
	}
	if created.Methods[0] != "GET" || created.Methods[1] != "POST" {
		t.Fatalf("expected normalized methods [GET POST], got %v", created.Methods)
	}

	listResp := performRequest(t, h, http.MethodGet, "/api/rules", "")
	if listResp.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, listResp.Code)
	}

	list := decodeDataRules(t, listResp.Body.Bytes())
	if len(list) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(list))
	}

	getResp := performRequest(t, h, http.MethodGet, "/api/rules/"+created.ID, "")
	if getResp.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, getResp.Code)
	}

	updateBody := `{
		"name": "users-limit-updated",
		"pattern": "/api/users/:id",
		"methods": ["PUT"],
		"priority": 20,
		"limit": 50,
		"window_seconds": 120,
		"identify_by": "header",
		"header_name": "X-API-Key",
		"enabled": false
	}`

	updateResp := performRequest(t, h, http.MethodPut, "/api/rules/"+created.ID, updateBody)
	if updateResp.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, updateResp.Code, updateResp.Body.String())
	}

	updated := decodeDataRule(t, updateResp.Body.Bytes())
	if updated.Name != "users-limit-updated" {
		t.Fatalf("expected updated name, got %q", updated.Name)
	}
	if updated.IdentifyBy != "header" || updated.HeaderName != "X-API-Key" {
		t.Fatalf("expected header identification, got identify_by=%q header_name=%q", updated.IdentifyBy, updated.HeaderName)
	}
	if updated.Enabled {
		t.Fatalf("expected enabled=false after update")
	}

	deleteResp := performRequest(t, h, http.MethodDelete, "/api/rules/"+created.ID, "")
	if deleteResp.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d", http.StatusNoContent, deleteResp.Code)
	}

	getDeletedResp := performRequest(t, h, http.MethodGet, "/api/rules/"+created.ID, "")
	if getDeletedResp.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, getDeletedResp.Code)
	}
}

func TestRulesAPI_CreateValidation(t *testing.T) {
	h := NewRulesHandler(NewInMemoryRepository())

	resp := performRequest(t, h, http.MethodPost, "/api/rules", `{"name":"","pattern":"/api/*","limit":10,"window_seconds":60}`)
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, resp.Code)
	}

	resp = performRequest(t, h, http.MethodPost, "/api/rules", `{"name":"n","pattern":"api/*","limit":10,"window_seconds":60}`)
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d for invalid pattern, got %d", http.StatusBadRequest, resp.Code)
	}

	resp = performRequest(t, h, http.MethodPost, "/api/rules", `{"name":"n","pattern":"/api/*","limit":0,"window_seconds":60}`)
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d for invalid limit, got %d", http.StatusBadRequest, resp.Code)
	}

	resp = performRequest(t, h, http.MethodPost, "/api/rules", `{"name":"n","pattern":"/api/*","limit":10,"window_seconds":60,"identify_by":"header"}`)
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d for missing header_name, got %d", http.StatusBadRequest, resp.Code)
	}
}

func TestRulesAPI_MethodNotAllowed(t *testing.T) {
	h := NewRulesHandler(NewInMemoryRepository())

	resp := performRequest(t, h, http.MethodPatch, "/api/rules", "")
	if resp.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, resp.Code)
	}

	resp = performRequest(t, h, http.MethodPost, "/api/rules/nonexistent", `{"name":"n","pattern":"/a","limit":1,"window_seconds":1}`)
	if resp.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, resp.Code)
	}
}

func TestRulesAPI_NotFound(t *testing.T) {
	h := NewRulesHandler(NewInMemoryRepository())

	resp := performRequest(t, h, http.MethodGet, "/api/rules/missing", "")
	if resp.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, resp.Code)
	}

	resp = performRequest(t, h, http.MethodDelete, "/api/rules/missing", "")
	if resp.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, resp.Code)
	}
}

func performRequest(t *testing.T, h http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()

	var req *http.Request
	if body == "" {
		req = httptest.NewRequest(method, path, nil)
	} else {
		req = httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	}

	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	return w
}

func decodeDataRule(t *testing.T, body []byte) Rule {
	t.Helper()

	var payload struct {
		Data Rule `json:"data"`
	}

	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("failed to decode response: %v body=%s", err, string(body))
	}

	return payload.Data
}

func decodeDataRules(t *testing.T, body []byte) []Rule {
	t.Helper()

	var payload struct {
		Data []Rule `json:"data"`
	}

	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("failed to decode response: %v body=%s", err, string(body))
	}

	return payload.Data
}