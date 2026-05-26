package main

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/peltastic/payment-microservices-reference/auth/internal/cache"
	"github.com/peltastic/payment-microservices-reference/auth/internal/config"
	"github.com/peltastic/payment-microservices-reference/auth/internal/db"
	"github.com/peltastic/payment-microservices-reference/auth/internal/handler"
	"github.com/peltastic/payment-microservices-reference/auth/internal/httpx"
	"github.com/peltastic/payment-microservices-reference/auth/internal/middleware"
	"github.com/peltastic/payment-microservices-reference/auth/internal/repository"
	"github.com/peltastic/payment-microservices-reference/auth/internal/service"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
)

type Server struct {
	Router *gin.Engine
}

func NewServer(cfg *config.Config) *Server {
	log := slog.Default().With("component", "server")
	log.Info("config loaded", "redis_address", cfg.Redis.Address, "redis_db", cfg.Redis.DB)

	db, err := db.InitDB(cfg.Database)

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
	log.Info("redis initialized", "address", cfg.Redis.Address, "db", cfg.Redis.DB)

	keysRepo := repository.NewKeysRepository(db)
	merchantsRepo := repository.NewMerchantsRepository(db)

	keysService := service.NewKeyService(redisClient, keysRepo, db)
	merchantService := service.NewMerchantService(db, keysRepo, merchantsRepo)

	keysHandler := handler.NewKeysHandler(merchantService, keysService)
	internalValidateHandler := handler.NewInternalValidateHandler(keysService)

	router := gin.New()
	router.Use(otelgin.Middleware(cfg.Telemetry.ServiceName), middleware.PrometheusMetrics(), middleware.RequestLogger(), gin.Recovery())
	router.GET("/health", func(c *gin.Context) {
		httpx.JSON(c, http.StatusOK, map[string]any{"status": "ok"})
	})
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))
	v1 := router.Group("/api/v1/auth", middleware.RequireInternalAuth())
	{
		keysGroup := v1.Group("/keys")
		{
			keysGroup.POST("/", keysHandler.CreateKey)
			keysGroup.POST("/:id/revoke", keysHandler.RevokeKey)
		}
		internalGroup := v1.Group("/internal")
		{
			internalGroup.POST("/validate", internalValidateHandler.ValidateKey)
		}
	}
	log.Info("auth service dependencies initialized")
	return &Server{Router: router}
}
