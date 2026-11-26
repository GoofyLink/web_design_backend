package transport

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/png"
	"looklook/app/design/internal/modules/canvas"
	"looklook/app/design/internal/modules/catalog"
	"looklook/app/design/internal/modules/operation"
	"looklook/app/design/internal/modules/workspace"
	"strings"
	"testing"
	"time"
)

func TestIntegrationB5B6(t *testing.T) {
	a, db := contentTestAPI(t)
	ctx := context.Background()
	client := testClient{t, a.Handler()}
	client.expect(200, "POST", "/api/v1/auth/register", "", LoginRequest{"ops_user", "Password-123"})
	client.expect(200, "POST", "/api/v1/auth/register", "", LoginRequest{"ops_other", "Password-123"})
	user, _ := client.login("user", "ops_user", "Password-123")
	other, _ := client.login("user", "ops_other", "Password-123")
	admin, _ := client.login("admin", "root_admin", "Strong-test-123")
	p, e := a.Identity.Authenticate(ctx, "user", user)
	if e != nil {
		t.Fatal(e)
	}
	var imageData bytes.Buffer
	png.Encode(&imageData, image.NewNRGBA(image.Rect(0, 0, 64, 64)))
	first := uploadTest(t, a, user, false, "image", imageData.Bytes(), 200)
	cover := uploadTest(t, a, user, false, "image", imageData.Bytes(), 200)
	c := canvas.Content{Width: 64, Height: 64, Engine: "fabric", EngineVersion: "6.7.1", SchemaVersion: 1, Document: json.RawMessage(`{"objects":[{"type":"Image","assetId":"` + first.ID + `"}]}`)}
	design := keyedTest(t, a.Handler(), user, "/api/v1/designs", "operations-create", DesignRequest{Title: "Operations", Content: c}, 200)
	var check struct {
		CheckID string `json:"checkId"`
	}
	json.Unmarshal(client.expect(200, "POST", "/api/v1/designs/"+design+"/export-check", user, operation.ExportRequest{Version: 1, Format: "png", Scale: 1}), &check)
	if check.CheckID == "" {
		t.Fatal("missing export check")
	}
	event := operation.ExportEvent{EventID: "export-event-001", CheckID: check.CheckID, Status: "succeeded"}
	client.expect(200, "POST", "/api/v1/export-events", user, event)
	client.expect(200, "POST", "/api/v1/export-events", user, event)
	event.Status = "failed"
	client.expect(409, "POST", "/api/v1/export-events", user, event)
	client.expect(404, "POST", "/api/v1/designs/"+design+"/export-check", other, operation.ExportRequest{Version: 1, Format: "png", Scale: 1})
	client.expect(409, "POST", "/api/v1/designs/"+design+"/export-check", user, operation.ExportRequest{Version: 9, Format: "png", Scale: 1})
	client.expect(200, "GET", "/api/admin/v1/export-events", admin, nil)
	stats := client.expect(200, "GET", "/api/admin/v1/statistics", admin, nil)
	if !strings.Contains(string(stats), `"exports_succeeded":1`) {
		t.Fatal(string(stats))
	}
	client.expect(401, "GET", "/api/admin/v1/statistics", user, nil)
	client.expect(200, "POST", "/api/v1/designs/"+design+"/cover", user, CoverRequest{1, cover.ID})
	client.expect(409, "POST", "/api/v1/designs/"+design+"/cover", user, CoverRequest{1, cover.ID})
	client.expect(404, "POST", "/api/v1/designs/"+design+"/cover", other, CoverRequest{2, cover.ID})
	blank := c
	blank.Document = json.RawMessage(`{"objects":[]}`)
	client.expect(200, "PUT", "/api/v1/designs/"+design+"/document", user, SaveDesignRequest{2, blank})
	client.expect(409, "DELETE", "/api/v1/assets/"+first.ID, user, nil)
	var history []workspace.History
	json.Unmarshal(client.expect(200, "GET", "/api/v1/designs/"+design+"/history", user, nil), &history)
	if len(history) != 2 {
		t.Fatal("history missing")
	}
	h := history[len(history)-1].ID
	historyPath := "/api/v1/designs/" + design + "/history/" + h
	client.expect(404, "GET", historyPath, other, nil)
	client.expect(200, "GET", historyPath, user, nil)
	client.expect(200, "POST", historyPath+"/restore", user, VersionRequest{3})
	client.expect(409, "POST", historyPath+"/restore", user, VersionRequest{3})
	folder := objectID(t, client.expect(200, "POST", "/api/v1/folders", user, FolderRequest{Name: "Work"}))
	client.expect(200, "PUT", "/api/v1/designs/"+design+"/folder", user, MoveRequest{4, folder})
	client.expect(404, "GET", "/api/v1/folders/"+folder+"/designs", other, nil)
	client.expect(404, "PUT", "/api/v1/designs/"+design+"/folder", other, MoveRequest{5, folder})
	client.expect(200, "GET", "/api/v1/folders/"+folder+"/designs", user, nil)
	client.expect(200, "DELETE", "/api/v1/folders/"+folder, user, VersionRequest{1})
	client.expect(200, "GET", "/api/v1/designs/"+design, user, nil)
	version := int64(5)
	for n := 0; n < 22; n++ {
		result, e := a.Workspace.Save(ctx, p.ID, mustID(t, design), version, blank, "history-trim")
		if e != nil {
			t.Fatal(e)
		}
		version = result.Version
	}
	history, e = a.Workspace.History(ctx, p.ID, mustID(t, design))
	if e != nil || len(history) != 20 {
		t.Fatal("history cap", len(history), e)
	}
	client.expect(200, "DELETE", "/api/v1/assets/"+first.ID, user, nil)
	client.expect(200, "DELETE", "/api/v1/assets/"+cover.ID, user, nil)
	category := objectID(t, client.expect(200, "POST", "/api/admin/v1/categories", admin, CategoryRequest{"Ops", 0, "active"}))
	content := catalog.Content{Title: "Topic template", CategoryID: category, Width: 64, Height: 64, Cover: "/api/v1/seed-assets/poster.svg", Engine: "fabric", EngineVersion: "6.7.1", SchemaVersion: 1, Document: json.RawMessage(`{"objects":[{"type":"Rect"}]}`)}
	template := objectID(t, client.expect(200, "POST", "/api/admin/v1/templates", admin, TemplateRequest{Content: content}))
	client.expect(200, "POST", "/api/admin/v1/templates/"+template+"/publish", admin, VersionRequest{1})
	client.expect(200, "PUT", "/api/v1/favorites/"+template, user, nil)
	client.expect(200, "PUT", "/api/v1/favorites/"+template, user, nil)
	topicInput := operation.TopicInput{Title: "Featured", Description: "Internal", Status: "published", Slot: "home", StartsAt: time.Now().Add(-time.Hour).UTC().Format(time.RFC3339), EndsAt: time.Now().Add(time.Hour).UTC().Format(time.RFC3339), TemplateIDs: []string{template}}
	topic := objectID(t, client.expect(200, "POST", "/api/admin/v1/topics", admin, topicInput))
	client.expect(200, "GET", "/api/v1/topics/"+topic, "", nil)
	home := client.expect(200, "GET", "/api/v1/home", "", nil)
	if !strings.Contains(string(home), "Featured") {
		t.Fatal("home missing topic")
	}
	topicInput.Version = 1
	topicInput.StartsAt = time.Now().Add(time.Hour).UTC().Format(time.RFC3339)
	topicInput.EndsAt = time.Now().Add(2 * time.Hour).UTC().Format(time.RFC3339)
	client.expect(200, "PUT", "/api/admin/v1/topics/"+topic, admin, topicInput)
	client.expect(404, "GET", "/api/v1/topics/"+topic, "", nil)
	client.expect(409, "PUT", "/api/admin/v1/topics/"+topic, admin, topicInput)
	client.expect(200, "POST", "/api/admin/v1/templates/"+template+"/offline", admin, VersionRequest{2})
	var favorites struct {
		Total int64 `json:"total"`
	}
	json.Unmarshal(client.expect(200, "GET", "/api/v1/favorites", user, nil), &favorites)
	if favorites.Total != 0 {
		t.Fatal("offline favorite leaked")
	}
	topicInput.Version = 2
	topicInput.Status = "offline"
	client.expect(200, "PUT", "/api/admin/v1/topics/"+topic, admin, topicInput)
	client.expect(200, "DELETE", "/api/admin/v1/topics/"+topic, admin, VersionRequest{3})
	// Event checks expire and ownership remains enforced.
	json.Unmarshal(client.expect(200, "POST", "/api/v1/designs/"+design+"/export-check", user, operation.ExportRequest{Version: version, Format: "jpeg", Scale: 1}), &check)
	if _, e = db.Exec(`UPDATE design_export_checks SET expires_at=DATE_SUB(UTC_TIMESTAMP(),INTERVAL 1 SECOND) WHERE id=?`, check.CheckID); e != nil {
		t.Fatal(e)
	}
	client.expect(409, "POST", "/api/v1/export-events", user, operation.ExportEvent{EventID: "expired-event", CheckID: check.CheckID, Status: "failed"})
	client.expect(200, "DELETE", "/api/v1/designs/"+design, user, VersionRequest{version})
	client.expect(200, "DELETE", "/api/v1/trash/"+design, user, VersionRequest{version + 1})
	usage, e := a.Assets.Usage(ctx, p.ID)
	if e != nil || usage["usedBytes"] != 0 {
		t.Fatal("history purge leaked quota", usage, e)
	}
}
