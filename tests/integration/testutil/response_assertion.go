package testutil

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

// ResponseAssertion provides fluent assertions for HTTP responses.
type ResponseAssertion struct {
	t    *testing.T
	resp *httptest.ResponseRecorder
}

func (a *ResponseAssertion) Status(expectedCode int) *ResponseAssertion {
	if a.resp.Code != expectedCode {
		a.t.Fatalf("expected status %d, got %d. body=%s", expectedCode, a.resp.Code, a.resp.Body.String())
	}
	return a
}

func (a *ResponseAssertion) JSONPath(path string, expected any) *ResponseAssertion {
	value, err := a.extractJSONPath(path)
	if err != nil {
		a.t.Fatalf("failed to read JSON path %q: %v. body=%s", path, err, a.resp.Body.String())
	}

	if fmt.Sprint(value) != fmt.Sprint(expected) {
		a.t.Fatalf("json path %q expected=%v got=%v. body=%s", path, expected, value, a.resp.Body.String())
	}
	return a
}

func (a *ResponseAssertion) JSONPathExists(path string) *ResponseAssertion {
	_, err := a.extractJSONPath(path)
	if err != nil {
		a.t.Fatalf("expected JSON path %q to exist: %v. body=%s", path, err, a.resp.Body.String())
	}
	return a
}

func (a *ResponseAssertion) GetBody() string {
	return a.resp.Body.String()
}

func (a *ResponseAssertion) extractJSONPath(path string) (any, error) {
	var payload any
	if err := json.Unmarshal(a.resp.Body.Bytes(), &payload); err != nil {
		return nil, fmt.Errorf("invalid json: %w", err)
	}

	current := payload
	parts := strings.Split(path, ".")
	for _, part := range parts {
		switch node := current.(type) {
		case map[string]any:
			next, ok := node[part]
			if !ok {
				return nil, fmt.Errorf("key %q not found", part)
			}
			current = next
		case []any:
			idx, err := strconv.Atoi(part)
			if err != nil {
				return nil, fmt.Errorf("expected array index, got %q", part)
			}
			if idx < 0 || idx >= len(node) {
				return nil, fmt.Errorf("array index out of range: %d", idx)
			}
			current = node[idx]
		default:
			return nil, fmt.Errorf("cannot traverse %q on type %T", part, current)
		}
	}

	return current, nil
}
