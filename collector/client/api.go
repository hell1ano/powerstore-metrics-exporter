/*
 Copyright (c) 2024-2025 Dell Inc. or its subsidiaries. All Rights Reserved.

 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
*/

package client

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/tidwall/gjson"
	"powerstore-metrics-exporter/utils"
	"strconv"
	"sync"
)

type RequestBody struct {
	Entity   string `json:"entity"`
	EntityID string `json:"entity_id"`
	Interval string `json:"interval"`
}

// PowerstoreModuleID This map stores the mapping relationships of the ip, module type, module id, and module name of the powerstore
var inventoryMu sync.RWMutex

// PowerstoreModuleID is retained for compatibility. Runtime readers use ModuleIDs.
var PowerstoreModuleID = make(map[string]map[string]map[string]gjson.Result)

func (c *Client) getData(path, method, body string) (string, error) {
	utils.ReqCounter <- 1
	result, err := c.getResource(method, path, body)
	<-utils.ReqCounter
	if err == nil && (!gjson.Valid(result) || !gjson.Parse(result).IsArray()) {
		return "", errors.New("PowerStore returned invalid JSON or a non-array response")
	}
	return result, err
}

func (c *Client) GetCluster() (string, error) {
	return c.getData("cluster?select=*&limit="+strconv.Itoa(c.limit), "GET", "")
}

func (c *Client) GetPort(portType string) (string, error) {
	return c.getData(portType+"?select=*&limit="+strconv.Itoa(c.limit), "GET", "")
}

func (c *Client) GetHardware(hardwareType string) (string, error) {
	return c.getData("hardware?select=*&type=eq."+hardwareType+"&limit="+strconv.Itoa(c.limit), "GET", "")
}

func (c *Client) GetVolume() (string, error) {
	if c.version == "v3" {
		return c.getData("volume_list_cma_view?select=*&limit="+strconv.Itoa(c.limit), "GET", "")
	}
	return c.getData("volume?select=*&limit="+strconv.Itoa(c.limit), "GET", "")
}

func (c *Client) GetAppliance() (string, error) {
	return c.getData("appliance?select=*&limit="+strconv.Itoa(c.limit), "GET", "")
}

func (c *Client) GetNas() (string, error) {
	return c.getData("nas_server?select=*&limit="+strconv.Itoa(c.limit), "GET", "")
}

func (c *Client) GetNasDetail() (string, error) {
	return c.getData("nas_server_list_cma_view?select=*&limit="+strconv.Itoa(c.limit), "GET", "")
}

func (c *Client) GetVolumeGroup() (string, error) {
	return c.getData("volume_group_list_cma_view?select=*&limit="+strconv.Itoa(c.limit), "GET", "")
}

func (c *Client) GetMetro() (string, error) {
	return c.getData("replication_session?type=eq.Metro_Active_Active&select=*", "GET", "")
}

func (c *Client) GetWitness() (string, error) {
	return c.getData("witness?select=*", "GET", "")
}

func (c *Client) GetPerf(id string) (string, error) {
	var body = &RequestBody{
		Entity:   "performance_metrics_by_appliance",
		EntityID: id,
		Interval: "Five_Mins",
	}
	entityBody, err := json.Marshal(body)
	if err != nil {
		return "", err
	}
	return c.getData("metrics/generate", "POST", string(entityBody))
}

func (c *Client) GetCap(id string) (string, error) {
	interval := c.capacityInterval
	if interval == "" {
		interval = "Five_Mins"
	}
	var body = &RequestBody{
		Entity:   "space_metrics_by_appliance",
		EntityID: id,
		Interval: interval,
	}
	entityBody, err := json.Marshal(body)
	if err != nil {
		return "", err
	}
	return c.getData("metrics/generate", "POST", string(entityBody))
}

func (c *Client) GetMetricVg(id string) (string, error) {
	var body = &RequestBody{
		Entity:   "performance_metrics_by_vg",
		EntityID: id,
		Interval: "Five_Mins",
	}
	entityBody, err := json.Marshal(body)
	if err != nil {
		return "", err
	}
	return c.getData("metrics/generate", "POST", string(entityBody))
}

