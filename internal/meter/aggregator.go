package meter

import (
	"errors"
	"fmt"
	"time"

	bridgeconfig "ha-ev-charging-bridge/internal/config"
	"ha-ev-charging-bridge/internal/events"
)

const (
	aggregationAverage = "average"
	aggregationLast    = "last"
	outsideSave        = "save"
	outsideDrop        = "drop"
)

type Aggregator struct {
	config bridgeconfig.Meter
	values []events.MeterValue
}

func NewAggregator(config bridgeconfig.Meter) (*Aggregator, error) {
	switch config.Aggregation {
	case aggregationAverage, aggregationLast:
	default:
		return nil, fmt.Errorf("unsupported aggregation %q", config.Aggregation)
	}
	switch config.OutsideSessionStorage {
	case outsideSave, outsideDrop:
	default:
		return nil, fmt.Errorf("unsupported outside session storage %q", config.OutsideSessionStorage)
	}
	return &Aggregator{config: config}, nil
}

func (a *Aggregator) Observe(value events.MeterValue, activeSessionID string) {
	if activeSessionID == "" && a.config.OutsideSessionStorage == outsideDrop {
		return
	}
	value.MeterID = a.config.MeterID
	value.EntityID = a.config.EntityID
	value.Unit = a.config.Unit
	value.SessionID = activeSessionID
	a.values = append(a.values, value)
}

func (a *Aggregator) Flush(at time.Time) (events.MeterValue, bool, error) {
	if len(a.values) == 0 {
		return events.MeterValue{}, false, nil
	}

	result := a.values[len(a.values)-1]
	result.ObservedAt = at
	switch a.config.Aggregation {
	case aggregationAverage:
		var total float64
		for _, value := range a.values {
			total += value.Value
		}
		result.Value = total / float64(len(a.values))
	case aggregationLast:
		result.Value = a.values[len(a.values)-1].Value
	default:
		return events.MeterValue{}, false, errors.New("invalid aggregation state")
	}

	a.values = nil
	return result, true, nil
}
