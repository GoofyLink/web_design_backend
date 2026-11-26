package transport

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"github.com/go-sql-driver/mysql"
	"github.com/golang-jwt/jwt/v4"
	"golang.org/x/image/font/gofont/goregular"
	"image"
	"image/png"
	"looklook/app/design/internal/config"
	"looklook/app/design/internal/modules/asset"
	"looklook/app/design/internal/modules/canvas"
	"looklook/app/design/internal/modules/catalog"
	"looklook/app/design/internal/modules/identity"
	"looklook/app/design/internal/platform"
	"looklook/app/design/migrations"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func contentTestAPI(t *testing.T) (*API, *sql.DB) {
	t.Helper()
	dsn := os.Getenv("DESIGN_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("set DESIGN_TEST_MYSQL_DSN to run real database tests")
	}
	cfg, e := mysql.ParseDSN(dsn)
	if e != nil {
		t.Fatal(e)
	}
	root, e := platform.Open(dsn)
	if e != nil {
		t.Fatal(e)
	}
	name := "design_it_" + platform.Random()[:12]
	if _, e = root.Exec("CREATE DATABASE " + name + " CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci"); e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { root.Exec("DROP DATABASE " + name); root.Close() })
	cfg.DBName = name
	db, e := platform.Open(cfg.FormatDSN())
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { db.Close() })
	if e = migrations.Apply(context.Background(), db); e != nil {
		t.Fatal(e)
	}
	c := config.Config{AllowPublicRegistration: true, JWTSecret: strings.Repeat("test-secret-", 4), StorageDir: t.TempDir(), AllowedOrigins: []string{"http://localhost:5173"}}
	i := &identity.Service{DB: db, Secret: []byte(c.JWTSecret), Access: 15 * time.Minute, Refresh: 7 * 24 * time.Hour}
	a := New(c, i, &catalog.Service{DB: db})
	if _, e = i.CreateAdmin(context.Background(), 0, "root_admin", "Strong-test-123", nil, true, "bootstrap"); e != nil {
		t.Fatal(e)
	}
	return a, db
}
func uploadTest(t *testing.T, a *API, token string, admin bool, kind string, data []byte, want int) asset.Asset {
	t.Helper()
	var b bytes.Buffer
	m := multipart.NewWriter(&b)
	f, e := m.CreateFormFile("file", "test.bin")
	if e != nil {
		t.Fatal(e)
	}
	f.Write(data)
	m.WriteField("kind", kind)
	m.WriteField("license", "Go font BSD / test self-created image")
	m.Close()
	path := "/api/v1/assets"
	if admin {
		path = "/api/admin/v1/assets"
	}
	r := httptest.NewRequest("POST", path, &b)
	r.Header.Set("Content-Type", m.FormDataContentType())
	r.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	a.Handler().ServeHTTP(w, r)
	if w.Code != want {
		t.Fatalf("upload want=%d got=%d %s", want, w.Code, w.Body.String())
	}
	var out struct {
		Data asset.Asset `json:"data"`
	}
	json.Unmarshal(w.Body.Bytes(), &out)
	return out.Data
}
func keyedTest(t *testing.T, h http.Handler, token, path, key string, body any, want int) string {
	t.Helper()
	b, e := json.Marshal(body)
	if e != nil {
		t.Fatal(e)
	}
	r := httptest.NewRequest("POST", path, bytes.NewReader(b))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Authorization", "Bearer "+token)
	r.Header.Set("Idempotency-Key", key)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != want {
		t.Fatalf("%s want=%d got=%d %s", path, want, w.Code, w.Body.String())
	}
	if want != 200 {
		return ""
	}
	var out struct {
		Data json.RawMessage `json:"data"`
	}
	json.Unmarshal(w.Body.Bytes(), &out)
	return objectID(t, out.Data)
}
func TestIntegrationB3B4(t *testing.T) {
	a, db := contentTestAPI(t)
	ctx := context.Background()
	client := testClient{t, a.Handler()}
	client.expect(200, "POST", "/api/v1/auth/register", "", LoginRequest{"alice", "Password-123"})
	client.expect(200, "POST", "/api/v1/auth/register", "", LoginRequest{"bob", "Password-123"})
	user, cookie := client.login("user", "alice", "Password-123")
	other, _ := client.login("user", "bob", "Password-123")
	admin, _ := client.login("admin", "root_admin", "Strong-test-123")
	p, e := a.Identity.Authenticate(ctx, "user", user)
	if e != nil {
		t.Fatal(e)
	}
	var pngData bytes.Buffer
	png.Encode(&pngData, image.NewNRGBA(image.Rect(0, 0, 16, 16)))
	private := uploadTest(t, a, user, false, "image", pngData.Bytes(), 200)
	public := uploadTest(t, a, admin, true, "image", pngData.Bytes(), 200)
	font := uploadTest(t, a, admin, true, "font", goregular.TTF, 200)
	uploadTest(t, a, user, false, "font", goregular.TTF, 403)
	uploadTest(t, a, user, false, "image", []byte("<svg><script/></svg>"), 400)
	uploadTest(t, a, user, false, "image", pngData.Bytes()[:30], 400)
	uploadTest(t, a, admin, true, "font", []byte("not a font"), 400)
	uploadTest(t, a, user, false, "image", bytes.Repeat([]byte{'x'}, int(asset.MaxUpload)+1), 413)
	client.expect(404, "GET", "/api/v1/assets/"+private.ID, other, nil)
	client.expect(404, "GET", "/api/admin/v1/assets/"+private.ID, admin, nil)
	file := func(url, token string, want int) {
		t.Helper()
		r := httptest.NewRequest("GET", url, nil)
		if token != "" {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		w := httptest.NewRecorder()
		a.Handler().ServeHTTP(w, r)
		if w.Code != want {
			t.Fatalf("file want=%d got=%d %s", want, w.Code, w.Body.String())
		}
		if want == 200 && !bytes.Equal(w.Body.Bytes(), pngData.Bytes()) {
			t.Fatal("file byte integrity failed")
		}
	}
	file(private.URL, "", 404)
	file(private.URL, other, 404)
	file(private.URL, user, 200)
	file(public.URL, "", 200)
	var access struct {
		URL string `json:"url"`
	}
	json.Unmarshal(client.expect(200, "POST", "/api/v1/assets/"+private.ID+"/access-url", user, nil), &access)
	file(access.URL, "", 200)
	file(access.URL+"tampered", "", 401)
	file(strings.Replace(access.URL, "/"+private.ID+"/", "/"+public.ID+"/", 1), "", 401)
	expired, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, fileClaims{Asset: mustID(t, private.ID), Account: p.ID, Kind: p.Kind, Session: p.Session, RegisteredClaims: jwt.RegisteredClaims{Issuer: "design-file", Audience: jwt.ClaimStrings{"design-file"}, ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Minute))}}).SignedString(a.Identity.Secret)
	file(private.URL+"?ticket="+expired, "", 401)
	c := canvas.Content{Width: 600, Height: 800, Engine: "fabric", EngineVersion: "6.7.1", SchemaVersion: 1, Document: json.RawMessage(`{"version":"6.7.1","objects":[{"type":"Image","assetId":"` + private.ID + `","src":"asset:` + private.ID + `"},{"type":"Textbox","text":"设计 Hello","fontFamily":"asset-font-` + font.ID + `","fontAssetId":"` + font.ID + `"}]}`), CoverAssetID: private.ID}
	body := DesignRequest{Title: "My design", Content: c}
	design := keyedTest(t, a.Handler(), user, "/api/v1/designs", "create-0001", body, 200)
	if keyedTest(t, a.Handler(), user, "/api/v1/designs", "create-0001", body, 200) != design {
		t.Fatal("idempotency failed")
	}
	body.Title = "different"
	keyedTest(t, a.Handler(), user, "/api/v1/designs", "create-0001", body, 409)
	keyedTest(t, a.Handler(), other, "/api/v1/designs", "foreign-0001", body, 404)
	client.expect(404, "GET", "/api/v1/designs/"+design, other, nil)
	client.expect(409, "DELETE", "/api/v1/assets/"+private.ID, user, nil)
	client.expect(409, "PATCH", "/api/admin/v1/assets/"+font.ID+"/status", admin, StatusRequest{"disabled"})
	client.expect(200, "PUT", "/api/v1/designs/"+design+"/document", user, SaveDesignRequest{1, c})
	client.expect(409, "PUT", "/api/v1/designs/"+design+"/document", user, SaveDesignRequest{1, c})
	// Both writers start from the same version; exactly one commits.
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for n := 0; n < 2; n++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := a.Workspace.Save(ctx, p.ID, mustID(t, design), 2, c, "race")
			results <- err
		}()
	}
	wg.Wait()
	close(results)
	success, conflicts := 0, 0
	for err := range results {
		if err == nil {
			success++
		} else if err.Error() == "作品版本冲突，请重新加载或另存副本" {
			conflicts++
		} else {
			t.Fatal(err)
		}
	}
	if success != 1 || conflicts != 1 {
		t.Fatal("save concurrency", success, conflicts)
	}
	copyID := keyedTest(t, a.Handler(), user, "/api/v1/designs/"+design+"/copy", "copy-0001", CopyRequest{}, 200)
	client.expect(200, "PATCH", "/api/v1/designs/"+copyID, user, RenameDesignRequest{1, "Independent"})
	client.expect(200, "DELETE", "/api/v1/designs/"+design, user, VersionRequest{3})
	client.expect(404, "GET", "/api/v1/designs/"+design, user, nil)
	client.expect(200, "GET", "/api/v1/trash", user, nil)
	client.expect(200, "POST", "/api/v1/designs/"+design+"/restore", user, VersionRequest{4})
	client.expect(200, "GET", "/api/v1/designs/"+design, user, nil)
	client.expect(200, "DELETE", "/api/v1/designs/"+design, user, VersionRequest{5})
	client.expect(200, "DELETE", "/api/v1/trash/"+design, user, VersionRequest{6})
	client.expect(409, "DELETE", "/api/v1/assets/"+private.ID, user, nil)
	client.expect(200, "DELETE", "/api/v1/designs/"+copyID, user, VersionRequest{2})
	if _, e = db.Exec(`UPDATE design_designs SET deleted_at=DATE_SUB(UTC_TIMESTAMP(),INTERVAL 31 DAY) WHERE id=?`, copyID); e != nil {
		t.Fatal(e)
	}
	client.expect(409, "POST", "/api/v1/designs/"+copyID+"/restore", user, VersionRequest{3})
	if n, e := a.Workspace.Cleanup(ctx); e != nil || n != 1 {
		t.Fatal("trash cleanup", n, e)
	}
	client.expect(200, "DELETE", "/api/v1/assets/"+private.ID, user, nil)
	usage, e := a.Assets.Usage(ctx, p.ID)
	if e != nil || usage["usedBytes"] != 0 {
		t.Fatal("quota not released", usage, e)
	}
	// Template draft/publication/revision isolation and independent use.
	category := objectID(t, client.expect(200, "POST", "/api/admin/v1/categories", admin, CategoryRequest{"Poster", 1, "active"}))
	templateContent := catalog.Content{Title: "Published", CategoryID: category, Width: 600, Height: 800, Cover: public.URL, CoverAssetID: public.ID, Engine: "fabric", EngineVersion: "6.7.1", SchemaVersion: 1, Document: json.RawMessage(`{"objects":[{"type":"Image","assetId":"` + public.ID + `","src":"asset:` + public.ID + `"}]}`)}
	template := objectID(t, client.expect(200, "POST", "/api/admin/v1/templates", admin, TemplateRequest{Content: templateContent}))
	client.expect(200, "POST", "/api/admin/v1/templates/"+template+"/publish", admin, VersionRequest{1})
	used := keyedTest(t, a.Handler(), user, "/api/v1/templates/"+template+"/use", "template-0001", CopyRequest{}, 200)
	if keyedTest(t, a.Handler(), user, "/api/v1/templates/"+template+"/use", "template-0001", CopyRequest{}, 200) != used {
		t.Fatal("template retry")
	}
	var uses int
	if e = db.QueryRow(`SELECT use_count FROM design_templates WHERE id=?`, template).Scan(&uses); e != nil || uses != 1 {
		t.Fatal("use_count", uses, e)
	}
	templateContent.Title = "Draft only"
	client.expect(200, "PUT", "/api/admin/v1/templates/"+template, admin, TemplateRequest{3 - 1, templateContent})
	if strings.Contains(string(client.expect(200, "GET", "/api/v1/templates/"+template, "", nil)), "Draft only") {
		t.Fatal("draft leaked")
	}
	var revisions struct {
		Items []catalog.Revision `json:"items"`
	}
	json.Unmarshal(client.expect(200, "GET", "/api/admin/v1/templates/"+template+"/revisions", admin, nil), &revisions)
	if len(revisions.Items) != 1 {
		t.Fatal("revision missing")
	}
	revisionPath := "/api/admin/v1/templates/" + template + "/revisions/" + revisions.Items[0].ID
	raw := client.expect(200, "GET", revisionPath, admin, nil)
	if !strings.Contains(string(raw), "Published") {
		t.Fatal("revision changed")
	}
	client.expect(200, "GET", "/api/admin/v1/templates/"+template+"/preview", admin, nil)
	client.expect(200, "POST", "/api/admin/v1/templates/"+template+"/offline", admin, VersionRequest{3})
	keyedTest(t, a.Handler(), user, "/api/v1/templates/"+template+"/use", "template-0002", CopyRequest{}, 404)
	client.expect(200, "DELETE", "/api/admin/v1/templates/"+template, admin, VersionRequest{4})
	client.expect(200, "GET", "/api/v1/designs/"+used, user, nil)
	client.expect(409, "DELETE", "/api/admin/v1/assets/"+public.ID, admin, nil)
	client.expect(200, "DELETE", "/api/v1/designs/"+used, user, VersionRequest{1})
	client.expect(200, "DELETE", "/api/v1/trash/"+used, user, VersionRequest{2})
	client.expect(200, "DELETE", "/api/admin/v1/assets/"+public.ID, admin, nil)
	// DB failure compensates disk rename; exhausted quota is atomic.
	a.Assets.Limit = 1
	uploadTest(t, a, user, false, "image", pngData.Bytes(), 413)
	a.Assets.Limit = 0
	originalDB := a.Assets.DB
	dead, e := sql.Open("mysql", "root@tcp(127.0.0.1:1)/missing?timeout=50ms")
	if e != nil {
		t.Fatal(e)
	}
	a.Assets.DB = dead
	uploadTest(t, a, user, false, "image", pngData.Bytes(), 500)
	a.Assets.DB = originalDB
	dead.Close()
	entries, e := os.ReadDir(filepath.Join(a.Config.StorageDir, "files"))
	if e != nil {
		t.Fatal(e)
	}
	if len(entries) != 3 {
		t.Fatalf("failed upload left files: %d", len(entries))
	}
	orphan := filepath.Join(a.Config.StorageDir, "files", platform.Random())
	os.WriteFile(orphan, []byte("orphan"), 0600)
	old := time.Now().Add(-25 * time.Hour)
	os.Chtimes(orphan, old, old)
	if _, e = a.Assets.Cleanup(ctx); e != nil {
		t.Fatal(e)
	}
	entries, e = os.ReadDir(filepath.Join(a.Config.StorageDir, "files"))
	if e != nil || len(entries) != 1 {
		t.Fatal("cleanup should retain active font", len(entries), e)
	}
	client.expect(200, "GET", "/api/v1/fonts", "", nil)
	client.expect(200, "GET", "/api/v1/storage", user, nil)
	// A live resource ticket must stop working after session logout.
	remaining := uploadTest(t, a, user, false, "image", pngData.Bytes(), 200)
	json.Unmarshal(client.expect(200, "POST", "/api/v1/assets/"+remaining.ID+"/access-url", user, nil), &access)
	status, _, w := client.call("POST", "/api/v1/auth/logout", user, nil, cookie)
	if status != 200 {
		t.Fatal(w.Body.String())
	}
	file(access.URL, "", 401)
}
