package coreserver

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func workflowRequest(s *Server, method, path, payload string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, "http://127.0.0.1:17823"+path, strings.NewReader(payload))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	return w
}

func TestRemovedEndpointsNotFound(t *testing.T) {
	s, err := New(t.TempDir(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"status", "probe", "discover", "launch", "command"} {
		for _, method := range []string{"GET", "POST", "PUT", "DELETE"} {
			t.Run(method+"/"+path, func(t *testing.T) {
				w := workflowRequest(s, method, "/api/l2d/"+path, `{}`)
				if w.Code != 404 {
					t.Fatalf("removed endpoint returned %d: %s", w.Code, w.Body.String())
				}
			})
		}
	}
}

func TestCubismConfig(t *testing.T) {
	s, err := New(t.TempDir(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, value := range []*string{newString(filepath.Join(t.TempDir(), "CubismEditor5.exe")), nil, newString("")} {
		payload := map[string]any{"imageBaseUrl": "https://api.openai.com/v1", "imageModel": "test"}
		want := s.snapshot().CubismExecutable
		if value != nil {
			payload["cubismExecutable"] = *value
			want = *value
		}
		b, err := json.Marshal(payload)
		if err != nil {
			t.Fatal(err)
		}
		if w := workflowRequest(s, "PUT", "/api/config", string(b)); w.Code != 200 {
			t.Fatal(w.Code, w.Body.String())
		}
		w := workflowRequest(s, "GET", "/api/config", "")
		var out map[string]any
		if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &out) != nil || out["cubismExecutable"] != want {
			t.Fatalf("config read failed: %d %s", w.Code, w.Body.String())
		}
		if len(out) != 5 || out["mattingDevice"] != "gpu" {
			t.Fatalf("unexpected public config fields: %v", out)
		}
		reloaded, err := New(s.root, s.web)
		if err != nil || reloaded.snapshot().CubismExecutable != want {
			t.Fatalf("Cubism config not persisted: %v", err)
		}
	}
	for _, key := range []string{"unknownExecutable", "unknownUrl"} {
		payload := `{"imageBaseUrl":"https://api.openai.com/v1","imageModel":"test","` + key + `":"removed"}`
		if w := workflowRequest(s, "PUT", "/api/config", payload); w.Code != 400 {
			t.Fatalf("removed config field accepted: %s: %d", key, w.Code)
		}
	}
}

func newString(value string) *string { return &value }

func TestConfigSaveDropsRemovedFieldsAndPreservesSecret(t *testing.T) {
	root := t.TempDir()
	configPath := filepath.Join(root, "config.json")
	legacy := `{"imageBaseUrl":"https://api.openai.com/v1","imageModel":"test","imageApiKey":"saved-secret","cubismExecutable":"saved-editor","unknownExecutable":"removed","unknownUrl":"removed"}`
	if err := os.WriteFile(configPath, []byte(legacy), 0600); err != nil {
		t.Fatal(err)
	}
	s, err := New(root, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	w := workflowRequest(s, "PUT", "/api/config", `{"imageBaseUrl":"https://api.openai.com/v1","imageModel":"updated"}`)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	saved, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	var out map[string]any
	if err := json.Unmarshal(saved, &out); err != nil {
		t.Fatal(err)
	}
	if out["imageApiKey"] != "saved-secret" || out["cubismExecutable"] != "saved-editor" || out["imageModel"] != "updated" {
		t.Fatal("omitted settings not preserved or new settings not saved")
	}
	for _, key := range []string{"unknownExecutable", "unknownUrl"} {
		if _, exists := out[key]; exists {
			t.Fatalf("removed field still persisted: %s", key)
		}
	}
}

func TestCubismAndInspectRoutes(t *testing.T) {
	s, err := New(t.TempDir(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	w := workflowRequest(s, "POST", "/api/l2d/cubism/discover", `{}`)
	var discovery struct {
		Candidates []string `json:"candidates"`
	}
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &discovery) != nil || discovery.Candidates == nil {
		t.Fatalf("Cubism discovery failed: %d %s", w.Code, w.Body.String())
	}
	if w := workflowRequest(s, "POST", "/api/l2d/cubism/launch", `{}`); w.Code != 400 || !strings.Contains(w.Body.String(), "CubismEditor") {
		t.Fatalf("Cubism launch validation missing: %d %s", w.Code, w.Body.String())
	}
	model := filepath.Join(t.TempDir(), "test.model3.json")
	if err := os.WriteFile(model, []byte(`{"Version":3,"FileReferences":{"Moc":"test.moc3","Textures":["texture.png"]}}`), 0600); err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(map[string]string{"modelPath": model})
	if err != nil {
		t.Fatal(err)
	}
	w = workflowRequest(s, "POST", "/api/l2d/inspect", string(payload))
	var inspection struct {
		Ready   bool     `json:"ready"`
		Missing []string `json:"missing"`
	}
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &inspection) != nil || inspection.Ready || len(inspection.Missing) != 2 {
		t.Fatalf("inspect route failed: %d %s", w.Code, w.Body.String())
	}
}
