package main

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/peltastic/payment-microservices-reference/payment/internal/bank"
	"github.com/peltastic/payment-microservices-reference/payment/internal/cache"
	"github.com/peltastic/payment-microservices-reference/payment/internal/config"
	"github.com/peltastic/payment-microservices-reference/payment/internal/db"
	"github.com/peltastic/payment-microservices-reference/payment/internal/handler"
	"github.com/peltastic/payment-microservices-reference/payment/internal/httpx"
	"github.com/peltastic/payment-microservices-reference/payment/internal/kafka"
	"github.com/peltastic/payment-microservices-reference/payment/internal/middleware"
	"github.com/peltastic/payment-microservices-reference/payment/internal/repository"
	"github.com/peltastic/payment-microservices-reference/payment/internal/service"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
)

type Server struct {
	Router          *gin.Engine
	recoveryService *service.RecoveryService
	kafkaProducer   *kafka.Producer
}

func NewServer(cfg *config.Config) *Server {
	log := slog.Default().With("component", "server")

	database, err := db.InitDB(cfg.Database)
	if err != nil {
		exitOnError("failed to initialize database", err)
	}
	log.Info("database initialized")

	redisClient, err := cache.InitRedis(cache.Config{
		Address:  cfg.Redis.Address,
		Password: cfg.Redis.Password,
		Db:       cfg.Redis.DB,
	})
	if err != nil {
		exitOnError("failed to initialize redis", err)
	}
	log.Info("redis initialized",
		"address", cfg.Redis.Address,
		"db", cfg.Redis.DB,
	)

	paymentRepo := repository.NewPaymentsRepository(database)
	idemKeyRepo := repository.NewIdemKeyRepository(database)

	kafkaProducer := kafka.NewProducer(cfg.Kafka)
	bankClient := bank.NewClient(cfg.Bank)

	paymentService := service.NewPaymentService(paymentRepo, redisClient, idemKeyRepo, kafkaProducer, bankClient)
	recoveryService := service.NewRecoveryService(paymentRepo, paymentService)

	paymentHandler := handler.NewPaymentHandler(paymentService)

	router := gin.New()
	router.Use(otelgin.Middleware(cfg.Telemetry.ServiceName), middleware.PrometheusMetrics(), middleware.RequestLogger(), gin.Recovery())
	router.GET("/health", func(c *gin.Context) {
		httpx.JSON(c, http.StatusOK, map[string]any{"status": "ok"})
	})
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	paymentsGroup := router.Group("/payments", middleware.RequireInternalAuth())
	{
		paymentsGroup.POST("/", paymentHandler.CreatePayment)
		paymentsGroup.GET("/", paymentHandler.ListPayments)
		paymentsGroup.GET("/:id", paymentHandler.GetPayment)
	}

	return &Server{
		Router:          router,
		recoveryService: recoveryService,
		kafkaProducer:   kafkaProducer,
	}
}

func (s *Server) Run(addr string) error {
	log := slog.Default().With("component", "server")
	if s.kafkaProducer != nil {
		defer s.kafkaProducer.Close()
	}

	if s.recoveryService != nil {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go s.recoveryService.StartRecoveryJob(ctx)
	}

	log.Info("payment http server listening", "addr", addr)
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