func (c *Client) GetMetricVolume(id string) (string, error) {
	var body = &RequestBody{
		Entity:   "performance_metrics_by_volume",
		EntityID: id,
		Interval: "Five_Mins",
	}
	entityBody, err := json.Marshal(body)
	if err != nil {
		return "", err
	}
	return c.getData("metrics/generate", "POST", string(entityBody))
}

func (c *Client) GetMetricFcPort(id string) (string, error) {
	var body = &RequestBody{
		Entity:   "performance_metrics_by_fe_fc_port",
		EntityID: id,
		Interval: "Five_Mins",
	}
	entityBody, err := json.Marshal(body)
	if err != nil {
		return "", err
	}
	return c.getData("metrics/generate", "POST", string(entityBody))
}

func (c *Client) GetMetricEthPort(id string) (string, error) {
	var body = &RequestBody{
		Entity:   "performance_metrics_by_fe_eth_port",
		EntityID: id,
		Interval: "Five_Mins",
	}
	entityBody, err := json.Marshal(body)
	if err != nil {
		return "", err
	}
	return c.getData("metrics/generate", "POST", string(entityBody))
}

func (c *Client) GetMetricAppliance(id string) (string, error) {
	var body = &RequestBody{
		Entity:   "performance_metrics_by_appliance",
		EntityID: id,
		Interval: "Five_Mins",
	}
	entityBody, err := json.Marshal(body)
	if err != nil {
		return "", err
	}
	return c.getData("metrics/generate", "POST", string(entityBody))
}

func (c *Client) GetWearMetricByDrive(id string) (string, error) {
	var body = &RequestBody{
		Entity:   "wear_metrics_by_drive",
		EntityID: id,
		Interval: "Five_Mins",
	}
	entityBody, err := json.Marshal(body)
	if err != nil {
		return "", err
	}
	return c.getData("metrics/generate", "POST", string(entityBody))
}

func (c *Client) GetMetricByNas(id string) (string, error) {
	var body = &RequestBody{
		Entity:   "performance_metrics_by_nas_server",
		EntityID: id,
		Interval: "Five_Mins",
	}
	entityBody, err := json.Marshal(body)
	if err != nil {
		return "", err
	}
	return c.getData("metrics/generate", "POST", string(entityBody))
}

func (c *Client) GetFilesystemCap(id string) (string, error) {
	var body = &RequestBody{
		Entity:   "space_metrics_by_file_system",
		EntityID: id,
		Interval: "Five_Mins",
	}
	entityBody, err := json.Marshal(body)
	if err != nil {
		return "", err
	}
	return c.getData("metrics/generate", "POST", string(entityBody))
}

func (c *Client) GetMetricsFilesystem(id string) (string, error) {
	var body = &RequestBody{
		Entity:   "performance_metrics_by_file_system",
		EntityID: id,
		Interval: "Five_Mins",
	}
	entityBody, err := json.Marshal(body)
	if err != nil {
		return "", err
	}
	return c.getData("metrics/generate", "POST", string(entityBody))
}

func (c *Client) GetApplianceId() (string, error) {
	return c.getData("appliance?select=id,name&limit="+strconv.Itoa(c.limit), "GET", "")
}

func (c *Client) GetVolumeGroupId() (string, error) {
	return c.getData("volume_group_list_cma_view?select=id,name,appliance_ids&limit="+strconv.Itoa(c.limit), "GET", "")
}

func (c *Client) GetVolumeId() (string, error) {
	if c.version == "v3" {
		return c.getData("volume_list_cma_view?select=id,name&limit="+strconv.Itoa(c.limit), "GET", "")
	}
	return c.getData("volume?select=id,name&limit="+strconv.Itoa(c.limit), "GET", "")
}

