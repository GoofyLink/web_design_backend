package transport

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/jpeg"
	"looklook/app/design/internal/modules/canvas"
	"looklook/app/design/internal/modules/catalog"
	"looklook/app/design/internal/platform"
	"os"
	"sync"
	"testing"
)

func TestContentTransactionRollback(t *testing.T) {
	a, db := contentTestAPI(t)
	ctx := context.Background()
	client := testClient{t, a.Handler()}
	client.expect(200, "POST", "/api/v1/auth/register", "", LoginRequest{"quota_user", "Password-123"})
	token, _ := client.login("user", "quota_user", "Password-123")
	admin, _ := client.login("admin", "root_admin", "Strong-test-123")
	p, e := a.Identity.Authenticate(ctx, "user", token)
	if e != nil {
		t.Fatal(e)
	}
	var data bytes.Buffer
	jpeg.Encode(&data, image.NewRGBA(image.Rect(0, 0, 8, 8)), nil)
	// Concurrent uploads must not both consume the last available quota.
	a.Assets.Limit = int64(data.Len())
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, e := a.Assets.Upload(ctx, p.ID, false, "image", "quota.jpg", "", bytes.NewReader(data.Bytes()), "quota")
			results <- e
		}()
	}
	wg.Wait()
	close(results)
	success, full := 0, 0
	for e := range results {
		if e == nil {
			success++
		} else if v, ok := e.(*platform.Error); ok && v.Status == 413 {
			full++
		} else {
			t.Fatal(e)
		}
	}
	if success != 1 || full != 1 {
		t.Fatal(success, full)
	}
	a.Assets.Limit = 0
	webp, e := os.ReadFile("testdata/solid.webp")
	if e != nil {
		t.Fatal(e)
	}
	uploadTest(t, a, token, false, "image", webp, 200)
	private := uploadTest(t, a, token, false, "image", data.Bytes(), 200)
	second := uploadTest(t, a, token, false, "image", data.Bytes(), 200)
	c := canvas.Content{Width: 100, Height: 100, Engine: "fabric", EngineVersion: "6.7.1", SchemaVersion: 1, Document: json.RawMessage(`{"objects":[{"type":"Image","assetId":"` + private.ID + `"}]}`)}
	design := keyedTest(t, a.Handler(), token, "/api/v1/designs", "quota-design", DesignRequest{Title: "Quota", Content: c}, 200)
	usage, e := a.Assets.Usage(ctx, p.ID)
	if e != nil {
		t.Fatal(e)
	}
	a.Workspace.Limit = usage["usedBytes"]
	changed := c
	changed.Document = json.RawMessage(`{"objects":[{"type":"Image","assetId":"` + second.ID + `"},{"type":"Rect","width":100,"height":100}]}`)
	client.expect(413, "PUT", "/api/v1/designs/"+design+"/document", token, SaveDesignRequest{1, changed})
	client.expect(409, "DELETE", "/api/v1/assets/"+private.ID, token, nil)
	client.expect(200, "DELETE", "/api/v1/assets/"+second.ID, token, nil)
	got, e := a.Workspace.Get(ctx, p.ID, mustID(t, design))
	if e != nil || got.Version != 1 {
		t.Fatal("rollback changed version", got.Version, e)
	}
	// A private upload may never be attached to a public template draft.
	category := objectID(t, client.expect(200, "POST", "/api/admin/v1/categories", admin, CategoryRequest{"Private rejection", 0, "active"}))
	content := catalog.Content{Title: "Invalid private", CategoryID: category, Width: 100, Height: 100, Cover: private.URL, CoverAssetID: private.ID, Engine: "fabric", EngineVersion: "6.7.1", SchemaVersion: 1, Document: c.Document}
	client.expect(404, "POST", "/api/admin/v1/templates", admin, TemplateRequest{Content: content})
	var n int
	if e = db.QueryRow(`SELECT COUNT(*) FROM design_templates`).Scan(&n); e != nil || n != 0 {
		t.Fatal("invalid template not rolled back", n, e)
	}
	// Persistence through a newly constructed API and real file reopen.
	again := New(a.Config, a.Identity, a.Catalog)
	if _, e = again.Workspace.Get(ctx, p.ID, mustID(t, design)); e != nil {
		t.Fatal(e)
	}
	_, f, e := again.Assets.Open(ctx, mustID(t, private.ID), p.ID, false)
	if e != nil {
		t.Fatal(e)
	}
	f.Close()
}
