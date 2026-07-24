package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

const deviceTypeSmartPlug = "smart_plug"

type deviceFile struct {
	Devices []deviceConfig `yaml:"devices"`
}

type deviceConfig struct {
	Name           string `yaml:"name"`
	Type           string `yaml:"type"`
	EntityID       string `yaml:"entity_id"`
	EnergyEntityID string `yaml:"energy_entity_id"`
}

func loadDevices(path string) ([]deviceConfig, error) {
	payload, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read device config %q: %w", path, err)
	}

	var file deviceFile
	decoder := yaml.NewDecoder(bytes.NewReader(payload))
	decoder.KnownFields(true)
	if err := decoder.Decode(&file); err != nil {
		return nil, fmt.Errorf("parse device config %q: %w", path, err)
	}
	if len(file.Devices) == 0 {
		return nil, errors.New("device config must define at least one device")
	}

	for i := range file.Devices {
		file.Devices[i].Name = strings.TrimSpace(file.Devices[i].Name)
		file.Devices[i].Type = strings.TrimSpace(file.Devices[i].Type)
		file.Devices[i].EntityID = strings.TrimSpace(file.Devices[i].EntityID)
		file.Devices[i].EnergyEntityID = strings.TrimSpace(file.Devices[i].EnergyEntityID)
		if file.Devices[i].Name == "" {
			file.Devices[i].Name = fmt.Sprintf("device-%d", i+1)
		}
		if err := validateDevice(file.Devices[i]); err != nil {
			return nil, fmt.Errorf("device %q: %w", file.Devices[i].Name, err)
		}
	}

	return file.Devices, nil
}

func validateDevice(device deviceConfig) error {
	switch device.Type {
	case deviceTypeSmartPlug:
		if device.EntityID == "" {
			return errors.New("smart_plug requires entity_id")
		}
		if device.EnergyEntityID == "" {
			return errors.New("smart_plug requires energy_entity_id")
		}
	case "charger":
		return errors.New("charger devices are not supported yet")
	default:
		return fmt.Errorf("unsupported type %q", device.Type)
	}

	return nil
}

func firstSmartPlug(devices []deviceConfig) (deviceConfig, int, error) {
	var selected deviceConfig
	count := 0
	for _, device := range devices {
		if device.Type != deviceTypeSmartPlug {
			continue
		}
		count++
		if count == 1 {
			selected = device
		}
	}
	if count == 0 {
		return deviceConfig{}, 0, errors.New("device config does not contain a smart_plug device")
	}
	return selected, count, nil
}
