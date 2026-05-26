package main

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/peltastic/payment-microservices-reference/ledger/internal/config"
	appLogger "github.com/peltastic/payment-microservices-reference/ledger/internal/logger"
	"github.com/peltastic/payment-microservices-reference/ledger/internal/telemetry"
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

	log.Info("starting ledger service", "port", cfg.App.Port)
	server := NewServer(cfg)
	if err := server.Run(":" + cfg.App.Port); err != nil {
		exitOnError("failed to run ledger server", err)
	}
}

func exitOnError(message string, err error) {
	if err == nil {
		return
	}

	slog.Default().With("component", "server").Error(message, "error", err)
	os.Exit(1)
}
