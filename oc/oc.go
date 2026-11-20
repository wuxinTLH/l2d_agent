// Package oc owns image-provider calls and local image assets. It has no GUI dependency.
package oc

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/draw"
	_ "image/jpeg"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const MaxImageBytes = 32 << 20

var validID = regexp.MustCompile(`^[a-f0-9]{24}$`)

type Asset struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	URL       string    `json:"url"`
	Width     int       `json:"width"`
	Height    int       `json:"height"`
	CreatedAt time.Time `json:"createdAt"`
}
type Provider struct {
	BaseURL string
	Model   string
	Key     string
}
type Store struct {
	Root   string
	Client *http.Client
}

func New(root string) *Store {
	return &Store{Root: root, Client: &http.Client{Timeout: 180 * time.Second}}
}
func ID() string {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}
func (s *Store) Path(id string) (string, error) {
	if !validID.MatchString(id) {
		return "", errors.New("invalid asset id")
	}
	return filepath.Join(s.Root, id+".png"), nil
}
func decode(data []byte) (image.Image, error) {
	if len(data) > MaxImageBytes {
		return nil, errors.New("image exceeds 32 MiB")
	}
	c, _, e := image.DecodeConfig(bytes.NewReader(data))
	if e != nil {
		return nil, e
	}
	if c.Width <= 0 || c.Height <= 0 || int64(c.Width)*int64(c.Height) > 40_000_000 {
		return nil, errors.New("image exceeds 40 megapixels")
	}
	im, _, e := image.Decode(bytes.NewReader(data))
	return im, e
}
func (s *Store) Import(data []byte, name string) (Asset, error) {
	im, e := decode(data)
	if e != nil {
		return Asset{}, e
	}
	return s.save(im, name)
}
func (s *Store) save(im image.Image, name string) (Asset, error) {
	if e := os.MkdirAll(s.Root, 0700); e != nil {
		return Asset{}, e
	}
	id := ID()
	p, _ := s.Path(id)
	f, e := os.OpenFile(p, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if e != nil {
		return Asset{}, e
	}
	e = png.Encode(f, im)
	if e == nil {
		st, statErr := f.Stat()
		if statErr != nil {
			e = statErr
		} else if st.Size() > MaxImageBytes {
			e = errors.New("normalized PNG exceeds 32 MiB; resize image before importing")
		}
	}
	ce := f.Close()
	if e == nil {
		e = ce
	}
	if e != nil {
		os.Remove(p)
		return Asset{}, e
	}
	a := Asset{id, name, "/assets/" + id + ".png", im.Bounds().Dx(), im.Bounds().Dy(), time.Now().UTC()}
	b, _ := json.MarshalIndent(a, "", "  ")
	if e = os.WriteFile(filepath.Join(s.Root, id+".json"), b, 0600); e != nil {
		os.Remove(p)
		return Asset{}, e
	}
	return a, nil
}
func (s *Store) List() ([]Asset, error) {
	out := []Asset{}
	entries, e := os.ReadDir(s.Root)
	if os.IsNotExist(e) {
		return out, nil
	}
	if e != nil {
		return nil, e
	}
	for _, v := range entries {
		if strings.HasSuffix(v.Name(), ".json") && validID.MatchString(strings.TrimSuffix(v.Name(), ".json")) {
			b, e := os.ReadFile(filepath.Join(s.Root, v.Name()))
			var a Asset
			if e == nil && json.Unmarshal(b, &a) == nil {
				out = append(out, a)
			}
		}
	}
	return out, nil
}

type Part struct {
	Name   string `json:"name"`
	X      int    `json:"x"`
	Y      int    `json:"y"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}
type SplitResult struct {
	ID          string  `json:"id"`
	Parts       []Asset `json:"parts"`
	ManifestURL string  `json:"manifestUrl"`
}

func (s *Store) Split(assetID string, parts []Part) (SplitResult, error) {
	p, e := s.Path(assetID)
	if e != nil {
		return SplitResult{}, e
	}
	b, e := os.ReadFile(p)
	if e != nil {
		return SplitResult{}, e
	}
	im, e := decode(b)
	if e != nil {
		return SplitResult{}, e
	}
	if len(parts) == 0 || len(parts) > 100 {
		return SplitResult{}, errors.New("provide 1 to 100 parts")
	}
	for _, v := range parts {
		if strings.TrimSpace(v.Name) == "" || v.X < 0 || v.Y < 0 || v.Width <= 0 || v.Height <= 0 || v.Width > im.Bounds().Dx() || v.Height > im.Bounds().Dy() || v.X > im.Bounds().Dx()-v.Width || v.Y > im.Bounds().Dy()-v.Height {
			return SplitResult{}, errors.New("part name or pixel bounds invalid")
		}
	}
	result := SplitResult{ID: ID(), Parts: []Asset{}}
	for _, v := range parts {
		dst := image.NewNRGBA(image.Rect(0, 0, v.Width, v.Height))
		draw.Draw(dst, dst.Bounds(), im, image.Pt(v.X, v.Y), draw.Src)
		a, e := s.save(dst, v.Name)
		if e != nil {
			return SplitResult{}, e
		}
		result.Parts = append(result.Parts, a)
	}
	result.ManifestURL = "/assets/" + result.ID + ".manifest.json"
	manifest := map[string]any{"version": 1, "sourceAssetId": assetID, "canvas": map[string]int{"width": im.Bounds().Dx(), "height": im.Bounds().Dy()}, "regions": parts, "assets": result.Parts, "note": "Rectangular crops only; not semantic segmentation, occlusion repair, rigging or moc3."}
	data, _ := json.MarshalIndent(manifest, "", "  ")
	e = os.WriteFile(filepath.Join(s.Root, result.ID+".manifest.json"), data, 0600)
	return result, e
}
func endpoint(p Provider, path string) (string, error) {
	u, e := url.Parse(p.BaseURL)
	if e != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return "", errors.New("provider base URL must be http(s), without credentials/query/fragment")
	}
	return strings.TrimRight(p.BaseURL, "/") + path, nil
}
func (s *Store) Generate(ctx context.Context, p Provider, prompt, negative, size string) (Asset, error) {
	if strings.TrimSpace(prompt) == "" {
		return Asset{}, errors.New("prompt required")
	}
	if size != "1024x1024" && size != "1024x1536" && size != "1536x1024" && size != "auto" {
		return Asset{}, errors.New("unsupported size")
	}
	if negative != "" {
		prompt += "\nAvoid: " + negative
	}
	data, _ := json.Marshal(map[string]any{"model": p.Model, "prompt": prompt, "size": size, "n": 1})
	return s.request(ctx, p, "/images/generations", "application/json", bytes.NewReader(data))
}
func (s *Store) Edit(ctx context.Context, p Provider, data []byte, prompt string) (Asset, error) {
	if strings.TrimSpace(prompt) == "" {
		return Asset{}, errors.New("edit prompt required")
	}
	if _, e := decode(data); e != nil {
		return Asset{}, e
	}
	var normalized bytes.Buffer
	im, _ := decode(data)
	if e := png.Encode(&normalized, im); e != nil {
		return Asset{}, e
	}
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	w.WriteField("model", p.Model)
	w.WriteField("prompt", prompt)
	w.WriteField("n", "1")
	f, e := w.CreateFormFile("image", "source.png")
	if e != nil {
		return Asset{}, e
	}
	f.Write(normalized.Bytes())
	w.Close()
	return s.request(ctx, p, "/images/edits", w.FormDataContentType(), &body)
}
func readBounded(r io.Reader, max int64) ([]byte, error) {
	b, e := io.ReadAll(io.LimitReader(r, max+1))
	if e == nil && int64(len(b)) > max {
		e = errors.New("response too large")
	}
	return b, e
}
func (s *Store) request(ctx context.Context, p Provider, path, contentType string, body io.Reader) (Asset, error) {
	ep, e := endpoint(p, path)
	if e != nil {
		return Asset{}, e
	}
	req, e := http.NewRequestWithContext(ctx, "POST", ep, body)
	if e != nil {
		return Asset{}, e
	}
	req.Header.Set("Content-Type", contentType)
	if p.Key != "" {
		req.Header.Set("Authorization", "Bearer "+p.Key)
	}
	client := *s.Client
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error { return http.ErrUseLastResponse }
	res, e := client.Do(req)
	if e != nil {
		return Asset{}, errors.New("image provider connection failed")
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return Asset{}, fmt.Errorf("image provider returned HTTP %d; verify model, endpoint, credentials and quota", res.StatusCode)
	}
	b, e := readBounded(res.Body, 48<<20)
	if e != nil {
		return Asset{}, e
	}
	var out struct {
		Data []struct {
			Base64 string `json:"b64_json"`
			URL    string `json:"url"`
		} `json:"data"`
	}
	if json.Unmarshal(b, &out) != nil || len(out.Data) == 0 {
		return Asset{}, errors.New("provider returned no image data")
	}
	var img []byte
	if out.Data[0].Base64 != "" {
		img, e = base64.StdEncoding.DecodeString(out.Data[0].Base64)
	} else {
		u, err := url.Parse(out.Data[0].URL)
		if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil {
			return Asset{}, errors.New("provider image URL must be HTTPS; prefer b64_json")
		}
		r, err := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
		if err != nil {
			return Asset{}, err
		}
		resp, err := client.Do(r)
		if err != nil {
			return Asset{}, errors.New("image download failed")
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			return Asset{}, errors.New("image download returned non-200")
		}
		img, e = readBounded(resp.Body, MaxImageBytes)
	}
	if e != nil {
		return Asset{}, errors.New("invalid or oversized provider image")
	}
	return s.Import(img, "Generated OC")
}
