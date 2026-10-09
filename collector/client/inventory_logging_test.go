package client

import (
	"bytes"
	"fmt"
	"github.com/go-kit/log"
	"net/http"
	"net/http/httptest"
	"powerstore-metrics-exporter/utils"
	"strings"
	"testing"
)

func TestInvalidInventoryLogsObjectIDs(t *testing.T) {
	utils.InitReqCounter(1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/file_system" {
			fmt.Fprint(w, `[{"id":"fs-no-name","name":""},{"id":"fs-other-no-name"}]`)
			return
		}
		fmt.Fprint(w, `[]`)
	}))
	defer srv.Close()
	var output bytes.Buffer
	logger := log.NewLogfmtLogger(&output)
	c := &Client{IP: "synthetic-array", baseUrl: srv.URL + "/", http: srv.Client(), logger: logger}
	defer delete(PowerstoreModuleID, c.IP)
	c.InitModuleID(logger)
	for _, text := range []string{"level=error", "invalid inventory response", "ip=synthetic-array", "module=filesystem", "fs-no-name", "fs-other-no-name", "inventory name is empty"} {
		if !strings.Contains(output.String(), text) {
			t.Errorf("missing diagnostic %s", text)
		}
	}
	if ModuleIDs(c.IP)["filesystem"] != nil {
		t.Fatal("invalid inventory accepted")
	}
}
