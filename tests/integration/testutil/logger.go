package testutil

import "github.com/lifenetwork-ai/life-cloud-agent-node/internal/platform/logger"

// TestLogger is a silent logger used in tests.
type TestLogger struct{}

func (l *TestLogger) SetLogLevel(level logger.Level)               {}
func (l *TestLogger) GetLogLevel() logger.Level                    { return logger.InfoLevel }
func (l *TestLogger) Debug(message string, fields ...logger.Field) {}
func (l *TestLogger) Info(message string, fields ...logger.Field)  {}
func (l *TestLogger) Warn(message string, fields ...logger.Field)  {}
func (l *TestLogger) Error(message string, fields ...logger.Field) {}
func (l *TestLogger) Fatal(message string, fields ...logger.Field) {}
func (l *TestLogger) Panic(message string, fields ...logger.Field) {}
func (l *TestLogger) With(fields ...logger.Field) logger.Logger    { return l }

func NewTestLogger() logger.Logger {
	return &TestLogger{}
}
