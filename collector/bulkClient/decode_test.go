package bulkClient

import (
	"strings"
	"testing"

	"github.com/tidwall/gjson"
)

func TestDecodePreservesMissingValues(t *testing.T) {
	data, err := decodeCSV("performance_metrics_by_volume.csv", strings.NewReader("volume_id,appliance_id,timestamp,avg_read_iops,avg_write_iops\nv1,a1,2026-01-01T00:00:00Z,0,\n"))
	if err != nil {
		t.Fatal(err)
	}
	row := gjson.Parse(data).Array()[0]
	if !row.Get("avg_read_iops").Exists() || row.Get("avg_read_iops").Float() != 0 {
		t.Fatal("lost genuine zero")
	}
	if row.Get("avg_write_iops").Exists() || row.Get("avg_latency").Exists() {
		t.Fatal("manufactured absent value")
	}
}

func TestDecodeRejectsInvalidSchemas(t *testing.T) {
	for _, csv := range []string{
		"volume_id,timestamp,avg_read_iops\nv1,t,1\n",
		"volume_id,appliance_id,timestamp\nv1,a1,t\n",
		"volume_id,appliance_id,timestamp,avg_read_iops\n,a1,t,1\n",
		"volume_id,appliance_id,timestamp,avg_read_iops\nv1,a1,t,\n",
		"volume_id,appliance_id,timestamp,avg_read_iops\nv1,a1,t,NaN\n",
		"volume_id,appliance_id,timestamp,avg_read_iops,avg_read_iops\nv1,a1,t,1,2\n",
	} {
		if _, err := decodeCSV("performance_metrics_by_volume.csv", strings.NewReader(csv)); err == nil {
			t.Fatal("invalid schema accepted")
		}
	}
}
