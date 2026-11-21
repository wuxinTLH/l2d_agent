package coreserver

import (
	"encoding/json"
	"testing"
)

func TestMattingDeviceConfig(t *testing.T) {
	s, err := New(t.TempDir(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, device := range []string{"cpu", "gpu", "invalid"} {
		payload := `{"imageBaseUrl":"https://api.openai.com/v1","imageModel":"test","mattingDevice":"` + device + `"}`
		w := workflowRequest(s, "PUT", "/api/config", payload)
		if device == "invalid" {
			if w.Code != 400 {
				t.Fatalf("invalid accepted %d", w.Code)
			}
			continue
		}
		if w.Code != 200 {
			t.Fatal(w.Body.String())
		}
		w = workflowRequest(s, "GET", "/api/config", "")
		var out map[string]any
		json.Unmarshal(w.Body.Bytes(), &out)
		if out["mattingDevice"] != device {
			t.Fatalf("device not saved %v", out)
		}
	}
}
