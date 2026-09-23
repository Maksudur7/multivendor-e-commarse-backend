package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/compress"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/helmet"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/yourusername/ecom-backend/config"
	"github.com/yourusername/ecom-backend/internal/auth"
	"github.com/yourusername/ecom-backend/internal/dispute"
	"github.com/yourusername/ecom-backend/internal/notification"
	"github.com/yourusername/ecom-backend/internal/order"
	"github.com/yourusername/ecom-backend/internal/payment"
	"github.com/yourusername/ecom-backend/internal/product"
	"github.com/yourusername/ecom-backend/internal/promotion"
	"github.com/yourusername/ecom-backend/internal/reseller"
	"github.com/yourusername/ecom-backend/internal/shipping"
	"github.com/yourusername/ecom-backend/internal/user"
	"github.com/yourusername/ecom-backend/internal/vendor"
	"github.com/yourusername/ecom-backend/internal/wallet"
	"github.com/yourusername/ecom-backend/pkg/database"
	"github.com/yourusername/ecom-backend/pkg/middleware"
	"github.com/yourusername/ecom-backend/pkg/response"
)

func main() {
	// 1. Configure zerolog output
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339})

	log.Info().Msg("🚀 Starting Enterprise Multi-Vendor E-Commerce Backend Server...")

	// 2. Load Configuration
	cfg, err := config.LoadConfig(".")
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to load application configuration")
	}

	// 3. Connect PostgreSQL Connection Pool (pgx/v5)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pgPool, err := database.NewPostgresPool(ctx, cfg.DB.DSN(), cfg.DB.MaxConns, cfg.DB.MinConns)
	if err != nil {
		log.Warn().Err(err).Msg("Database connection warning (running with mock fallback mode if DB not yet up)")
	} else {
		defer pgPool.Close()
		log.Info().Msg("✅ PostgreSQL database connected successfully")
	}

	// 4. Connect Redis Client
	redisClient, err := database.NewRedisClient(ctx, cfg.Redis.URL(), cfg.Redis.MaxRetries)
	if err != nil {
		log.Warn().Err(err).Msg("Redis connection warning (running in degraded mode without cache)")
	} else {
		defer redisClient.Close()
		log.Info().Msg("✅ Redis cache & queue connected successfully")
	}

	// 5. Initialize Fiber Web App
	app := fiber.New(fiber.Config{
		AppName:      cfg.App.Name + " v1.0.0",
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			code := fiber.StatusInternalServerError
			if e, ok := err.(*fiber.Error); ok {
				code = e.Code
			}
			return response.Error(c, code, err.Error(), nil)
		},
	})

	// 6. Global Middleware Pipeline
	app.Use(middleware.Logger())
	app.Use(middleware.RequestID())
	app.Use(helmet.New())
	app.Use(compress.New(compress.Config{Level: compress.LevelBestSpeed}))
	app.Use(cors.New(cors.Config{
		AllowOrigins:     cfg.CORS.AllowedOrigins,
		AllowHeaders:     "Origin, Content-Type, Accept, Authorization",
		AllowMethods:     "GET, POST, PUT, PATCH, DELETE, OPTIONS",
		AllowCredentials: true,
	}))

	// 7. Health Check Endpoint
	app.Get("/health", func(c *fiber.Ctx) error {
		return response.Success(c, fiber.StatusOK, "Enterprise E-Commerce API Server is Healthy", fiber.Map{
			"app_name":    cfg.App.Name,
			"environment": cfg.App.Env,
			"timestamp":   time.Now().Format(time.RFC3339),
			"version":     "1.0.0",
		})
	})

	// 8. Register API v1 Group
	v1 := app.Group("/api/v1")

	// Middleware instance
	authMw := middleware.JWTAuth(cfg.JWT.AccessSecret, redisClient)

	// Domain Handlers Initialization & Route Registration
	authHandler := auth.NewHandler(cfg, pgPool, redisClient)
	authHandler.RegisterRoutes(v1, authMw)

	userHandler := user.NewHandler(pgPool)
	userHandler.RegisterRoutes(v1, authMw)

	vendorHandler := vendor.NewHandler(pgPool)
	vendorHandler.RegisterRoutes(v1, authMw)

	resellerHandler := reseller.NewHandler(pgPool)
	resellerHandler.RegisterRoutes(v1, authMw)

	productHandler := product.NewHandler(pgPool)
	productHandler.RegisterRoutes(v1, authMw)

	orderHandler := order.NewHandler(pgPool)
	orderHandler.RegisterRoutes(v1, authMw)

	paymentHandler := payment.NewHandler(pgPool)
	paymentHandler.RegisterRoutes(v1, authMw)

	walletHandler := wallet.NewHandler(pgPool)
	walletHandler.RegisterRoutes(v1, authMw)

	shippingHandler := shipping.NewHandler(pgPool)
	shippingHandler.RegisterRoutes(v1, authMw)

	disputeHandler := dispute.NewHandler(pgPool)
	disputeHandler.RegisterRoutes(v1, authMw)

	promotionHandler := promotion.NewHandler(pgPool)
	promotionHandler.RegisterRoutes(v1, authMw)

	notificationHandler := notification.NewHandler(pgPool)
	notificationHandler.RegisterRoutes(v1, authMw)

	// 9. Graceful Shutdown Signal Channel
	shutdownChan := make(chan os.Signal, 1)
	signal.Notify(shutdownChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		port := fmt.Sprintf(":%d", cfg.App.Port)
		log.Info().Msgf("⚡ Server running on http://localhost%s", port)
		if err := app.Listen(port); err != nil {
			log.Error().Err(err).Msg("Server listen error")
		}
	}()

	<-shutdownChan
	log.Info().Msg("⏳ Shutting down server gracefully...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := app.ShutdownWithContext(shutdownCtx); err != nil {
		log.Error().Err(err).Msg("Error during graceful shutdown")
	} else {
		log.Info().Msg("🛑 Server stopped cleanly")
	}
}