func (c *Client) GetEthPortId() (string, error) {
	return c.getData("eth_port?select=id,name&limit="+strconv.Itoa(c.limit), "GET", "")
}

func (c *Client) GetFcPortId() (string, error) {
	return c.getData("fc_port?select=id,name&limit="+strconv.Itoa(c.limit), "GET", "")
}

func (c *Client) GetDrivesId() (string, error) {
	return c.getData("hardware?select=id,name&type=eq.Drive&limit="+strconv.Itoa(c.limit), "GET", "")
}

func (c *Client) GetNasId() (string, error) {
	return c.getData("nas_server?select=id,name&limit="+strconv.Itoa(c.limit), "GET", "")
}

func (c *Client) GetFilesystemId() (string, error) {
	return c.getData("file_system?select=id,name,nas_server_id&limit="+strconv.Itoa(c.limit), "GET", "")
}

// ModuleIDs returns an immutable snapshot; refreshes replace maps atomically.
func ModuleIDs(ip string) map[string]map[string]gjson.Result {
	inventoryMu.RLock()
	defer inventoryMu.RUnlock()
	return PowerstoreModuleID[ip]
}

func (c *Client) InitModuleID(logger log.Logger) {
	c.loadInventory(logger, false)
}

// RetryMissingInventory only retries failed resource classes. Successful empty
// classes are non-nil and are not rediscovered by the recovery timer.
func (c *Client) RetryMissingInventory(logger log.Logger) {
	c.loadInventory(logger, true)
}

func (c *Client) loadInventory(logger log.Logger, missingOnly bool) {
	c.inventoryLoadMu.Lock()
	defer c.inventoryLoadMu.Unlock()
	snapshot := make(map[string]map[string]gjson.Result)
	if missingOnly {
		for module, entries := range ModuleIDs(c.IP) {
			snapshot[module] = entries
		}
	}

	loaders := map[string]func() (string, error){
		"appliance": c.GetApplianceId, "volume": c.GetVolumeId, "volumegroup": c.GetVolumeGroupId,
		"ethport": c.GetEthPortId, "fcport": c.GetFcPortId, "drive": c.GetDrivesId,
		"nas": c.GetNasId, "filesystem": c.GetFilesystemId,
	}
	for module, load := range loaders {
		if missingOnly && snapshot[module] != nil {
			continue
		}
		data, err := load()
		if err != nil {
			level.Error(logger).Log("msg", "inventory refresh failed", "ip", c.IP, "module", module, "err", err)
			continue
		}
		snapshot[module], err = resultToMap(data)
		if err != nil {
			level.Error(logger).Log("msg", "invalid inventory response", "ip", c.IP, "module", module, "err", err)
			continue
		}
		if module == "filesystem" {
			owners := make(map[string]gjson.Result)
			for _, filesystem := range gjson.Parse(data).Array() {
				owners[filesystem.Get("id").String()] = filesystem.Get("nas_server_id")
			}
			snapshot["filesystem_nas"] = owners
		}
	}
	inventoryMu.Lock()
	PowerstoreModuleID[c.IP] = snapshot
	inventoryMu.Unlock()
}

// resultToMap Convert http response body to map structure
func resultToMap(result string) (map[string]gjson.Result, error) {
	resultMap := make(map[string]gjson.Result)
	var failures []error
	for row, entity := range gjson.Parse(result).Array() {
		id := entity.Get("id").String()
		if id == "" {
			failures = append(failures, fmt.Errorf("inventory row %d: missing id (name=%q)", row+1, entity.Get("name").String()))
			continue
		}
		if entity.Get("name").String() == "" {
			failures = append(failures, fmt.Errorf("id=%q: inventory name is empty", id))
		}
		if _, duplicate := resultMap[id]; duplicate {
			failures = append(failures, fmt.Errorf("id=%q: duplicate inventory ID", id))
		}
		resultMap[id] = entity.Get("name")
	}
	if err := errors.Join(failures...); err != nil {
		return nil, err
	}
	return resultMap, nil
}
