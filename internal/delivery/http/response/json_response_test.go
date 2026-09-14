package response

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestErrorPassesErrorsPayloadThrough(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	details := []map[string]string{{
		"field": "name",
		"code":  "INVALID_NAME",
	}}

	Error(context, http.StatusBadRequest, "INVALID_REQUEST", "invalid request", details)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	var got Response
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &got))
	require.Equal(t, http.StatusBadRequest, got.Status)
	require.Equal(t, "INVALID_REQUEST", got.Code)
	require.Equal(t, "invalid request", got.Message)
	require.NotNil(t, got.Errors)
}

func TestErrorDefaultsEmptyMessage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)

	Error(context, http.StatusInternalServerError, "INTERNAL_ERROR", "", nil)

	require.Equal(t, http.StatusInternalServerError, recorder.Code)
	var got Response
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &got))
	require.Equal(t, http.StatusInternalServerError, got.Status)
	require.Equal(t, "INTERNAL_ERROR", got.Code)
	require.Equal(t, "Failed", got.Message)
}
