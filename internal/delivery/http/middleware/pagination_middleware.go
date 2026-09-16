package middleware

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/lifenetwork-ai/life-cloud-agent-node/constants"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/delivery/http/response"
)

// Default Create a new pagination middleware with default values
func DefaultPagination() gin.HandlerFunc {
	return New(
		constants.DefaultPageText,
		constants.DefaultSizeText,
		strconv.Itoa(constants.DefaultPage),
		strconv.Itoa(constants.DefaultPageSize),
		constants.DefaultMinPageSize,
		constants.DefaultMaxPageSize,
	)
}

// New Create a new pagniation middleware with custom values
func New(pageText, sizeText, defaultPage, defaultPageSize string, minPageSize, maxPageSize int) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Extract the page from the query string and convert it to an integer
		pageStr := c.DefaultQuery(pageText, defaultPage)
		page, err := strconv.Atoi(pageStr)
		if err != nil {
			response.Error(c, http.StatusBadRequest, "INVALID_PAGE", "page number must be an integer", nil)
			return
		}

		// Validate for positive page number
		if page < 0 {
			response.Error(c, http.StatusBadRequest, "INVALID_PAGE", "page number must be positive", nil)
			return
		}

		// Extract the size from the query string and convert it to an integer.
		// Backward-compatible: accept both `size` and `page_size`.
		sizeStr := c.Query(sizeText)
		if sizeStr == "" {
			if alt := c.Query(constants.AltPageSizeText); alt != "" {
				sizeStr = alt
			} else {
				sizeStr = defaultPageSize
			}
		}
		size, err := strconv.Atoi(sizeStr)
		if err != nil {
			response.Error(c, http.StatusBadRequest, "INVALID_PAGE_SIZE", "page size must be an integer", nil)
			return
		}

		// Validate for min and max page size
		if size < minPageSize || size > maxPageSize {
			response.Error(c, http.StatusBadRequest, "INVALID_PAGE_SIZE", "page size must be between "+strconv.Itoa(minPageSize)+" and "+strconv.Itoa(maxPageSize), nil)
			return
		}

		// Set the page and size in the gin context
		c.Set(pageText, page)
		c.Set(sizeText, size)
		if sizeText != constants.AltPageSizeText {
			c.Set(constants.AltPageSizeText, size)
		}

		c.Next()
	}
}
