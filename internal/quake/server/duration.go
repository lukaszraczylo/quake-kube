// Package server runs the ioq3ded dedicated server and renders its configuration.
package server

import (
	"encoding/json"
	"fmt"
	"time"
)

// Duration is a time.Duration that reads and writes as a Go duration string
// such as "15m" in YAML and JSON.
type Duration struct {
	time.Duration
}

// UnmarshalJSON parses a duration string.
func (d *Duration) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return fmt.Errorf("duration must be a string like \"15m\": %w", err)
	}
	v, err := time.ParseDuration(s)
	if err != nil {
		return err
	}
	d.Duration = v
	return nil
}

// MarshalJSON writes the duration string.
func (d Duration) MarshalJSON() ([]byte, error) {
	return json.Marshal(d.String())
}
