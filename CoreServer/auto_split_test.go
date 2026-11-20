package coreserver

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAutoSplitRouteValidationAndBusy(t *testing.T) {
	s, err := New(t.TempDir(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	h := s.Handler()
	request := func(payload string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("POST", "http://127.0.0.1:17823/api/oc/auto-split", strings.NewReader(payload))
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	if w := request(`{"assetId":"../x","parts":[]}`); w.Code != 400 {
		t.Fatalf("invalid request: %d", w.Code)
	}
	if w := request(`{} {}`); w.Code != 400 {
		t.Fatalf("trailing JSON: %d", w.Code)
	}
	s.autoSplitMu.Lock()
	w := request(`{"assetId":"000000000000000000000000","parts":["head"]}`)
	s.autoSplitMu.Unlock()
	if w.Code != 409 {
		t.Fatalf("duplicate paid request not rejected: %d", w.Code)
	}
}
