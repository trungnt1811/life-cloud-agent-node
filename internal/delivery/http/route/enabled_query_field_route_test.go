package route

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/delivery/http/handlers"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/mocks"
)

func TestSetupEnabledQueryFieldRoutesRequiresCredentials(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	handler := handlers.NewEnabledQueryFieldHandler(mocks.NewMockEnabledQueryFieldUseCase(ctrl), nil)

	t.Run("missing credentials skips registration", func(t *testing.T) {
		router := gin.New()
		SetupEnabledQueryFieldRoutes(router, handler, AdminAuthOptions{}, nil)

		req := httptest.NewRequest(http.MethodGet, "/admin/query-fields", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		require.Equal(t, http.StatusNotFound, rec.Code)
	})

	t.Run("partial credentials skips registration", func(t *testing.T) {
		router := gin.New()
		SetupEnabledQueryFieldRoutes(router, handler, AdminAuthOptions{Username: "admin"}, nil)

		req := httptest.NewRequest(http.MethodGet, "/admin/query-fields", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		require.Equal(t, http.StatusNotFound, rec.Code)
	})

	t.Run("credentials register protected route", func(t *testing.T) {
		useCase := mocks.NewMockEnabledQueryFieldUseCase(ctrl)
		useCase.EXPECT().ListQueryFields(gomock.Any()).Return(nil, nil)
		securedHandler := handlers.NewEnabledQueryFieldHandler(useCase, nil)

		router := gin.New()
		SetupEnabledQueryFieldRoutes(router, securedHandler, AdminAuthOptions{
			Username: "admin",
			Password: "secret",
		}, nil)

		unauth := httptest.NewRequest(http.MethodGet, "/admin/query-fields", nil)
		unauthRec := httptest.NewRecorder()
		router.ServeHTTP(unauthRec, unauth)
		require.Equal(t, http.StatusUnauthorized, unauthRec.Code)

		auth := httptest.NewRequest(http.MethodGet, "/admin/query-fields", nil)
		auth.SetBasicAuth("admin", "secret")
		authRec := httptest.NewRecorder()
		router.ServeHTTP(authRec, auth)
		require.Equal(t, http.StatusOK, authRec.Code)
	})
}
