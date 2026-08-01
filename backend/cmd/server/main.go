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
	"taverna/backend/internal/assembly"
	"taverna/backend/internal/catalog"
	"taverna/backend/internal/config"
	"taverna/backend/internal/database"
	"taverna/backend/internal/exportjob"
	"taverna/backend/internal/modelasset"
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

	catalogService, err := catalog.NewService()
	if err != nil {
		logger.Error("catalog seed failed", "error", err)
		os.Exit(1)
	}
	catalogHandler := catalog.NewHandler(catalogService)
	projectRepo := project.NewRepository(pool)
	modelAssetRepo := modelasset.NewRepository(pool)
	modelAssetHandler := modelasset.NewHandler(modelAssetRepo, catalogService, cfg.ModelAssetDir, cfg.MaxModelAssetBytes, logger)
	assemblyService := assembly.NewService(projectRepo, modelAssetRepo, catalogService)
	assemblyHandler := assembly.NewHandler(assemblyService, logger)
	projectHandler := project.NewHandler(projectRepo, catalogService, modelAssetRepo, logger)
	exportRepo := exportjob.NewRepository(pool)
	exportHandler := exportjob.NewHandler(exportRepo, assemblyService, logger)
	router := app.NewRouter(projectHandler, catalogHandler, exportHandler, modelAssetHandler, assemblyHandler, pool, cfg.CORSAllowedOrigins)

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
