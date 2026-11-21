package l2d

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func fakeCubism(t *testing.T, handle func(cubismEnvelope) (string, any)) (string, func()) {
	t.Helper()
	var mu sync.Mutex
	var conns []*websocket.Conn
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
		if err != nil {
			return
		}
		mu.Lock()
		conns = append(conns, conn)
		mu.Unlock()
		defer conn.Close()
		for {
			var req cubismEnvelope
			if conn.ReadJSON(&req) != nil {
				return
			}
			if req.Version != "1.0.0" || req.Type != "Request" || req.RequestId == "" {
				t.Error("invalid request envelope")
			}
			typ, data := handle(req)
			if typ == "disconnect" {
				return
			}
			if typ == "silent" {
				continue
			}
			raw, _ := json.Marshal(data)
			// Unsolicited and incorrectly correlated messages must never complete a call.
			_ = conn.WriteJSON(cubismEnvelope{RequestId: req.RequestId, Method: req.Method, Type: "Event", Data: json.RawMessage(`{"wrong":true}`)})
			_ = conn.WriteJSON(cubismEnvelope{RequestId: "wrong", Method: req.Method, Type: "Response", Data: json.RawMessage(`{"wrong":true}`)})
			_ = conn.WriteJSON(cubismEnvelope{RequestId: req.RequestId, Method: "wrong", Type: "Response", Data: json.RawMessage(`{"wrong":true}`)})
			if conn.WriteJSON(cubismEnvelope{RequestId: req.RequestId, Method: req.Method, Type: typ, Data: raw}) != nil {
				return
			}
		}
	}))
	return "ws" + strings.TrimPrefix(server.URL, "http"), func() {
		mu.Lock()
		for _, c := range conns {
			_ = c.Close()
		}
		mu.Unlock()
		server.Close()
	}
}

func TestCubismBridgeRegistrationApprovalPersistence(t *testing.T) {
	var mu sync.Mutex
	registers, checks, calls := 0, 0, 0
	approved := false
	endpoint, stop := fakeCubism(t, func(req cubismEnvelope) (string, any) {
		mu.Lock()
		defer mu.Unlock()
		switch req.Method {
		case "RegisterPlugin":
			registers++
			var data map[string]string
			_ = json.Unmarshal(req.Data, &data)
			if data["Name"] != "人人皆可建模" || data["Token"] != "old-secret" {
				t.Error("registration payload mismatch")
			}
			return "Response", map[string]any{"Token": "new-secret"}
		case "GetIsApproval":
			checks++
			return "Response", map[string]any{"Result": approved}
		default:
			calls++
			return "Response", map[string]any{"ModelUID": "model-1"}
		}
	})
	defer stop()
	b := NewCubismBridge(endpoint, "old-secret")
	defer b.Close()
	status := b.Status(context.Background())
	if !status.Connected || status.Approved || b.Token() != "new-secret" {
		t.Fatalf("status: %+v", status)
	}
	encoded, _ := json.Marshal(status)
	if strings.Contains(string(encoded), "secret") {
		t.Fatal("status leaked token")
	}
	if _, err := b.Call(context.Background(), "GetCurrentModelUID", nil); err == nil {
		t.Fatal("denied operation accepted")
	}
	mu.Lock()
	approved = true
	mu.Unlock()
	if s := b.Status(context.Background()); !s.Approved {
		t.Fatalf("approval not rechecked: %+v", s)
	}
	raw, err := b.Call(context.Background(), "GetCurrentModelUID", json.RawMessage(`{}`))
	if err != nil || string(raw) != `{"ModelUID":"model-1"}` {
		t.Fatalf("response correlation failed: %s %v", raw, err)
	}
	mu.Lock()
	defer mu.Unlock()
	if registers != 1 || checks != 4 || calls != 1 {
		t.Fatalf("unexpected session counts %d %d %d", registers, checks, calls)
	}
}

func TestCubismBridgeReconnectAndErrors(t *testing.T) {
	var registrations atomic.Int32
	endpoint, stop := fakeCubism(t, func(req cubismEnvelope) (string, any) {
		switch req.Method {
		case "RegisterPlugin":
			registrations.Add(1)
			var d map[string]string
			_ = json.Unmarshal(req.Data, &d)
			if registrations.Load() > 1 && d["Token"] != "private-token" {
				t.Error("reconnect did not reuse token")
			}
			return "Response", map[string]any{"Token": "private-token"}
		case "GetIsApproval":
			return "Response", map[string]any{"Result": true}
		case "GetDocuments":
			return "disconnect", nil
		case "GetCurrentModelUID":
			return "Error", map[string]any{"ErrorType": "InvalidModel", "Token": "private-token"}
		default:
			return "Error", map[string]any{"ErrorType": "private-token"}
		}
	})
	defer stop()
	b := NewCubismBridge(endpoint, "")
	defer b.Close()
	if _, err := b.Call(context.Background(), "GetDocuments", nil); err == nil {
		t.Fatal("disconnect not reported")
	}
	if _, err := b.Call(context.Background(), "GetCurrentModelUID", nil); err == nil || err.Error() != "Cubism API error: InvalidModel" {
		t.Fatalf("error: %v", err)
	}
	if _, err := b.Call(context.Background(), "GetParameters", json.RawMessage(`{"DocumentUID":"doc"}`)); err == nil || strings.Contains(err.Error(), "private-token") {
		t.Fatalf("unsafe error: %v", err)
	}
	if registrations.Load() != 2 {
		t.Fatalf("registrations %d", registrations.Load())
	}
}

