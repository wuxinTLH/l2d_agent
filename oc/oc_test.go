package oc

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func fixture(t *testing.T) []byte {
	t.Helper()
	im := image.NewNRGBA(image.Rect(0, 0, 8, 6))
	im.Set(2, 1, color.NRGBA{R: 255, A: 255})
	var b bytes.Buffer
	if e := png.Encode(&b, im); e != nil {
		t.Fatal(e)
	}
	return b.Bytes()
}
func TestImportSplit(t *testing.T) {
	s := New(t.TempDir())
	a, e := s.Import(fixture(t), "test")
	if e != nil || a.Width != 8 || a.Height != 6 {
		t.Fatalf("import: %+v %v", a, e)
	}
	out, e := s.Split(a.ID, []Part{{Name: "eye", X: 2, Y: 1, Width: 2, Height: 2}})
	if e != nil || len(out.Parts) != 1 {
		t.Fatalf("split: %+v %v", out, e)
	}
	p, _ := s.Path(out.Parts[0].ID)
	b, _ := os.ReadFile(p)
	im, e := decode(b)
	if e != nil || im.Bounds().Dx() != 2 {
		t.Fatal("invalid crop")
	}
	r, _, _, alpha := im.At(0, 0).RGBA()
	if r != 65535 || alpha != 65535 {
		t.Fatal("crop location or alpha lost")
	}
	_, _, _, alpha = im.At(1, 1).RGBA()
	if alpha != 0 {
		t.Fatal("transparent pixel lost")
	}
	list, e := s.List()
	if e != nil || len(list) != 2 {
		t.Fatal("asset list missing")
	}
	for _, part := range []Part{{Name: "bad", X: -1, Width: 1, Height: 1}, {Name: "bad", Width: 9, Height: 1}, {Name: "bad", X: 8, Width: 1, Height: 1}, {Name: "", Width: 1, Height: 1}} {
		if _, e = s.Split(a.ID, []Part{part}); e == nil {
			t.Fatal("accepted invalid region")
		}
	}
	if _, e = s.Path("../secret"); e == nil {
		t.Fatal("accepted path traversal")
	}
	if _, e = s.Import([]byte("not image"), "bad"); e == nil {
		t.Fatal("accepted invalid image")
	}
}
func TestProvider(t *testing.T) {
	img := fixture(t)
	var calls int
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Header.Get("Authorization") != "Bearer private" {
			t.Error("missing authorization")
		}
		switch r.URL.Path {
		case "/v1/images/generations":
			if r.Header.Get("Content-Type") != "application/json" {
				t.Error("wrong content type")
			}
		case "/v1/images/edits":
			if e := r.ParseMultipartForm(1 << 20); e != nil {
				t.Error(e)
			}
			f, _, e := r.FormFile("image")
			if e != nil {
				t.Error(e)
			} else {
				f.Close()
			}
		default:
			t.Error("wrong endpoint")
		}
		fmt.Fprintf(w, `{"data":[{"b64_json":%q}]}`, base64.StdEncoding.EncodeToString(img))
	}))
	defer provider.Close()
	s := New(t.TempDir())
	p := Provider{BaseURL: provider.URL + "/v1", Model: "test", Key: "private"}
	if _, e := s.Generate(context.Background(), p, "character", "bad", "1024x1024"); e != nil {
		t.Fatal(e)
	}
	if _, e := s.Edit(context.Background(), p, img, "change hair"); e != nil {
		t.Fatal(e)
	}
	if calls != 2 {
		t.Fatal("wrong calls")
	}
	if _, e := s.Generate(context.Background(), p, "", "", "1024x1024"); e == nil {
		t.Fatal("accepted empty prompt")
	}
}
func TestProviderErrorsAreRedacted(t *testing.T) {
	p := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Error(w, "secret-key-sensitive", 401) }))
	defer p.Close()
	s := New(t.TempDir())
	_, e := s.Generate(context.Background(), Provider{BaseURL: p.URL}, "oc", "", "auto")
	if e == nil || bytes.Contains([]byte(e.Error()), []byte("sensitive")) {
		t.Fatal("provider body leaked")
	}
}
