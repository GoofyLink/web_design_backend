package transport

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// Static hosting is opt-in. API paths never fall back to a SPA page.
func (a *API) NotFoundHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fallback := a.ErrorHandler(404, "接口不存在")
		if (r.Method != "GET" && r.Method != "HEAD") || r.URL.Path == "/api" || strings.HasPrefix(r.URL.Path, "/api/") || r.URL.Path == "/healthz" || r.URL.Path == "/readyz" {
			fallback.ServeHTTP(w, r)
			return
		}
		if a.serveDocs(w, r) {
			return
		}
		root := a.Config.SiteDir
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "admin" || strings.HasPrefix(path, "admin/") {
			root = a.Config.AdminDir
			path = strings.TrimPrefix(strings.TrimPrefix(path, "admin"), "/")
		}
		if root == "" {
			fallback.ServeHTTP(w, r)
			return
		}
		for _, part := range strings.FieldsFunc(path, func(r rune) bool { return r == '/' || r == '\\' }) {
			if part == ".." || strings.Contains(part, ":") || strings.HasPrefix(part, ".") {
				fallback.ServeHTTP(w, r)
				return
			}
		}
		base, e := filepath.EvalSymlinks(root)
		if e != nil {
			fallback.ServeHTTP(w, r)
			return
		}
		base, e = filepath.Abs(base)
		if e != nil {
			fallback.ServeHTTP(w, r)
			return
		}
		candidate := filepath.Join(base, filepath.FromSlash(path))
		stat, e := os.Stat(candidate)
		if e != nil || stat.IsDir() {
			if filepath.Ext(path) != "" {
				fallback.ServeHTTP(w, r)
				return
			}
			candidate = filepath.Join(base, "index.html")
		}
		actual, e := filepath.EvalSymlinks(candidate)
		if e != nil {
			fallback.ServeHTTP(w, r)
			return
		}
		relative, e := filepath.Rel(base, actual)
		if e != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			fallback.ServeHTTP(w, r)
			return
		}
		f, e := os.Open(actual)
		if e != nil {
			fallback.ServeHTTP(w, r)
			return
		}
		defer f.Close()
		info, e := f.Stat()
		if e != nil || !info.Mode().IsRegular() {
			fallback.ServeHTTP(w, r)
			return
		}
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Cache-Control", "no-cache")
		http.ServeContent(w, r, info.Name(), info.ModTime(), f)
	})
}
