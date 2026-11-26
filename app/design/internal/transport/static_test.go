package transport

import (
	"looklook/app/design/internal/config"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestStaticHostingDoesNotCaptureAPI(t *testing.T) {
	site := t.TempDir()
	admin := t.TempDir()
	os.WriteFile(filepath.Join(site, "index.html"), []byte("user-site"), 0600)
	os.WriteFile(filepath.Join(admin, "index.html"), []byte("admin-site"), 0600)
	a := New(config.Config{SiteDir: site, AdminDir: admin}, nil, nil)
	for _, v := range []struct {
		path   string
		status int
		body   string
	}{{"/", 200, "user-site"}, {"/design/editor", 200, "user-site"}, {"/admin/users", 200, "admin-site"}, {"/api/v1/missing", 404, ""}, {"/assets/missing.js", 404, ""}, {"/.env", 404, ""}, {"/../outside", 404, ""}} {
		w := httptest.NewRecorder()
		a.Handler().ServeHTTP(w, httptest.NewRequest("GET", v.path, nil))
		if w.Code != v.status || (v.body != "" && w.Body.String() != v.body) {
			t.Fatalf("%s: %d %s", v.path, w.Code, w.Body.String())
		}
	}
}
