package l2d

import (
	"context"
	"encoding/json"
	"github.com/gorilla/websocket"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInspect(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "test.model3.json")
	os.WriteFile(p, []byte(`{"Version":3,"FileReferences":{"Moc":"test.moc3","Textures":["texture.png"]}}`), 0600)
	out, e := Inspect(p)
	if e != nil || out.Ready || len(out.Missing) != 2 {
		t.Fatalf("%+v %v", out, e)
	}
	os.WriteFile(filepath.Join(dir, "test.moc3"), []byte("placeholder-not-compiled-model"), 0600)
	os.WriteFile(filepath.Join(dir, "texture.png"), []byte("placeholder"), 0600)
	out, e = Inspect(p)
	if e != nil || !out.Ready {
		t.Fatalf("%+v %v", out, e)
	}
	os.WriteFile(p, []byte(`{"Version":3,"FileReferences":{"Moc":"../outside.moc3","Textures":["texture.png"]}}`), 0600)
	if _, e = Inspect(p); e == nil {
		t.Fatal("accepted traversal")
	}
	if _, e = Inspect(filepath.Join(dir, "fake.png")); e == nil {
		t.Fatal("accepted non-model")
	}
}
func TestSendOnlyCommand(t *testing.T) {
	up := websocket.Upgrader{}
	received := make(chan string, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, e := up.Upgrade(w, r, nil)
		if e != nil {
			return
		}
		defer c.Close()
		_, b, e := c.ReadMessage()
		if e == nil {
			received <- string(b)
		}
	}))
	defer srv.Close()
	msg := json.RawMessage(`{"msg":13200,"msgId":3,"data":{"id":0,"type":0,"mtn":"Tap:Wave"}}`)
	b, e := Command(context.Background(), "ws"+strings.TrimPrefix(srv.URL, "http")+"/api", msg)
	if e != nil || !strings.Contains(string(b), `"verified":false`) {
		t.Fatalf("%s %v", b, e)
	}
	if got := <-received; got != string(msg) {
		t.Fatal("wrong message")
	}
}
func TestEndpoint(t *testing.T) {
	for _, s := range []string{DefaultExAPI, "ws://[::1]:10086/api"} {
		if e := ValidateURL(s); e != nil {
			t.Fatal(e)
		}
	}
	for _, s := range []string{"ws://example.com:10086/api", "http://127.0.0.1:10086/api", "ws://127.0.0.1:10086/other", "ws://user@127.0.0.1:10086/api"} {
		if ValidateURL(s) == nil {
			t.Fatal("accepted " + s)
		}
	}
}
func TestCommand(t *testing.T) {
	up := websocket.Upgrader{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	defer srv.Close()
	msg := json.RawMessage(`{"msg":10000,"msgId":1}`)
	b, e := Command(context.Background(), "ws"+strings.TrimPrefix(srv.URL, "http")+"/api", msg)
	if e != nil || string(b) != string(msg) {
		t.Fatalf("%s %v", b, e)
	}
}
