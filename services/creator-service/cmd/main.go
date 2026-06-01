package main

import (
	"log/slog"
	"net/http"
	"os"

	"github.com/joho/godotenv"
	"github.com/nouvadev/veritas/pkg/database"
	creatorapi "github.com/nouvadev/veritas/services/creator-service/internal/api"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		AddSource: true,
	}))

	err := godotenv.Load()
	if err != nil {
		logger.Warn("Error loading .env file, continuing without it...")
	}

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		logger.Error("DATABASE_URL environment variable is not set")
		os.Exit(1)
	}

	dbpool, err := database.ConnectDB(databaseURL)
	if err != nil {
		logger.Error("failed to connect to database", "err", err)
		os.Exit(1)
	}
	defer dbpool.Close()

	logger.Info("database connection pool established")

	PORT := os.Getenv("CREATOR_PORT")
	if PORT == "" {
		PORT = "8081"
	}
	logger.Info("starting server", "addr", PORT)

	err = http.ListenAndServe(":"+PORT, creatorapi.Routes(logger, dbpool))
	if err != nil {
		logger.Error("server error", "err", err)
		os.Exit(1)
	}

}
