package route

import (
	"github.com/gin-gonic/gin"
	"net/http"
)

func registerHealth(r *gin.Engine, checks []func() bool) {
	r.GET("/livez", func(c *gin.Context) { c.String(http.StatusOK, "alive\n") })
	r.GET("/readyz", func(c *gin.Context) {
		if len(checks) == 0 {
			c.String(http.StatusServiceUnavailable, "not ready\n")
			return
		}
		for _, ready := range checks {
			if !ready() {
				c.String(http.StatusServiceUnavailable, "not ready\n")
				return
			}
		}
		c.String(http.StatusOK, "ready\n")
	})
}
