package testutil

import (
	"net/http"
	"testing"
)

var sharedRouter http.Handler

// SetRouter sets a shared router for integration tests.
func SetRouter(router http.Handler) {
	sharedRouter = router
}

// NewRestAssured creates a request builder using the shared test router.
func NewRestAssured(t *testing.T) *RequestBuilder {
	t.Helper()
	if sharedRouter == nil {
		t.Fatal("testutil.SetRouter() must be called before NewRestAssured()")
	}
	return NewRequestBuilder(t, sharedRouter)
}
