package main

import (
	"context"
	"errors"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"hearthlane-relay/internal/server"
	"hearthlane-relay/internal/store"
)

func main() {
	bind := envOr("RELAY_BIND", "0.0.0.0")
	port := envOr("RELAY_PORT", "8080")
	dataFile := envOr("RELAY_DATA_FILE", "state.json")
	token := os.Getenv("RELAY_TOKEN")

	logger := log.New(os.Stdout, "relay: ", log.LstdFlags|log.LUTC)

	sts := store.New(dataFile)
	st, err := sts.Load()
	if err != nil {
		logger.Fatalf("cannot start: %v", err)
	}

	if token == "" {
		logger.Printf("warning: authentication disabled, no RELAY_TOKEN configured")
	} else {
		logger.Printf("authentication enabled")
	}
	logger.Printf("loaded %d device(s) from %s", st.DeviceCount(), dataFile)

	srv := server.New(st, sts, token, logger)
	httpSrv := &http.Server{
		Addr:              net.JoinHostPort(bind, port),
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		logger.Printf("listening on %s", httpSrv.Addr)
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		logger.Fatalf("server error: %v", err)
	case <-ctx.Done():
		logger.Printf("shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := httpSrv.Shutdown(shutdownCtx); err != nil {
			logger.Printf("shutdown error: %v", err)
		}
		logger.Printf("shutdown complete")
	}
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
