package coreserver

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSecurityAndConfig(t *testing.T) {
	s, e := New(t.TempDir(), t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	h := s.Handler()
	request := func(method, path, payload, host, origin string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, "http://127.0.0.1:17823"+path, strings.NewReader(payload))
		if host != "" {
			r.Host = host
		}
		if origin != "" {
			r.Header.Set("Origin", origin)
		}
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	if w := request("GET", "/api/health", "", "evil.example", ""); w.Code != 403 {
		t.Fatal("DNS rebinding allowed")
	}
	if w := request("POST", "/api/l2d/launch", "{}", "", "https://evil.example"); w.Code != 403 {
		t.Fatal("cross origin allowed")
	}
	payload := `{"imageBaseUrl":"https://api.openai.com/v1","imageModel":"test","imageApiKey":"super-secret","viewerExecutable":"","exapiUrl":"ws://127.0.0.1:10086/api"}`
	if w := request("PUT", "/api/config", payload, "", ""); w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	w := request("GET", "/api/config", "", "", "")
	if strings.Contains(w.Body.String(), "super-secret") {
		t.Fatal("key leaked")
	}
	var cfg map[string]any
	json.Unmarshal(w.Body.Bytes(), &cfg)
	if cfg["hasImageKey"] != true {
		t.Fatal("key not saved")
	}
	s2, e := New(s.root, s.web)
	if e != nil || s2.snapshot().ImageAPIKey != "super-secret" {
		t.Fatal("not persisted")
	}
	if w := request("POST", "/api/oc/split", `{"assetId":"../x","parts":[]}`, "", ""); w.Code != 400 {
		t.Fatal("bad split accepted")
	}
	if w := request("PUT", "/api/config", payload+"{}", "", ""); w.Code != 400 {
		t.Fatal("trailing JSON accepted")
	}
	if w := request("GET", "/api/assets", "", "", ""); w.Code != 200 || strings.TrimSpace(w.Body.String()) != "[]" {
		t.Fatal(w.Body.String())
	}
}
