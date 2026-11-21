package oc

import (
	"bytes"
	"encoding/binary"
	"errors"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

const MaxPSDBytes int64 = 512 << 20

type PSDRequest struct {
	Width  int        `json:"width"`
	Height int        `json:"height"`
	Layers []PSDLayer `json:"layers"`
}
type PSDLayer struct {
	AssetID string `json:"assetId"`
	Name    string `json:"name"`
	// Group is an optional PSD folder name. Equal adjacent groups share a folder;
	// separated runs remain separate folders so the first-input-top order is kept.
	Group string `json:"group,omitempty"`
	X     int    `json:"x"`
	Y     int    `json:"y"`
	// Width and Height optionally resize the full asset for this export only.
	// Both zero preserve the original dimensions; otherwise both must be positive.
	Width  int `json:"width,omitempty"`
	Height int `json:"height,omitempty"`
}
type PSDResult struct {
	ID         string `json:"id"`
	URL        string `json:"url"`
	Width      int    `json:"width"`
	Height     int    `json:"height"`
	LayerCount int    `json:"layerCount"`
}

type psdSource struct {
	path, name       string
	group            string
	x, y, w, h       int
	sourceW, sourceH int // original dimensions, checked again after decoding
}

func psdNumber(w io.Writer, v any) { _ = binary.Write(w, binary.BigEndian, v) }

// ExportPSD creates a PSD v1 RGB/8 document with normal-blended straight RGBA
// layers. The first requested layer is topmost. Signed offsets are preserved;
// pixels outside the canvas are retained in the layer but clipped in the merge.
// Group names create pass-through folders without changing the submitted stacking
// order. Semantic Cubism drawing order must be expressed by that stacking order;
// PSD folders do not set Cubism's numeric draw-order properties.
// No network calls or third-party codecs are used.
func (s *Store) ExportPSD(req PSDRequest) (PSDResult, error) {
	assetOperations.Lock()
	defer assetOperations.Unlock()
	if req.Width < 1 || req.Height < 1 || req.Width > 8192 || req.Height > 8192 || int64(req.Width)*int64(req.Height) > 40_000_000 {
		return PSDResult{}, errors.New("PSD canvas must be 1..8192 per axis and at most 40 megapixels")
	}
	if len(req.Layers) < 1 || len(req.Layers) > 64 {
		return PSDResult{}, errors.New("PSD requires 1..64 layers")
	}
	sources := make([]psdSource, 0, len(req.Layers))
	var pixels int64
	for _, l := range req.Layers {
		a, err := s.readAsset(l.AssetID)
		if err != nil {
			return PSDResult{}, err
		}
		p, _ := s.Path(l.AssetID)
		f, err := os.Open(p)
		if err != nil {
			return PSDResult{}, err
		}
		st, err := f.Stat()
		if err != nil {
			f.Close()
			return PSDResult{}, err
		}
		if st.Size() > MaxImageBytes {
			f.Close()
			return PSDResult{}, errors.New("asset exceeds 32 MiB")
		}
		c, err := png.DecodeConfig(f)
		f.Close()
		if err != nil {
			return PSDResult{}, err
		}
		if c.Width < 1 || c.Height < 1 || c.Width > 8192 || c.Height > 8192 || int64(c.Width)*int64(c.Height) > 40_000_000 {
			return PSDResult{}, errors.New("PSD layer dimensions exceed limits")
		}
		w, h := c.Width, c.Height
		if l.Width != 0 || l.Height != 0 {
			if l.Width < 1 || l.Height < 1 || l.Width > 8192 || l.Height > 8192 || int64(l.Width)*int64(l.Height) > 40_000_000 {
				return PSDResult{}, errors.New("PSD layer output dimensions must both be 1..8192 and at most 40 megapixels")
			}
			w, h = l.Width, l.Height
		}
		pixels += int64(w) * int64(h)
		if pixels > 80_000_000 {
			return PSDResult{}, errors.New("PSD layers exceed 80 megapixels combined")
		}
		// Bounds fit PSD signed 32-bit coordinates; constrain pathological off-canvas input.
		if l.X < -8192 || l.X > 8192 || l.Y < -8192 || l.Y > 8192 {
			return PSDResult{}, errors.New("PSD layer offset must be within -8192..8192")
		}
		name := l.Name
		if strings.TrimSpace(name) == "" {
			name = a.Name
		}
		if strings.TrimSpace(name) == "" {
			name = "Layer"
		}
		if !utf8.ValidString(name) || len(name) > 4096 || strings.ContainsRune(name, 0) {
			return PSDResult{}, errors.New("invalid PSD layer name")
		}
		if !utf8.ValidString(l.Group) || len(l.Group) > 4096 || strings.ContainsRune(l.Group, 0) {
			return PSDResult{}, errors.New("invalid PSD group name")
		}
		src := psdSource{path: p, name: name, group: l.Group, x: l.X, y: l.Y, w: w, h: h, sourceW: c.Width, sourceH: c.Height}
		sources = append(sources, src)
	}
	// PSD stores layers bottom-to-top, opposite to the request/UI order.
	// Channel image data below must follow this same record order.
	entries := psdLayerRecords(sources)
	var records bytes.Buffer
	for _, entry := range entries {
		writePSDLayerRecord(&records, entry.source, entry.section)
	}
	// Layer info count includes section dividers, whose empty RGBA channels
	// contain only the two-byte raw-compression word for each channel.
	// A negative count marks the fourth merged channel as transparency.
	layerLen := int64(2+records.Len()) + pixels*4 + int64(len(entries))*8
	if layerLen%2 != 0 {
		layerLen++
	}
	size := int64(26+4+4+4) + 4 + layerLen + 4 + 2 + int64(req.Width)*int64(req.Height)*4
	if size > MaxPSDBytes {
		return PSDResult{}, errors.New("PSD output exceeds 512 MiB")
	}
	dir := filepath.Join(s.Root, "exports")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return PSDResult{}, err
	}
	if st, err := os.Lstat(dir); err != nil || !st.IsDir() || st.Mode()&os.ModeSymlink != 0 {
		return PSDResult{}, errors.New("exports must be a real directory")
	}
	f, err := os.CreateTemp(dir, ".psd-*")
	if err != nil {
		return PSDResult{}, err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	// All writes go through a sticky-error writer, including encoding helpers.
	out := &psdWriter{w: f}
	out.Write([]byte("8BPS"))
	psdNumber(out, uint16(1))
	out.Write(make([]byte, 6))
	psdNumber(out, uint16(4))
	psdNumber(out, uint32(req.Height))
	psdNumber(out, uint32(req.Width))
	psdNumber(out, uint16(8))
	psdNumber(out, uint16(3))
	psdNumber(out, uint32(0))
	psdNumber(out, uint32(0)) // color mode data, image resources
	psdNumber(out, uint32(4+layerLen+4))
	psdNumber(out, uint32(layerLen))
	psdNumber(out, int16(-len(entries)))
	out.Write(records.Bytes())
	for _, entry := range entries {
		if entry.section != 0 {
			for c := 0; c < 4; c++ {
				psdNumber(out, uint16(0))
			}
			continue
		}
		im, e := loadPSDImage(entry.source)
		if e != nil {
			f.Close()
			return PSDResult{}, e
		}
		writePSDChannels(out, im)
	}
	if (2+int64(records.Len())+pixels*4+int64(len(entries))*8)%2 != 0 {
		out.Write([]byte{0})
	}
	psdNumber(out, uint32(0)) // global layer mask
	merged := image.NewNRGBA(image.Rect(0, 0, req.Width, req.Height))
	for i := len(sources) - 1; i >= 0; i-- {
		src := sources[i]
		im, e := loadPSDImage(src)
		if e != nil {
			f.Close()
			return PSDResult{}, e
		}
		draw.Draw(merged, image.Rect(src.x, src.y, src.x+src.w, src.y+src.h), im, im.Bounds().Min, draw.Over)
	}
	psdNumber(out, uint16(0)) // merged raw compression, followed by planar RGBA
	writePSDPlanes(out, merged)
	if err = out.err; err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return PSDResult{}, err
	}
	id := ID()
	path := filepath.Join(dir, id+".psd")
	if err = os.Rename(tmp, path); err != nil {
		return PSDResult{}, err
	}
	return PSDResult{id, "/exports/" + id + ".psd", req.Width, req.Height, len(sources)}, nil
}

