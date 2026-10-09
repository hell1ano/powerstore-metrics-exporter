package generalCollector

import (
	"fmt"
	"github.com/tidwall/gjson"
	"testing"
)

func TestPortSpeedParsing(t *testing.T) {
	for _, tc := range []struct {
		value string
		want  float64
		fail  bool
	}{
		{"Auto", 0, false}, {"25_Gbps", 25, false}, {"100_Mbps", 0.1, false}, {"10_Mbps", 0.01, false}, {"1000_Mbps", 1, false},
		{"", 0, true}, {"garbage", 0, true}, {"x_Gbps", 0, true}, {"NaN_Gbps", 0, true},
		{"+Inf_Gbps", 0, true}, {"-1_Gbps", 0, true}, {"10_unknown", 0, true},
	} {
		t.Run(tc.value, func(t *testing.T) {
			got, err := getPortFloatDate("current_speed", gjson.Parse(fmt.Sprintf("%q", tc.value)))
			if (err != nil) != tc.fail || (!tc.fail && got != tc.want) {
				t.Fatalf("got %v, %v; want %v, failure=%v", got, err, tc.want, tc.fail)
			}
		})
	}
	if got, err := getPortFloatDate("current_speed", gjson.Parse("null")); got != 0 || err != nil {
		t.Fatalf("null: %v %v", got, err)
	}
}
