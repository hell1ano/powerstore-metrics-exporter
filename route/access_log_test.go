package route

import (
	"bytes"
	"github.com/gin-gonic/gin"
	"github.com/go-kit/log"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAccessLogShowsCallerWithoutSensitiveHeadersOrQuery(t *testing.T) {
	var output bytes.Buffer
	r := gin.New()
	r.Use(requestLogging(log.NewLogfmtLogger(&output)))
	r.GET("/metrics/array/nas", func(c *gin.Context) { c.Status(200) })
	previous := ""
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest("GET", "/metrics/array/nas?password=secret-query", nil)
		req.RemoteAddr = "192.0.2.10:50000"
		req.Header.Set("User-Agent", "synthetic-monitor")
		req.Header.Set("X-Forwarded-For", "spoofed-peer")
		req.Header.Set("Authorization", "secret-auth")
		req.Header.Set("Cookie", "secret-cookie")
		req.Header.Set("X-Request-ID", "untrusted-id")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		id := w.Header().Get("X-Request-ID")
		if id == "" || id == previous || id == "untrusted-id" {
			t.Fatal("request ID not generated uniquely")
		}
		previous = id
	}
	text := output.String()
	for _, want := range []string{"peer_ip=192.0.2.10", "user_agent=synthetic-monitor", "path=/metrics/array/nas", "status=200", "duration=", "request_id="} {
		if !strings.Contains(text, want) {
			t.Errorf("missing %s", want)
		}
	}
	for _, secret := range []string{"secret-query", "secret-auth", "secret-cookie", "spoofed-peer"} {
		if strings.Contains(text, secret) {
			t.Errorf("logged %s", secret)
		}
	}
}
