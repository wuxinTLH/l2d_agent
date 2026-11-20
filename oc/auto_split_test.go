package oc

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func autoSplitPNG(t *testing.T, kind string) []byte {
	t.Helper()
	im := image.NewNRGBA(image.Rect(0, 0, 32, 32))
	for y := 0; y < 32; y++ {
		for x := 0; x < 32; x++ {
			switch kind {
			case "white", "outline":
				im.SetNRGBA(x, y, color.NRGBA{R: 255, G: 255, B: 255, A: 255})
			case "near-white":
				im.SetNRGBA(x, y, color.NRGBA{R: 250, G: 250, B: 250, A: 255})
			case "faint":
				im.SetNRGBA(x, y, color.NRGBA{R: 255, G: 255, B: 255, A: 1})
			}
		}
	}
	if kind == "outline" {
		for x := 8; x < 24; x++ {
			im.SetNRGBA(x, 8, color.NRGBA{R: 30, G: 30, B: 30, A: 255})
		}
	}
	var b bytes.Buffer
	if err := png.Encode(&b, im); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func TestAutoSplitSequentialSourceNamesAndManifest(t *testing.T) {
	s := New(t.TempDir())
	source, err := s.Import(fixture(t), "original")
	if err != nil {
		t.Fatal(err)
	}
	sourcePath, _ := s.Path(source.ID)
	original, _ := os.ReadFile(sourcePath)
	names := []string{"white hair", "left arm", "../right arm"}
	calls := 0
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		index := calls
		calls++
		if r.URL.Path != "/v1/images/edits" || r.Header.Get("Authorization") != "Bearer test-only" {
			t.Error("wrong provider endpoint or auth")
		}
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Error(err)
			return
		}
		defer r.MultipartForm.RemoveAll()
		if r.FormValue("n") != "1" || r.FormValue("model") != "authorized-model" {
			t.Error("model or n missing")
		}
		prompt := r.FormValue("prompt")
		if !strings.Contains(prompt, names[index]) || !strings.Contains(prompt, "occluded") || !strings.Contains(prompt, "transparent") || !strings.Contains(prompt, "NO grid") {
			t.Error("incomplete extraction prompt")
		}
		f, _, err := r.FormFile("image")
		if err != nil {
			t.Error(err)
			return
		}
		data, _ := io.ReadAll(f)
		f.Close()
		if !bytes.Equal(data, original) {
			t.Error("request is not the complete original")
		}
		if index == 1 {
			http.Error(w, "sensitive provider response", 422)
			return
		}
		fmt.Fprintf(w, `{"data":[{"b64_json":%q}]}`, base64.StdEncoding.EncodeToString(autoSplitPNG(t, "outline")))
	}))
	defer provider.Close()
	out, err := s.AutoSplit(context.Background(), Provider{BaseURL: provider.URL + "/v1", Model: "authorized-model", Key: "test-only"}, source.ID, names)
	if err != nil || calls != 3 || len(out.Parts) != 2 || len(out.Failures) != 1 {
		t.Fatalf("unexpected split: %+v %v calls=%d", out, err, calls)
	}
	if out.Failures[0].Name != names[1] || strings.Contains(out.Failures[0].Error, "sensitive") {
		t.Fatal("failure name or error redaction")
	}
	for i, a := range out.Parts {
		want := names[i*2]
		data, err := os.ReadFile(filepath.Join(s.Root, a.ID+".json"))
		var stored Asset
		if err != nil || json.Unmarshal(data, &stored) != nil || stored.Name != want || a.Name != want {
			t.Fatal("part name not safely persisted")
		}
		path, _ := s.Path(a.ID)
		data, _ = os.ReadFile(path)
		im, _ := decode(data)
		r, g, b, alpha := im.At(0, 0).RGBA()
		if r != 65535 || g != 65535 || b != 65535 || alpha != 65535 {
			t.Fatal("white hair pixels altered")
		}
	}
	assets, _ := s.List()
	if len(assets) != 3 {
		t.Fatalf("duplicate generated assets: %d", len(assets))
	}
	manifestData, err := os.ReadFile(filepath.Join(s.Root, out.ID+".manifest.json"))
	var manifest struct {
		Source    string         `json:"sourceAssetId"`
		Canvas    map[string]int `json:"canvas"`
		Alignment string         `json:"alignment"`
		AutoSplitResult
	}
	if err != nil || json.Unmarshal(manifestData, &manifest) != nil || manifest.Source != source.ID || manifest.Canvas["width"] != 8 || manifest.Canvas["height"] != 6 || len(manifest.Parts) != 2 || len(manifest.Failures) != 1 || !strings.Contains(manifest.Alignment, "unknown") || manifest.ManifestURL != out.ManifestURL {
		t.Fatalf("invalid provenance manifest: %s %v", manifestData, err)
	}
}

