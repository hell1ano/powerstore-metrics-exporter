package utils

import (
	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"net/http"
	"time"
)

type apiLogTransport struct {
	next        http.RoundTripper
	logger      log.Logger
	array, kind string
}

func LogAPITransport(next http.RoundTripper, logger log.Logger, array, kind string) http.RoundTripper {
	return &apiLogTransport{next, logger, array, kind}
}
func (t *apiLogTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	start := time.Now()
	resp, err := t.next.RoundTrip(req)
	status := 0
	if resp != nil {
		status = resp.StatusCode
	}
	logger := level.Info(t.logger)
	if err != nil || status >= 400 {
		logger = level.Warn(t.logger)
	}
	// Never include query strings, bodies, credentials or raw transport errors
	// (which may contain a full URL). Duration ends when response headers arrive.
	_ = logger.Log("stream", "powerstore_api", "msg", "PowerStore HTTP request", "ip", t.array, "api", t.kind, "method", req.Method, "path", req.URL.Path, "status", status, "duration", time.Since(start), "transport_error", err != nil)
	return resp, err
}
