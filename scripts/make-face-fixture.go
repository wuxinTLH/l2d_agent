//go:build ignore

// Run from the repository root: go run scripts/make-face-fixture.go
// Only local mathematical artwork is used; no providers, user assets or desktop input.
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
	"os"
	"path/filepath"

	"local/l2dagent/oc"
)

var skin = color.NRGBA{255, 226, 208, 255}
var ink = color.NRGBA{72, 43, 61, 255}
var clear = color.NRGBA{}

type painter func(x, y float64) color.NRGBA
type layer struct {
	name    string
	rect    image.Rectangle
	paint   painter
	role    string
	closeDY int
}
type spec struct {
	Order   int    `json:"topToBottom"`
	Name    string `json:"name"`
	X       int    `json:"x"`
	Y       int    `json:"y"`
	Width   int    `json:"width"`
	Height  int    `json:"height"`
	AssetID string `json:"assetId"`
	PNG     string `json:"png"`
	Role    string `json:"role"`
	CloseDY int    `json:"closeTranslateY,omitempty"`
}

func ellipse(x, y, cx, cy, rx, ry float64) bool {
	return math.Pow((x-cx)/rx, 2)+math.Pow((y-cy)/ry, 2) <= 1
}
func line(x, y, ax, ay, bx, by, r float64) bool {
	dx, dy := bx-ax, by-ay
	t := ((x-ax)*dx + (y-ay)*dy) / (dx*dx + dy*dy)
	t = math.Max(0, math.Min(1, t))
	return math.Hypot(x-ax-t*dx, y-ay-t*dy) <= r
}

