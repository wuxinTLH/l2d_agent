package coreserver

import (
	"errors"
	"local/l2dagent/l2d"
	"local/l2dagent/oc"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sync"
)

var exportName = regexp.MustCompile(`^[a-f0-9]{24}\.psd$`)

// Serializes export and deletion so selected image files cannot disappear mid-export.
var assetWorkflowMu sync.Mutex

func (s *Server) assetWorkflow(mux *http.ServeMux) {
	mux.HandleFunc("DELETE /api/assets/{id}", func(w http.ResponseWriter, r *http.Request) {
		assetWorkflowMu.Lock()
		defer assetWorkflowMu.Unlock()
		if err := s.assets.Delete(r.PathValue("id")); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				jsonOut(w, 404, map[string]string{"error": "asset not found"})
			} else {
				fail(w, err)
			}
			return
		}
		jsonOut(w, 200, map[string]string{"status": "deleted", "id": r.PathValue("id")})
	})
	mux.HandleFunc("POST /api/oc/export-psd", func(w http.ResponseWriter, r *http.Request) {
		var req oc.PSDRequest
		if err := body(w, r, &req); err != nil {
			fail(w, err)
			return
		}
		assetWorkflowMu.Lock()
		defer assetWorkflowMu.Unlock()
		result, err := s.assets.ExportPSD(req)
		if err != nil {
			fail(w, err)
			return
		}
		jsonOut(w, 201, result)
	})
	mux.HandleFunc("GET /exports/{name}", func(w http.ResponseWriter, r *http.Request) {
		name := r.PathValue("name")
		if !exportName.MatchString(name) {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "image/vnd.adobe.photoshop")
		w.Header().Set("Content-Disposition", `attachment; filename="`+name+`"`)
		http.ServeFile(w, r, filepath.Join(s.assets.Root, "exports", name))
	})
	mux.HandleFunc("POST /api/l2d/cubism/open-psd", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ExportID string `json:"exportId"`
		}
		if err := body(w, r, &req); err != nil {
			fail(w, err)
			return
		}
		if !exportName.MatchString(req.ExportID + ".psd") {
			fail(w, errors.New("invalid export id"))
			return
		}
		path, err := filepath.Abs(filepath.Join(s.assets.Root, "exports", req.ExportID+".psd"))
		if err != nil {
			fail(w, err)
			return
		}
		if err = l2d.OpenCubismPSD(s.snapshot().CubismExecutable, path); err != nil {
			fail(w, err)
			return
		}
		jsonOut(w, 200, map[string]string{"status": "started", "note": "PSD path forwarded through the official editor document launcher; confirm Model Settings in Cubism. If no import dialog appears, download PSD and use File > Open. Import and model creation are not verified."})
	})
}
