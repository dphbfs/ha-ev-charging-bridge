package main

import (
	"flag"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"
)

const defaultEventType = "state_changed"

const (
	defaultStartThresholdW  = 200
	defaultEndThresholdW    = 50
	defaultEndDebounce      = 10 * time.Second
	defaultDeviceConfigPath = "devices.yaml"
	defaultIngressStorePath = "var/ingress-events.jsonl"
	defaultActiveStorePath  = "var/current-session.json"
	defaultSessionStorePath = "var/sessions.jsonl"
	defaultLogFilePath      = "log/app.log"
)

type config struct {
	HAURL           string
	Token           string
	EventType       string
	DeviceConfig    string
	StartThresholdW float64
	EndThresholdW   float64
	EndDebounce     time.Duration
	IngressStore    string
	ActiveStore     string
	SessionStore    string
	LogFile         string
}

func parseConfig() config {
	cfg := config{}
	flag.StringVar(&cfg.HAURL, "ha-url", os.Getenv("HA_URL"), "Home Assistant base URL, e.g. http://homeassistant.local:8123")
	flag.StringVar(&cfg.Token, "token", os.Getenv("HA_TOKEN"), "Home Assistant long-lived access token")
	flag.StringVar(&cfg.EventType, "event-type", envOrDefault("HA_EVENT_TYPE", defaultEventType), "Home Assistant event type to subscribe to; empty subscribes to all events")
	flag.StringVar(&cfg.DeviceConfig, "device-config", envOrDefault("DEVICE_CONFIG", defaultDeviceConfigPath), "Path to YAML device configuration")
	flag.Float64Var(&cfg.StartThresholdW, "start-threshold-w", envFloatOrDefault("SESSION_START_THRESHOLD_W", defaultStartThresholdW), "Power threshold in watts that starts a session")
	flag.Float64Var(&cfg.EndThresholdW, "end-threshold-w", envFloatOrDefault("SESSION_END_THRESHOLD_W", defaultEndThresholdW), "Power threshold in watts that can end a session")
	flag.DurationVar(&cfg.EndDebounce, "end-debounce", envDurationOrDefault("SESSION_END_DEBOUNCE", defaultEndDebounce), "How long power must remain below the end threshold before ending a session")
	flag.StringVar(&cfg.IngressStore, "ingress-store", envOrDefault("INGRESS_STORE", defaultIngressStorePath), "Path to append raw received Home Assistant events as JSON lines")
	flag.StringVar(&cfg.ActiveStore, "active-store", envOrDefault("ACTIVE_SESSION_STORE", defaultActiveStorePath), "Path to write the current in-progress session as JSON")
	flag.StringVar(&cfg.SessionStore, "session-store", envOrDefault("SESSION_STORE", defaultSessionStorePath), "Path to append completed sessions as JSON lines")
	flag.StringVar(&cfg.LogFile, "log-file", envOrDefault("LOG_FILE", defaultLogFilePath), "Path to append application logs")
	flag.Parse()
	return cfg
}

func envOrDefault(name, fallback string) string {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	return value
}

func envFloatOrDefault(name string, fallback float64) float64 {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		slog.Warn("invalid float environment variable; using default", "name", name, "value", value, "default", fallback)
		return fallback
	}
	return parsed
}

func envDurationOrDefault(name string, fallback time.Duration) time.Duration {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		slog.Warn("invalid duration environment variable; using default", "name", name, "value", value, "default", fallback)
		return fallback
	}
	return parsed
}
