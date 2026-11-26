package transport

import (
	"context"
	"looklook/app/design/internal/modules/task"
	"looklook/app/design/internal/platform"
	"sync"
	"testing"
)

func TestTaskConcurrentQuotaAndLease(t *testing.T) {
	a, db := contentTestAPI(t)
	ctx := context.Background()
	client := testClient{t, a.Handler()}
	uid := objectID(t, client.expect(200, "POST", "/api/v1/auth/register", "", LoginRequest{"lease_user", "Password-123"}))
	admin, _ := client.login("admin", "root_admin", "Strong-test-123")
	provider := objectID(t, client.expect(200, "POST", "/api/admin/v1/providers", admin, task.ProviderInput{Name: "Configured but never called", Kind: "image", Endpoint: "https://example.invalid/v1/images/generations", Model: "example", APIKey: "not-a-real-key", Status: "active"}))
	owner := mustID(t, uid)
	client.expect(200, "PUT", "/api/admin/v1/users/"+uid+"/task-quota", admin, QuotaRequest{1})
	var wg sync.WaitGroup
	results := make(chan error, 2)
	jobs := make(chan task.Job, 2)
	for _, key := range []string{"concurrent-job-a", "concurrent-job-b"} {
		key := key
		wg.Add(1)
		go func() {
			defer wg.Done()
			j, e := a.Tasks.Create(ctx, owner, task.CreateRequest{ProviderID: provider, Kind: "ai-image", Prompt: "test", Size: 256}, key, "race")
			jobs <- j
			results <- e
		}()
	}
	wg.Wait()
	close(results)
	close(jobs)
	success, full := 0, 0
	for e := range results {
		if e == nil {
			success++
		} else if v, ok := e.(*platform.Error); ok && v.Status == 409 {
			full++
		} else {
			t.Fatal(e)
		}
	}
	if success != 1 || full != 1 {
		t.Fatal("overbooked quota", success, full)
	}
	claims := make(chan task.Job, 2)
	errs := make(chan error, 2)
	for n := 0; n < 2; n++ {
		wg.Add(1)
		go func() { defer wg.Done(); j, e := a.Tasks.Claim(ctx); claims <- j; errs <- e }()
	}
	wg.Wait()
	close(claims)
	close(errs)
	for e := range errs {
		if e != nil {
			t.Fatal(e)
		}
	}
	var claim task.Job
	claimed := 0
	for j := range claims {
		if j.ID != "" {
			claimed++
			claim = j
		}
	}
	if claimed != 1 {
		t.Fatal("multiple workers claimed same job")
	}
	for i := 0; i < 3; i++ {
		if _, e := db.Exec(`UPDATE design_jobs SET lease_until=DATE_SUB(UTC_TIMESTAMP(),INTERVAL 1 SECOND) WHERE id=?`, claim.ID); e != nil {
			t.Fatal(e)
		}
		j, e := a.Tasks.Claim(ctx)
		if e != nil {
			t.Fatal(e)
		}
		if i < 2 && j.ID == "" {
			t.Fatal("lease not recovered")
		}
		if i == 2 && j.ID != "" {
			t.Fatal("exhausted job claimed")
		}
	}
	q, e := a.Tasks.Quota(ctx, owner)
	if e != nil || q["reservedUnits"] != 0 || q["consumedUnits"] != 0 {
		t.Fatal("lease exhaustion leaked quota", q, e)
	}
	j, e := a.Tasks.Get(ctx, owner, mustID(t, claim.ID), false)
	if e != nil || j.Status != "failed" {
		t.Fatal("lease failure", j, e)
	}
}
