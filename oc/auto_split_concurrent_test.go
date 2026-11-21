package oc

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestAutoSplitConcurrentLimitAndRequestedOrder(t *testing.T) {
	s := New(t.TempDir())
	source, err := s.Import(fixture(t), "source")
	if err != nil {
		t.Fatal(err)
	}
	names := []string{"part0", "part1", "part2", "part3", "part4", "part5"}
	bounds := map[string]AutoSplitBounds{
		"part0": {X: 1, Y: 1, Width: 4, Height: 3},
		"part3": {X: 3, Y: 2, Width: 4, Height: 3},
	}
	gates := make([]chan struct{}, len(names))
	for i := range gates {
		gates[i] = make(chan struct{}, 1)
	}
	started := make(chan int, len(names))
	var active, maximum atomic.Int32
	calls := make([]atomic.Int32, len(names))
	payload := base64.StdEncoding.EncodeToString(autoSplitPNG(t, "outline"))
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Error(err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		defer r.MultipartForm.RemoveAll()
		index := -1
		for i, name := range names {
			if strings.Contains(r.FormValue("prompt"), fmt.Sprintf("%q", name)) {
				index = i
				break
			}
		}
		if index < 0 {
			t.Error("unknown part requested")
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		calls[index].Add(1)
		n := active.Add(1)
		for old := maximum.Load(); n > old && !maximum.CompareAndSwap(old, n); old = maximum.Load() {
		}
		started <- index
		<-gates[index]
		active.Add(-1)
		if index == 1 || index == 4 {
			w.WriteHeader(http.StatusUnprocessableEntity)
			return
		}
		fmt.Fprintf(w, `{"data":[{"b64_json":%q}]}`, payload)
	}))
	defer func() {
		for _, gate := range gates {
			close(gate)
		}
		provider.Close()
	}()
	type response struct {
		out AutoSplitResult
		err error
	}
	done := make(chan response, 1)
	go func() {
		out, err := s.AutoSplitWithBounds(context.Background(), Provider{BaseURL: provider.URL, Model: "mock"}, source.ID, names, bounds)
		done <- response{out, err}
	}()
	waitStart := func() int {
		t.Helper()
		select {
		case index := <-started:
			return index
		case <-time.After(5 * time.Second):
			t.Fatal("concurrent work did not start")
			return -1
		}
	}
	seen := map[int]bool{}
	for i := 0; i < 3; i++ {
		seen[waitStart()] = true
	}
	if !seen[0] || !seen[1] || !seen[2] || maximum.Load() != 3 {
		t.Fatalf("first wave was not exactly three overlapping calls: %v max=%d", seen, maximum.Load())
	}
	// Complete the first wave backwards. Each freed slot must start exactly the
	// next requested part, without waiting for an earlier slow part.
	for i, index := range []int{2, 1, 0} {
		gates[index] <- struct{}{}
		if got := waitStart(); got != i+3 {
			t.Fatalf("next part = %d, want %d", got, i+3)
		}
	}
	for _, index := range []int{5, 4, 3} {
		gates[index] <- struct{}{}
	}
	select {
	case got := <-done:
		if got.err != nil || len(got.out.Parts) != 4 || len(got.out.Failures) != 2 || len(got.out.Placements) != 4 {
			t.Fatalf("unexpected result: %+v %v", got.out, got.err)
		}
		for i, index := range []int{0, 2, 3, 5} {
			a := got.out.Parts[i]
			placement := got.out.Placements[a.ID]
			if a.Name != names[index] || placement.AlphaBounds.Width == 0 {
				t.Fatalf("part order or placement lost: %+v", got.out)
			}
			if rect, ok := bounds[a.Name]; ok {
				if placement.SourceBounds == nil || *placement.SourceBounds != rect {
					t.Fatalf("concurrent source bounds mixed up for %q: %+v", a.Name, placement)
				}
			} else if placement.SourceBounds != nil {
				t.Fatalf("unexpected source bounds for %q", a.Name)
			}
		}
		if got.out.Failures[0].Name != names[1] || got.out.Failures[1].Name != names[4] {
			t.Fatalf("failures not in requested order: %+v", got.out.Failures)
		}
		data, err := os.ReadFile(filepath.Join(s.Root, got.out.ID+".manifest.json"))
		var manifest AutoSplitResult
		if err != nil || json.Unmarshal(data, &manifest) != nil || len(manifest.Parts) != 4 || manifest.Parts[0].Name != names[0] || manifest.Failures[0].Name != names[1] {
			t.Fatalf("manifest missing ordered results: %s %v", data, err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("split did not finish")
	}
	if maximum.Load() != 3 || active.Load() != 0 {
		t.Fatalf("concurrency bound violated: max=%d active=%d", maximum.Load(), active.Load())
	}
	for i := range calls {
		if calls[i].Load() != 1 {
			t.Fatalf("part %q called %d times; no retries allowed", names[i], calls[i].Load())
		}
	}
}
