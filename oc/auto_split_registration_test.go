package oc

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func registrationPNG(t *testing.T, im image.Image) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := png.Encode(&b, im); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func TestAutoSplitAlphaValidation(t *testing.T) {
	for _, kind := range []string{"opaque", "checkerboard", "token-alpha", "translucent-background", "white-art"} {
		t.Run(kind, func(t *testing.T) {
			im := image.NewNRGBA(image.Rect(0, 0, 40, 40))
			for y := 0; y < 40; y++ {
				for x := 0; x < 40; x++ {
					c := color.NRGBA{R: 255, G: 255, B: 255, A: 255}
					switch kind {
					case "checkerboard":
						if (x/5+y/5)%2 == 0 {
							c.R = 160
							c.G = 160
							c.B = 160
						}
					case "translucent-background":
						c.A = 254
					case "white-art":
						if x < 10 || x >= 30 || y < 10 || y >= 30 {
							c.A = 0
						}
					}
					im.SetNRGBA(x, y, c)
				}
			}
			if kind == "token-alpha" {
				im.SetNRGBA(0, 0, color.NRGBA{})
			}
			err := autoSplitContent(im)
			if kind == "white-art" {
				if err != nil {
					t.Fatal(err)
				}
			} else if err == nil {
				t.Fatal("accepted fake background")
			}
		})
	}
}

func TestAutoSplitFitsOnlyWithExplicitSourceBounds(t *testing.T) {
	generated := image.NewNRGBA(image.Rect(0, 0, 80, 80))
	for y := 20; y < 40; y++ {
		for x := 10; x < 70; x++ {
			generated.SetNRGBA(x, y, color.NRGBA{R: 255, G: 255, B: 255, A: 255})
		}
	}
	payload := registrationPNG(t, generated)
	calls := 0
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Error(err)
			return
		}
		defer r.MultipartForm.RemoveAll()
		prompt := r.FormValue("prompt")
		if !strings.Contains(prompt, "Do not zoom") || !strings.Contains(prompt, "8 x 6") || strings.Contains(prompt, "if supported") {
			t.Error("missing strict scale/alpha contract")
		}
		fmt.Fprintf(w, `{"data":[{"b64_json":%q}]}`, base64.StdEncoding.EncodeToString(payload))
	}))
	defer provider.Close()
	s := New(t.TempDir())
	source, err := s.Import(fixture(t), "original")
	if err != nil {
		t.Fatal(err)
	}
	sourcePath, _ := s.Path(source.ID)
	before, _ := os.ReadFile(sourcePath)
	p := Provider{BaseURL: provider.URL, Model: "local-mock"}
	rejected, err := s.AutoSplit(context.Background(), p, source.ID, []string{"eye"})
	if err != nil || len(rejected.Parts) != 1 || len(rejected.Failures) != 0 || !strings.Contains(rejected.Parts[0].ProcessingWarning, "canvas") {
		t.Fatalf("unexpected rejection: %+v %v", rejected, err)
	}
	rect := AutoSplitBounds{X: 2, Y: 1, Width: 4, Height: 4}
	out, err := s.AutoSplitWithBounds(context.Background(), p, source.ID, []string{"eye"}, map[string]AutoSplitBounds{"eye": rect})
	if err != nil || len(out.Parts) != 1 || len(out.Failures) != 0 {
		t.Fatalf("fit failed: %+v %v", out, err)
	}
	a := out.Parts[0]
	path, _ := s.Path(a.ID)
	data, _ := os.ReadFile(path)
	im, err := decode(data)
	if err != nil || im.Bounds() != image.Rect(0, 0, 4, 1) || a.Width != 4 || a.Height != 1 || a.SourcePlacement == nil || a.SourcePlacement.X != 2 || a.SourcePlacement.Y != 2 || a.SourcePlacement.CanvasWidth != 8 || a.SourcePlacement.CanvasHeight != 6 {
		t.Fatal("wrong full source canvas")
	}
	placement := out.Placements[a.ID]
	if placement.AlphaBounds != (AutoSplitBounds{X: 0, Y: 0, Width: 4, Height: 1}) || placement.SourceBounds == nil || *placement.SourceBounds != rect {
		t.Fatalf("bad placement: %+v", placement)
	}
	r, g, b, alpha := im.At(1, 0).RGBA()
	if r != 65535 || g != 65535 || b != 65535 || alpha != 65535 {
		t.Fatal("white artwork erased")
	}
	after, _ := os.ReadFile(sourcePath)
	if !bytes.Equal(before, after) {
		t.Fatal("source changed")
	}
	for _, bounds := range []map[string]AutoSplitBounds{
		{"unknown": rect}, {"eye": {X: -1, Width: 1, Height: 1}}, {"eye": {X: 7, Width: 2, Height: 1}}, {"eye": {Width: 0, Height: 1}}, {"eye": {X: 1, Width: int(^uint(0) >> 1), Height: 1}},
	} {
		if _, err := s.AutoSplitWithBounds(context.Background(), p, source.ID, []string{"eye"}, bounds); err == nil {
			t.Fatal("accepted invalid bounds")
		}
	}
	if calls != 2 {
		t.Fatalf("invalid bounds made requests: %d", calls)
	}
}

func TestAutoSplitOpaqueRetainedEvenWithBounds(t *testing.T) {
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `{"data":[{"b64_json":%q}]}`, base64.StdEncoding.EncodeToString(autoSplitPNG(t, "white")))
	}))
	defer provider.Close()
	s := New(t.TempDir())
	source, _ := s.Import(fixture(t), "source")
	out, err := s.AutoSplitWithBounds(context.Background(), Provider{BaseURL: provider.URL, Model: "mock"}, source.ID, []string{"hair"}, map[string]AutoSplitBounds{"hair": {X: 1, Y: 1, Width: 4, Height: 4}})
	if err != nil || len(out.Parts) != 1 || len(out.Failures) != 0 || out.Parts[0].ProcessingWarning == "" || out.Warnings[out.Parts[0].ID] == "" {
		t.Fatalf("opaque not retained with warning: %+v %v", out, err)
	}
	a := out.Parts[0]
	if a.Width != 32 || a.Height != 32 || a.SourcePlacement == nil || a.SourcePlacement.VerifiedScale || out.Placements[a.ID].SourceBounds != nil || out.Placements[a.ID].AlphaBounds != (AutoSplitBounds{}) {
		t.Fatalf("fabricated raw registration: %+v %+v", a, out.Placements[a.ID])
	}
	path, _ := s.Path(a.ID)
	data, _ := os.ReadFile(path)
	if !bytes.Equal(data, autoSplitPNG(t, "white")) {
		t.Fatal("raw provider PNG rewritten")
	}
	manifestData, err := os.ReadFile(filepath.Join(s.Root, out.ID+".manifest.json"))
	var manifest AutoSplitResult
	if err != nil || json.Unmarshal(manifestData, &manifest) != nil || manifest.Warnings[a.ID] != a.ProcessingWarning || len(manifest.Parts) != 1 || manifest.Parts[0].ProcessingWarning != a.ProcessingWarning {
		t.Fatalf("fallback warning missing from manifest: %s %v", manifestData, err)
	}
	assets, _ := s.List()
	if len(assets) != 2 {
		t.Fatal("raw output discarded")
	}
}
