// Package rules provides a pattern-based rule matching engine for
// per-route rate limiting configuration.
package rules

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"
)

// IdentifyBy determines how to extract the client identifier from a request.
type IdentifyBy string

const (
	// IdentifyByIP extracts the client IP address (default).
	IdentifyByIP IdentifyBy = "ip"
	// IdentifyByHeader extracts a value from a specific request header.
	IdentifyByHeader IdentifyBy = "header"
)

// Rule defines a rate limiting rule that applies to matched requests.
type Rule struct {
	// Name is a human-readable identifier for the rule.
	Name string
	// Pattern is the path pattern to match against.
	// Supports exact paths (/api/health), named parameters (/users/:id),
	// and wildcards (/api/*).
	Pattern string
	// Methods lists the HTTP methods this rule applies to.
	// An empty slice matches all methods.
	Methods []string
	// Priority determines match order; higher values are checked first.
	// When multiple rules match, the highest priority wins.
	Priority int
	// Limit is the maximum number of requests allowed per Window.
	Limit int64
	// Window is the sliding window duration for rate limiting.
	Window time.Duration
	// IdentifyBy determines how to extract the client identifier.
	// Defaults to IdentifyByIP if empty.
	IdentifyBy IdentifyBy
	// HeaderName is the header to use when IdentifyBy is IdentifyByHeader.
	HeaderName string
}

// Match holds a matched rule along with any extracted path parameters.
type Match struct {
	// Rule is the matched rate limiting rule.
	Rule Rule
	// Params contains named path parameters extracted from the request path.
	// For example, pattern "/users/:id" matching "/users/42" yields {"id": "42"}.
	Params map[string]string
}

// Matcher evaluates request paths and methods against compiled rules.
type Matcher struct {
	compiled []compiledRule
}

type compiledRule struct {
	rule       Rule
	regex      *regexp.Regexp
	paramNames []string
	methods    map[string]bool // nil means all methods
}

// New compiles the provided rules and returns a Matcher.
// Rules are sorted by priority descending so the highest-priority
// matching rule is returned first. Returns an error if any rule
// has an invalid pattern.
func New(rules []Rule) (*Matcher, error) {
	if len(rules) == 0 {
		return &Matcher{}, nil
	}

	compiled := make([]compiledRule, 0, len(rules))
	for _, r := range rules {
		cr, err := compile(r)
		if err != nil {
			return nil, fmt.Errorf("rules: failed to compile rule %q: %w", r.Name, err)
		}
		compiled = append(compiled, cr)
	}

	sort.SliceStable(compiled, func(i, j int) bool {
		return compiled[i].rule.Priority > compiled[j].rule.Priority
	})

	return &Matcher{compiled: compiled}, nil
}

// Match returns the highest-priority rule matching the given method and path.
// Returns nil if no rule matches.
func (m *Matcher) Match(method, path string) *Match {
	upperMethod := strings.ToUpper(method)

	for _, cr := range m.compiled {
		if cr.methods != nil && !cr.methods[upperMethod] {
			continue
		}

		matches := cr.regex.FindStringSubmatch(path)
		if matches == nil {
			continue
		}

		var params map[string]string
		if len(cr.paramNames) > 0 {
			params = make(map[string]string, len(cr.paramNames))
			for i, name := range cr.paramNames {
				if i+1 < len(matches) {
					params[name] = matches[i+1]
				}
			}
		}

		return &Match{
			Rule:   cr.rule,
			Params: params,
		}
	}

	return nil
}

// compile converts a Rule into a compiledRule with a pre-compiled regex.
func compile(r Rule) (compiledRule, error) {
	if r.Pattern == "" {
		return compiledRule{}, fmt.Errorf("pattern is required")
	}
	if r.IdentifyBy == IdentifyByHeader && r.HeaderName == "" {
		return compiledRule{}, fmt.Errorf("HeaderName is required when IdentifyBy is %q", IdentifyByHeader)
	}

	pattern, paramNames, err := patternToRegex(r.Pattern)
	if err != nil {
		return compiledRule{}, err
	}

	regex, err := regexp.Compile(pattern)
	if err != nil {
		return compiledRule{}, fmt.Errorf("invalid pattern %q: %w", r.Pattern, err)
	}

	var methods map[string]bool
	if len(r.Methods) > 0 {
		methods = make(map[string]bool, len(r.Methods))
		for _, m := range r.Methods {
			methods[strings.ToUpper(m)] = true
		}
	}

	return compiledRule{
		rule:       r,
		regex:      regex,
		paramNames: paramNames,
		methods:    methods,
	}, nil
}

// patternToRegex converts a path pattern to a regex string and extracts
// parameter names.
//
// Supported patterns:
//   - /exact/path    -> ^/exact/path$
//   - /users/:id     -> ^/users/([^/]+)$
//   - /api/*         -> ^/api/(.*)
func patternToRegex(pattern string) (string, []string, error) {
	if pattern[0] != '/' {
		return "", nil, fmt.Errorf("pattern must start with /")
	}

	var (
		result     strings.Builder
		paramNames []string
	)

	result.WriteString("^")

	segments := strings.Split(pattern, "/")
	for i, seg := range segments {
		if i == 0 {
			continue // skip the empty segment before the leading /
		}

		result.WriteString("/")

		switch {
		case seg == "*":
			if i != len(segments)-1 {
				return "", nil, fmt.Errorf("wildcard (*) must be the last segment")
			}
			result.WriteString("(.*)")
			paramNames = append(paramNames, "*")
		case strings.HasPrefix(seg, ":"):
			name := seg[1:]
			if name == "" {
				return "", nil, fmt.Errorf("empty parameter name in pattern")
			}
			if !isValidIdentifier(name) {
				return "", nil, fmt.Errorf("invalid parameter name %q: must start with letter or underscore, followed by letters, digits, or underscores", name)
			}
			result.WriteString("([^/]+)")
			paramNames = append(paramNames, name)
		default:
			result.WriteString(regexp.QuoteMeta(seg))
		}
	}

	// Only add $ anchor if the pattern does not end with a wildcard
	if !strings.HasSuffix(pattern, "*") {
		result.WriteString("$")
	}

	return result.String(), paramNames, nil
}

// isValidIdentifier checks if a string is a valid Go-style identifier.
// Valid identifiers start with a letter or underscore, followed by
// letters, digits, or underscores.
func isValidIdentifier(s string) bool {
	if s == "" {
		return false
	}
	for i, r := range s {
		if i == 0 {
			if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || r == '_') {
				return false
			}
		} else {
			if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_') {
				return false
			}
		}
	}
	return true
}
