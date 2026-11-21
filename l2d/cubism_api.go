package l2d

// Official protocol: https://docs.live2d.com/en/cubism-editor-manual/external-application-integration-api/
// Schemas: https://docs.live2d.com/en/cubism-editor-manual/external-application-integration-api-list/
import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const DefaultCubismEndpoint = "ws://127.0.0.1:22033"
const cubismRequestTimeout = 5 * time.Second
const cubismReadLimit = 2 << 20

// CubismBridgeStatus deliberately excludes the authentication token.
type CubismBridgeStatus struct {
	Connected bool   `json:"connected"`
	Approved  bool   `json:"approved"`
	Note      string `json:"note"`
}

// CubismBridge owns one persistent, serialized Editor session. Keep it alive
// across HTTP requests; call Close when replacing its endpoint or shutting down.
// Failed requests are never replayed: a mutation may already have reached Editor.
type CubismBridge struct {
	requestMu sync.Mutex
	stateMu   sync.Mutex
	endpoint  string
	token     string
	conn      *websocket.Conn
	closed    bool
	sequence  uint64
}

func NewCubismBridge(endpoint, token string) *CubismBridge {
	if endpoint == "" {
		endpoint = DefaultCubismEndpoint
	}
	return &CubismBridge{endpoint: endpoint, token: token}
}

// Token is secret; callers may persist it privately, never in API responses/logs.
func (b *CubismBridge) Token() string {
	b.stateMu.Lock()
	defer b.stateMu.Unlock()
	return b.token
}

// Close interrupts in-flight I/O and permanently closes this bridge.
func (b *CubismBridge) Close() error {
	b.stateMu.Lock()
	defer b.stateMu.Unlock()
	b.closed = true
	if b.conn != nil {
		_ = b.conn.Close()
		b.conn = nil
	}
	return nil
}

// acquire lets cancellation interrupt a request waiting behind another request.
func (b *CubismBridge) acquire(ctx context.Context) error {
	tick := time.NewTicker(5 * time.Millisecond)
	defer tick.Stop()
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		if b.requestMu.TryLock() {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-tick.C:
		}
	}
}

func (b *CubismBridge) Status(ctx context.Context) CubismBridgeStatus {
	ctx, cancel := context.WithTimeout(ctx, cubismRequestTimeout)
	defer cancel()
	if err := b.acquire(ctx); err != nil {
		return CubismBridgeStatus{Note: err.Error()}
	}
	defer b.requestMu.Unlock()
	if err := b.connect(ctx); err != nil {
		return CubismBridgeStatus{Note: err.Error()}
	}
	approved, err := b.approval(ctx)
	if err != nil {
		b.stateMu.Lock()
		connected := b.conn != nil
		b.stateMu.Unlock()
		return CubismBridgeStatus{Connected: connected, Note: err.Error()}
	}
	if !approved {
		return CubismBridgeStatus{Connected: true, Note: "Allow 人人皆可建模 in Cubism Editor external application integration settings."}
	}
	return CubismBridgeStatus{Connected: true, Approved: true, Note: "Cubism Editor external application integration is ready."}
}

// Call accepts only the documented subset and returns the response Data object.
// Approval is checked before every call, including mutations.
func (b *CubismBridge) Call(ctx context.Context, method string, data json.RawMessage) (json.RawMessage, error) {
	validated, err := validateCubismData(method, data)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, cubismRequestTimeout)
	defer cancel()
	if err := b.acquire(ctx); err != nil {
		return nil, err
	}
	defer b.requestMu.Unlock()
	if err := b.connect(ctx); err != nil {
		return nil, err
	}
	approved, err := b.approval(ctx)
	if err != nil {
		return nil, err
	}
	if !approved {
		return nil, errors.New("Cubism approval required: allow 人人皆可建模 in external application integration settings")
	}
	return b.exchange(ctx, method, validated)
}

