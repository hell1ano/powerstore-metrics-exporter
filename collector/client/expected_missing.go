package client

import (
	"fmt"
	"strings"
)

// Copy the configuration so collection can read it concurrently without mutation.
func parseExpectedMissing(config map[string][]string) (map[string]map[string]bool, error) {
	result := make(map[string]map[string]bool)
	for collector, ids := range config {
		switch collector {
		case "filesystem_capacity", "filesystem_performance", "volume_performance":
		default:
			return nil, fmt.Errorf("unsupported expectedMissingMetrics collector %q", collector)
		}
		result[collector] = make(map[string]bool)
		for _, id := range ids {
			if strings.TrimSpace(id) == "" || strings.TrimSpace(id) != id || strings.ContainsAny(id, "*?[]") {
				return nil, fmt.Errorf("expectedMissingMetrics requires exact nonempty object IDs: %q", id)
			}
			result[collector][id] = true
		}
	}
	return result, nil
}

func (c *Client) MissingMetricsExpected(collector, id string) bool {
	return c.expectedMissingMetrics[collector][id]
}
