// Package coreserver coordinates independent local modules over a loopback-only HTTP API.
package coreserver

import (
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"local/l2dagent/l2d"
	"local/l2dagent/oc"
)

type Config struct {
	ImageBaseURL     string `json:"imageBaseUrl"`
	ImageModel       string `json:"imageModel"`
	ImageAPIKey      string `json:"imageApiKey,omitempty"`
	CubismExecutable string `json:"cubismExecutable"`
	MattingDevice    string `json:"mattingDevice"`
}
type Server struct {
	root           string
	web            string
	mu             sync.RWMutex
	config         Config
	assets         *oc.Store
	autoSplitMu    sync.Mutex
	cubismMu       sync.Mutex
	cubismBridge   *l2d.CubismBridge
	cubismEndpoint string
}

func New(root, web string) (*Server, error) {
	if e := os.MkdirAll(root, 0700); e != nil {
		return nil, e
	}
	s := &Server{root: root, web: web, config: Config{ImageBaseURL: "https://api.openai.com/v1", ImageModel: "gpt-image-1"}, assets: oc.New(filepath.Join(root, "assets"))}
	b, e := os.ReadFile(filepath.Join(root, "config.json"))
	if e == nil {
		if e = json.Unmarshal(b, &s.config); e != nil {
			return nil, errors.New("invalid local configuration")
		}
	} else if !os.IsNotExist(e) {
		return nil, e
	}
	return s, nil
}
func (s *Server) snapshot() Config { s.mu.RLock(); defer s.mu.RUnlock(); return s.config }
func jsonOut(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
func fail(w http.ResponseWriter, e error) { jsonOut(w, 400, map[string]string{"error": e.Error()}) }
func body(w http.ResponseWriter, r *http.Request, v any) error {
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		return errors.New("Content-Type application/json required")
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if e := d.Decode(v); e != nil {
		return errors.New("invalid JSON request")
	}
	if d.Decode(new(any)) != io.EOF {
		return errors.New("only one JSON value allowed")
	}
	return nil
}
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	s.assetWorkflow(mux)
	s.cubismAutomation(mux)
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		jsonOut(w, 200, map[string]any{"status": "ok", "modules": map[string]string{"oc": "available", "l2d": "existing-model companion"}})
	})
	mux.HandleFunc("GET /api/config", func(w http.ResponseWriter, r *http.Request) {
		c := s.snapshot()
		jsonOut(w, 200, map[string]any{"imageBaseUrl": c.ImageBaseURL, "imageModel": c.ImageModel, "hasImageKey": c.ImageAPIKey != "", "cubismExecutable": c.CubismExecutable, "mattingDevice": effectiveMattingDevice(c.MattingDevice)})
	})
	mux.HandleFunc("PUT /api/config", s.putConfig)
	mux.HandleFunc("GET /api/assets", func(w http.ResponseWriter, r *http.Request) {
		v, e := s.assets.List()
		if e != nil {
			fail(w, e)
			return
		}
		jsonOut(w, 200, v)
	})
	mux.HandleFunc("POST /api/oc/import", s.importImage)
	mux.HandleFunc("POST /api/oc/edit", s.editImage)
	mux.HandleFunc("POST /api/oc/generate", func(w http.ResponseWriter, r *http.Request) {
		var v struct {
			Prompt   string `json:"prompt"`
			Negative string `json:"negativePrompt"`
			Size     string `json:"size"`
		}
		if e := body(w, r, &v); e != nil {
			fail(w, e)
			return
		}
		c := s.snapshot()
		a, e := s.assets.Generate(r.Context(), oc.Provider{BaseURL: c.ImageBaseURL, Model: c.ImageModel, Key: c.ImageAPIKey}, v.Prompt, v.Negative, v.Size)
		if e != nil {
			fail(w, e)
			return
		}
		jsonOut(w, 201, a)
	})
	mux.HandleFunc("POST /api/oc/auto-split", func(w http.ResponseWriter, r *http.Request) {
		var v struct {
			AssetID string                        `json:"assetId"`
			Parts   []string                      `json:"parts"`
			Bounds  map[string]oc.AutoSplitBounds `json:"bounds"`
		}
		if e := body(w, r, &v); e != nil {
			fail(w, e)
			return
		}
		if !s.autoSplitMu.TryLock() {
			jsonOut(w, 409, map[string]string{"error": "API 拆分正在进行，请勿重复提交付费请求。"})
			return
		}
		defer s.autoSplitMu.Unlock()
		c := s.snapshot()
		out, e := s.assets.AutoSplitWithBounds(r.Context(), oc.Provider{BaseURL: c.ImageBaseURL, Model: c.ImageModel, Key: c.ImageAPIKey, MattingDevice: effectiveMattingDevice(c.MattingDevice)}, v.AssetID, v.Parts, v.Bounds)
		if e != nil {
			fail(w, e)
			return
		}
		jsonOut(w, 200, out)
	})
	mux.HandleFunc("POST /api/oc/split", func(w http.ResponseWriter, r *http.Request) {
		var v struct {
			AssetID string    `json:"assetId"`
			Parts   []oc.Part `json:"parts"`
		}
		if e := body(w, r, &v); e != nil {
			fail(w, e)
			return
		}
		a, e := s.assets.Split(v.AssetID, v.Parts)
		if e != nil {
			fail(w, e)
			return
		}
		jsonOut(w, 201, a)
	})
	mux.HandleFunc("POST /api/l2d/cubism/discover", func(w http.ResponseWriter, r *http.Request) {
		jsonOut(w, 200, map[string]any{"candidates": l2d.DiscoverCubism(), "note": "Cubism Editor paths only; installers excluded. Verify the path and save it in settings. No automatic launch or mesh/rig generation."})
	})
	mux.HandleFunc("POST /api/l2d/cubism/launch", func(w http.ResponseWriter, r *http.Request) {
		if e := l2d.LaunchCubism(s.snapshot().CubismExecutable); e != nil {
			fail(w, e)
			return
		}
		jsonOut(w, 200, map[string]string{"status": "started", "note": "Cubism process started; editor readiness, license and project state are unverified."})
	})
	mux.HandleFunc("POST /api/l2d/inspect", func(w http.ResponseWriter, r *http.Request) {
		var v struct {
			ModelPath string `json:"modelPath"`
		}
		if e := body(w, r, &v); e != nil {
			fail(w, e)
			return
		}
		out, e := l2d.Inspect(v.ModelPath)
		if e != nil {
			fail(w, e)
			return
		}
		jsonOut(w, 200, out)
	})
	assetName := regexp.MustCompile(`^[a-f0-9]{24}\.(png|manifest\.json)$`)
	mux.HandleFunc("GET /assets/{name}", func(w http.ResponseWriter, r *http.Request) {
		n := r.PathValue("name")
		if !assetName.MatchString(n) {
			http.NotFound(w, r)
			return
		}
		http.ServeFile(w, r, filepath.Join(s.assets.Root, n))
	})
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		jsonOut(w, 404, map[string]string{"error": "unknown API route or method"})
	})
	mux.Handle("/", http.FileServer(http.Dir(s.web)))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Cache-Control", "no-store")
		host, _, e := net.SplitHostPort(r.Host)
		if e != nil {
			host = r.Host
		}
		ip := net.ParseIP(host)
		if host != "localhost" && (ip == nil || !ip.IsLoopback()) {
			http.Error(w, "loopback host required", 403)
			return
		}
		if origin := r.Header.Get("Origin"); origin != "" {
			u, e := url.Parse(origin)
			allowed := e == nil && u.Scheme == "http" && (u.Host == r.Host || u.Host == "127.0.0.1:5173" || u.Host == "localhost:5173")
			if !allowed {
				http.Error(w, "cross-origin request rejected", 403)
				return
			}
		}
		if r.Header.Get("Sec-Fetch-Site") == "cross-site" {
			http.Error(w, "cross-site request rejected", 403)
			return
		}
		mux.ServeHTTP(w, r)
	})
}
func (s *Server) putConfig(w http.ResponseWriter, r *http.Request) {
	var v struct {
		ImageBaseURL     string  `json:"imageBaseUrl"`
		ImageModel       string  `json:"imageModel"`
		ImageAPIKey      *string `json:"imageApiKey"`
		CubismExecutable *string `json:"cubismExecutable"`
		MattingDevice    *string `json:"mattingDevice"`
	}
	if e := body(w, r, &v); e != nil {
		fail(w, e)
		return
	}
	u, e := url.Parse(v.ImageBaseURL)
	if e != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") || u.User != nil || u.RawQuery != "" || u.Fragment != "" || strings.TrimSpace(v.ImageModel) == "" {
		fail(w, errors.New("invalid image base URL or model"))
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	c := s.config
	c.ImageBaseURL = v.ImageBaseURL
	c.ImageModel = v.ImageModel
	if v.MattingDevice != nil {
		if *v.MattingDevice != "gpu" && *v.MattingDevice != "cpu" {
			fail(w, errors.New("mattingDevice must be gpu or cpu"))
			return
		}
		c.MattingDevice = *v.MattingDevice
	}
	if v.CubismExecutable != nil {
		c.CubismExecutable = *v.CubismExecutable
	}
	if v.ImageAPIKey != nil {
		c.ImageAPIKey = *v.ImageAPIKey
	}
	b, _ := json.MarshalIndent(c, "", "  ")
	tmp, e := os.CreateTemp(s.root, "config-*.tmp")
	if e != nil {
		fail(w, e)
		return
	}
	name := tmp.Name()
	defer os.Remove(name)
	tmp.Chmod(0600)
	_, e = tmp.Write(b)
	ce := tmp.Close()
	if e == nil {
		e = ce
	}
	if e == nil {
		e = os.Rename(name, filepath.Join(s.root, "config.json"))
	}
	if e != nil {
		fail(w, errors.New("cannot save configuration"))
		return
	}
	s.config = c
	jsonOut(w, 200, map[string]bool{"saved": true})
}
func (s *Server) upload(w http.ResponseWriter, r *http.Request) ([]byte, error) {
	r.Body = http.MaxBytesReader(w, r.Body, oc.MaxImageBytes+1<<20)
	if e := r.ParseMultipartForm(1 << 20); e != nil {
		return nil, errors.New("invalid multipart upload or image exceeds 32 MiB")
	}
	return s.uploadParsed(r)
}
func (s *Server) uploadParsed(r *http.Request) ([]byte, error) {
	f, _, e := r.FormFile("image")
	if e != nil {
		return nil, e
	}
	defer f.Close()
	b, e := io.ReadAll(io.LimitReader(f, oc.MaxImageBytes+1))
	if len(b) > oc.MaxImageBytes {
		return nil, errors.New("image exceeds 32 MiB")
	}
	return b, e
}
func (s *Server) importImage(w http.ResponseWriter, r *http.Request) {
	b, e := s.upload(w, r)
	if r.MultipartForm != nil {
		defer r.MultipartForm.RemoveAll()
	}
	if e != nil {
		fail(w, e)
		return
	}
	a, e := s.assets.Import(b, "Imported OC")
	if e != nil {
		fail(w, e)
		return
	}
	jsonOut(w, 201, a)
}
func (s *Server) editImage(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, oc.MaxImageBytes+1<<20)
	if e := r.ParseMultipartForm(1 << 20); e != nil {
		fail(w, errors.New("invalid edit upload"))
		return
	}
	defer r.MultipartForm.RemoveAll()
	b, e := s.uploadParsed(r)
	if errors.Is(e, http.ErrMissingFile) {
		var path string
		path, e = s.assets.Path(r.FormValue("sourceId"))
		if e == nil {
			b, e = os.ReadFile(path)
		}
	}
	if e != nil {
		fail(w, e)
		return
	}
	c := s.snapshot()
	a, e := s.assets.Edit(r.Context(), oc.Provider{BaseURL: c.ImageBaseURL, Model: c.ImageModel, Key: c.ImageAPIKey}, b, r.FormValue("prompt"))
	if e != nil {
		fail(w, e)
		return
	}
	jsonOut(w, 201, a)
}
