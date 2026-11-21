package l2d

import (
	"context"
	"strings"
	"sync"
	"testing"
)

func TestCubismBridgeReadLimit(t *testing.T) {
	endpoint, stop := fakeCubism(t, func(req cubismEnvelope) (string, any) {
		switch req.Method {
		case "RegisterPlugin":
			return "Response", map[string]any{"Token": "secret"}
		case "GetIsApproval":
			return "Response", map[string]any{"Result": true}
		default:
			return "Response", map[string]any{"oversized": strings.Repeat("x", cubismReadLimit)}
		}
	})
	defer stop()
	b := NewCubismBridge(endpoint, "")
	defer b.Close()
	if _, err := b.Call(context.Background(), "GetDocuments", nil); err == nil {
		t.Fatal("oversized response accepted")
	}
}

func TestCubismBridgeConcurrentCallsSerialized(t *testing.T) {
	endpoint, stop := fakeCubism(t, func(req cubismEnvelope) (string, any) {
		switch req.Method {
		case "RegisterPlugin":
			return "Response", map[string]any{"Token": "secret"}
		case "GetIsApproval":
			return "Response", map[string]any{"Result": true}
		default:
			return "Response", map[string]any{"ModelUID": "m"}
		}
	})
	defer stop()
	b := NewCubismBridge(endpoint, "")
	defer b.Close()
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := b.Call(context.Background(), "GetCurrentModelUID", nil); err != nil {
				t.Errorf("concurrent call: %v", err)
			}
		}()
	}
	wg.Wait()
}

func TestCubismBridgeMalformedApproval(t *testing.T) {
	endpoint, stop := fakeCubism(t, func(req cubismEnvelope) (string, any) {
		if req.Method == "RegisterPlugin" {
			return "Response", map[string]any{"Token": "secret"}
		}
		return "Response", map[string]any{"Result": "true", "Token": "secret"}
	})
	defer stop()
	b := NewCubismBridge(endpoint, "")
	defer b.Close()
	if status := b.Status(context.Background()); status.Approved || strings.Contains(status.Note, "secret") {
		t.Fatalf("invalid approval accepted: %+v", status)
	}
}
