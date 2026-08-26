package main

import (
	"log/slog"
	"net/http"
	"os"
	"time"

	"ihealth-export/internal/app"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	address := env("IHEALTH_ADDR", ":8080")
	database := env("IHEALTH_DB", "/data/ihealth.db")
	token := os.Getenv("IHEALTH_TOKEN")
	if token == "" {
		logger.Error("IHEALTH_TOKEN is required")
		os.Exit(1)
	}
	store, err := app.OpenStore(database)
	if err != nil {
		logger.Error("open database", "error", err)
		os.Exit(1)
	}
	defer store.Close()
	server := &http.Server{Addr: address, Handler: app.NewServer(store, token, logger), ReadHeaderTimeout: 10 * time.Second, ReadTimeout: 30 * time.Second, WriteTimeout: 2 * time.Minute, IdleTimeout: 2 * time.Minute}
	logger.Info("server started", "address", address, "database", database)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Error("server stopped", "error", err)
		os.Exit(1)
	}
}
func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
