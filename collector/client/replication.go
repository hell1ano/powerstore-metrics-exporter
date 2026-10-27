package client

import "time"

// Share the existing Metro query with session monitoring. Embedded rule data
// avoids a separate replication_rule request. Do not filter by type, role or
// volume inventory: destination and child filesystem sessions are monitored too.
// Cache successful responses and errors for one minute, including concurrent
// scrapes. Pagination still follows the array's Content-Range contract.
func (c *Client) GetReplicationSessions() (string, error) {
	data, err, _ := c.replicationStatus.get(time.Minute, time.Now, func() (string, error) {
		return c.getData("replication_session?select=*,replication_rule(id,name,rpo,alert_threshold)", "GET", "")
	})
	return data, err
}
