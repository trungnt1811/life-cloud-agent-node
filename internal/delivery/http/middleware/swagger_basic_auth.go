package middleware

import (
	"crypto/subtle"
	"encoding/base64"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// SwaggerBasicAuth protects Swagger endpoints using HTTP Basic Auth.
// If credentials are missing, it is recommended to not register the swagger route.
func SwaggerBasicAuth(username, password string) gin.HandlerFunc {
	realm := "swagger"

	unauthorized := func(c *gin.Context) {
		c.Header("WWW-Authenticate", `Basic realm="`+realm+`"`)
		c.AbortWithStatus(http.StatusUnauthorized)
	}

	return func(c *gin.Context) {
		auth := c.GetHeader("Authorization")
		if auth == "" || !strings.HasPrefix(auth, "Basic ") {
			unauthorized(c)
			return
		}

		payload, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(auth, "Basic "))
		if err != nil {
			unauthorized(c)
			return
		}

		pair := string(payload)
		before, after, ok := strings.Cut(pair, ":")
		if !ok {
			unauthorized(c)
			return
		}

		u := before
		p := after

		userOK := subtle.ConstantTimeCompare([]byte(u), []byte(username)) == 1
		passOK := subtle.ConstantTimeCompare([]byte(p), []byte(password)) == 1
		if !userOK || !passOK {
			unauthorized(c)
			return
		}

		c.Next()
	}
}