type psdWriter struct {
	w   io.Writer
	err error
}

func (w *psdWriter) Write(p []byte) (int, error) {
	if w.err != nil {
		return 0, w.err
	}
	n, e := w.w.Write(p)
	if e == nil && n != len(p) {
		e = io.ErrShortWrite
	}
	w.err = e
	return n, e
}
func loadPSDImage(src psdSource) (*image.NRGBA, error) {
	f, err := os.Open(src.path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	im, err := png.Decode(io.LimitReader(f, MaxImageBytes+1))
	if err != nil {
		return nil, err
	}
	if im.Bounds().Dx() != src.sourceW || im.Bounds().Dy() != src.sourceH {
		return nil, errors.New("asset changed during export")
	}
	if src.w != src.sourceW || src.h != src.sourceH {
		return resizePSDImage(im, src.w, src.h), nil
	}
	if n, ok := im.(*image.NRGBA); ok {
		return n, nil
	}
	n := image.NewNRGBA(image.Rect(0, 0, src.w, src.h))
	// Color conversion yields unpremultiplied samples, unlike Color.RGBA().
	for y := 0; y < src.h; y++ {
		for x := 0; x < src.w; x++ {
			n.SetNRGBA(x, y, color.NRGBAModel.Convert(im.At(x+im.Bounds().Min.X, y+im.Bounds().Min.Y)).(color.NRGBA))
		}
	}
	return n, nil
}

// resizePSDImage scales the entire asset, including transparent margins, to the
// exact requested size. Interpolate premultiplied samples and unpremultiply only
// at the end, so hidden RGB cannot bleed into partially transparent edge pixels.
func resizePSDImage(im image.Image, w, h int) *image.NRGBA {
	out := image.NewNRGBA(image.Rect(0, 0, w, h))
	src := im.Bounds()
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			sx := (float64(x)+0.5)*float64(src.Dx())/float64(w) - 0.5 + float64(src.Min.X)
			sy := (float64(y)+0.5)*float64(src.Dy())/float64(h) - 0.5 + float64(src.Min.Y)
			x0, y0 := int(math.Floor(sx)), int(math.Floor(sy))
			fx, fy := sx-float64(x0), sy-float64(y0)
			var sum [4]float64
			for j := 0; j < 2; j++ {
				for i := 0; i < 2; i++ {
					px := max(src.Min.X, min(src.Max.X-1, x0+i))
					py := max(src.Min.Y, min(src.Max.Y-1, y0+j))
					r, g, b, a := im.At(px, py).RGBA()
					wx, wy := 1-fx, 1-fy
					if i == 1 {
						wx = fx
					}
					if j == 1 {
						wy = fy
					}
					weight := wx * wy
					sum[0] += float64(r) * weight
					sum[1] += float64(g) * weight
					sum[2] += float64(b) * weight
					sum[3] += float64(a) * weight
				}
			}
			if sum[3] > 0 {
				out.SetNRGBA(x, y, color.NRGBA{
					R: uint8(math.Round(math.Min(255, sum[0]*255/sum[3]))),
					G: uint8(math.Round(math.Min(255, sum[1]*255/sum[3]))),
					B: uint8(math.Round(math.Min(255, sum[2]*255/sum[3]))),
					A: uint8(math.Round(sum[3] / 257)),
				})
			}
		}
	}
	return out
}

