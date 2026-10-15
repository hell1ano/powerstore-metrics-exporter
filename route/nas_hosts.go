package route

import (
	"crypto/sha256"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	dto "github.com/prometheus/client_model/go"
	"github.com/tidwall/gjson"
	"powerstore-metrics-exporter/collector/client"
)

type nasOwner struct {
	Key  string `json:"{#NASKEY}"`
	Name string `json:"{#NASNAME}"`
	IP   string `json:"{#ARRAYIP}"`
	ID   string `json:"{#NASID}"`
}

func nasHostKey(name string) string { return fmt.Sprintf("%x", sha256.Sum256([]byte(name))) }

// One discovery owner spans all configured arrays. Identity never includes array
// address or NAS UUID, both of which can change during a move/failover.
func discoverNAS(ips []string, snapshot func(string) map[string]map[string]gjson.Result) ([]nasOwner, error) {
	result := []nasOwner{}
	names := map[string]string{}
	for _, ip := range ips {
		inv := snapshot(ip)
		if inv["nas"] == nil || inv["nas_destination"] == nil || inv["nas_policy"] == nil {
			return nil, fmt.Errorf("NAS inventory unavailable for array %s", ip)
		}
		for id, n := range inv["nas"] {
			name := n.String()
			role := inv["nas_destination"][id]
			policy := inv["nas_policy"][id]
			if name == "" || id == "" || (role.Type != gjson.True && role.Type != gjson.False) {
				return nil, fmt.Errorf("NAS identity or role unavailable for array %s NAS %s", ip, id)
			}
			if role.Type == gjson.True && policy.Type == gjson.String && strings.TrimSpace(policy.String()) != "" {
				continue
			}
			if prev, ok := names[name]; ok {
				return nil, fmt.Errorf("multiple eligible NAS objects named %q (%s and %s); ownership is ambiguous", name, prev, ip)
			}
			names[name] = ip
			result = append(result, nasOwner{nasHostKey(name), name, ip, id})
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Key < result[j].Key })
	return result, nil
}

// Share array-level collection across NAS hosts. Errors are cached too, to avoid
// repeated retries for each child. This cache does not drive PowerStore polling.
type nasGatherCache struct {
	mu     sync.Mutex
	source prometheus.Gatherer
	at     time.Time
	data   []*dto.MetricFamily
	err    error
}

func (c *nasGatherCache) Gather() ([]*dto.MetricFamily, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.at.IsZero() || time.Since(c.at) >= time.Minute {
		c.data, c.err = c.source.Gather()
		c.at = time.Now()
	}
	return c.data, c.err
}

type nasArrayMetrics struct{ nas, file *nasGatherCache }

func nasMetricSubset(data []*dto.MetricFamily, owner nasOwner, component string) []*dto.MetricFamily {
	result := []*dto.MetricFamily{}
	for _, f := range data {
		selected := []*dto.Metric{}
		for _, m := range f.Metric {
			labels := map[string]string{}
			for _, l := range m.Label {
				labels[l.GetName()] = l.GetValue()
			}
			match := false
			if strings.HasPrefix(f.GetName(), "powerstore_collector_") {
				match = true
			} else if component == "file" {
				match = labels["nas_server_id"] == owner.ID
			} else {
				match = (f.GetName() == "powerstore_nas_server_operational_status" && labels["name"] == owner.Name) || (strings.HasPrefix(f.GetName(), "powerstore_metricNas_") && labels["nas_id"] == owner.Name)
			}
			if match {
				selected = append(selected, m)
			}
		}
		if len(selected) > 0 {
			result = append(result, &dto.MetricFamily{Name: f.Name, Help: f.Help, Type: f.Type, Metric: selected})
		}
	}
	return result
}

func registerNASHosts(r *gin.Engine, ips []string, arrays map[string]nasArrayMetrics, logger log.Logger) {
	r.GET("/discovery/nas", func(c *gin.Context) {
		owners, err := discoverNAS(ips, client.ModuleIDs)
		if err != nil {
			level.Warn(logger).Log("msg", "NAS host discovery unavailable", "err", err)
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, owners)
	})
	r.GET("/nas-hosts/:key/:component", func(c *gin.Context) {
		component := c.Param("component")
		if component != "nas" && component != "file" {
			c.Status(http.StatusNotFound)
			return
		}
		owners, err := discoverNAS(ips, client.ModuleIDs)
		if err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
			return
		}
		for _, owner := range owners {
			if owner.Key != c.Param("key") {
				continue
			}
			sources, ok := arrays[owner.IP]
			if !ok {
				c.Status(http.StatusServiceUnavailable)
				return
			}
			cache := sources.nas
			if component == "file" {
				cache = sources.file
			}
			if cache == nil {
				c.Status(http.StatusServiceUnavailable)
				return
			}
			gather := prometheus.GathererFunc(func() ([]*dto.MetricFamily, error) {
				data, err := cache.Gather()
				if err != nil {
					return nil, err
				}
				return nasMetricSubset(data, owner, component), nil
			})
			promhttp.HandlerFor(gather, promhttp.HandlerOpts{ErrorHandling: promhttp.HTTPErrorOnError}).ServeHTTP(c.Writer, c.Request)
			return
		}
		c.JSON(http.StatusNotFound, gin.H{"error": "NAS has no eligible source in current inventory"})
	})
}
