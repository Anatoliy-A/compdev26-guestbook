package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"guestbook/internal/store"
	"guestbook/internal/web"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(log); err != nil {
		log.Error("fatal", "error", err)
		os.Exit(1)
	}
}

func run(log *slog.Logger) error {
	st, err := newStore(log)
	if err != nil {
		return err
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           web.New(st, log),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		log.Info("listening", "addr", srv.Addr)
		errCh <- srv.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
	}

	log.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return srv.Shutdown(shutdownCtx)
}

func newStore(log *slog.Logger) (store.Store, error) {
	if os.Getenv("GUESTBOOK_STORE") == "memory" {
		log.Warn("using in-memory store; entries are lost on restart")
		return store.NewMemory(), nil
	}
	endpoint := os.Getenv("COSMOS_ENDPOINT")
	database := os.Getenv("COSMOS_DATABASE")
	container := os.Getenv("COSMOS_CONTAINER")
	if endpoint == "" || database == "" || container == "" {
		return nil, errors.New("COSMOS_ENDPOINT, COSMOS_DATABASE and COSMOS_CONTAINER are required (or set GUESTBOOK_STORE=memory)")
	}
	return store.NewCosmos(endpoint, database, container)
}