func (b *CubismBridge) connect(ctx context.Context) error {
	b.stateMu.Lock()
	if b.closed {
		b.stateMu.Unlock()
		return errors.New("Cubism bridge is closed")
	}
	if b.conn != nil {
		b.stateMu.Unlock()
		return nil
	}
	token := b.token
	b.stateMu.Unlock()
	u, err := url.Parse(b.endpoint)
	if err != nil || u.Host == "" || (u.Scheme != "ws" && u.Scheme != "wss") || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return errors.New("invalid Cubism WebSocket endpoint")
	}
	dialer := websocket.Dialer{HandshakeTimeout: cubismRequestTimeout}
	conn, response, err := dialer.DialContext(ctx, b.endpoint, nil)
	if response != nil && response.Body != nil {
		_ = response.Body.Close()
	}
	if err != nil {
		return cubismTransportError(ctx)
	}
	conn.SetReadLimit(cubismReadLimit)
	b.stateMu.Lock()
	if b.closed {
		b.stateMu.Unlock()
		_ = conn.Close()
		return errors.New("Cubism bridge is closed")
	}
	b.conn = conn
	b.stateMu.Unlock()
	data := struct {
		Name  string
		Token string `json:",omitempty"`
	}{Name: "人人皆可建模", Token: token}
	raw, _ := json.Marshal(data)
	reply, err := b.exchange(ctx, "RegisterPlugin", raw)
	if err != nil {
		b.disconnect(conn)
		return err
	}
	var registered struct{ Token string }
	if json.Unmarshal(reply, &registered) != nil || registered.Token == "" || len(registered.Token) > 65536 {
		b.disconnect(conn)
		return errors.New("invalid Cubism registration response")
	}
	b.stateMu.Lock()
	b.token = registered.Token
	b.stateMu.Unlock()
	return nil
}

func (b *CubismBridge) approval(ctx context.Context) (bool, error) {
	raw, err := b.exchange(ctx, "GetIsApproval", json.RawMessage(`{}`))
	if err != nil {
		return false, err
	}
	var result struct{ Result *bool }
	if json.Unmarshal(raw, &result) != nil || result.Result == nil {
		return false, errors.New("invalid Cubism approval response")
	}
	return *result.Result, nil
}

func (b *CubismBridge) disconnect(conn *websocket.Conn) {
	_ = conn.Close()
	b.stateMu.Lock()
	if b.conn == conn {
		b.conn = nil
	}
	b.stateMu.Unlock()
}

func cubismTransportError(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	// Never expose raw server close reasons, endpoint credentials, or echoed tokens.
	return errors.New("Cubism WebSocket communication failed; verify Editor external integration is enabled")
}

type cubismEnvelope struct {
	Version   string
	RequestId string
	Type      string
	Method    string
	Data      json.RawMessage
}

func (b *CubismBridge) exchange(ctx context.Context, method string, data json.RawMessage) (json.RawMessage, error) {
	b.stateMu.Lock()
	conn := b.conn
	b.stateMu.Unlock()
	if conn == nil {
		return nil, errors.New("Cubism session is disconnected")
	}
	deadline := time.Now().Add(cubismRequestTimeout)
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	_ = conn.SetWriteDeadline(deadline)
	_ = conn.SetReadDeadline(deadline)
	done := make(chan struct{})
	stop := context.AfterFunc(ctx, func() { _ = conn.Close(); close(done) })
	defer func() {
		if !stop() {
			<-done
		}
	}()
	fail := func() (json.RawMessage, error) { b.disconnect(conn); return nil, cubismTransportError(ctx) }
	b.sequence++
	id := strconv.FormatUint(b.sequence, 10)
	if err := conn.WriteJSON(cubismEnvelope{Version: "1.0.0", RequestId: id, Type: "Request", Method: method, Data: data}); err != nil {
		return fail()
	}
	for {
		var message cubismEnvelope
		if err := conn.ReadJSON(&message); err != nil {
			return fail()
		}
		if message.RequestId != id || message.Method != method || (message.Type != "Response" && message.Type != "Error") {
			continue
		}
		if message.Type == "Error" {
			var result struct{ ErrorType string }
			_ = json.Unmarshal(message.Data, &result)
			// Only fixed official error names may be exposed, never untrusted payloads.
			switch result.ErrorType {
			case "InvalidJson", "UnsupportedVersion", "MethodNotFound", "InvalidType", "InvalidData", "PluginNotRegistered", "InvalidParameter", "InvalidModel", "InvalidDocument", "InvalidView":
				return nil, fmt.Errorf("Cubism API error: %s", result.ErrorType)
			default:
				return nil, errors.New("Cubism API returned an error")
			}
		}
		if len(message.Data) == 0 {
			return json.RawMessage(`{}`), nil
		}
		return message.Data, nil
	}
}