type psdLayerRecord struct {
	source  psdSource
	section uint32 // lsct: 1 = open folder, 3 = bounding section divider
}

// Build wire order directly: bottom boundary, children bottom-to-top, folder.
// Do not gather non-adjacent names: doing so would change visible layer order.
func psdLayerRecords(sources []psdSource) []psdLayerRecord {
	entries := make([]psdLayerRecord, 0, len(sources))
	for i := len(sources) - 1; i >= 0; {
		group := sources[i].group
		if group == "" {
			entries = append(entries, psdLayerRecord{source: sources[i]})
			i--
			continue
		}
		entries = append(entries, psdLayerRecord{source: psdSource{name: "</Layer group>"}, section: 3})
		for i >= 0 && sources[i].group == group {
			entries = append(entries, psdLayerRecord{source: sources[i]})
			i--
		}
		entries = append(entries, psdLayerRecord{source: psdSource{name: group}, section: 1})
	}
	return entries
}

func writePSDLayerRecord(w *bytes.Buffer, s psdSource, section uint32) {
	psdNumber(w, int32(s.y))
	psdNumber(w, int32(s.x))
	psdNumber(w, int32(s.y+s.h))
	psdNumber(w, int32(s.x+s.w))
	psdNumber(w, uint16(4))
	for _, id := range []int16{0, 1, 2, -1} {
		psdNumber(w, id)
		psdNumber(w, uint32(2+s.w*s.h))
	}
	if section == 1 {
		w.WriteString("8BIMpass") // pass-through folder preserves child compositing
	} else {
		w.WriteString("8BIMnorm")
	}
	w.Write([]byte{255, 0, 0, 0})
	var extra bytes.Buffer
	psdNumber(&extra, uint32(0))
	psdNumber(&extra, uint32(0)) // mask, blending ranges
	// ASCII fallback Pascal string; Unicode authoritative name is in luni.
	fallback := []byte(s.name)
	for i, b := range fallback {
		if b < 32 || b > 126 {
			fallback[i] = '?'
		}
	}
	if len(fallback) > 255 {
		fallback = fallback[:255]
	}
	extra.WriteByte(byte(len(fallback)))
	extra.Write(fallback)
	for (1+len(fallback))%4 != 0 {
		extra.WriteByte(0)
		fallback = append(fallback, 0)
	}
	u := utf16.Encode([]rune(s.name))
	extra.WriteString("8BIMluni")
	psdNumber(&extra, uint32(4+2*len(u)))
	psdNumber(&extra, uint32(len(u)))
	for _, v := range u {
		psdNumber(&extra, v)
	}
	if section != 0 {
		extra.WriteString("8BIMlsct")
		psdNumber(&extra, uint32(12))
		psdNumber(&extra, section)
		extra.WriteString("8BIM")
		if section == 1 {
			extra.WriteString("pass")
		} else {
			extra.WriteString("norm")
		}
	}
	psdNumber(w, uint32(extra.Len()))
	w.Write(extra.Bytes())
}
func writePSDChannels(w io.Writer, im *image.NRGBA) {
	for c := 0; c < 4; c++ {
		psdNumber(w, uint16(0))
		writePSDPlane(w, im, c)
	}
}
func writePSDPlanes(w io.Writer, im *image.NRGBA) {
	for c := 0; c < 4; c++ {
		writePSDPlane(w, im, c)
	}
}
func writePSDPlane(w io.Writer, im *image.NRGBA, c int) {
	row := make([]byte, im.Bounds().Dx())
	for y := 0; y < im.Bounds().Dy(); y++ {
		for x := range row {
			row[x] = im.Pix[y*im.Stride+x*4+c]
		}
		_, _ = w.Write(row)
	}
}
