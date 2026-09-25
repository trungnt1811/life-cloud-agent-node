package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/lifenetwork-ai/life-cloud-agent-node/constants"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/platform/logger"
)

func GracefulTimeout(timeout time.Duration) time.Duration {
	if timeout <= 0 {
		return constants.DefaultGracefulShutdown
	}
	return timeout
}

func WaitForShutdownSignal(
	ctx context.Context,
	cancel context.CancelFunc,
	server *http.Server,
	serverErrC <-chan error,
	gracefulShutdownTimeout time.Duration,
	log logger.Logger,
) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if cancel == nil {
		cancel = func() {}
	}
	if log == nil {
		log = logger.GetLogger()
	}

	sigC := make(chan os.Signal, 1)
	signal.Notify(sigC, syscall.SIGTERM, syscall.SIGINT)
	defer signal.Stop(sigC)

	var componentErr error
	select {
	case sig := <-sigC:
		log.Info("Received shutdown signal", logger.String("signal", sig.String()))
	case err := <-serverErrC:
		if err != nil {
			componentErr = fmt.Errorf("application component stopped unexpectedly: %w", err)
		}
	case <-ctx.Done():
		log.Info("Application context canceled; shutting down", logger.Err(ctx.Err()))
	}

	log.Debug("Shutting down gracefully")
	cancel()
	if server == nil {
		return componentErr
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), GracefulTimeout(gracefulShutdownTimeout))
	defer shutdownCancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		return errors.Join(componentErr, fmt.Errorf("shutdown http server: %w", err))
	}
	return componentErr
}
