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

	"taverna/backend/internal/app"
	"taverna/backend/internal/catalog"
	"taverna/backend/internal/config"
	"taverna/backend/internal/database"
	"taverna/backend/internal/exportjob"
	"taverna/backend/internal/project"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	cfg := config.Load()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := database.ConnectWithRetry(ctx, cfg.DatabaseURL, logger)
	if err != nil {
		logger.Error("database connection failed", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	if err := database.RunMigrations(ctx, pool, cfg.MigrationsDir, logger); err != nil {
		logger.Error("database migration failed", "error", err)
		os.Exit(1)
	}

	catalogService := catalog.NewService()
	catalogHandler := catalog.NewHandler(catalogService)
	projectRepo := project.NewRepository(pool)
	projectHandler := project.NewHandler(projectRepo, catalogService, logger)
	exportRepo := exportjob.NewRepository(pool)
	exportHandler := exportjob.NewHandler(exportRepo, logger)
	router := app.NewRouter(projectHandler, catalogHandler, exportHandler, pool, cfg.CORSAllowedOrigins)

	server := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		logger.Info("http server listening", "port", cfg.Port)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("http server failed", "error", err)
			stop()
		}
	}()

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("http server shutdown failed", "error", err)
	}
}
