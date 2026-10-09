package bulkClient

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"reflect"
	"strconv"
	"strings"
)

var csvModels = map[string]reflect.Type{
	"performance_metrics_by_appliance.csv":   reflect.TypeOf(PerformanceMetricsByAppliance{}),
	"performance_metrics_by_fe_eth_port.csv": reflect.TypeOf(PerformanceMetricsByFeEthPort{}),
	"performance_metrics_by_fe_fc_port.csv":  reflect.TypeOf(PerformanceMetricsByFeFcPort{}),
	"performance_metrics_by_file_system.csv": reflect.TypeOf(PerformanceMetricsByFileSystem{}),
	"performance_metrics_by_nas_server.csv":  reflect.TypeOf(PerformanceMetricsByNasServer{}),
	"performance_metrics_by_volume.csv":      reflect.TypeOf(PerformanceMetricsByVolume{}),
	"performance_metrics_by_vg.csv":          reflect.TypeOf(PerformanceMetricsByVg{}),
	"space_metrics_by_appliance.csv":         reflect.TypeOf(SpaceMetricsByAppliance{}),
	"space_metrics_by_file_system.csv":       reflect.TypeOf(SpaceMetricsByFilesystem{}),
	"wear_metrics_by_drive.csv":              reflect.TypeOf(WearMetricsByDrive{}),
}

// decodeCSV uses the model's types but retains field presence. Optional missing
// columns and empty numeric cells are absent, never synthesized as zero.
// Identity fields and timestamp are mandatory, as is at least one known numeric
// measurement per row. Unknown columns are ignored for forward compatibility.
func decodeCSV(filename string, input io.Reader) (string, error) {
	model, ok := csvModels[filename]
	if !ok {
		return "", fmt.Errorf("unknown CSV model")
	}
	r := csv.NewReader(input)
	header, err := r.Read()
	if err != nil {
		return "", fmt.Errorf("read CSV header: %w", err)
	}
	columns := map[string]int{}
	for i, column := range header {
		if _, exists := columns[column]; exists {
			return "", fmt.Errorf("duplicate CSV column %s", column)
		}
		columns[column] = i
	}
	numericColumns := 0
	for i := 0; i < model.NumField(); i++ {
		field := model.Field(i)
		_, present := columns[field.Tag.Get("csv")]
		if field.Type.Kind() == reflect.String && !present {
			return "", fmt.Errorf("missing required column %s", field.Tag.Get("csv"))
		}
		if field.Type.Kind() != reflect.String && present {
			numericColumns++
		}
	}
	if numericColumns == 0 {
		return "", fmt.Errorf("CSV has no known measurement columns")
	}
	records := make([]map[string]interface{}, 0)
	for rowNumber := 2; ; rowNumber++ {
		row, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", fmt.Errorf("invalid CSV row %d", rowNumber)
		}
		record := map[string]interface{}{}
		measurements := 0
		for i := 0; i < model.NumField(); i++ {
			field := model.Field(i)
			column, present := columns[field.Tag.Get("csv")]
			if !present {
				continue
			}
			value := strings.TrimSpace(row[column])
			key := field.Tag.Get("json")
			if field.Type.Kind() == reflect.String {
				if value == "" {
					return "", fmt.Errorf("empty required %s at row %d", key, rowNumber)
				}
				record[key] = value
				continue
			}
			if value == "" {
				continue
			}
			if field.Type.Kind() == reflect.Float64 {
				n, err := strconv.ParseFloat(value, 64)
				if err != nil || math.IsNaN(n) || math.IsInf(n, 0) {
					return "", fmt.Errorf("invalid number for %s at row %d", key, rowNumber)
				}
				record[key] = n
			} else {
				n, err := strconv.ParseInt(value, 10, 64)
				if err != nil {
					return "", fmt.Errorf("invalid integer for %s at row %d", key, rowNumber)
				}
				record[key] = n
			}
			measurements++
		}
		if measurements == 0 {
			return "", fmt.Errorf("no measurements at row %d", rowNumber)
		}
		records = append(records, record)
	}
	data, err := json.Marshal(records)
	return string(data), err
}
