package l2d

import (
	"context"
	"github.com/gorilla/websocket"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDiscoverBounded(t *testing.T) {
	root := t.TempDir()
	exe := filepath.Join(root, "Steam", "steamapps", "common", "Live2DViewerEX", "Live2DViewerEX.exe")
	if err := os.MkdirAll(filepath.Dir(exe), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(exe, []byte("fixture-not-an-executable"), 0600); err != nil {
		t.Fatal(err)
	}
	found := discoverIn([]string{"", root, root})
	if len(found) != 1 || found[0] != exe {
		t.Fatalf("unexpected discovery: %v", found)
	}
}
func TestDiscoverProjectViewer(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	dir := filepath.Join(root, "L2dViewerEX")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(dir, "Live2DViewerEX.exe")
	if err := os.WriteFile(exe, []byte("fixture-not-an-executable"), 0600); err != nil {
		t.Fatal(err)
	}
	found := Discover()
	if len(found) == 0 || found[0] != exe {
		t.Fatalf("project path not preferred: %v", found)
	}
}

func TestProbeEchoAndMismatch(t *testing.T) {
	for _, mismatch := range []bool{false, true} {
		t.Run(map[bool]string{false: "echo", true: "mismatch"}[mismatch], func(t *testing.T) {
			up := websocket.Upgrader{}
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				c, err := up.Upgrade(w, r, nil)
				if err != nil {
					return
				}
				defer c.Close()
				k, b, err := c.ReadMessage()
				if err != nil {
					return
				}
				if mismatch {
					b = []byte(`{"msg":10000,"msgId":-1}`)
				}
				c.WriteMessage(k, b)
			}))
			defer srv.Close()
			err := Probe(context.Background(), "ws"+strings.TrimPrefix(srv.URL, "http")+"/api")
			if (err != nil) != mismatch {
				t.Fatalf("mismatch=%v err=%v", mismatch, err)
			}
		})
	}
	if Probe(context.Background(), "ws://external.example:10086/api") == nil {
		t.Fatal("accepted external endpoint")
	}
}
