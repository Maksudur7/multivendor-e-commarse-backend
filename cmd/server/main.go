package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/yourusername/ecom-backend/config"
	"github.com/yourusername/ecom-backend/pkg/serverapp"
)

func main() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339})
	log.Info().Msg("🚀 Starting Enterprise Multi-Vendor E-Commerce Backend Server...")

	cfg, err := config.LoadConfig(".")
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to load application configuration")
	}

	app, err := serverapp.BuildApp(cfg)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to build application instance")
	}

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