// Limits below are bridge safety limits, not claimed Editor/model parameter ranges.
// Actual model-specific ranges remain enforced by Editor.
func validateCubismData(method string, data json.RawMessage) (json.RawMessage, error) {
	allowed := map[string]bool{"GetDocuments": true, "GetCurrentModelUID": true, "GetParameters": true, "GetParameterValues": true, "SetParameterValues": true, "ClearParameterValues": true}
	if !allowed[method] {
		return nil, errors.New("Cubism method is not allowed")
	}
	if len(bytes.TrimSpace(data)) == 0 {
		data = json.RawMessage(`{}`)
	}
	if len(data) > 256<<10 {
		return nil, errors.New("Cubism request data is too large")
	}
	var object map[string]json.RawMessage
	if json.Unmarshal(data, &object) != nil || object == nil {
		return nil, errors.New("Cubism data must be a JSON object")
	}
	fields := map[string]bool{}
	switch method {
	case "GetParameters":
		fields["ModelUID"] = true
		fields["DocumentUID"] = true
	case "GetParameterValues":
		fields["ModelUID"] = true
		fields["Ids"] = true
	case "SetParameterValues":
		fields["ModelUID"] = true
		fields["Parameters"] = true
	case "ClearParameterValues":
		fields["ModelUID"] = true
	}
	for key := range object {
		if !fields[key] {
			return nil, errors.New("unexpected Cubism request field")
		}
	}
	validString := func(raw json.RawMessage) bool {
		var s string
		return json.Unmarshal(raw, &s) == nil && strings.TrimSpace(s) != "" && len(s) <= 1024 && !strings.ContainsAny(s, "\x00\r\n")
	}
	for _, key := range []string{"ModelUID", "DocumentUID"} {
		if raw, ok := object[key]; ok && !validString(raw) {
			return nil, errors.New("invalid Cubism UID")
		}
	}
	if method == "GetParameters" {
		if object["ModelUID"] == nil && object["DocumentUID"] == nil {
			return nil, errors.New("ModelUID or DocumentUID is required")
		}
	}
	if method == "GetParameterValues" || method == "SetParameterValues" || method == "ClearParameterValues" {
		if object["ModelUID"] == nil {
			return nil, errors.New("ModelUID is required")
		}
	}
	if raw, ok := object["Ids"]; ok {
		var ids []json.RawMessage
		if json.Unmarshal(raw, &ids) != nil || ids == nil || len(ids) > 1024 {
			return nil, errors.New("invalid Cubism parameter IDs")
		}
		for _, id := range ids {
			if !validString(id) {
				return nil, errors.New("invalid Cubism parameter ID")
			}
		}
	}
	if method == "SetParameterValues" {
		var parameters []map[string]json.RawMessage
		if json.Unmarshal(object["Parameters"], &parameters) != nil || len(parameters) == 0 || len(parameters) > 1024 {
			return nil, errors.New("Parameters must contain 1 to 1024 values")
		}
		seen := map[string]bool{}
		for _, p := range parameters {
			if len(p) != 2 || !validString(p["Id"]) {
				return nil, errors.New("invalid Cubism parameter shape")
			}
			var id string
			_ = json.Unmarshal(p["Id"], &id)
			var value float64
			raw := p["Value"]
			if raw == nil || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) || json.Unmarshal(raw, &value) != nil || math.IsNaN(value) || math.IsInf(value, 0) || math.Abs(value) > 1e6 || seen[id] {
				return nil, errors.New("invalid Cubism parameter value")
			}
			seen[id] = true
		}
	}
	// Re-encode the validated object; no arbitrary protocol envelope can pass through.
	raw, _ := json.Marshal(object)
	return raw, nil
}
