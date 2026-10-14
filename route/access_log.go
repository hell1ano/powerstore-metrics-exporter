package route

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"net"
	"strings"
	"sync/atomic"
	"time"
)

func requestLogging(logger log.Logger) gin.HandlerFunc {
	var sequence atomic.Uint64
	prefix := fmt.Sprintf("%x", time.Now().UnixNano())
	return func(c *gin.Context) {
		started := time.Now()
		id := fmt.Sprintf("%s-%d", prefix, sequence.Add(1))
		c.Header("X-Request-ID", id)
		peer, _, err := net.SplitHostPort(c.Request.RemoteAddr)
		if err != nil {
			peer = c.Request.RemoteAddr
		}
		// Log the actual socket peer; forwarded headers are not trusted identity.
		agent := strings.NewReplacer("\r", " ", "\n", " ").Replace(c.Request.UserAgent())
		if len(agent) > 256 {
			agent = agent[:256]
		}
		c.Next()
		level.Info(logger).Log("msg", "exporter HTTP request", "request_id", id,
			"peer_ip", peer, "user_agent", agent, "method", c.Request.Method,
			"path", c.Request.URL.Path, "status", c.Writer.Status(), "duration", time.Since(started))
	}
}
