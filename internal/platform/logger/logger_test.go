package logger

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestZapLoggerSetLogLevelUpdatesAtomicLevel(t *testing.T) {
	log := newZapLogger(InfoLevel)

	log.SetLogLevel(DebugLevel)

	require.Equal(t, DebugLevel, log.GetLogLevel())
}

func TestPackageSetLogLevelUpdatesSingleton(t *testing.T) {
	previous := instance
	t.Cleanup(func() {
		mu.Lock()
		defer mu.Unlock()
		instance = previous
	})

	SetLogger(newZapLogger(InfoLevel))

	SetLogLevel(ErrorLevel)

	require.Equal(t, ErrorLevel, GetLogger().GetLogLevel())
}
