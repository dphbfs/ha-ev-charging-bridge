package config

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const DefaultEventType = "state_changed"

const (
	defaultStartThresholdW = 200
	defaultEndThresholdW   = 50
	defaultEndDebounce     = 10 * time.Second
	defaultConfigFilePath  = "config.yaml"
	defaultRuntimeDirName  = ".ha-ev-charging-bridge"
	defaultAPIAddr         = "127.0.0.1:8080"
)

type Runtime struct {
	ConfigFile      string
	HAURL           string
	Token           string
	EventType       string
	Chargers        []Charger
	StartThresholdW float64
	EndThresholdW   float64
	EndDebounce     time.Duration
	DatabasePath    string
	LogFile         string
	APIAddr         string
}

type V1 struct {
	HomeAssistant HomeAssistant `yaml:"home_assistant"`
	HAEntities    HAEntities    `yaml:"ha_entities"`
	Chargers      []Charger     `yaml:"chargers"`
	Retention     Retention     `yaml:"retention"`
	Runtime       Paths         `yaml:"runtime"`
}

type HAEntities map[string]map[string]string

type HomeAssistant struct {
	URL        string   `yaml:"url"`
	Token      string   `yaml:"token"`
	EventTypes []string `yaml:"event_types"`
}

type Charger struct {
	ChargerID    string         `yaml:"charger_id"`
	ChargerName  string         `yaml:"charger_name"`
	EVSEID       string         `yaml:"evse_id"`
	ConnectorID  string         `yaml:"connector_id"`
	MeterID      string         `yaml:"meter_id"`
	Entities     EntityMapping  `yaml:"entities"`
	Availability Availability   `yaml:"availability"`
	Start        PowerThreshold `yaml:"start"`
	Stop         PowerThreshold `yaml:"stop"`
	Meters       []Meter        `yaml:"meters"`
}

type EntityMapping struct {
	PowerW       string `yaml:"power_w"`
	EnergyKWh    string `yaml:"energy_kwh"`
	Availability string `yaml:"availability"`
	Fault        string `yaml:"fault"`
	Plug         string `yaml:"plug"`
}

type Availability struct {
	EntityID         string `yaml:"entity_id"`
	AvailableState   string `yaml:"available_state"`
	UnavailableState string `yaml:"unavailable_state"`
	UnavailableAfter string `yaml:"unavailable_after"`
}

type PowerThreshold struct {
	Type       string           `yaml:"type"`
	EntityID   string           `yaml:"entity_id"`
	State      string           `yaml:"state"`
	Reason     string           `yaml:"reason"`
	ThresholdW float64          `yaml:"threshold_w"`
	Duration   string           `yaml:"duration"`
	Events     []Event          `yaml:"events"`
	Rules      []PowerThreshold `yaml:"-"`
}

type powerThresholdYAML PowerThreshold

func (r *PowerThreshold) UnmarshalYAML(value *yaml.Node) error {
	switch value.Kind {
	case yaml.SequenceNode:
		var rules []powerThresholdYAML
		if err := value.Decode(&rules); err != nil {
			return err
		}
		converted := make([]PowerThreshold, len(rules))
		for i, rule := range rules {
			converted[i] = PowerThreshold(rule)
		}
		*r = firstPowerThresholdOrZero(converted)
		r.Rules = converted
		return nil
	case yaml.MappingNode:
		var rule powerThresholdYAML
		if err := value.Decode(&rule); err != nil {
			return err
		}
		*r = PowerThreshold(rule)
		r.Rules = []PowerThreshold{*r}
		return nil
	default:
		return fmt.Errorf("expected mapping or sequence")
	}
}

type Event struct {
	EntityID string `yaml:"entity_id"`
	State    string `yaml:"state"`
	Reason   string `yaml:"reason"`
}

type Meter struct {
	MeterID               string `yaml:"meter_id"`
	EntityID              string `yaml:"entity_id"`
	Unit                  string `yaml:"unit"`
	Aggregation           string `yaml:"aggregation"`
	OutsideSessionStorage string `yaml:"outside_session_storage"`
}

type Retention struct {
	MeterValues     string `yaml:"meter_values"`
	LifecycleEvents string `yaml:"lifecycle_events"`
	RawEvents       string `yaml:"raw_events"`
}

type Paths struct {
	DatabasePath string `yaml:"database_path"`
	LogFile      string `yaml:"log_file"`
	APIAddr      string `yaml:"api_addr"`
}

