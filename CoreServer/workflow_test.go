package coreserver

import (
	"encoding/json"
	"github.com/gorilla/websocket"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWorkflowRoutes(t *testing.T) {
	s, err := New(t.TempDir(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	up := websocket.Upgrader{}
	peer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, e := up.Upgrade(w, r, nil)
		if e != nil {
			return
		}
		defer c.Close()
		k, b, e := c.ReadMessage()
		if e == nil {
			c.WriteMessage(k, b)
		}
	}))
	defer peer.Close()
	s.config.ExAPIURL = "ws" + strings.TrimPrefix(peer.URL, "http") + "/api"
	h := s.Handler()
	for _, path := range []string{"/api/l2d/probe", "/api/l2d/discover"} {
		r := httptest.NewRequest("POST", "http://127.0.0.1:17823"+path, strings.NewReader("{}"))
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != 200 {
			t.Fatal(w.Code, w.Body.String())
		}
		var out map[string]any
		if json.Unmarshal(w.Body.Bytes(), &out) != nil {
			t.Fatal("invalid JSON")
		}
		if path == "/api/l2d/probe" && out["connected"] != true {
			t.Fatal(out)
		}
		if path == "/api/l2d/discover" && out["candidates"] == nil {
			t.Fatal("candidates must be array")
		}
	}
}
