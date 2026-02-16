//go:build integration

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Siruyy/gatify/internal/api"
)

type failingRuleRepository struct {
	err error
}

func (r *failingRuleRepository) Create(context.Context, api.Rule) (api.Rule, error) {
	return api.Rule{}, r.err
}

func (r *failingRuleRepository) List(context.Context) ([]api.Rule, error) {
	return nil, r.err
}

func (r *failingRuleRepository) GetByID(context.Context, string) (api.Rule, error) {
	return api.Rule{}, r.err
}

func (r *failingRuleRepository) Update(context.Context, string, api.Rule) (api.Rule, error) {
	return api.Rule{}, r.err
}

func (r *failingRuleRepository) Delete(context.Context, string) error {
	return r.err
}

func TestManagementAPI_AuthIntegration(t *testing.T) {
	server := newManagementAPITestServer(t, api.NewInMemoryRepository(), "super-secret")
	t.Cleanup(server.Close)

	resp := doRequest(t, server, requestSpec{
		method: http.MethodGet,
		path:   "/api/rules",
	})
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected %d, got %d body=%s", http.StatusUnauthorized, resp.StatusCode, resp.Body)
	}

	resp = doRequest(t, server, requestSpec{
		method: http.MethodGet,
		path:   "/api/rules",
		token:  "wrong-token",
	})
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected %d, got %d body=%s", http.StatusForbidden, resp.StatusCode, resp.Body)
	}
}

func TestManagementAPI_CRUDHappyPathIntegration(t *testing.T) {
	server := newManagementAPITestServer(t, api.NewInMemoryRepository(), "super-secret")
	t.Cleanup(server.Close)

	createBody := `{
		"name":"users-limit",
		"pattern":"/api/users/:id",
		"methods":["GET","POST"],
		"priority":5,
		"limit":100,
		"window_seconds":60,
		"identify_by":"ip",
		"enabled":true
	}`

	createResp := doRequest(t, server, requestSpec{
		method: http.MethodPost,
		path:   "/api/rules",
		token:  "super-secret",
		body:   createBody,
	})
	if createResp.StatusCode != http.StatusCreated {
		t.Fatalf("expected %d, got %d body=%s", http.StatusCreated, createResp.StatusCode, createResp.Body)
	}

	created := decodeRulePayload(t, createResp.Body)
	if created.ID == "" {
		t.Fatal("expected non-empty rule id")
	}

	listResp := doRequest(t, server, requestSpec{
		method: http.MethodGet,
		path:   "/api/rules",
		token:  "super-secret",
	})
	if listResp.StatusCode != http.StatusOK {
		t.Fatalf("expected %d, got %d body=%s", http.StatusOK, listResp.StatusCode, listResp.Body)
	}

	rules := decodeRulesPayload(t, listResp.Body)
	if len(rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(rules))
	}

	updateBody := `{
		"name":"users-limit-updated",
		"pattern":"/api/users/:id",
		"methods":["PUT"],
		"priority":7,
		"limit":50,
		"window_seconds":120,
		"identify_by":"header",
		"header_name":"X-User-ID",
		"enabled":false
	}`

	updateResp := doRequest(t, server, requestSpec{
		method: http.MethodPut,
		path:   "/api/rules/" + created.ID,
		token:  "super-secret",
		body:   updateBody,
	})
	if updateResp.StatusCode != http.StatusOK {
		t.Fatalf("expected %d, got %d body=%s", http.StatusOK, updateResp.StatusCode, updateResp.Body)
	}

	updated := decodeRulePayload(t, updateResp.Body)
	if updated.Name != "users-limit-updated" {
		t.Fatalf("expected updated name, got %q", updated.Name)
	}
	if updated.IdentifyBy != "header" || updated.HeaderName != "X-User-ID" {
		t.Fatalf("unexpected identify_by/header_name: %q / %q", updated.IdentifyBy, updated.HeaderName)
	}

	deleteResp := doRequest(t, server, requestSpec{
		method: http.MethodDelete,
		path:   "/api/rules/" + created.ID,
		token:  "super-secret",
	})
	if deleteResp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected %d, got %d body=%s", http.StatusNoContent, deleteResp.StatusCode, deleteResp.Body)
	}

	getDeletedResp := doRequest(t, server, requestSpec{
		method: http.MethodGet,
		path:   "/api/rules/" + created.ID,
		token:  "super-secret",
	})
	if getDeletedResp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected %d, got %d body=%s", http.StatusNotFound, getDeletedResp.StatusCode, getDeletedResp.Body)
	}
}

