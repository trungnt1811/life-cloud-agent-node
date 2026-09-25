package server

import (
	"context"
	"errors"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestWaitForShutdownSignalReturnsComponentFailure(t *testing.T) {
	errC := make(chan error, 1)
	errC <- errors.New("federated client worker: credentials changed")
	err := WaitForShutdownSignal(context.Background(), nil, nil, errC, 0, nil)
	require.ErrorContains(t, err, "application component stopped unexpectedly")
	require.ErrorContains(t, err, "federated client worker: credentials changed")
}

func TestWaitForShutdownSignalShutsDownHTTPAfterComponentFailure(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	server := &http.Server{
		ReadHeaderTimeout: time.Second,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
	}
	serveDone := make(chan error, 1)
	go func() { serveDone <- server.Serve(listener) }()
	errC := make(chan error, 1)
	errC <- errors.New("worker failed")
	err = WaitForShutdownSignal(context.Background(), nil, server, errC, time.Second, nil)
	require.ErrorContains(t, err, "worker failed")
	select {
	case serveErr := <-serveDone:
		require.ErrorIs(t, serveErr, http.ErrServerClosed)
	case <-time.After(time.Second):
		t.Fatal("HTTP server was not shut down after worker failure")
	}
}
