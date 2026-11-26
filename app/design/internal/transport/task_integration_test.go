package transport

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"image"
	"image/png"
	"looklook/app/design/internal/modules/canvas"
	"looklook/app/design/internal/modules/task"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestIntegrationB8(t *testing.T) {
	a, db := contentTestAPI(t)
	ctx := context.Background()
	a.Tasks.AllowPrivate = true
	client := testClient{t, a.Handler()}
	uid := objectID(t, client.expect(200, "POST", "/api/v1/auth/register", "", LoginRequest{"task_user", "Password-123"}))
	client.expect(200, "POST", "/api/v1/auth/register", "", LoginRequest{"task_other", "Password-123"})
	user, _ := client.login("user", "task_user", "Password-123")
	other, _ := client.login("user", "task_other", "Password-123")
	admin, _ := client.login("admin", "root_admin", "Strong-test-123")
	owner := mustID(t, uid)
	var pngData bytes.Buffer
	png.Encode(&pngData, image.NewNRGBA(image.Rect(0, 0, 256, 256)))
	var calls atomic.Int32
	var mode atomic.Int32
	started, release := make(chan struct{}, 1), make(chan struct{})
	providerServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Header.Get("Authorization") != "Bearer test-provider-secret" || !strings.HasPrefix(r.Header.Get("Idempotency-Key"), "design-job-") {
			http.Error(w, "bad auth/key", 400)
			return
		}
		switch mode.Load() {
		case 1:
			http.Error(w, "busy", 503)
			return
		case 2:
			started <- struct{}{}
			<-release
		case 3:
			json.NewEncoder(w).Encode(map[string]any{"data": []any{map[string]any{"b64_json": "invalid"}}})
			return
		}
		if strings.Contains(r.URL.Path, "renderer") {
			json.NewEncoder(w).Encode(map[string]any{"imageBase64": base64.StdEncoding.EncodeToString(pngData.Bytes())})
		} else {
			json.NewEncoder(w).Encode(map[string]any{"data": []any{map[string]any{"b64_json": base64.StdEncoding.EncodeToString(pngData.Bytes())}}})
		}
	}))
	defer providerServer.Close()
	config := task.ProviderInput{Name: "Test image", Kind: "image", Endpoint: providerServer.URL + "/images/generations", Model: "test-image", APIKey: "test-provider-secret", Status: "disabled"}
	provider := objectID(t, client.expect(200, "POST", "/api/admin/v1/providers", admin, config))
	request := task.CreateRequest{ProviderID: provider, Kind: "ai-image", Prompt: "Internal test", Size: 256}
	keyedTest(t, a.Handler(), user, "/api/v1/jobs", "disabled-job-001", request, 503)
	if calls.Load() != 0 {
		t.Fatal("disabled provider was called")
	}
	configs := client.expect(200, "GET", "/api/admin/v1/providers", admin, nil)
	if strings.Contains(string(configs), "test-provider-secret") || strings.Contains(string(configs), "key_cipher") {
		t.Fatal("provider secret leaked")
	}
	var stored string
	if e := db.QueryRow(`SELECT key_cipher FROM design_providers WHERE id=?`, provider).Scan(&stored); e != nil || stored == "test-provider-secret" || stored == "" {
		t.Fatal("secret not encrypted", e)
	}
	config.Status = "active"
	config.Version = 1
	config.APIKey = ""
	client.expect(200, "PUT", "/api/admin/v1/providers/"+provider, admin, config)
	keyedTest(t, a.Handler(), user, "/api/v1/jobs", "no-quota-job", request, 409)
	client.expect(200, "PUT", "/api/admin/v1/users/"+uid+"/task-quota", admin, QuotaRequest{10})
	client.expect(401, "GET", "/api/admin/v1/providers", user, nil)
	job := keyedTest(t, a.Handler(), user, "/api/v1/jobs", "image-job-001", request, 200)
	if keyedTest(t, a.Handler(), user, "/api/v1/jobs", "image-job-001", request, 200) != job {
		t.Fatal("job dedupe")
	}
	changed := request
	changed.Prompt = "different"
	keyedTest(t, a.Handler(), user, "/api/v1/jobs", "image-job-001", changed, 409)
	client.expect(404, "GET", "/api/v1/jobs/"+job, other, nil)
	claimed, e := a.Tasks.Claim(ctx)
	if e != nil || claimed.ID != job {
		t.Fatal("claim", claimed.ID, e)
	}
	if e = a.Tasks.Execute(ctx, claimed); e != nil {
		t.Fatal(e)
	}
	if e = a.Tasks.Finish(ctx, claimed, nil); e != nil {
		t.Fatal(e)
	}
	if e = a.Tasks.Finish(ctx, claimed, nil); e == nil {
		t.Fatal("double completion accepted")
	}
	done, e := a.Tasks.Get(ctx, owner, mustID(t, job), false)
	if e != nil || done.Status != "succeeded" || len(done.Results) != 1 || done.CompletedItems != 1 {
		t.Fatal("job result", done, e)
	}
	client.expect(409, "DELETE", "/api/v1/assets/"+done.Results[0].ID, user, nil)
	q, e := a.Tasks.Quota(ctx, owner)
	if e != nil || q["reservedUnits"] != 0 || q["consumedUnits"] != 1 {
		t.Fatal("quota consume", q, e)
	}
	// Successful outputs remain in the private asset library when task metadata is removed.
	client.expect(200, "DELETE", "/api/v1/jobs/"+job, user, nil)
	client.expect(200, "GET", "/api/v1/assets/"+done.Results[0].ID, user, nil)
	client.expect(200, "DELETE", "/api/v1/assets/"+done.Results[0].ID, user, nil)
	// Cancellation during an upstream response must reject the stale output commit.
	mode.Store(2)
	cancelJob := keyedTest(t, a.Handler(), user, "/api/v1/jobs", "cancel-running", request, 200)
	claimed, e = a.Tasks.Claim(ctx)
	if e != nil {
		t.Fatal(e)
	}
	result := make(chan error, 1)
	go func() { result <- a.Tasks.Execute(ctx, claimed) }()
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("upstream not called")
	}
	client.expect(200, "POST", "/api/v1/jobs/"+cancelJob+"/cancel", user, nil)
	close(release)
	if e = <-result; e == nil {
		t.Fatal("cancelled execution committed")
	}
	if e = a.Tasks.Finish(ctx, claimed, nil); e == nil {
		t.Fatal("cancelled execution finalized")
	}
	q, e = a.Tasks.Quota(ctx, owner)
	if e != nil || q["reservedUnits"] != 0 || q["consumedUnits"] != 1 {
		t.Fatal("cancel refund", q, e)
	}
	mode.Store(0)
	// Lease recovery uses a new fencing token. A stale worker cannot finish.
	leaseJob := keyedTest(t, a.Handler(), user, "/api/v1/jobs", "lease-job-001", request, 200)
	old, e := a.Tasks.Claim(ctx)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = db.Exec(`UPDATE design_jobs SET lease_until=DATE_SUB(UTC_TIMESTAMP(),INTERVAL 1 SECOND) WHERE id=?`, leaseJob); e != nil {
		t.Fatal(e)
	}
	fresh, e := a.Tasks.Claim(ctx)
	if e != nil || fresh.ID != old.ID || fresh.Lease == old.Lease || fresh.Attempts != 2 {
		t.Fatal("lease reclaim", fresh, e)
	}
	if e = a.Tasks.Execute(ctx, old); e == nil {
		t.Fatal("stale worker executed")
	}
	if e = a.Tasks.Execute(ctx, fresh); e != nil {
		t.Fatal(e)
	}
	if e = a.Tasks.Finish(ctx, fresh, nil); e != nil {
		t.Fatal(e)
	}
	client.expect(200, "GET", "/api/admin/v1/jobs/"+leaseJob+"/attempts", admin, nil)
	// Retryable upstream errors reserve once, then release once at retry exhaustion.
	mode.Store(1)
	retryJob := keyedTest(t, a.Handler(), user, "/api/v1/jobs", "retry-job-001", request, 200)
	for i := 0; i < 3; i++ {
		claimed, e = a.Tasks.Claim(ctx)
		if e != nil || claimed.ID != retryJob {
			t.Fatal("retry claim", claimed, e)
		}
		cause := a.Tasks.Execute(ctx, claimed)
		if cause == nil {
			t.Fatal("503 accepted")
		}
		if e = a.Tasks.Finish(ctx, claimed, cause); e != nil {
			t.Fatal(e)
		}
		if _, e = db.Exec(`UPDATE design_jobs SET lease_until=NULL WHERE id=?`, retryJob); e != nil {
			t.Fatal(e)
		}
	}
	failed, e := a.Tasks.Get(ctx, owner, mustID(t, retryJob), false)
	if e != nil || failed.Status != "failed" {
		t.Fatal("retry exhaustion", failed, e)
	}
	q, e = a.Tasks.Quota(ctx, owner)
	if e != nil || q["reservedUnits"] != 0 || q["consumedUnits"] != 2 {
		t.Fatal("retry refund", q, e)
	}
	client.expect(200, "POST", "/api/admin/v1/jobs/"+retryJob+"/retry", admin, nil)
	mode.Store(0)
	claimed, e = a.Tasks.Claim(ctx)
	if e != nil {
		t.Fatal(e)
	}
	if e = a.Tasks.Execute(ctx, claimed); e != nil {
		t.Fatal(e)
	}
	if e = a.Tasks.Finish(ctx, claimed, nil); e != nil {
		t.Fatal(e)
	}
	// Batch snapshots survive source deletion and return a ZIP of validated images.
	rendererConfig := task.ProviderInput{Name: "Test renderer", Kind: "renderer", Endpoint: providerServer.URL + "/renderer", APIKey: "test-provider-secret", Status: "active"}
	renderer := objectID(t, client.expect(200, "POST", "/api/admin/v1/providers", admin, rendererConfig))
	content := canvas.Content{Width: 256, Height: 256, Engine: "fabric", EngineVersion: "6.7.1", SchemaVersion: 1, Document: json.RawMessage(`{"objects":[{"type":"Rect","width":100,"height":100}]}`)}
	design := keyedTest(t, a.Handler(), user, "/api/v1/designs", "batch-source", DesignRequest{Title: "Batch", Content: content}, 200)
	batchRequest := task.CreateRequest{ProviderID: renderer, Kind: "batch-export", Format: "png", Items: []task.Item{{DesignID: design, Version: 1}}}
	batch := keyedTest(t, a.Handler(), user, "/api/v1/jobs", "batch-job-001", batchRequest, 200)
	client.expect(200, "DELETE", "/api/v1/designs/"+design, user, VersionRequest{1})
	client.expect(200, "DELETE", "/api/v1/trash/"+design, user, VersionRequest{2})
	claimed, e = a.Tasks.Claim(ctx)
	if e != nil || claimed.ID != batch {
		t.Fatal("batch claim", claimed, e)
	}
	if e = a.Tasks.Execute(ctx, claimed); e != nil {
		t.Fatal(e)
	}
	if e = a.Tasks.Finish(ctx, claimed, nil); e != nil {
		t.Fatal(e)
	}
	download := httptest.NewRequest("GET", "/api/v1/jobs/"+batch+"/download", nil)
	download.Header.Set("Authorization", "Bearer "+user)
	w := httptest.NewRecorder()
	a.Handler().ServeHTTP(w, download)
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	archive, e := zip.NewReader(bytes.NewReader(w.Body.Bytes()), int64(w.Body.Len()))
	if e != nil || len(archive.File) != 1 {
		t.Fatal("invalid ZIP", e)
	}
	// Private result bodies stay hidden from admin inspection.
	detail := client.expect(200, "GET", "/api/admin/v1/jobs/"+batch, admin, nil)
	if strings.Contains(string(detail), "results") || strings.Contains(string(detail), "document") {
		t.Fatal("private result leaked to admin")
	}
	client.expect(200, "GET", "/api/v1/job-capabilities", user, nil)
	client.expect(200, "GET", "/api/v1/task-quota", user, nil)
	client.expect(200, "GET", "/api/admin/v1/jobs", admin, nil)
}
