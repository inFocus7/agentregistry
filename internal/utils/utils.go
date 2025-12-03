package utils

import (
	"fmt"
	"net"
	"strings"
)

// FindAvailablePort finds an available port on localhost
func FindAvailablePort() (uint16, error) {
	// Try to bind to port 0, which tells the OS to pick an available port
	listener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		return 0, fmt.Errorf("failed to find available port: %w", err)
	}
	defer func() { _ = listener.Close() }()

	// Get the port that was assigned
	addr := listener.Addr().(*net.TCPAddr)
	return uint16(addr.Port), nil
}

// ParseKeyValuePairs parses KEY=VALUE pairs.
func ParseKeyValuePairs(pairs []string) (map[string]string, error) {
	result := make(map[string]string)
	for _, pair := range pairs {
		parts := strings.SplitN(pair, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid key=value pair (missing =): %s", pair)
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		if key == "" {
			return nil, fmt.Errorf("invalid key=value pair (empty key): %s", pair)
		}
		result[key] = value
	}
	return result, nil
}
