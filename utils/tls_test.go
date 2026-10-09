package utils

import (
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestArrayCertificateVerification(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) }))
	defer srv.Close()
	caFile := filepath.Join(t.TempDir(), "array-ca.pem")
	if err := os.WriteFile(caFile, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: srv.Certificate().Raw}), 0600); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name            string
		storage         Storage
		wrongHost, fail bool
	}{
		{"untrusted default", Storage{}, false, true},
		{"trusted CA", Storage{TLSCAFile: caFile}, false, false},
		{"wrong host", Storage{TLSCAFile: caFile}, true, true},
		{"explicit insecure", Storage{TLSInsecureSkipVerify: true}, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			config, err := ArrayTLSConfig(tc.storage)
			if err != nil {
				t.Fatal(err)
			}
			if tc.wrongHost {
				config.ServerName = "wrong.invalid"
			}
			transport := &http.Transport{TLSClientConfig: config}
			defer transport.CloseIdleConnections()
			c := &http.Client{Transport: transport}
			response, err := c.Get(srv.URL)
			if response != nil {
				response.Body.Close()
			}
			if (err != nil) != tc.fail {
				t.Fatalf("unexpected TLS result: %v", err)
			}
		})
	}
	if _, err := ArrayTLSConfig(Storage{TLSCAFile: "missing-ca-file"}); err == nil {
		t.Fatal("accepted missing CA")
	}
	if err := os.WriteFile(caFile, []byte("not a certificate"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := ArrayTLSConfig(Storage{TLSCAFile: caFile}); err == nil {
		t.Fatal("accepted invalid CA")
	}
}
