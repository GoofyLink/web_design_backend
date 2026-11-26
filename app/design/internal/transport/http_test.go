package transport

import (
	"looklook/app/design/internal/config"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOriginAndBodyBoundaries(t *testing.T) {
	a := New(config.Config{AllowedOrigins: []string{"http://localhost:5173"}}, nil, nil)
	h := a.Handler()
	r := httptest.NewRequest("OPTIONS", "/api/v1/auth/login", nil)
	r.Header.Set("Origin", "https://evil.example")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatal(w.Code)
	}
	r = httptest.NewRequest("OPTIONS", "/api/v1/auth/login", nil)
	r.Header.Set("Origin", "http://localhost:5173")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 204 || w.Header().Get("Access-Control-Allow-Origin") != "http://localhost:5173" {
		t.Fatal("valid preflight failed")
	}
	for _, body := range []string{`{"username":"a","password":"b","admin":true}`, `{"username":"a"} {"username":"b"}`, `null`} {
		r = httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		_, e := decode[LoginRequest](httptest.NewRecorder(), r)
		if e == nil {
			t.Fatal("accepted invalid body")
		}
	}
	// Registry has unique endpoints and OpenAPI covers every runtime route.
	seen := map[string]bool{}
	spec := a.OpenAPI()
	paths := spec["paths"].(map[string]any)
	for _, route := range a.Routes() {
		k := route.Method + route.Path
		if seen[k] {
			t.Fatal("duplicate route", k)
		}
		seen[k] = true
		if paths[apiPath(route.Path)] == nil {
			t.Fatal("undocumented route", k)
		}
	}
}
