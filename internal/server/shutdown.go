package server

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/lifenetwork-ai/go-backend-template/constants"
	"github.com/lifenetwork-ai/go-backend-template/internal/platform/logger"
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

	select {
	case sig := <-sigC:
		log.Info("Received shutdown signal", logger.String("signal", sig.String()))
	case err := <-serverErrC:
		cancel()
		if err != nil {
			return fmt.Errorf("http server stopped unexpectedly: %w", err)
		}
		return nil
	case <-ctx.Done():
		log.Info("Application context canceled; shutting down", logger.Err(ctx.Err()))
	}

	log.Debug("Shutting down gracefully")
	cancel()
	if server == nil {
		return nil
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), GracefulTimeout(gracefulShutdownTimeout))
	defer shutdownCancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown http server: %w", err)
	}
	return nil
}
