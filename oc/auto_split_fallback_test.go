package oc

import (
	"bytes"
	"encoding/json"
	"errors"
	"image"
	"os"
	"path/filepath"
	"testing"
)

func TestAutoSplitRawFallbackPreservesOriginal(t *testing.T) {
	for _, mode := range []string{"missing-model", "poor-mask", "empty-mask", "nil-mask"} {
		t.Run(mode, func(t *testing.T) {
			s := New(t.TempDir())
			source, err := s.Import(fixture(t), "source")
			if err != nil {
				t.Fatal(err)
			}
			sourcePath, _ := s.Path(source.ID)
			sourceBefore, _ := os.ReadFile(sourcePath)
			a, err := s.Import(autoSplitPNG(t, "near-white"), "generated")
			if err != nil {
				t.Fatal(err)
			}
			path, _ := s.Path(a.ID)
			before, _ := os.ReadFile(path)
			a.SourcePlacement = &SourcePlacement{AssetID: source.ID}
			rect := AutoSplitBounds{X: 1, Y: 1, Width: 4, Height: 4}
			placement := AutoSplitPlacement{SourceBounds: &rect}
			calls := 0
			matte := func(input string, device ...string) (image.Image, error) {
				calls++
				if input != path || len(device) != 1 || device[0] != "cpu" {
					t.Fatal("incorrect local matting input")
				}
				switch mode {
				case "missing-model":
					return nil, errors.New("model unavailable")
				case "poor-mask":
					return decode(autoSplitPNG(t, "white"))
				case "empty-mask":
					return image.NewNRGBA(image.Rect(0, 0, 32, 32)), nil
				default:
					return nil, nil
				}
			}
			err = s.acceptAutoSplitPartWithMatte(&a, "hair", image.Rect(0, 0, 8, 6), &rect, &placement, matte, "cpu")
			if err != nil || calls != 1 {
				t.Fatalf("fallback failed: %v calls=%d", err, calls)
			}
			after, _ := os.ReadFile(path)
			sourceAfter, _ := os.ReadFile(sourcePath)
			if !bytes.Equal(before, after) || !bytes.Equal(sourceBefore, sourceAfter) {
				t.Fatal("fallback changed generated or source bytes")
			}
			if a.Width != 32 || a.Height != 32 || a.Name != "hair" || a.ProcessingWarning == "" {
				t.Fatalf("raw metadata lost: %+v", a)
			}
			if a.SourcePlacement == nil || a.SourcePlacement.AssetID != source.ID || a.SourcePlacement.VerifiedScale || a.SourcePlacement.X != 0 || a.SourcePlacement.Y != 0 || a.SourcePlacement.CanvasWidth != 8 || a.SourcePlacement.CanvasHeight != 6 {
				t.Fatalf("incorrect raw provenance: %+v", a.SourcePlacement)
			}
			if placement.SourceBounds != nil || placement.AlphaBounds != (AutoSplitBounds{}) {
				t.Fatalf("fabricated validated bounds: %+v", placement)
			}
			data, err := os.ReadFile(filepath.Join(s.Root, a.ID+".json"))
			var stored Asset
			if err != nil || json.Unmarshal(data, &stored) != nil || stored.ProcessingWarning != a.ProcessingWarning || stored.SourcePlacement.VerifiedScale {
				t.Fatalf("warning not persistent: %s %v", data, err)
			}
		})
	}
}

func TestAutoSplitLocalMatteSuccessStillRegisters(t *testing.T) {
	s := New(t.TempDir())
	a, err := s.Import(autoSplitPNG(t, "white"), "generated")
	if err != nil {
		t.Fatal(err)
	}
	rect := AutoSplitBounds{X: 1, Y: 1, Width: 4, Height: 4}
	placement := AutoSplitPlacement{SourceBounds: &rect}
	calls := 0
	err = s.acceptAutoSplitPartWithMatte(&a, "hair", image.Rect(0, 0, 8, 6), &rect, &placement, func(string, ...string) (image.Image, error) {
		calls++
		return decode(autoSplitPNG(t, "outline"))
	})
	if err != nil || calls != 1 || a.ProcessingWarning != "" || a.SourcePlacement == nil || !a.SourcePlacement.VerifiedScale || placement.AlphaBounds.Width == 0 {
		t.Fatalf("validated matting regressed: %+v %+v %v", a, placement, err)
	}
}

func TestAutoSplitValidAlphaSkipsLocalMatte(t *testing.T) {
	s := New(t.TempDir())
	a, err := s.Import(autoSplitPNG(t, "outline"), "generated")
	if err != nil {
		t.Fatal(err)
	}
	placement := AutoSplitPlacement{}
	err = s.acceptAutoSplitPartWithMatte(&a, "hair", image.Rect(0, 0, 8, 6), nil, &placement, func(string, ...string) (image.Image, error) { t.Fatal("valid alpha invoked matting"); return nil, nil })
	if err != nil || a.ProcessingWarning != "" {
		t.Fatalf("valid alpha regressed: %+v %v", a, err)
	}
}
