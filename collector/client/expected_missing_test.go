package client

import "testing"

func TestExpectedMissingConfiguration(t *testing.T) {
	for _, config := range []map[string][]string{
		{"typo": {"id"}}, {"volume_performance": {""}}, {"volume_performance": {"*"}}, {"filesystem_capacity": {" id"}},
	} {
		if _, err := parseExpectedMissing(config); err == nil {
			t.Fatalf("accepted invalid config %v", config)
		}
	}
	config := map[string][]string{"volume_performance": {"one"}}
	parsed, err := parseExpectedMissing(config)
	if err != nil {
		t.Fatal(err)
	}
	config["volume_performance"][0] = "two"
	c := &Client{expectedMissingMetrics: parsed}
	if !c.MissingMetricsExpected("volume_performance", "one") || c.MissingMetricsExpected("volume_performance", "two") || c.MissingMetricsExpected("filesystem_capacity", "one") {
		t.Fatal("configuration not copied or scoped")
	}
}
