package main

import (
	"log/slog"
	"net/http"
	"os"
	"yoopass-api/internal/config"
	"yoopass-api/internal/http-server/handlers/fetch"
	"yoopass-api/internal/http-server/handlers/save"
	redis "yoopass-api/internal/storage"

	"github.com/go-chi/chi"
)

const (
	envLocal = "local"
	envDev   = "dev"
	envProd  = "prod"
)

func main() {
	log := setupLogger()

	cfg := config.MustLoad(log)

	redis, err := redis.New(cfg.StoragePath)
	if err != nil {
		log.Error("Failed to initialize storage", slog.Any("error", err))
		os.Exit(1)
	}

	router := chi.NewRouter()

	router.Get("/{alias}/{key}", fetch.New(log, redis))
	router.Post("/add", save.New(log, redis))

	log.Info("Server started on ", slog.String("address", cfg.HTTPServer.Address))

	srv := &http.Server{
		Addr:         cfg.Address,
		Handler:      router,
		ReadTimeout:  cfg.HTTPServer.Timeout,
		WriteTimeout: cfg.HTTPServer.Timeout,
		IdleTimeout:  cfg.HTTPServer.IdleTimeout,
	}

	if err := srv.ListenAndServe(); err != nil {
		log.Error("failed to start server", slog.Any("error", err))
	}

	log.Error("server stopped")
}

func setupLogger() *slog.Logger {
	return slog.New(
		slog.NewJSONHandler(
			os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}),
	)
}