// Four-by-four supersampling makes the hand-authored geometry clean at native size.
func raster(l layer) *image.NRGBA {
	im := image.NewNRGBA(image.Rect(0, 0, l.rect.Dx(), l.rect.Dy()))
	for y := 0; y < im.Bounds().Dy(); y++ {
		for x := 0; x < im.Bounds().Dx(); x++ {
			var r, g, b, a float64
			for sy := 0; sy < 4; sy++ {
				for sx := 0; sx < 4; sx++ {
					c := l.paint(float64(x+l.rect.Min.X)+(float64(sx)+.5)/4, float64(y+l.rect.Min.Y)+(float64(sy)+.5)/4)
					alpha := float64(c.A)
					a += alpha
					r += float64(c.R) * alpha
					g += float64(c.G) * alpha
					b += float64(c.B) * alpha
				}
			}
			if a > 0 {
				im.SetNRGBA(x, y, color.NRGBA{uint8(math.Round(r / a)), uint8(math.Round(g / a)), uint8(math.Round(b / a)), uint8(math.Round(a / 16))})
			}
		}
	}
	return im
}
func writePNG(path string, im image.Image) error {
	f, e := os.Create(path)
	if e != nil {
		return e
	}
	e = png.Encode(f, im)
	ce := f.Close()
	if e != nil {
		return e
	}
	return ce
}
func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func main() {
	root, e := filepath.Abs(filepath.Join(".local", "qa", "face-fixture"))
	must(e)
	must(os.MkdirAll(root, 0700))
	store := oc.New(filepath.Join(root, "assets"))
	var layers []layer
	// Topmost lower covers hide the descending upper edge at the closed endpoint.
	for _, eye := range []struct {
		name string
		cx   float64
	}{{"Eye_L", 224}, {"Eye_R", 416}} {
		cx := eye.cx
		layers = append(layers, layer{eye.name + "_LowerLid", image.Rect(int(cx)-76, 216, int(cx)+76, 300), func(x, y float64) color.NRGBA {
			if !ellipse(x, y, 320, 229, 171, 169) {
				return clear
			}
			t := (x - cx) / 62
			edge := 250 - 30*math.Min(1, t*t)
			if y < edge {
				return clear
			}
			if math.Abs(t) < 1 && y < edge+2.4 {
				return ink
			}
			return skin
		}, "Fixed lower skin cover and closed-eye contour; keep above upper cover", 0})
	}
	for _, eye := range []struct {
		name string
		cx   float64
	}{{"Eye_L", 224}, {"Eye_R", 416}} {
		cx := eye.cx
		layers = append(layers, layer{eye.name + "_UpperLid", image.Rect(int(cx)-76, 110, int(cx)+76, 228), func(x, y float64) color.NRGBA {
			if !ellipse(x, y, 320, 229, 171, 169) {
				return clear
			}
			t := (x - cx) / 62
			edge := 198 + 22*math.Min(1, t*t)
			if line(x, y, cx-60, 218, cx-71, 210, 2.8) || line(x, y, cx+60, 218, cx+71, 210, 2.8) {
				return ink
			}
			if y > edge {
				return clear
			}
			if math.Abs(t) < 1 && y > edge-5 {
				return ink
			}
			return skin
		}, "Translate this skin cover down 56 canvas pixels to close; never scale sclera or iris", 56})
	}
	layers = append(layers, layer{"Brows", image.Rect(150, 164, 490, 190), func(x, y float64) color.NRGBA {
		for _, cx := range []float64{224, 416} {
			t := (x - cx) / 53
			if math.Abs(t) < 1 && math.Abs(y-(174+6*t*t)) < 3.1 {
				return color.NRGBA{111, 69, 74, 255}
			}
		}
		return clear
	}, "Paired brows; static", 0})
	for _, eye := range []struct {
		name string
		cx   float64
	}{{"Eye_L", 224}, {"Eye_R", 416}} {
		cx := eye.cx
		layers = append(layers, layer{eye.name + "_Iris", image.Rect(int(cx)-29, 187, int(cx)+29, 255), func(x, y float64) color.NRGBA {
			if !ellipse(x, y, cx, 221, 28, 33) {
				return clear
			}
			if !ellipse(x, y, cx, 221, 25, 30) {
				return ink
			}
			if ellipse(x, y, cx-9, 209, 7, 8) || ellipse(x, y, cx+10, 234, 3, 3) {
				return color.NRGBA{255, 255, 248, 255}
			}
			if ellipse(x, y, cx, 219, 9, 20) {
				return color.NRGBA{31, 54, 79, 255}
			}
			v := math.Max(0, math.Min(1, (y-194)/59))
			return color.NRGBA{uint8(47 + 48*v), uint8(105 + 83*v), uint8(141 + 62*v), 255}
		}, "Independent unscaled blue iris/pupil/highlights; upper/lower covers occlude it", 0})
		layers = append(layers, layer{eye.name + "_Sclera", image.Rect(int(cx)-63, 188, int(cx)+63, 261), func(x, y float64) color.NRGBA {
			if !ellipse(x, y, cx, 224, 62, 36) {
				return clear
			}
			if y < 207 {
				return color.NRGBA{222, 226, 240, 255}
			}
			return color.NRGBA{255, 253, 249, 255}
		}, "Independent unscaled white of eye", 0})
	}
	layers = append(layers, layer{"Mouth_LowerCover", image.Rect(274, 332, 366, 385), func(x, y float64) color.NRGBA {
		t := (x - 320) / 41
		edge := 334 + 24*math.Sqrt(math.Max(0, 1-t*t))
		if y < edge {
			return clear
		}
		if math.Abs(t) < 1 && y < edge+2.5 {
			return color.NRGBA{160, 73, 91, 255}
		}
		return skin
	}, "Fixed lower skin cover and closed-mouth lip contour; above upper cover", 0})
	layers = append(layers, layer{"Mouth_UpperCover", image.Rect(274, 190, 366, 336), func(x, y float64) color.NRGBA {
		t := (x - 320) / 41
		edge := 334 - 24*math.Sqrt(math.Max(0, 1-t*t))
		if y > edge {
			return clear
		}
		return skin
	}, "Translate down 50 canvas pixels at ParamMouthOpenY=0, zero at 1; no cavity/tongue scaling", 50})
	layers = append(layers, layer{"Mouth_LipOutline", image.Rect(278, 309, 362, 359), func(x, y float64) color.NRGBA {
		if ellipse(x, y, 320, 334, 41, 24) && !ellipse(x, y, 320, 334, 37, 20) {
			return color.NRGBA{160, 73, 91, 255}
		}
		return clear
	}, "Separate lip ring; cavity and tongue are independent", 0})
	layers = append(layers, layer{"Mouth_Tongue", image.Rect(293, 334, 347, 355), func(x, y float64) color.NRGBA {
		if ellipse(x, y, 320, 345, 26, 10) && ellipse(x, y, 320, 334, 37, 20) {
			if math.Abs(x-320) < 1 && y > 341 {
				return color.NRGBA{194, 88, 112, 255}
			}
			return color.NRGBA{246, 133, 153, 255}
		}
		return clear
	}, "Separate tongue inside cavity", 0})
	layers = append(layers, layer{"Mouth_Cavity", image.Rect(282, 313, 358, 355), func(x, y float64) color.NRGBA {
		if ellipse(x, y, 320, 334, 37, 20) {
			return color.NRGBA{83, 39, 62, 255}
		}
		return clear
	}, "Separate dark mouth interior", 0})
	layers = append(layers, layer{"Face_Details", image.Rect(144, 266, 496, 314), func(x, y float64) color.NRGBA {
		if ellipse(x, y, 187, 287, 31, 11) || ellipse(x, y, 453, 287, 31, 11) {
			return color.NRGBA{248, 175, 166, 255}
		}
		if line(x, y, 320, 274, 315, 291, 1.7) || line(x, y, 315, 291, 323, 292, 1.7) {
			return color.NRGBA{218, 156, 141, 255}
		}
		return clear
	}, "Blush and nose; static", 0})
	layers = append(layers, layer{"Face_Base", image.Rect(0, 0, 640, 480), func(x, y float64) color.NRGBA {
		c := color.NRGBA{238, 243, 248, 255}
		if ellipse(x, y, 320, 453, 154, 83) {
			c = color.NRGBA{66, 102, 137, 255}
		}
		if x > 276 && x < 364 && y > 349 && y < 420 {
			c = skin
		}
		if ellipse(x, y, 320, 219, 190, 190) {
			c = color.NRGBA{89, 56, 72, 255}
		}
		if ellipse(x, y, 151, 240, 23, 40) || ellipse(x, y, 489, 240, 23, 40) {
			c = skin
		}
		if ellipse(x, y, 320, 229, 171, 169) {
			c = skin
		}
		if y < 110 && ellipse(x, y, 320, 89, 178, 60) {
			c = color.NRGBA{89, 56, 72, 255}
			if y < 129 && ellipse(x, y, 280, 100, 107, 32) {
				c = color.NRGBA{118, 76, 89, 255}
			}
		}
		return c
	}, "Original geometric test face, hair, ears, neck and shirt; no user/AI artwork", 0})
	// Brows and blush must remain above moving skin covers. Keep each eye's four
	// layers adjacent for deterministic selection in the Cubism layer list.
	// Anatomical labels: viewer-left cx224 is Eye_R; viewer-right cx416 is Eye_L.
	for i := range layers {
		if len(layers[i].name) > 5 && layers[i].name[:5] == "Eye_L" {
			layers[i].name = "Eye_R" + layers[i].name[5:]
		} else if len(layers[i].name) > 5 && layers[i].name[:5] == "Eye_R" {
			layers[i].name = "Eye_L" + layers[i].name[5:]
		}
	}
	order := []string{"Brows", "Face_Details", "Eye_R_LowerLid", "Eye_R_UpperLid", "Eye_R_Iris", "Eye_R_Sclera", "Eye_L_LowerLid", "Eye_L_UpperLid", "Eye_L_Iris", "Eye_L_Sclera", "Mouth_LowerCover", "Mouth_UpperCover", "Mouth_LipOutline", "Mouth_Tongue", "Mouth_Cavity", "Face_Base"}
	ordered := make([]layer, 0, len(layers))
	for _, name := range order {
		for _, l := range layers {
			if l.name == name {
				ordered = append(ordered, l)
			}
		}
	}
	layers = ordered
	if len(layers) > 20 {
		must(fmt.Errorf("FREE mesh budget exceeded: %d", len(layers)))
	}
	req := oc.PSDRequest{Width: 640, Height: 480}
	specs := make([]spec, 0, len(layers))
	images := make([]*image.NRGBA, len(layers))
	for i, l := range layers {
		im := raster(l)
		images[i] = im
		var b bytes.Buffer
		must(png.Encode(&b, im))
		a, err := store.Import(b.Bytes(), l.name)
		must(err)
		path, err := store.Path(a.ID)
		must(err)
		req.Layers = append(req.Layers, oc.PSDLayer{AssetID: a.ID, Name: l.name, X: l.rect.Min.X, Y: l.rect.Min.Y})
		specs = append(specs, spec{i + 1, l.name, l.rect.Min.X, l.rect.Min.Y, l.rect.Dx(), l.rect.Dy(), a.ID, path, l.role, l.closeDY})
	}
	result, err := store.ExportPSD(req)
	must(err)
	source := filepath.Join(store.Root, "exports", result.ID+".psd")
	data, err := os.ReadFile(source)
	must(err)
	psd := filepath.Join(root, "face-fixture.psd")
	must(os.WriteFile(psd, data, 0600))
	preview := func(closed bool) *image.NRGBA {
		out := image.NewNRGBA(image.Rect(0, 0, 640, 480))
		for i := len(layers) - 1; i >= 0; i-- {
			r := layers[i].rect
			if closed {
				r = r.Add(image.Pt(0, layers[i].closeDY))
			}
			draw.Draw(out, r, images[i], image.Point{}, draw.Over)
		}
		return out
	}
	openPath := filepath.Join(root, "preview-open.png")
	closedPath := filepath.Join(root, "preview-closed.png")
	must(writePNG(openPath, preview(false)))
	must(writePNG(closedPath, preview(true)))
	manifest := struct {
		PSDPath          string `json:"psdPath"`
		PreviewPNG       string `json:"previewPNG"`
		ClosedPreviewPNG string `json:"closedPreviewPNG"`
		Width            int    `json:"width"`
		Height           int    `json:"height"`
		MeshCount        int    `json:"meshCount"`
		Order            string `json:"order"`
		CloseRecipe      string `json:"closeRecipe"`
		Layers           []spec `json:"layers"`
	}{psd, openPath, closedPath, 640, 480, len(layers), "first is topmost; no groups; all layers initially visible", "Eye_L_UpperLid and Eye_R_UpperLid: translate canvas Y +56 at EyeOpen=0, +0 at EyeOpen=1. Keep lower lids, sclera and iris unscaled; preserve stacking. Lower cover masks upper edge at closure. Intermediate values interpolate translation only. Anatomical Eye_R is viewer-left and Eye_L viewer-right. Mouth_UpperCover: translate canvas Y +50 at ParamMouthOpenY=0, +0 at 1; Mouth_LowerCover stays fixed above it. Keep lip/cavity/tongue unscaled.", specs}
	b, err := json.MarshalIndent(manifest, "", "  ")
	must(err)
	must(os.WriteFile(filepath.Join(root, "fixture.json"), append(b, '\n'), 0600))
	fmt.Println(string(b))
}
