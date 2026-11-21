package coreserver

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"net/http/httptest"
	"testing"
)

func TestAssetsPSDWorkflow(t *testing.T) {
	s, err := New(t.TempDir(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	im := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	im.SetNRGBA(0, 0, color.NRGBA{R: 255, A: 128})
	var b bytes.Buffer
	png.Encode(&b, im)
	a, err := s.assets.Import(b.Bytes(), "眼睛")
	if err != nil {
		t.Fatal(err)
	}
	h := s.Handler()
	request := func(method, path, payload string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, "http://127.0.0.1:17823"+path, bytes.NewBufferString(payload))
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	w := request("POST", "/api/oc/export-psd", `{"width":4,"height":4,"layers":[{"assetId":"`+a.ID+`","name":"左眼睛","x":1,"y":1}]}`)
	if w.Code != 201 {
		t.Fatalf("export %d %s", w.Code, w.Body.String())
	}
	var out struct{ ID, URL string }
	json.Unmarshal(w.Body.Bytes(), &out)
	w = request("GET", out.URL, "")
	if w.Code != 200 || !bytes.HasPrefix(w.Body.Bytes(), []byte("8BPS")) {
		t.Fatalf("bad PSD download %d", w.Code)
	}
	w = request("POST", "/api/l2d/cubism/open-psd", `{"exportId":"../../evil"}`)
	if w.Code != 400 {
		t.Fatal(w.Code)
	}
	w = request("DELETE", "/api/assets/"+a.ID, "")
	if w.Code != 200 {
		t.Fatalf("delete %d %s", w.Code, w.Body.String())
	}
	w = request("DELETE", "/api/assets/"+a.ID, "")
	if w.Code != 404 {
		t.Fatal(w.Code)
	}
	w = request("GET", out.URL, "")
	if w.Code != 200 {
		t.Fatal("deleting source removed saved export")
	}
	w = request("POST", "/api/l2d/cubism/connect", `{"endpoint":"ws://example.com:22033"}`)
	if w.Code != 400 {
		t.Fatal(w.Code)
	}
	w = request("POST", "/api/l2d/cubism/command", `{"method":"GetDocuments","data":{}}`)
	if w.Code != 400 {
		t.Fatal(w.Code)
	}
}