func TestAutoSplitRejectsInvalidBeforeRequests(t *testing.T) {
	calls := 0
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls++; w.WriteHeader(500) }))
	defer provider.Close()
	s := New(t.TempDir())
	a, _ := s.Import(fixture(t), "source")
	p := Provider{BaseURL: provider.URL, Model: "test"}
	for _, names := range [][]string{{" "}, {"arm", " ARM "}, {strings.Repeat("界", 81)}, {"line\nbreak"}, make([]string, 13)} {
		if _, err := s.AutoSplit(context.Background(), p, a.ID, names); err == nil {
			t.Fatalf("accepted invalid names: %q", names)
		}
	}
	for _, id := range []string{"../secret", strings.Repeat("a", 24)} {
		if _, err := s.AutoSplit(context.Background(), p, id, []string{"head"}); err == nil {
			t.Fatal("accepted invalid/missing source")
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := s.AutoSplit(ctx, p, a.ID, nil); !errors.Is(err, context.Canceled) {
		t.Fatalf("lost cancellation: %v", err)
	}
	if calls != 0 {
		t.Fatalf("invalid input made %d paid requests", calls)
	}
	names, err := autoSplitNames(nil)
	if err != nil || len(names) != 10 {
		t.Fatal("missing defaults")
	}
}

func TestAutoSplitRejectsBlankAndCleansOnlyGenerated(t *testing.T) {
	for _, kind := range []string{"transparent", "white", "near-white", "faint"} {
		t.Run(kind, func(t *testing.T) {
			s := New(t.TempDir())
			a, _ := s.Import(fixture(t), "source")
			path, _ := s.Path(a.ID)
			before, _ := os.ReadFile(path)
			calls := 0
			provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				fmt.Fprintf(w, `{"data":[{"b64_json":%q}]}`, base64.StdEncoding.EncodeToString(autoSplitPNG(t, kind)))
			}))
			defer provider.Close()
			out, err := s.AutoSplit(context.Background(), Provider{BaseURL: provider.URL, Model: "test"}, a.ID, []string{"head", "hair"})
			if err != nil || calls != 2 || len(out.Parts) != 0 || len(out.Failures) != 2 {
				t.Fatalf("accepted blank or stopped: %+v %v", out, err)
			}
			after, _ := os.ReadFile(path)
			list, _ := s.List()
			entries, _ := os.ReadDir(s.Root)
			if !bytes.Equal(before, after) || len(list) != 1 || len(entries) != 3 {
				t.Fatal("cleanup changed source or retained rejected output")
			}
		})
	}
}

func TestAutoSplitCancellationStopsRemaining(t *testing.T) {
	s := New(t.TempDir())
	a, _ := s.Import(fixture(t), "source")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	calls := 0
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		cancel()
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer provider.Close()
	out, err := s.AutoSplit(ctx, Provider{BaseURL: provider.URL, Model: "test"}, a.ID, []string{"head", "hair"})
	if !errors.Is(err, context.Canceled) || calls != 1 || len(out.Failures) != 1 || out.ManifestURL == "" {
		t.Fatalf("cancellation not propagated: %+v %v calls=%d", out, err, calls)
	}
}