func TestManagementAPI_ErrorCasesIntegration(t *testing.T) {
	server := newManagementAPITestServer(t, api.NewInMemoryRepository(), "super-secret")
	t.Cleanup(server.Close)

	badJSONResp := doRequest(t, server, requestSpec{
		method: http.MethodPost,
		path:   "/api/rules",
		token:  "super-secret",
		body:   `{`,
	})
	if badJSONResp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected %d, got %d body=%s", http.StatusBadRequest, badJSONResp.StatusCode, badJSONResp.Body)
	}

	unknownFieldResp := doRequest(t, server, requestSpec{
		method: http.MethodPost,
		path:   "/api/rules",
		token:  "super-secret",
		body:   `{"name":"n","pattern":"/a","limit":1,"window_seconds":1,"unknown":true}`,
	})
	if unknownFieldResp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected %d, got %d body=%s", http.StatusBadRequest, unknownFieldResp.StatusCode, unknownFieldResp.Body)
	}

	notFoundResp := doRequest(t, server, requestSpec{
		method: http.MethodGet,
		path:   "/api/rules/missing",
		token:  "super-secret",
	})
	if notFoundResp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected %d, got %d body=%s", http.StatusNotFound, notFoundResp.StatusCode, notFoundResp.Body)
	}
}

func TestManagementAPI_InternalServerErrorIntegration(t *testing.T) {
	server := newManagementAPITestServer(t, &failingRuleRepository{err: errors.New("db unavailable")}, "super-secret")
	t.Cleanup(server.Close)

	listResp := doRequest(t, server, requestSpec{
		method: http.MethodGet,
		path:   "/api/rules",
		token:  "super-secret",
	})
	if listResp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("expected %d, got %d body=%s", http.StatusInternalServerError, listResp.StatusCode, listResp.Body)
	}

	createResp := doRequest(t, server, requestSpec{
		method: http.MethodPost,
		path:   "/api/rules",
		token:  "super-secret",
		body:   `{"name":"n","pattern":"/a","limit":1,"window_seconds":1}`,
	})
	if createResp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("expected %d, got %d body=%s", http.StatusInternalServerError, createResp.StatusCode, createResp.Body)
	}

	getResp := doRequest(t, server, requestSpec{
		method: http.MethodGet,
		path:   "/api/rules/rule_1",
		token:  "super-secret",
	})
	if getResp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("expected %d, got %d body=%s", http.StatusInternalServerError, getResp.StatusCode, getResp.Body)
	}
}

type requestSpec struct {
	method string
	path   string
	token  string
	body   string
}

type responseSpec struct {
	StatusCode int
	Body       string
}

func newManagementAPITestServer(t *testing.T, repo api.RuleRepository, adminToken string) *httptest.Server {
	t.Helper()

	h := api.NewRulesHandler(repo)
	protected := requireAdminToken(adminToken, h)

	mux := http.NewServeMux()
	mux.Handle("/api/rules", protected)
	mux.Handle("/api/rules/", protected)

	return httptest.NewServer(mux)
}

func doRequest(t *testing.T, server *httptest.Server, reqSpec requestSpec) responseSpec {
	t.Helper()

	var bodyReader io.Reader
	if reqSpec.body != "" {
		bodyReader = bytes.NewBufferString(reqSpec.body)
	}

	req, err := http.NewRequest(reqSpec.method, server.URL+reqSpec.path, bodyReader)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	if reqSpec.body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if strings.TrimSpace(reqSpec.token) != "" {
		req.Header.Set("Authorization", "Bearer "+reqSpec.token)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	bodyBytes, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		t.Fatalf("failed to read response body: %v", readErr)
	}

	return responseSpec{StatusCode: resp.StatusCode, Body: string(bodyBytes)}
}

func decodeRulePayload(t *testing.T, body string) api.Rule {
	t.Helper()

	var payload struct {
		Data api.Rule `json:"data"`
	}

	if err := json.Unmarshal([]byte(body), &payload); err != nil {
		t.Fatalf("failed to decode rule payload: %v body=%s", err, body)
	}

	return payload.Data
}

func decodeRulesPayload(t *testing.T, body string) []api.Rule {
	t.Helper()

	var payload struct {
		Data []api.Rule `json:"data"`
	}

	if err := json.Unmarshal([]byte(body), &payload); err != nil {
		t.Fatalf("failed to decode rules payload: %v body=%s", err, body)
	}

	return payload.Data
}
