package main

import (
	"log/slog"
	"os"

	bridgeconfig "ha-ev-charging-bridge/internal/config"
)

func main() {
	if err := bridgeconfig.LoadDotEnv(".env"); err != nil {
		slog.Warn("could not load .env", "error", err)
	}

	cfg, err := bridgeconfig.ParseRuntime()
	if err != nil {
		slog.Error("configuration failed", "error", err)
		os.Exit(1)
	}
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
