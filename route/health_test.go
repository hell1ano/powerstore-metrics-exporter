package route

import (
	"github.com/gin-gonic/gin"
	"net/http/httptest"
	"testing"
)

func TestHealthDistinguishesInitializationAndRecovery(t *testing.T) {
	gin.SetMode(gin.TestMode)
	a, b := false, false
	r := gin.New()
	registerHealth(r, []func() bool{func() bool { return a }, func() bool { return b }})
	for _, tc := range []struct {
		a, b  bool
		ready int
	}{{false, false, 503}, {true, false, 503}, {true, true, 200}, {false, true, 503}, {true, true, 200}} {
		a, b = tc.a, tc.b
		for path, want := range map[string]int{"/livez": 200, "/readyz": tc.ready} {
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
			if w.Code != want {
				t.Fatalf("%s: got %d want %d", path, w.Code, want)
			}
		}
	}
}
