package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Siruyy/gatify/internal/rules"
)

var (
	// ErrNotFound indicates the requested rule does not exist.
	ErrNotFound = errors.New("rule not found")
)

// Rule is the persisted API representation of a rate limit rule.
type Rule struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	Pattern       string    `json:"pattern"`
	Methods       []string  `json:"methods,omitempty"`
	Priority      int       `json:"priority"`
	Limit         int64     `json:"limit"`
	WindowSeconds int64     `json:"window_seconds"`
	IdentifyBy    string    `json:"identify_by"`
	HeaderName    string    `json:"header_name,omitempty"`
	Enabled       bool      `json:"enabled"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// RuleRequest is the request payload for create/update operations.
type RuleRequest struct {
	Name          string  `json:"name"`
	Pattern       string  `json:"pattern"`
	Methods       []string `json:"methods,omitempty"`
	Priority      int     `json:"priority"`
	Limit         int64   `json:"limit"`
	WindowSeconds int64   `json:"window_seconds"`
	IdentifyBy    string  `json:"identify_by,omitempty"`
	HeaderName    string  `json:"header_name,omitempty"`
	Enabled       *bool   `json:"enabled,omitempty"`
}

// RuleRepository defines persistence behavior for rules API.
type RuleRepository interface {
	Create(ctx context.Context, rule Rule) (Rule, error)
	List(ctx context.Context) ([]Rule, error)
	GetByID(ctx context.Context, id string) (Rule, error)
	Update(ctx context.Context, id string, rule Rule) (Rule, error)
	Delete(ctx context.Context, id string) error
}

// InMemoryRepository is a thread-safe in-memory implementation of RuleRepository.
type InMemoryRepository struct {
	mu     sync.RWMutex
	rules  map[string]Rule
	nextID uint64
}

// NewInMemoryRepository creates a new in-memory repository.
func NewInMemoryRepository() *InMemoryRepository {
	return &InMemoryRepository{rules: make(map[string]Rule)}
}

// Create stores a new rule.
func (r *InMemoryRepository) Create(_ context.Context, rule Rule) (Rule, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now().UTC()
	r.nextID++
	id := "rule_" + strconv.FormatUint(r.nextID, 10)

	rule.ID = id
	rule.CreatedAt = now
	rule.UpdatedAt = now

	r.rules[id] = cloneRule(rule)

	return cloneRule(rule), nil
}

// List returns all stored rules sorted by creation time then id.
func (r *InMemoryRepository) List(_ context.Context) ([]Rule, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]Rule, 0, len(r.rules))
	for _, rule := range r.rules {
		out = append(out, cloneRule(rule))
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].CreatedAt.Equal(out[j].CreatedAt) {
			return out[i].ID < out[j].ID
		}
		return out[i].CreatedAt.Before(out[j].CreatedAt)
	})

	return out, nil
}

// GetByID retrieves a rule by id.
func (r *InMemoryRepository) GetByID(_ context.Context, id string) (Rule, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	rule, ok := r.rules[id]
	if !ok {
		return Rule{}, ErrNotFound
	}

	return cloneRule(rule), nil
}

// Update replaces an existing rule.
func (r *InMemoryRepository) Update(_ context.Context, id string, rule Rule) (Rule, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	existing, ok := r.rules[id]
	if !ok {
		return Rule{}, ErrNotFound
	}

	rule.ID = id
	rule.CreatedAt = existing.CreatedAt
	rule.UpdatedAt = time.Now().UTC()

	r.rules[id] = cloneRule(rule)

	return cloneRule(rule), nil
}

// Delete removes a rule by id.
func (r *InMemoryRepository) Delete(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.rules[id]; !ok {
		return ErrNotFound
	}

	delete(r.rules, id)

	return nil
}

// RulesHandler provides REST CRUD endpoints for rules.
type RulesHandler struct {
	repo RuleRepository
}

// NewRulesHandler creates a rules REST API handler.
func NewRulesHandler(repo RuleRepository) *RulesHandler {
	return &RulesHandler{repo: repo}
}

// ServeHTTP handles /api/rules and /api/rules/:id.
func (h *RulesHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	switch {
	case path == "/api/rules":
		h.handleCollection(w, r)
		return
	case strings.HasPrefix(path, "/api/rules/"):
		id := strings.TrimPrefix(path, "/api/rules/")
		if id == "" {
			h.handleCollection(w, r)
			return
		}
		if strings.Contains(id, "/") {
			http.NotFound(w, r)
			return
		}
		h.handleItem(w, r, id)
		return
	default:
		http.NotFound(w, r)
		return
	}
}

func (h *RulesHandler) handleCollection(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		rulesList, err := h.repo.List(r.Context())
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list rules"})
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{"data": rulesList})
	case http.MethodPost:
		var req RuleRequest
		if err := decodeJSON(r, &req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}

		rule, err := validateAndBuildRule(req)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}

		created, err := h.repo.Create(r.Context(), rule)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to create rule"})
			return
		}

		writeJSON(w, http.StatusCreated, map[string]any{"data": created})
	default:
		w.Header().Set("Allow", "GET, POST")
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func (h *RulesHandler) handleItem(w http.ResponseWriter, r *http.Request, id string) {
	switch r.Method {
	case http.MethodGet:
		rule, err := h.repo.GetByID(r.Context(), id)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				writeJSON(w, http.StatusNotFound, map[string]string{"error": "rule not found"})
				return
			}

			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to get rule"})
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{"data": rule})
	case http.MethodPut:
		var req RuleRequest
		if err := decodeJSON(r, &req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}

		rule, err := validateAndBuildRule(req)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}

		updated, err := h.repo.Update(r.Context(), id, rule)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				writeJSON(w, http.StatusNotFound, map[string]string{"error": "rule not found"})
				return
			}

			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to update rule"})
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{"data": updated})
	case http.MethodDelete:
		err := h.repo.Delete(r.Context(), id)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				writeJSON(w, http.StatusNotFound, map[string]string{"error": "rule not found"})
				return
			}

			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to delete rule"})
			return
		}

		w.WriteHeader(http.StatusNoContent)
	default:
		w.Header().Set("Allow", "GET, PUT, DELETE")
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func decodeJSON(r *http.Request, dst any) error {
	if r.Body == nil {
		return fmt.Errorf("request body is required")
	}

	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	if err := dec.Decode(dst); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}

	if err := dec.Decode(&struct{}{}); err == nil {
		return fmt.Errorf("request body must contain a single JSON object")
	} else if !errors.Is(err, io.EOF) {
		return fmt.Errorf("request body must contain a single JSON object")
	}

	return nil
}

func validateAndBuildRule(req RuleRequest) (Rule, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return Rule{}, fmt.Errorf("name is required")
	}

	pattern := strings.TrimSpace(req.Pattern)
	if pattern == "" {
		return Rule{}, fmt.Errorf("pattern is required")
	}

	if req.Limit <= 0 {
		return Rule{}, fmt.Errorf("limit must be greater than 0")
	}

	if req.WindowSeconds <= 0 {
		return Rule{}, fmt.Errorf("window_seconds must be greater than 0")
	}

	identifyBy := rules.IdentifyBy(strings.ToLower(strings.TrimSpace(req.IdentifyBy)))
	if identifyBy == "" {
		identifyBy = rules.IdentifyByIP
	}

	if identifyBy != rules.IdentifyByIP && identifyBy != rules.IdentifyByHeader {
		return Rule{}, fmt.Errorf("identify_by must be one of: ip, header")
	}

	headerName := strings.TrimSpace(req.HeaderName)
	if identifyBy == rules.IdentifyByHeader && headerName == "" {
		return Rule{}, fmt.Errorf("header_name is required when identify_by=header")
	}

	normalizedMethods := make([]string, 0, len(req.Methods))
	seenMethods := make(map[string]bool, len(req.Methods))
	for _, method := range req.Methods {
		m := strings.ToUpper(strings.TrimSpace(method))
		if m == "" {
			return Rule{}, fmt.Errorf("methods cannot contain empty values")
		}
		if !isValidHTTPMethod(m) {
			return Rule{}, fmt.Errorf("invalid HTTP method %q", m)
		}
		if seenMethods[m] {
			continue
		}
		seenMethods[m] = true
		normalizedMethods = append(normalizedMethods, m)
	}

	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	engineRule := rules.Rule{
		Name:       name,
		Pattern:    pattern,
		Methods:    normalizedMethods,
		Priority:   req.Priority,
		Limit:      req.Limit,
		Window:     time.Duration(req.WindowSeconds) * time.Second,
		IdentifyBy: identifyBy,
		HeaderName: headerName,
	}

	if _, err := rules.New([]rules.Rule{engineRule}); err != nil {
		return Rule{}, err
	}

	return Rule{
		Name:          name,
		Pattern:       pattern,
		Methods:       normalizedMethods,
		Priority:      req.Priority,
		Limit:         req.Limit,
		WindowSeconds: req.WindowSeconds,
		IdentifyBy:    string(identifyBy),
		HeaderName:    headerName,
		Enabled:       enabled,
	}, nil
}

func isValidHTTPMethod(method string) bool {
	switch method {
	case http.MethodGet,
		http.MethodHead,
		http.MethodPost,
		http.MethodPut,
		http.MethodPatch,
		http.MethodDelete,
		http.MethodConnect,
		http.MethodOptions,
		http.MethodTrace:
		return true
	default:
		return false
	}
}

func cloneRule(rule Rule) Rule {
	copyRule := rule
	if rule.Methods != nil {
		copyRule.Methods = append([]string(nil), rule.Methods...)
	}
	return copyRule
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	payload, err := json.Marshal(body)
	if err != nil {
		log.Printf("api: failed to encode JSON response: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if _, err := w.Write(append(payload, '\n')); err != nil {
		log.Printf("api: failed to write JSON response: %v", err)
	}
}