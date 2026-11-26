package transport

import (
	"encoding/json"
	"looklook/app/design/internal/config"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestEmbeddedDocs(t *testing.T) {
	a := New(config.Config{}, nil, nil)
	h := a.Handler()
	for _, path := range []string{"/docs", "/docs/", "/openapi.json"} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		if w.Code != 200 {
			t.Fatalf("%s: %d", path, w.Code)
		}
		if path == "/openapi.json" {
			var spec struct {
				Paths map[string]map[string]any `json:"paths"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &spec); err != nil {
				t.Fatal(err)
			}
			count := 0
			for _, methods := range spec.Paths {
				count += len(methods)
			}
			if count != len(a.Routes()) {
				t.Fatalf("missing operations: %d", count)
			}
		} else if strings.Count(w.Body.String(), "<details>") != len(a.Routes()) {
			t.Fatal("missing documentation entries")
		}
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/", nil))
	if w.Code != 302 || w.Header().Get("Location") != "/docs" {
		t.Fatal("missing root redirect")
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/docs?q=not-an-endpoint", nil))
	if strings.Contains(w.Body.String(), "<details>") {
		t.Fatal("search did not filter")
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("POST", "/docs", nil))
	if w.Code != 404 {
		t.Fatal("unexpected method accepted")
	}
}
