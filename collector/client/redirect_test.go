package client

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-kit/log"
	"powerstore-metrics-exporter/utils"
)

func TestAuthenticationRedirectRetriesOnceWithoutFollowing(t *testing.T) {
	utils.InitReqCounter(2)
	for _, persistent := range []bool{false, true} {
		t.Run(fmt.Sprint(persistent), func(t *testing.T) {
			logins, requests, redirects := 0, 0, 0
			srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/api/rest/login_session":
					logins++
					w.Header().Set("Dell-Emc-Token", "renewed")
					fmt.Fprint(w, `[]`)
				case "/api/rest/cluster":
					requests++
					if persistent || r.Header.Get("DELL-EMC-TOKEN") != "renewed" {
						http.Redirect(w, r, "/login-page", http.StatusFound)
						return
					}
					fmt.Fprint(w, `[{"id":"cluster"}]`)
				default:
					redirects++
					fmt.Fprint(w, "login HTML")
				}
			}))
			defer srv.Close()
			c, err := NewConfiguredClient(utils.Storage{Ip: strings.TrimPrefix(srv.URL, "https://"), User: "test", Password: "test", Version: "v3", TLSInsecureSkipVerify: true}, log.NewNopLogger())
			if err != nil {
				t.Fatal(err)
			}
			data, err := c.GetCluster()
			if (err != nil) != persistent || (!persistent && data != `[{"id":"cluster"}]`) {
				t.Fatalf("data=%q err=%v", data, err)
			}
			if logins != 1 || requests != 2 || redirects != 0 {
				t.Fatalf("logins=%d requests=%d followed redirects=%d", logins, requests, redirects)
			}
		})
	}
}
