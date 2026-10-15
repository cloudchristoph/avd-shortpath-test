// Package config holds the embedded default test targets.
package config

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"time"
)

//go:embed servers.json
var serversJSON []byte

// Servers are the default test targets.
type Servers struct {
	// TURNServer is resolved (A records only) and every address is tested,
	// like avdnettest.exe's --avd-turn-server.
	TURNServer string `json:"turnServer"`
	Port       uint16 `json:"port"`
	// TimeoutMS is the wait for each response. No retransmissions.
	TimeoutMS int `json:"timeoutMs"`
}

// Timeout returns TimeoutMS as a duration.
func (s Servers) Timeout() time.Duration {
	return time.Duration(s.TimeoutMS) * time.Millisecond
}

// Default returns the embedded defaults.
func Default() Servers {
	s, err := Parse(serversJSON)
	if err != nil {
		panic(err) // embedded file is checked by tests
	}
	return s
}

// Parse reads a servers file.
func Parse(b []byte) (Servers, error) {
	var s Servers
	if err := json.Unmarshal(b, &s); err != nil {
		return Servers{}, fmt.Errorf("servers: %w", err)
	}
	if s.TURNServer == "" || s.Port == 0 || s.TimeoutMS <= 0 {
		return Servers{}, fmt.Errorf("servers: turnServer, port and timeoutMs are required")
	}
	return s, nil
}
