package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseV1LoadsValidYAML(t *testing.T) {
	cfg, err := ParseV1([]byte(validV1ConfigYAML()), mapLookup(map[string]string{
		"HA_URL":   "http://ha.test:8123",
		"HA_TOKEN": "test-token",
	}))
	if err != nil {
		t.Fatalf("ParseV1() error = %v", err)
	}

	if cfg.HomeAssistant.URL != "http://ha.test:8123" {
		t.Fatalf("HomeAssistant.URL = %q, want %q", cfg.HomeAssistant.URL, "http://ha.test:8123")
	}
	if len(cfg.Chargers) != 1 {
		t.Fatalf("len(Chargers) = %d, want 1", len(cfg.Chargers))
	}
	charger := cfg.Chargers[0]
	if charger.ChargerID != "charger-1" || charger.EVSEID != "evse-1" || charger.ConnectorID != "connector-1" || charger.MeterID != "meter-1" {
		t.Fatalf("charger identity = %#v", charger)
	}
	if charger.Start.ThresholdW != 200 || charger.Stop.ThresholdW != 50 {
		t.Fatalf("thresholds = start %.1f stop %.1f, want 200 and 50", charger.Start.ThresholdW, charger.Stop.ThresholdW)
	}
}

func TestParseV1RejectsMissingRequiredChargerFields(t *testing.T) {
	payload := strings.Replace(validV1ConfigYAML(), "  - charger_id: charger-1\n", "  -\n", 1)
	_, err := ParseV1([]byte(payload), mapLookup(map[string]string{
		"HA_URL":   "http://ha.test:8123",
		"HA_TOKEN": "test-token",
	}))
	if err == nil {
		t.Fatal("ParseV1() error = nil, want missing charger_id error")
	}
	if !strings.Contains(err.Error(), "chargers[0].charger_id is required") {
		t.Fatalf("ParseV1() error = %q, want missing charger_id", err)
	}
}

func TestParseV1ResolvesProcessEnvironment(t *testing.T) {
	cfg, err := ParseV1([]byte(validV1ConfigYAML()), mapLookup(map[string]string{
		"HA_URL":   "http://from-env.test:8123",
		"HA_TOKEN": "from-env-token",
	}))
	if err != nil {
		t.Fatalf("ParseV1() error = %v", err)
	}

	if cfg.HomeAssistant.URL != "http://from-env.test:8123" {
		t.Fatalf("HomeAssistant.URL = %q, want env value", cfg.HomeAssistant.URL)
	}
	if cfg.HomeAssistant.Token != "from-env-token" {
		t.Fatalf("HomeAssistant.Token = %q, want env value", cfg.HomeAssistant.Token)
	}
}

func TestLoadDotEnvSupportsConfigEnvironmentInterpolation(t *testing.T) {
	t.Setenv("HA_URL", "")
	t.Setenv("HA_TOKEN", "")

	dir := t.TempDir()
	dotEnvPath := filepath.Join(dir, ".env")
	if err := os.WriteFile(dotEnvPath, []byte("HA_URL=http://dotenv.test:8123\nHA_TOKEN=dotenv-token\n"), 0600); err != nil {
		t.Fatalf("write .env: %v", err)
	}
	if err := LoadDotEnv(dotEnvPath); err != nil {
		t.Fatalf("LoadDotEnv() error = %v", err)
	}

	cfg, err := ParseV1([]byte(validV1ConfigYAML()), os.LookupEnv)
	if err != nil {
		t.Fatalf("ParseV1() error = %v", err)
	}
	if cfg.HomeAssistant.URL != "http://dotenv.test:8123" {
		t.Fatalf("HomeAssistant.URL = %q, want dotenv value", cfg.HomeAssistant.URL)
	}
}

func TestExampleConfigContainsNoRealSecretsOrPersonalHostnames(t *testing.T) {
	payload, err := os.ReadFile("../../config.yaml")
	if err != nil {
		t.Fatalf("read config.yaml: %v", err)
	}
	content := string(payload)
	for _, forbidden := range []string{"eyJ", "http://homeassistant.local", "https://homeassistant.local"} {
		if strings.Contains(content, forbidden) {
			t.Fatalf("config.yaml contains forbidden example value %q", forbidden)
		}
	}
}

func mapLookup(values map[string]string) func(string) (string, bool) {
	return func(name string) (string, bool) {
		value, ok := values[name]
		return value, ok
	}
}

func validV1ConfigYAML() string {
	return `home_assistant:
  url: ${HA_URL}
  token: ${HA_TOKEN}
  event_types:
    - state_changed
chargers:
  - charger_id: charger-1
    evse_id: evse-1
    connector_id: connector-1
    meter_id: meter-1
    entities:
      power_w: sensor.ev_charger_power
      energy_kwh: sensor.ev_charger_energy
      availability: binary_sensor.ev_charger_online
    availability:
      entity_id: binary_sensor.ev_charger_online
      available_state: "on"
      unavailable_state: "off"
      unavailable_after: 2m
    start:
      type: power_threshold
      entity_id: sensor.ev_charger_power
      threshold_w: 200
      duration: 0s
    stop:
      type: power_threshold
      entity_id: sensor.ev_charger_power
      threshold_w: 50
      duration: 10s
    meters:
      - meter_id: meter-1
        entity_id: sensor.ev_charger_power
        unit: W
        aggregation: average
        outside_session_storage: drop
      - meter_id: meter-1
        entity_id: sensor.ev_charger_energy
        unit: kWh
        aggregation: last
        outside_session_storage: save
retention:
  meter_values: 90d
  lifecycle_events: 365d
  raw_events: 7d
runtime:
  database_path: var/bridge.db
  log_file: log/app.log
`
}
