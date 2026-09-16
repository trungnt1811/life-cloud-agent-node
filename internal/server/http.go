package server

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/lifenetwork-ai/life-cloud-agent-node/constants"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/platform/logger"
)

type HTTPConfig struct {
	Port              uint32
	ReadHeaderTimeout time.Duration
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
}

func StartHTTP(handler http.Handler, config HTTPConfig, log logger.Logger) (*http.Server, <-chan error) {
	if log == nil {
		log = logger.GetLogger()
	}
	httpServer := &http.Server{
		Addr:              fmt.Sprintf("0.0.0.0:%v", config.Port),
		Handler:           handler,
		ReadHeaderTimeout: durationOrDefault(config.ReadHeaderTimeout, constants.ReadHeaderTimeout),
		ReadTimeout:       durationOrDefault(config.ReadTimeout, constants.ReadTimeout),
		WriteTimeout:      durationOrDefault(config.WriteTimeout, constants.WriteTimeout),
		IdleTimeout:       durationOrDefault(config.IdleTimeout, constants.DefaultTimeout),
	}

	errC := make(chan error, 1)
	go func() {
		log.Info("HTTP server starting", logger.Uint64("port", uint64(config.Port)))
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errC <- err
			return
		}
		errC <- nil
	}()

	return httpServer, errC
}

func durationOrDefault(value, fallback time.Duration) time.Duration {
	if value > 0 {
		return value
	}
	return fallback
}
