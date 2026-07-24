package main

import (
	"log/slog"
	"os"
)

func main() {
	if err := loadDotEnv(".env"); err != nil {
		slog.Warn("could not load .env", "error", err)
	}

	cfg := parseConfig()
	logFile, err := setupLogging(cfg.LogFile)
	if err != nil {
		slog.Error("setup logging failed", "error", err)
		os.Exit(1)
	}
	if logFile != nil {
		defer logFile.Close()
	}

	if err := run(cfg); err != nil {
		slog.Error("bridge stopped", "error", err)
		os.Exit(1)
	}
}
