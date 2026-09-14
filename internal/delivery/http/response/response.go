package response

import "github.com/gin-gonic/gin"

func Error(c *gin.Context, status int, code, msg string, errors interface{}) {
	if msg == "" {
		msg = "Failed"
	}
	res := Response{
		Status:  status,
		Code:    code,
		Message: msg,
	}
	if errors != nil {
		res.Errors = errors
	}
	c.Abort()
	c.JSON(status, res)
}
