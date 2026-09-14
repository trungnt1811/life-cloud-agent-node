package logger

import (
	"fmt"
	"os"
	"sort"
	"sync"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Level defines the severity level for logging.
type Level string

const (
	DebugLevel Level = "debug"
	InfoLevel  Level = "info"
	WarnLevel  Level = "warn"
	ErrorLevel Level = "error"
	FatalLevel Level = "fatal"
	PanicLevel Level = "panic"
)

// Field is a structured logging field.
type Field = zap.Field

func String(key, value string) Field                 { return zap.String(key, value) }
func Stringer(key string, value fmt.Stringer) Field  { return zap.Stringer(key, value) }
func Strings(key string, values []string) Field      { return zap.Strings(key, values) }
func Bool(key string, value bool) Field              { return zap.Bool(key, value) }
func Int(key string, value int) Field                { return zap.Int(key, value) }
func Int64(key string, value int64) Field            { return zap.Int64(key, value) }
func Uint64(key string, value uint64) Field          { return zap.Uint64(key, value) }
func Float64(key string, value float64) Field        { return zap.Float64(key, value) }
func Duration(key string, value time.Duration) Field { return zap.Duration(key, value) }
func Any(key string, value any) Field                { return zap.Any(key, value) }
func Err(err error) Field                            { return zap.Error(err) }
func NamedErr(key string, err error) Field           { return zap.NamedError(key, err) }

// FieldsFromMap converts legacy map-based fields into deterministic structured fields.
func FieldsFromMap(fields map[string]any) []Field {
	if len(fields) == 0 {
		return nil
	}
	keys := make([]string, 0, len(fields))
	for key := range fields {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	result := make([]Field, 0, len(fields))
	for _, key := range keys {
		result = append(result, Any(key, fields[key]))
	}
	return result
}

// Logger interface defines common logging operations.
//
//go:generate mockgen -source=logger.go -destination=../../mocks/mock_logger.go -package=mocks
type Logger interface {
	// Level management
	SetLogLevel(level Level)
	GetLogLevel() Level

	// Structured logging methods.
	Debug(message string, fields ...Field)
	Info(message string, fields ...Field)
	Warn(message string, fields ...Field)
	Error(message string, fields ...Field)
	Fatal(message string, fields ...Field)
	Panic(message string, fields ...Field)
	With(fields ...Field) Logger
}

type zapLogger struct {
	logger       *zap.Logger
	currentLevel zap.AtomicLevel
}

var (
	instance Logger
	mu       sync.RWMutex
)

// mapLogLevel maps interfaces.Level to zapcore.Level.
func mapLogLevel(level Level) zapcore.Level {
	switch level {
	case DebugLevel:
		return zapcore.DebugLevel
	case InfoLevel:
		return zapcore.InfoLevel
	case WarnLevel:
		return zapcore.WarnLevel
	case ErrorLevel:
		return zapcore.ErrorLevel
	case FatalLevel:
		return zapcore.FatalLevel
	case PanicLevel:
		return zapcore.PanicLevel
	default:
		return zapcore.InfoLevel
	}
}

// customLevelEncoder replaces "level" with "severity".
func customLevelEncoder(level zapcore.Level, enc zapcore.PrimitiveArrayEncoder) {
	severityMapping := map[zapcore.Level]string{
		zapcore.DebugLevel: "DEBUG",
		zapcore.InfoLevel:  "INFO",
		zapcore.WarnLevel:  "WARNING",
		zapcore.ErrorLevel: "ERROR",
		zapcore.PanicLevel: "CRITICAL",
		zapcore.FatalLevel: "ALERT",
	}
	enc.AppendString(severityMapping[level])
}

// newZapLogger initializes a new zap-based logger instance.
func newZapLogger(level Level) *zapLogger {
	atomicLevel := zap.NewAtomicLevelAt(mapLogLevel(level))

	encoderConfig := zapcore.EncoderConfig{
		TimeKey:      "timestamp",
		LevelKey:     "severity",
		CallerKey:    "caller",
		MessageKey:   "message",
		EncodeLevel:  customLevelEncoder,
		EncodeTime:   zapcore.TimeEncoderOfLayout("2006-01-02T15:04:05Z07:00"),
		EncodeCaller: zapcore.ShortCallerEncoder,
	}

	core := zapcore.NewCore(
		zapcore.NewJSONEncoder(encoderConfig),
		zapcore.Lock(zapcore.AddSync(os.Stdout)),
		atomicLevel,
	)

	base := zap.New(core, zap.AddCaller(), zap.AddCallerSkip(1))

	return &zapLogger{
		logger:       base,
		currentLevel: atomicLevel,
	}
}

// GetLogger returns the singleton logger instance.
func GetLogger() Logger {
	mu.RLock()
	current := instance
	mu.RUnlock()
	if current != nil {
		return current
	}

	mu.Lock()
	defer mu.Unlock()
	if instance == nil {
		instance = newZapLogger(InfoLevel)
	}
	return instance
}

// SetLogger replaces the singleton logger instance.
func SetLogger(customLogger Logger) {
	if customLogger == nil {
		return
	}
	mu.Lock()
	defer mu.Unlock()
	instance = customLogger
}

// SetLogLevel sets the log level dynamically.
func SetLogLevel(level Level) {
	GetLogger().SetLogLevel(level)
}

// Logger Interface Implementation
func (z *zapLogger) SetLogLevel(level Level) {
	if z == nil {
		return
	}
	z.currentLevel.SetLevel(mapLogLevel(level))
}

func (z *zapLogger) GetLogLevel() Level {
	return Level(z.currentLevel.String())
}

// Individual Log Methods
func (z *zapLogger) Debug(message string, fields ...Field) { z.logger.Debug(message, fields...) }

func (z *zapLogger) Info(message string, fields ...Field) { z.logger.Info(message, fields...) }

func (z *zapLogger) Warn(message string, fields ...Field) { z.logger.Warn(message, fields...) }

func (z *zapLogger) Error(message string, fields ...Field) { z.logger.Error(message, fields...) }

func (z *zapLogger) Fatal(message string, fields ...Field) { z.logger.Fatal(message, fields...) }

func (z *zapLogger) Panic(message string, fields ...Field) { z.logger.Panic(message, fields...) }

func (z *zapLogger) With(fields ...Field) Logger {
	child := z.logger.With(fields...)
	return &zapLogger{
		logger:       child,
		currentLevel: z.currentLevel,
	}
}
