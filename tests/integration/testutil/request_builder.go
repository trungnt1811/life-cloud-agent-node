package testutil

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// RequestBuilder provides a fluent API for HTTP testing.
type RequestBuilder struct {
	t       *testing.T
	handler http.Handler

	method  string
	path    string
	headers map[string]string
	query   url.Values
	body    io.Reader
}

func NewRequestBuilder(t *testing.T, handler http.Handler) *RequestBuilder {
	t.Helper()
	return &RequestBuilder{
		t:       t,
		handler: handler,
		headers: make(map[string]string),
		query:   url.Values{},
	}
}

func (r *RequestBuilder) When() *RequestBuilder { return r }

func (r *RequestBuilder) GET(path string) *RequestBuilder {
	r.method = http.MethodGet
	r.path = path
	return r
}

func (r *RequestBuilder) POST(path string) *RequestBuilder {
	r.method = http.MethodPost
	r.path = path
	return r
}

func (r *RequestBuilder) Header(key, value string) *RequestBuilder {
	r.headers[key] = value
	return r
}

func (r *RequestBuilder) BearerToken(token string) *RequestBuilder {
	r.headers["Authorization"] = "Bearer " + token
	return r
}

func (r *RequestBuilder) BasicAuth(username, password string) *RequestBuilder {
	auth := username + ":" + password
	r.headers["Authorization"] = "Basic " + base64.StdEncoding.EncodeToString([]byte(auth))
	return r
}

func (r *RequestBuilder) Query(key, value string) *RequestBuilder {
	r.query.Add(key, value)
	return r
}

func (r *RequestBuilder) JSON(body any) *RequestBuilder {
	jsonBytes, err := json.Marshal(body)
	if err != nil {
		r.t.Fatalf("failed to marshal JSON body: %v", err)
	}
	r.body = bytes.NewReader(jsonBytes)
	r.headers["Content-Type"] = "application/json"
	return r
}

func (r *RequestBuilder) Body(body string) *RequestBuilder {
	r.body = strings.NewReader(body)
	return r
}

func (r *RequestBuilder) Then() *ResponseAssertion {
	targetURL := r.path
	if len(r.query) > 0 {
		targetURL += "?" + r.query.Encode()
	}

	req := httptest.NewRequest(r.method, targetURL, r.body)
	for key, value := range r.headers {
		req.Header.Set(key, value)
	}

	rec := httptest.NewRecorder()
	r.handler.ServeHTTP(rec, req)

	return &ResponseAssertion{t: r.t, resp: rec}
}
