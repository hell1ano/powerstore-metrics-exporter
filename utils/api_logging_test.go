package utils

import (
	"bytes"
	"errors"
	"github.com/go-kit/log"
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestAPIRequestLogging(t *testing.T) {
	for _, fail := range []bool{false, true} {
		var out bytes.Buffer
		underlying := roundTripFunc(func(r *http.Request) (*http.Response, error) {
			if fail {
				return nil, errors.New("secret transport detail")
			}
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader("payload"))}, nil
		})
		tr := LogAPITransport(underlying, log.NewLogfmtLogger(&out), "synthetic-array", "rest")
		req, _ := http.NewRequest("GET", "https://user:password@example.invalid/api/rest/volume?token=secret-query", nil)
		req.Header.Set("Authorization", "secret-auth")
		resp, err := tr.RoundTrip(req)
		if (err != nil) != fail {
			t.Fatal("transport error changed")
		}
		if resp != nil {
			body, _ := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			if string(body) != "payload" {
				t.Fatal("response body consumed")
			}
		}
		s := out.String()
		for _, secret := range []string{"password", "secret", "payload"} {
			if strings.Contains(s, secret) {
				t.Fatalf("sensitive data logged: %s", s)
			}
		}
		for _, field := range []string{"stream=powerstore_api", "ip=synthetic-array", "path=/api/rest/volume", "duration=", "transport_error="} {
			if !strings.Contains(s, field) {
				t.Fatalf("missing %s", field)
			}
		}
	}
}
