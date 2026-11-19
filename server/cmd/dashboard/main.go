package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"stock-dashboard/server/internal/app"
	"stock-dashboard/server/internal/config"
)

const (
	readHeaderTimeout = 5 * time.Second
	readTimeout       = 10 * time.Second
	writeTimeout      = 60 * time.Second
	idleTimeout       = 60 * time.Second
	maxHeaderBytes    = 1 << 20
	// The drain budget must cover the slowest allowed response, otherwise an
	// orderly restart fails while a request is still within WriteTimeout.
	shutdownGrace   = 5 * time.Second
	shutdownTimeout = writeTimeout + shutdownGrace
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(logger); err != nil {
		logger.Error("server stopped", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	cfg, err := config.Load(os.Getenv)
	if err != nil {
		return err
	}

	application, err := app.New(cfg, logger)
	if err != nil {
		return err
	}
	defer func() {
		_ = application.Close()
	}()

	address := net.JoinHostPort(cfg.BindHost, strconv.Itoa(cfg.Port))
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", address, err)
	}
	defer listener.Close()

	server := &http.Server{
		Handler:           application.Handler(),
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
		MaxHeaderBytes:    maxHeaderBytes,
		ErrorLog:          slog.NewLogLogger(logger.Handler(), slog.LevelWarn),
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	logger.Info("listening",
		"address", address,
		"data_dir", cfg.DataDir,
		"finnhub_key", cfg.FinnhubAPIKey != "",
		"twelvedata_key", cfg.TwelveDataAPIKey != "",
	)

	serveError := make(chan error, 1)
	go func() {
		serveError <- server.Serve(listener)
	}()

	select {
	case err := <-serveError:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		logger.Info("shutdown requested")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Warn("shutdown drain deadline reached", "error", err)
		_ = server.Close()
	}
	if err := <-serveError; err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	logger.Info("stopped")
	return nil
}
