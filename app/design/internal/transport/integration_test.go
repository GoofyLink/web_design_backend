package transport

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-sql-driver/mysql"
	"looklook/app/design/internal/config"
	"looklook/app/design/internal/modules/catalog"
	"looklook/app/design/internal/modules/identity"
	"looklook/app/design/internal/platform"
	"looklook/app/design/migrations"
)

type testClient struct {
	t *testing.T
	h http.Handler
}

func (c testClient) call(method, path, token string, body any, cookie *http.Cookie) (int, json.RawMessage, *httptest.ResponseRecorder) {
	c.t.Helper()
	var data []byte
	if body != nil {
		var e error
		data, e = json.Marshal(body)
		if e != nil {
			c.t.Fatal(e)
		}
	}
	req := httptest.NewRequest(method, path, bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "http://localhost:5173")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if cookie != nil {
		req.AddCookie(cookie)
	}
	w := httptest.NewRecorder()
	c.h.ServeHTTP(w, req)
	var out struct {
		Code      int             `json:"code"`
		Data      json.RawMessage `json:"data"`
		RequestID string          `json:"requestId"`
	}
	if e := json.Unmarshal(w.Body.Bytes(), &out); e != nil {
		c.t.Fatalf("%s %s invalid JSON %s", method, path, w.Body.String())
	}
	if out.RequestID == "" {
		c.t.Fatal("missing request ID")
	}
	if (w.Code == 200) != (out.Code == 0) {
		c.t.Fatal("HTTP/envelope inconsistency")
	}
	return w.Code, out.Data, w
}
func (c testClient) expect(want int, method, path, token string, body any) json.RawMessage {
	c.t.Helper()
	status, data, w := c.call(method, path, token, body, nil)
	if status != want {
		c.t.Fatalf("%s %s: want %d got %d %s", method, path, want, status, w.Body.String())
	}
	return data
}
func (c testClient) login(kind, username, password string) (string, *http.Cookie) {
	c.t.Helper()
	base := "/api/v1"
	if kind == "admin" {
		base = "/api/admin/v1"
	}
	status, data, w := c.call("POST", base+"/auth/login", "", LoginRequest{username, password}, nil)
	if status != 200 {
		c.t.Fatalf("login: %s", w.Body.String())
	}
	var out identity.Tokens
	json.Unmarshal(data, &out)
	cookies := w.Result().Cookies()
	if len(cookies) != 1 || !cookies[0].HttpOnly {
		c.t.Fatal("missing HttpOnly refresh cookie")
	}
	return out.AccessToken, cookies[0]
}
func objectID(t *testing.T, data json.RawMessage) string {
	t.Helper()
	var v struct {
		ID string `json:"id"`
	}
	if e := json.Unmarshal(data, &v); e != nil || v.ID == "" {
		t.Fatalf("missing ID: %s", data)
	}
	return v.ID
}
func TestIntegrationB1B2(t *testing.T) {
	dsn := os.Getenv("DESIGN_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("set DESIGN_TEST_MYSQL_DSN to a disposable MySQL server; test creates/drops its own design_it_* schema")
	}
	cfg, e := mysql.ParseDSN(dsn)
	if e != nil {
		t.Fatal(e)
	}
	admin, e := platform.Open(dsn)
	if e != nil {
		t.Fatal(e)
	}
	defer admin.Close()
	name := "design_it_" + platform.Random()[:12]
	if _, e = admin.Exec("CREATE DATABASE " + name + " CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci"); e != nil {
		t.Fatal(e)
	}
	defer admin.Exec("DROP DATABASE " + name)
	cfg.DBName = name
	db, e := platform.Open(cfg.FormatDSN())
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	ctx := context.Background()
	if e = migrations.Apply(ctx, db); e != nil {
		t.Fatal(e)
	}
	if e = migrations.Apply(ctx, db); e != nil {
		t.Fatalf("migration is not repeatable: %v", e)
	}
	c := config.Config{AllowPublicRegistration: true, JWTSecret: strings.Repeat("test-secret-", 4), StorageDir: t.TempDir(), AllowedOrigins: []string{"http://localhost:5173"}, AccessMinutes: 15, RefreshDays: 7}
	identities := &identity.Service{DB: db, Secret: []byte(c.JWTSecret), Access: 15 * time.Minute, Refresh: 7 * 24 * time.Hour}
	cat := &catalog.Service{DB: db}
	api := New(c, identities, cat)
	client := testClient{t, api.Handler()}
	adminID, e := identities.CreateAdmin(ctx, 0, "root_admin", "Strong-test-123", nil, true, "test-bootstrap")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = identities.CreateAdmin(ctx, 0, "root_admin2", "Strong-test-123", nil, true, "test-bootstrap"); e == nil {
		t.Fatal("bootstrap allowed twice")
	}
	client.expect(200, "GET", "/readyz", "", nil)
	uid := objectID(t, client.expect(200, "POST", "/api/v1/auth/register", "", LoginRequest{"alice", "Password-123"}))
	var storedPassword string
	if e = db.QueryRow(`SELECT password_hash FROM design_accounts WHERE id=?`, uid).Scan(&storedPassword); e != nil {
		t.Fatal(e)
	}
	if !strings.HasPrefix(storedPassword, "$2") || storedPassword == "Password-123" {
		t.Fatal("password is not bcrypt hashed")
	}
	client.expect(409, "POST", "/api/v1/auth/register", "", LoginRequest{"ALICE", "Password-123"})
	client.expect(400, "POST", "/api/v1/auth/register", "", map[string]any{"username": "bad", "password": "short"})
	client.expect(401, "POST", "/api/v1/auth/login", "", LoginRequest{"alice", "wrong-password"})
	user, userCookie := client.login("user", "alice", "Password-123")
	var storedRefresh string
	if e = db.QueryRow(`SELECT refresh_hash FROM design_sessions WHERE account_id=?`, uid).Scan(&storedRefresh); e != nil {
		t.Fatal(e)
	}
	if len(storedRefresh) != 64 || storedRefresh == userCookie.Value {
		t.Fatal("refresh token not stored as a digest")
	}
	super, adminCookie := client.login("admin", "root_admin", "Strong-test-123")
	if userCookie.Name == adminCookie.Name || userCookie.Path == adminCookie.Path {
		t.Fatal("user/admin cookie isolation failed")
	}
	client.expect(401, "GET", "/api/admin/v1/users", user, nil)
	client.expect(401, "GET", "/api/v1/me", super, nil)
	client.expect(200, "PATCH", "/api/v1/me", user, ProfileRequest{"Alice", ""})
	client.expect(200, "GET", "/api/admin/v1/user/info", super, nil)
	client.expect(409, "PATCH", "/api/admin/v1/admins/"+adminID+"/status", super, StatusRequest{"disabled"})
	rolesData := client.expect(200, "GET", "/api/admin/v1/roles", super, nil)
	var roles []identity.Role
	json.Unmarshal(rolesData, &roles)
	var superRole string
	for _, r := range roles {
		if r.Code == "super_admin" {
			superRole = r.ID
		}
	}
	editorRole := objectID(t, client.expect(200, "POST", "/api/admin/v1/roles", super, RoleRequest{"content_editor", "内容编辑", []string{"template:read", "template:write", "category:read"}}))
	client.expect(400, "POST", "/api/admin/v1/roles", super, RoleRequest{"bad_role", "错误角色", []string{"unknown:permission"}})
	editorID := objectID(t, client.expect(200, "POST", "/api/admin/v1/admins", super, AdminRequest{"editor", "Editor-pass-123", []string{editorRole}}))
	editor, _ := client.login("admin", "editor", "Editor-pass-123")
	client.expect(403, "GET", "/api/admin/v1/users", editor, nil)
	client.expect(409, "DELETE", "/api/admin/v1/roles/"+editorRole, super, nil)
	client.expect(409, "PUT", "/api/admin/v1/admins/"+adminID+"/roles", super, RolesRequest{[]string{editorRole}})
	category := objectID(t, client.expect(200, "POST", "/api/admin/v1/categories", super, CategoryRequest{"海报", 10, "active"}))
	content := catalog.Content{Title: "自制海报", CategoryID: category, Tags: []string{"示例"}, Width: 600, Height: 800}
	tid := objectID(t, client.expect(200, "POST", "/api/admin/v1/templates", super, TemplateRequest{Content: content}))
	client.expect(400, "POST", "/api/admin/v1/templates/"+tid+"/publish", super, VersionRequest{1})
	client.expect(404, "GET", "/api/v1/templates/"+tid, "", nil)
	content.Cover = "/api/v1/seed-assets/poster.svg"
	content.Engine = "fabric"
	content.EngineVersion = "6.0.0"
	content.SchemaVersion = 1
	content.Document = json.RawMessage(`{"objects":[{"type":"rect","width":600,"height":800,"fill":"#e7eef8"},{"type":"text","text":"DEMO","fontFamily":"sans-serif"}]}`)
	client.expect(200, "PUT", "/api/admin/v1/templates/"+tid, super, TemplateRequest{Version: 1, Content: content})
	client.expect(409, "PUT", "/api/admin/v1/templates/"+tid, super, TemplateRequest{Version: 1, Content: content})
	client.expect(403, "POST", "/api/admin/v1/templates/"+tid+"/publish", editor, VersionRequest{2})
	client.expect(200, "POST", "/api/admin/v1/templates/"+tid+"/publish", super, VersionRequest{2})
	public := client.expect(200, "GET", "/api/v1/templates/"+tid, "", nil)
	if bytes.Contains(public, []byte("document")) {
		t.Fatal("public endpoint leaked editor document")
	}
	content.Title = "尚未发布的新标题"
	client.expect(200, "PUT", "/api/admin/v1/templates/"+tid, editor, TemplateRequest{Version: 3, Content: content})
	public = client.expect(200, "GET", "/api/v1/templates/"+tid, "", nil)
	if bytes.Contains(public, []byte(content.Title)) {
		t.Fatal("draft leaked into public response")
	}
	list := client.expect(200, "GET", "/api/v1/templates?keyword=示例&ratio=portrait&categoryId="+category, "", nil)
	if !bytes.Contains(list, []byte(`"total":1`)) {
		t.Fatalf("search failed: %s", list)
	}
	client.expect(400, "GET", "/api/v1/templates?pageSize=101", "", nil)
	client.expect(409, "DELETE", "/api/admin/v1/categories/"+category, super, nil)
	client.expect(200, "PUT", "/api/admin/v1/categories/"+category, super, CategoryRequest{"海报", 10, "disabled"})
	client.expect(404, "GET", "/api/v1/templates/"+tid, "", nil)
	client.expect(200, "PUT", "/api/admin/v1/categories/"+category, super, CategoryRequest{"海报", 10, "active"})
	// Concurrent optimistic saves: exactly one wins, even across separate DB connections.
	numericID, _ := platform.ParseID(tid)
	actor, _ := platform.ParseID(adminID)
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, e := cat.SaveTemplate(ctx, actor, numericID, 4, content, "concurrent-save")
			errs <- e
		}()
	}
	wg.Wait()
	close(errs)
	success, conflicts := 0, 0
	for e := range errs {
		if e == nil {
			success++
		} else if p, ok := e.(*platform.Error); ok && p.Status == 409 {
			conflicts++
		} else {
			t.Fatal(e)
		}
	}
	if success != 1 || conflicts != 1 {
		t.Fatalf("save race: %d successes %d conflicts", success, conflicts)
	}
	status, data, w := client.call("POST", "/api/v1/auth/refresh", "", nil, userCookie)
	if status != 200 {
		t.Fatal(w.Body.String())
	}
	var refreshed identity.Tokens
	json.Unmarshal(data, &refreshed)
	nextCookie := w.Result().Cookies()[0]
	status, _, _ = client.call("POST", "/api/v1/auth/refresh", "", nil, userCookie)
	if status != 401 {
		t.Fatal("refresh replay accepted")
	}
	status, _, _ = client.call("POST", "/api/v1/auth/logout", "", nil, nextCookie)
	if status != 200 {
		t.Fatal("logout failed")
	}
	client.expect(401, "GET", "/api/v1/me", refreshed.AccessToken, nil)
	user, userCookie = client.login("user", "alice", "Password-123")
	client.expect(200, "PATCH", "/api/admin/v1/users/"+uid+"/status", super, StatusRequest{"disabled"})
	client.expect(401, "GET", "/api/v1/me", user, nil)
	status, _, _ = client.call("POST", "/api/v1/auth/refresh", "", nil, userCookie)
	if status != 401 {
		t.Fatal("disabled user refreshed")
	}
	client.expect(200, "PATCH", "/api/admin/v1/users/"+uid+"/status", super, StatusRequest{"active"})
	client.expect(401, "GET", "/api/v1/me", user, nil)
	user, _ = client.login("user", "alice", "Password-123")
	client.expect(200, "PUT", "/api/v1/me/password", user, PasswordRequest{"Password-123", "Updated-pass-123"})
	client.expect(401, "GET", "/api/v1/me", user, nil)
	client.expect(401, "POST", "/api/v1/auth/login", "", LoginRequest{"alice", "Password-123"})
	client.login("user", "alice", "Updated-pass-123")
	client.expect(200, "POST", "/api/admin/v1/templates/"+tid+"/offline", super, VersionRequest{5})
	client.expect(404, "GET", "/api/v1/templates/"+tid, "", nil)
	client.expect(200, "GET", "/api/admin/v1/audit-logs", super, nil)
	// Recreate the service and DB pool: persistence must not depend on process maps.
	reopened, e := platform.Open(cfg.FormatDSN())
	if e != nil {
		t.Fatal(e)
	}
	defer reopened.Close()
	identities2 := &identity.Service{DB: reopened, Secret: []byte(c.JWTSecret), Access: 15 * time.Minute, Refresh: 7 * 24 * time.Hour}
	fresh := testClient{t, New(c, identities2, &catalog.Service{DB: reopened}).Handler()}
	fresh.login("user", "alice", "Updated-pass-123")
	fresh.expect(200, "GET", "/api/admin/v1/templates/"+tid, super, nil)
	// Two concurrent demotions cannot eliminate the last active super administrator.
	second, e := identities.CreateAdmin(ctx, actor, "second_admin", "Second-pass-123", []int64{mustID(t, superRole)}, false, "test")
	if e != nil {
		t.Fatal(e)
	}
	errs2 := make(chan error, 2)
	for _, id := range []int64{actor, mustID(t, second)} {
		wg.Add(1)
		go func(n int64) {
			defer wg.Done()
			errs2 <- identities.SetStatus(ctx, actor, n, "admin", "disabled", "test")
		}(id)
	}
	wg.Wait()
	close(errs2)
	success, conflicts = 0, 0
	for e := range errs2 {
		if e == nil {
			success++
		} else if pe, ok := e.(*platform.Error); ok && pe.Status == 409 {
			conflicts++
		} else {
			t.Fatal(e)
		}
	}
	if success != 1 || conflicts != 1 {
		t.Fatal("last-admin protection failed")
	}
	_ = editorID
	// Fail readiness on incompatible schema instead of claiming healthy.
	if _, e = db.Exec(`UPDATE design_schema_migrations SET checksum='tampered'`); e != nil {
		t.Fatal(e)
	}
	client.expect(503, "GET", "/readyz", "", nil)
	var version string
	db.QueryRow("SELECT VERSION()").Scan(&version)
	t.Logf("B1/B2 integration passed on MySQL %s, disposable schema %s", version, name)
}
func mustID(t *testing.T, s string) int64 {
	t.Helper()
	id, e := platform.ParseID(s)
	if e != nil {
		t.Fatal(e)
	}
	return id
}
