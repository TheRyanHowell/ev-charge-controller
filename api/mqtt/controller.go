package mqtt

import (
	"context"
	"time"

	"ev-charge-controller/api/tasmota"
)

// Controller implements internal.PlugController using MQTT for power control
// and the dispatcher's energy cache for last-known energy.
type Controller struct {
	dispatcher *Dispatcher
	publisher  *Publisher
}

// NewController creates a PlugController backed by dispatcher + publisher.
func NewController(dispatcher *Dispatcher, publisher *Publisher) *Controller {
	return &Controller{dispatcher: dispatcher, publisher: publisher}
}

// SetPower publishes an ON/OFF command to the plug's MQTT topic.
func (c *Controller) SetPower(ctx context.Context, plugID string, on bool) error {
	return c.publisher.SetPower(ctx, plugID, on)
}

// SetPowerAndWait publishes an ON/OFF command and waits for stat/POWER confirmation.
// Returns (true, nil) if the plug confirmed the new state, (false, error) on timeout.
func (c *Controller) SetPowerAndWait(ctx context.Context, plugID string, on bool, timeout time.Duration) (bool, error) {
	return c.publisher.SetPowerAndWait(ctx, plugID, on, c.dispatcher, timeout)
}

// LastEnergy returns the most recent cached energy for plugID, or nil if none.
func (c *Controller) LastEnergy(plugID string) *tasmota.EnergyData {
	return c.dispatcher.LastEnergy(plugID)
}

// LastPowerState returns the most recent cached relay state for plugID.
// known is false if no state message has been received yet.
func (c *Controller) LastPowerState(plugID string) (on, known bool) {
	return c.dispatcher.LastPowerState(plugID)
}