func TestCubismBridgeValidation(t *testing.T) {
	invalid := []struct{ method, data string }{
		{"ImportPSD", `{}`}, {"RegisterPlugin", `{}`}, {"GetDocuments", `{"Token":"secret"}`},
		{"GetDocuments", `null`}, {"GetDocuments", `[]`}, {"GetParameters", `{}`},
		{"GetParameterValues", `{"ModelUID":null}`}, {"ClearParameterValues", `{"ModelUID":""}`},
		{"GetParameterValues", `{"ModelUID":"m","Ids":[1]}`}, {"GetParameterValues", `{"ModelUID":"m","Ids":null}`},
		{"SetParameterValues", `{"ModelUID":"m"}`}, {"SetParameterValues", `{"ModelUID":"m","Parameters":[]}`},
		{"SetParameterValues", `{"ModelUID":"m","Parameters":[{"Id":"P","Value":null}]}`},
		{"SetParameterValues", `{"ModelUID":"m","Parameters":[{"Id":"P","Value":"1"}]}`},
		{"SetParameterValues", `{"ModelUID":"m","Parameters":[{"Id":"P","Value":1e309}]}`},
		{"SetParameterValues", `{"ModelUID":"m","Parameters":[{"Id":"P","Value":1000001}]}`},
		{"SetParameterValues", `{"ModelUID":"m","Parameters":[{"Id":"P","Value":1,"Other":0}]}`},
		{"SetParameterValues", `{"ModelUID":"m","Parameters":[{"Id":"P","Value":1},{"Id":"P","Value":2}]}`},
	}
	for _, tc := range invalid {
		t.Run(tc.method+tc.data, func(t *testing.T) {
			if _, err := validateCubismData(tc.method, json.RawMessage(tc.data)); err == nil {
				t.Fatal("accepted invalid request")
			}
		})
	}
	valid := []struct{ method, data string }{
		{"GetDocuments", `{}`}, {"GetCurrentModelUID", `{}`}, {"GetParameters", `{"DocumentUID":"d"}`},
		{"GetParameters", `{"ModelUID":"m"}`}, {"GetParameterValues", `{"ModelUID":"m","Ids":["P"]}`},
		{"ClearParameterValues", `{"ModelUID":"m"}`}, {"SetParameterValues", `{"ModelUID":"m","Parameters":[{"Id":"P","Value":-30.5}]}`},
	}
	for _, tc := range valid {
		if _, err := validateCubismData(tc.method, json.RawMessage(tc.data)); err != nil {
			t.Errorf("valid %s: %v", tc.method, err)
		}
	}
}

func TestCubismBridgeCancellationAndClose(t *testing.T) {
	started := make(chan struct{}, 1)
	endpoint, stop := fakeCubism(t, func(req cubismEnvelope) (string, any) {
		if req.Method == "RegisterPlugin" {
			return "Response", map[string]any{"Token": "secret"}
		}
		started <- struct{}{}
		return "silent", nil
	})
	defer stop()
	b := NewCubismBridge(endpoint, "")
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan CubismBridgeStatus, 1)
	go func() { result <- b.Status(ctx) }()
	<-started
	waiting, cancelWaiting := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancelWaiting()
	begin := time.Now()
	s := b.Status(waiting)
	if s.Connected || time.Since(begin) > time.Second {
		t.Fatal("queued cancellation was not prompt")
	}
	cancel()
	select {
	case <-result:
	case <-time.After(time.Second):
		t.Fatal("I/O cancellation blocked")
	}
	_ = b.Close()
	if s := b.Status(context.Background()); s.Connected || !strings.Contains(s.Note, "closed") {
		t.Fatalf("closed status %+v", s)
	}
}

func TestCubismBridgeDefaultAndEndpointPrivacy(t *testing.T) {
	if b := NewCubismBridge("", ""); b.endpoint != DefaultCubismEndpoint {
		t.Fatal("default endpoint")
	}
	b := NewCubismBridge("ws://secret@localhost:1?token=secret", "secret")
	s := b.Status(context.Background())
	if s.Connected || strings.Contains(s.Note, "secret") {
		t.Fatalf("unsafe status %+v", s)
	}
}
