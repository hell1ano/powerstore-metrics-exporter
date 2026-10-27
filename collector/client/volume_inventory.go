package client

import "github.com/tidwall/gjson"

// UpdateVolumeInventory reuses the successful source-filtered volume status
// query. This refreshes source eligibility and names without extra REST calls.
// Concurrent collectors retain their immutable snapshot until the next scrape.
func (c *Client) UpdateVolumeInventory(data string) error {
	names, err := resultToMap(data)
	if err != nil {
		return err
	}
	appliances := map[string]gjson.Result{}
	for _, row := range gjson.Parse(data).Array() {
		appliances[row.Get("id").String()] = row.Get("appliance_id")
	}
	c.inventoryLoadMu.Lock()
	defer c.inventoryLoadMu.Unlock()
	inventoryMu.Lock()
	defer inventoryMu.Unlock()
	snapshot := map[string]map[string]gjson.Result{}
	for key, value := range PowerstoreModuleID[c.IP] {
		snapshot[key] = value
	}
	snapshot["volume"], snapshot["volume_appliance"] = names, appliances
	PowerstoreModuleID[c.IP] = snapshot
	return nil
}