func ParseRuntime() (Runtime, error) {
	cfg := Runtime{}
	flag.StringVar(&cfg.ConfigFile, "config", envOrDefault("CONFIG_FILE", defaultConfigFilePath), "Path to YAML application configuration")
	flag.StringVar(&cfg.HAURL, "ha-url", os.Getenv("HA_URL"), "Home Assistant base URL, e.g. http://home-assistant.example.local:8123")
	flag.StringVar(&cfg.Token, "token", os.Getenv("HA_TOKEN"), "Home Assistant long-lived access token")
	flag.StringVar(&cfg.EventType, "event-type", envOrDefault("HA_EVENT_TYPE", DefaultEventType), "Home Assistant event type to subscribe to; empty subscribes to all events")
	flag.Float64Var(&cfg.StartThresholdW, "start-threshold-w", envFloatOrDefault("SESSION_START_THRESHOLD_W", defaultStartThresholdW), "Power threshold in watts that starts a session")
	flag.Float64Var(&cfg.EndThresholdW, "end-threshold-w", envFloatOrDefault("SESSION_END_THRESHOLD_W", defaultEndThresholdW), "Power threshold in watts that can end a session")
	flag.DurationVar(&cfg.EndDebounce, "end-debounce", envDurationOrDefault("SESSION_END_DEBOUNCE", defaultEndDebounce), "How long power must remain below the end threshold before ending a session")
	flag.StringVar(&cfg.DatabasePath, "database", envOrDefault("DATABASE_PATH", defaultRuntimePath("bridge.db")), "Path to SQLite runtime database")
	flag.StringVar(&cfg.LogFile, "log-file", envOrDefault("LOG_FILE", defaultRuntimePath("app.log")), "Path to append application logs")
	flag.StringVar(&cfg.APIAddr, "api-addr", envOrDefault("API_ADDR", defaultAPIAddr), "HTTP API listen address; set empty to disable")
	flag.Parse()

	loaded, err := loadRuntimeConfig(cfg.ConfigFile)
	if err != nil {
		return Runtime{}, err
	}
	if loaded != nil {
		cfg = mergeRuntimeConfig(cfg, *loaded)
	}
	return cfg, nil
}

func LoadV1File(path string) (V1, error) {
	payload, err := os.ReadFile(path)
	if err != nil {
		return V1{}, fmt.Errorf("read config %q: %w", path, err)
	}
	return ParseV1(payload, os.LookupEnv)
}

func ParseV1(payload []byte, lookupEnv func(string) (string, bool)) (V1, error) {
	resolved, err := interpolateEnv(payload, lookupEnv)
	if err != nil {
		return V1{}, err
	}

	var cfg V1
	decoder := yaml.NewDecoder(bytes.NewReader(resolved))
	decoder.KnownFields(true)
	if err := decoder.Decode(&cfg); err != nil {
		return V1{}, fmt.Errorf("parse config: %w", err)
	}
	if err := cfg.validate(); err != nil {
		return V1{}, err
	}
	return cfg, nil
}

