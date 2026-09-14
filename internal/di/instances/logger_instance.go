package instances

import (
	"github.com/lifenetwork-ai/go-backend-template/internal/platform/logger"
)

var loggerInstance logger.Logger

// LoggerInstance returns a singleton instance of the logger.
func LoggerInstance() logger.Logger {
	if loggerInstance == nil {
		loggerInstance = logger.GetLogger()
	}
	return loggerInstance
}
