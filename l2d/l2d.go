// Package l2d inspects existing Cubism exports and controls a local viewer.
// It never claims to compile drawings to moc3 or author Cubism deformations.
package l2d

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gorilla/websocket"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const DefaultExAPI = "ws://127.0.0.1:10086/api"

type Inspection struct {
	ModelPath  string          `json:"modelPath"`
	Ready      bool            `json:"ready"`
	References []string        `json:"references"`
	Missing    []string        `json:"missing"`
	Parameters json.RawMessage `json:"parameters,omitempty"`
	Note       string          `json:"note"`
}

func Inspect(path string) (Inspection, error) {
	abs, e := filepath.Abs(path)
	if e != nil {
		return Inspection{}, e
	}
	if !strings.HasSuffix(strings.ToLower(abs), ".model3.json") {
		return Inspection{}, errors.New("select an existing Cubism .model3.json export")
	}
	f, e := os.Open(abs)
	if e != nil {
		return Inspection{}, e
	}
	defer f.Close()
	info, e := f.Stat()
	if e != nil || info.Size() > 4<<20 {
		return Inspection{}, errors.New("model configuration too large")
	}
	var model struct {
		Version        int
		FileReferences struct {
			Moc         string
			Textures    []string
			Physics     string
			Pose        string
			UserData    string
			DisplayInfo string
			Expressions []struct{ File string }
			Motions     map[string][]struct {
				File  string
				Sound string
			}
		}
		Groups json.RawMessage
	}
	if e = json.NewDecoder(f).Decode(&model); e != nil {
		return Inspection{}, errors.New("invalid model JSON")
	}
	if model.Version != 3 || model.FileReferences.Moc == "" || len(model.FileReferences.Textures) == 0 {
		return Inspection{}, errors.New("model3 must have Version 3, Moc and Textures")
	}
	refs := []string{model.FileReferences.Moc}
	refs = append(refs, model.FileReferences.Textures...)
	for _, p := range []string{model.FileReferences.Physics, model.FileReferences.Pose, model.FileReferences.UserData, model.FileReferences.DisplayInfo} {
		if p != "" {
			refs = append(refs, p)
		}
	}
	for _, v := range model.FileReferences.Expressions {
		refs = append(refs, v.File)
	}
	for _, vs := range model.FileReferences.Motions {
		for _, v := range vs {
			refs = append(refs, v.File)
			if v.Sound != "" {
				refs = append(refs, v.Sound)
			}
		}
	}
	out := Inspection{ModelPath: abs, Ready: true, References: refs, Missing: []string{}, Parameters: model.Groups, Note: "File integrity check only. Existing moc3 must be exported by Cubism; rig, gestures, visemes and physics require authoring. Groups are bindings, not a full parameter catalog."}
	root := filepath.Dir(abs)
	realRoot, e := filepath.EvalSymlinks(root)
	if e != nil {
		return Inspection{}, e
	}
	for _, r := range refs {
		clean := filepath.Clean(filepath.FromSlash(r))
		if filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) || strings.Contains(clean, ":") {
			return Inspection{}, fmt.Errorf("unsafe model reference: %s", r)
		}
		target := filepath.Join(root, clean)
		real, e := filepath.EvalSymlinks(target)
		if e == nil {
			rel, err := filepath.Rel(realRoot, real)
			if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
				return Inspection{}, errors.New("model symlink escapes model directory")
			}
		}
		st, err := os.Stat(target)
		if e != nil || err != nil || st.IsDir() || st.Size() == 0 {
			out.Missing = append(out.Missing, r)
			out.Ready = false
		}
	}
	return out, nil
}
func ValidateURL(raw string) error {
	u, e := url.Parse(raw)
	if e != nil || u.Scheme != "ws" || u.User != nil || u.Path != "/api" || u.RawQuery != "" || u.Fragment != "" {
		return errors.New("ExAPI must be ws://127.0.0.1:PORT/api")
	}
	ip := net.ParseIP(u.Hostname())
	if ip == nil || !ip.IsLoopback() || u.Port() == "" {
		return errors.New("ExAPI must use a loopback IP and explicit port")
	}
	return nil
}
func Command(ctx context.Context, endpoint string, message json.RawMessage) (json.RawMessage, error) {
	if e := ValidateURL(endpoint); e != nil {
		return nil, e
	}
	var obj map[string]any
	if json.Unmarshal(message, &obj) != nil || obj == nil {
		return nil, errors.New("message must be a JSON object")
	}
	if len(message) > 64<<10 {
		return nil, errors.New("command too large")
	}
	msg, ok := obj["msg"].(float64)
	msgID, hasID := obj["msgId"].(float64)
	if !ok || !hasID || msg != float64(int64(msg)) || msgID != float64(int64(msgID)) {
		return nil, errors.New("ExAPI msg and msgId must be integers")
	}
	dial := websocket.Dialer{HandshakeTimeout: 4 * time.Second, Proxy: nil}
	conn, _, e := dial.DialContext(ctx, endpoint, nil)
	if e != nil {
		return nil, errors.New("cannot connect to local ExAPI; enable it in ViewerEX and verify its port")
	}
	defer conn.Close()
	conn.SetReadLimit(1 << 20)
	conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	if e = conn.WriteMessage(websocket.TextMessage, message); e != nil {
		return nil, errors.New("ExAPI write failed")
	}
	// Only event registration has an immediate documented echo. Mutations are
	// send-only: a successful socket write does NOT verify application state.
	if msg != 10000 {
		return json.RawMessage(`{"status":"sent","verified":false,"note":"Socket write completed; ViewerEX application state is unverified. No response expected."}`), nil
	}
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	_, b, e := conn.ReadMessage()
	if e != nil {
		return nil, errors.New("ExAPI returned no message within 5s; command may have been delivered, do not blindly retry")
	}
	var reply struct {
		Msg   int64 `json:"msg"`
		MsgID int64 `json:"msgId"`
	}
	if json.Unmarshal(b, &reply) != nil || reply.Msg != 10000 || reply.MsgID != int64(msgID) {
		return nil, errors.New("ExAPI probe response did not match the registration request")
	}
	return b, nil
}
func Launch(path string) error {
	if !filepath.IsAbs(path) || !strings.EqualFold(filepath.Ext(path), ".exe") {
		return errors.New("configure an absolute ViewerEX .exe path")
	}
	st, e := os.Stat(path)
	if e != nil || st.IsDir() {
		return errors.New("ViewerEX executable not found")
	}
	cmd := exec.Command(path)
	cmd.Dir = filepath.Dir(path)
	if e = cmd.Start(); e != nil {
		return errors.New("ViewerEX launch failed")
	}
	go cmd.Wait()
	return nil
}
