package route

import (
	"github.com/gin-gonic/gin"

	"github.com/lifenetwork-ai/go-backend-template/internal/delivery/http/handlers"
)

// SetupExampleRoutes sets up routes for example operations.
func SetupExampleRoutes(r *gin.Engine, handler *handlers.ExampleHandler) {
	api := r.Group("/api/v1/examples")
	{
		api.GET("", handler.ListExamples)
		api.POST("", handler.CreateExample)
		api.GET("/:id", handler.GetExample)
		api.PUT("/:id", handler.UpdateExample)
		api.DELETE("/:id", handler.DeleteExample)
	}
}
