package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/peltastic/payment-microservices-reference/auth/internal/config"
	appLogger "github.com/peltastic/payment-microservices-reference/auth/internal/logger"
	"github.com/peltastic/payment-microservices-reference/auth/internal/telemetry"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		exitOnError("failed to load config", err)
	}

	appLogger.Init(cfg.Logger)
	log := slog.Default().With("component", "server")

	shutdownTracer, err := telemetry.InitTracer(context.Background(), cfg.Telemetry)
	if err != nil {
		exitOnError("failed to init tracer", err)
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := shutdownTracer(shutdownCtx); err != nil {
			log.Error("failed to shutdown tracer", "error", err)
		}
	}()
	log.Info("tracer initialized")

	log.Info("starting auth service", "port", cfg.App.Port)
	server := NewServer(cfg)
	if err := server.Run(":" + cfg.App.Port); err != nil {
		exitOnError("failed to run auth server", err)
	}
}

func (s *Server) Run(addr string) error {
	slog.Default().With("component", "server").Info("auth http server listening", "addr", addr)
	server := &http.Server{
		Addr:              addr,
		Handler:           s.Router,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	return server.ListenAndServe()
}

func exitOnError(message string, err error) {
	if err == nil {
		return
	}

	slog.Default().With("component", "server").Error(message, "error", err)
	os.Exit(1)
}
