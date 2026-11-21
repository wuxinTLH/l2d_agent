package coreserver

import (
	"encoding/json"
	"errors"
	"local/l2dagent/l2d"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// Token remains server-local, never in public config or API responses.
func (s *Server) cubismAutomation(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/l2d/cubism/connect", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Endpoint string `json:"endpoint"`
		}
		if err := body(w, r, &req); err != nil {
			fail(w, err)
			return
		}
		endpoint := req.Endpoint
		if endpoint == "" {
			endpoint = "ws://127.0.0.1:22033"
		}
		u, err := url.Parse(endpoint)
		if err != nil || u.Scheme != "ws" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Port() == "" || (u.Hostname() != "localhost" && (net.ParseIP(u.Hostname()) == nil || !net.ParseIP(u.Hostname()).IsLoopback())) {
			fail(w, errors.New("Cubism endpoint must be a loopback ws URL with explicit port"))
			return
		}
		s.cubismMu.Lock()
		defer s.cubismMu.Unlock()
		if s.cubismBridge == nil || s.cubismEndpoint != endpoint {
			if s.cubismBridge != nil {
				s.cubismBridge.Close()
			}
			token, _ := os.ReadFile(filepath.Join(s.root, "cubism-token"))
			s.cubismBridge = l2d.NewCubismBridge(endpoint, strings.TrimSpace(string(token)))
			s.cubismEndpoint = endpoint
		}
		status := s.cubismBridge.Status(r.Context())
		if err := s.saveCubismToken(); err != nil {
			fail(w, err)
			return
		}
		jsonOut(w, 200, status)
	})
	mux.HandleFunc("POST /api/l2d/cubism/command", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string          `json:"method"`
			Data   json.RawMessage `json:"data"`
		}
		if err := body(w, r, &req); err != nil {
			fail(w, err)
			return
		}
		s.cubismMu.Lock()
		defer s.cubismMu.Unlock()
		if s.cubismBridge == nil {
			fail(w, errors.New("connect to Cubism and allow plugin access first"))
			return
		}
		out, err := s.cubismBridge.Call(r.Context(), req.Method, req.Data)
		saveErr := s.saveCubismToken()
		if err != nil {
			fail(w, err)
			return
		}
		if saveErr != nil {
			fail(w, saveErr)
			return
		}
		jsonOut(w, 200, out)
	})
	mux.HandleFunc("POST /api/l2d/cubism/disconnect", func(w http.ResponseWriter, r *http.Request) {
		s.cubismMu.Lock()
		defer s.cubismMu.Unlock()
		if s.cubismBridge != nil {
			s.cubismBridge.Close()
			s.cubismBridge = nil
		}
		jsonOut(w, 200, map[string]string{"status": "disconnected"})
	})
}
func (s *Server) saveCubismToken() error {
	token := s.cubismBridge.Token()
	if token == "" {
		return nil
	}
	if err := os.WriteFile(filepath.Join(s.root, "cubism-token"), []byte(token), 0600); err != nil {
		return errors.New("cannot persist Cubism registration token")
	}
	return nil
}