func loadRuntimeConfig(path string) (*Runtime, error) {
	if strings.TrimSpace(path) == "" {
		return nil, nil
	}

	v1, err := LoadV1File(path)
	if errors.Is(err, os.ErrNotExist) && path == defaultConfigFilePath {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return v1.toRuntime(), nil
}

func (c V1) validate() error {
	if strings.TrimSpace(c.HomeAssistant.URL) == "" {
		return errors.New("home_assistant.url is required")
	}
	if strings.TrimSpace(c.HomeAssistant.Token) == "" {
		return errors.New("home_assistant.token is required")
	}
	if len(c.Chargers) == 0 {
		return errors.New("at least one charger is required")
	}

	chargerIDs := make(map[string]struct{}, len(c.Chargers))
	entityIDs := map[string]string{}
	for i, charger := range c.Chargers {
		name := fmt.Sprintf("chargers[%d]", i)
		if err := validateRequired(name+".charger_id", charger.ChargerID); err != nil {
			return err
		}
		if err := validateRequired(name+".evse_id", charger.EVSEID); err != nil {
			return err
		}
		if err := validateRequired(name+".connector_id", charger.ConnectorID); err != nil {
			return err
		}
		if err := validateRequired(name+".meter_id", charger.MeterID); err != nil {
			return err
		}
		if _, exists := chargerIDs[charger.ChargerID]; exists {
			return fmt.Errorf("duplicate charger_id %q", charger.ChargerID)
		}
		chargerIDs[charger.ChargerID] = struct{}{}

		if err := validatePowerThreshold(name+".start", charger.Start); err != nil {
			return err
		}
		if err := validateStopRules(name+".stop", charger.Stop); err != nil {
			return err
		}
		if err := validateRequired(name+".entities.power_w", charger.Entities.PowerW); err != nil {
			return err
		}
		if err := validateRequired(name+".entities.energy_kwh", charger.Entities.EnergyKWh); err != nil {
			return err
		}
		if strings.TrimSpace(charger.Availability.UnavailableAfter) != "" {
			if _, err := parseConfigDuration(charger.Availability.UnavailableAfter); err != nil {
				return fmt.Errorf("%s.availability.unavailable_after is invalid: %w", name, err)
			}
		}
		for j, meter := range charger.Meters {
			meterName := fmt.Sprintf("%s.meters[%d]", name, j)
			if err := validateRequired(meterName+".meter_id", meter.MeterID); err != nil {
				return err
			}
			if err := validateRequired(meterName+".entity_id", meter.EntityID); err != nil {
				return err
			}
			switch meter.Aggregation {
			case "average", "last":
			default:
				return fmt.Errorf("%s.aggregation must be average or last", meterName)
			}
			switch meter.OutsideSessionStorage {
			case "save", "drop":
			default:
				return fmt.Errorf("%s.outside_session_storage must be save or drop", meterName)
			}
		}
		for label, entityID := range map[string]string{
			"power_w":      charger.Entities.PowerW,
			"energy_kwh":   charger.Entities.EnergyKWh,
			"availability": charger.Entities.Availability,
		} {
			if strings.TrimSpace(entityID) == "" {
				continue
			}
			if owner, exists := entityIDs[entityID]; exists {
				return fmt.Errorf("duplicate entity_id %q used by %s and %s.%s", entityID, owner, name, label)
			}
			entityIDs[entityID] = name + "." + label
		}
	}
	for label, duration := range map[string]string{
		"retention.meter_values":     c.Retention.MeterValues,
		"retention.lifecycle_events": c.Retention.LifecycleEvents,
		"retention.raw_events":       c.Retention.RawEvents,
	} {
		if strings.TrimSpace(duration) == "" {
			continue
		}
		if _, err := parseConfigDuration(duration); err != nil {
			return fmt.Errorf("%s is invalid: %w", label, err)
		}
	}

	return nil
}

func validateRequired(name, value string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("%s is required", name)
	}
	return nil
}

func validatePowerThreshold(name string, rule PowerThreshold) error {
	if strings.TrimSpace(rule.Type) != "power_threshold" {
		return fmt.Errorf("%s.type must be power_threshold", name)
	}
	if err := validateRequired(name+".entity_id", rule.EntityID); err != nil {
		return err
	}
	if rule.ThresholdW < 0 {
		return fmt.Errorf("%s.threshold_w must be non-negative", name)
	}
	if strings.TrimSpace(rule.Duration) != "" {
		if _, err := parseConfigDuration(rule.Duration); err != nil {
			return fmt.Errorf("%s.duration is invalid: %w", name, err)
		}
	}
	return nil
}

func validateStopRules(name string, rule PowerThreshold) error {
	rules := rule.StopRules()
	if len(rules) == 0 {
		return fmt.Errorf("%s must contain at least one rule", name)
	}
	for i, stopRule := range rules {
		ruleName := name
		if len(rules) > 1 {
			ruleName = fmt.Sprintf("%s[%d]", name, i)
		}
		switch strings.TrimSpace(stopRule.Type) {
		case "power_threshold":
			if err := validatePowerThreshold(ruleName, stopRule); err != nil {
				return err
			}
		case "":
			return fmt.Errorf("%s.type is required", ruleName)
		default:
			if err := validateRequired(ruleName+".entity_id", stopRule.EntityID); err != nil {
				return err
			}
			if err := validateRequired(ruleName+".state", stopRule.State); err != nil {
				return err
			}
			if strings.TrimSpace(stopRule.Duration) != "" {
				if _, err := parseConfigDuration(stopRule.Duration); err != nil {
					return fmt.Errorf("%s.duration is invalid: %w", ruleName, err)
				}
			}
		}
		for j, event := range stopRule.Events {
			eventName := fmt.Sprintf("%s.events[%d]", ruleName, j)
			if err := validateRequired(eventName+".entity_id", event.EntityID); err != nil {
				return err
			}
			if err := validateRequired(eventName+".state", event.State); err != nil {
				return err
			}
		}
	}
	return nil
}

