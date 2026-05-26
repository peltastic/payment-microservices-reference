package main

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/peltastic/payment-microservices-reference/ledger/internal/config"
	"github.com/peltastic/payment-microservices-reference/ledger/internal/consumer"
	"github.com/peltastic/payment-microservices-reference/ledger/internal/db"
	"github.com/peltastic/payment-microservices-reference/ledger/internal/handler"
	"github.com/peltastic/payment-microservices-reference/ledger/internal/httpx"
	"github.com/peltastic/payment-microservices-reference/ledger/internal/middleware"
	"github.com/peltastic/payment-microservices-reference/ledger/internal/repository"
	"github.com/peltastic/payment-microservices-reference/ledger/internal/service"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
)

type Server struct {
	Router        *gin.Engine
	kafkaConsumer *consumer.KafkaConsumer
}

func NewServer(cfg *config.Config) *Server {
	log := slog.Default().With("component", "server")
	database, err := db.InitDB(cfg.Database)
	if err != nil {
		exitOnError("failed to initialize database", err)
	}
	log.Info("database initialized")

	processedEventRepo := repository.NewProcessedEventRepository(database)
	journalEntryRepo := repository.NewJournalEntryRepository(database)
	merchantBalanceRepo := repository.NewMerchantBalanceRepository(database)

	ledgerService := service.NewLedgerService(processedEventRepo, journalEntryRepo, merchantBalanceRepo, database)

	ledgerHandler := handler.NewLedgerHandler(ledgerService)

	kafkaConsumer := consumer.NewKafkaConsumer(ledgerService, cfg.Kafka)

	router := gin.New()
	router.Use(otelgin.Middleware(cfg.Telemetry.ServiceName), middleware.PrometheusMetrics(), middleware.RequestLogger(), gin.Recovery())
	router.GET("/health", func(c *gin.Context) {
		httpx.JSON(c, http.StatusOK, map[string]any{"status": "ok"})
	})
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	balanceGroup := router.Group("/balance", middleware.RequireInternalAuth())
	{
		balanceGroup.GET("/", ledgerHandler.GetBalance)
		balanceGroup.GET("/verify", ledgerHandler.VerifyBalance)
	}

	transactionsGroup := router.Group("/transactions", middleware.RequireInternalAuth())
	{
		transactionsGroup.POST("/payment-succeeded", ledgerHandler.HandlePaymentSucceeded)
	}

	log.Info("ledger service dependencies initialized")

	return &Server{Router: router, kafkaConsumer: kafkaConsumer}
}

func (s *Server) Run(addr string) error {
	log := slog.Default().With("component", "server")
	if s.kafkaConsumer != nil {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		defer s.kafkaConsumer.Close()

		s.kafkaConsumer.Start(ctx)
		log.Info("kafka consumer started")
	}

	log.Info("ledger http server listening", "addr", addr)
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