func (r PowerThreshold) StopRules() []PowerThreshold {
	if len(r.Rules) > 0 {
		return r.Rules
	}
	if strings.TrimSpace(r.Type) == "" {
		return nil
	}
	return []PowerThreshold{r}
}

func firstPowerThresholdOrZero(rules []PowerThreshold) PowerThreshold {
	for _, rule := range rules {
		if strings.TrimSpace(rule.Type) == "power_threshold" {
			return rule
		}
	}
	if len(rules) > 0 {
		return rules[0]
	}
	return PowerThreshold{}
}

func parseConfigDuration(value string) (time.Duration, error) {
	trimmed := strings.TrimSpace(value)
	if strings.HasSuffix(trimmed, "d") {
		days, err := strconv.ParseFloat(strings.TrimSuffix(trimmed, "d"), 64)
		if err != nil {
			return 0, err
		}
		return time.Duration(days * float64(24*time.Hour)), nil
	}
	return time.ParseDuration(trimmed)
}

func (c V1) toRuntime() *Runtime {
	cfg := &Runtime{
		HAURL:        c.HomeAssistant.URL,
		Token:        c.HomeAssistant.Token,
		EventType:    DefaultEventType,
		Chargers:     append([]Charger(nil), c.Chargers...),
		DatabasePath: envOrValue(c.Runtime.DatabasePath, defaultRuntimePath("bridge.db")),
		LogFile:      envOrValue(c.Runtime.LogFile, defaultRuntimePath("app.log")),
		APIAddr:      envOrValue(c.Runtime.APIAddr, defaultAPIAddr),
	}
	if len(c.HomeAssistant.EventTypes) > 0 {
		cfg.EventType = c.HomeAssistant.EventTypes[0]
	}
	if len(c.Chargers) > 0 {
		cfg.StartThresholdW = c.Chargers[0].Start.ThresholdW
		cfg.EndThresholdW = c.Chargers[0].Stop.ThresholdW
		if duration, err := parseConfigDuration(c.Chargers[0].Stop.Duration); err == nil && duration > 0 {
			cfg.EndDebounce = duration
		} else {
			cfg.EndDebounce = defaultEndDebounce
		}
	}
	return cfg
}

func mergeRuntimeConfig(current Runtime, loaded Runtime) Runtime {
	loaded.ConfigFile = current.ConfigFile
	flag.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "ha-url":
			loaded.HAURL = current.HAURL
		case "token":
			loaded.Token = current.Token
		case "event-type":
			loaded.EventType = current.EventType
		case "start-threshold-w":
			loaded.StartThresholdW = current.StartThresholdW
			for i := range loaded.Chargers {
				loaded.Chargers[i].Start.ThresholdW = current.StartThresholdW
			}
		case "end-threshold-w":
			loaded.EndThresholdW = current.EndThresholdW
			for i := range loaded.Chargers {
				loaded.Chargers[i].Stop.ThresholdW = current.EndThresholdW
			}
		case "end-debounce":
			loaded.EndDebounce = current.EndDebounce
			for i := range loaded.Chargers {
				loaded.Chargers[i].Stop.Duration = current.EndDebounce.String()
			}
		case "database":
			loaded.DatabasePath = current.DatabasePath
		case "log-file":
			loaded.LogFile = current.LogFile
		case "api-addr":
			loaded.APIAddr = current.APIAddr
		}
	})
	return loaded
}

func interpolateEnv(payload []byte, lookupEnv func(string) (string, bool)) ([]byte, error) {
	var missing []string
	resolved := os.Expand(string(payload), func(name string) string {
		value, ok := lookupEnv(name)
		if !ok {
			missing = append(missing, name)
			return ""
		}
		return value
	})
	if len(missing) > 0 {
		return nil, fmt.Errorf("missing environment variable %q", missing[0])
	}
	return []byte(resolved), nil
}

func envOrValue(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func defaultRuntimePath(name string) string {
	home, err := os.UserHomeDir()
	if err == nil && strings.TrimSpace(home) != "" {
		return filepath.Join(home, defaultRuntimeDirName, name)
	}
	return filepath.Join(defaultRuntimeDirName, name)
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
