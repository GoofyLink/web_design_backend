package task

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"looklook/app/design/internal/modules/asset"
	"looklook/app/design/internal/modules/canvas"
	"looklook/app/design/internal/platform"
	"net/http"
	"time"
)

type remoteError struct {
	code  string
	retry bool
}

func (e *remoteError) Error() string { return e.code }
func (s *Service) Claim(ctx context.Context) (out Job, err error) {
	err = platform.Tx(ctx, s.DB, func(t *sql.Tx) error {
		if e := platform.LockContent(ctx, t); e != nil {
			return e
		}
		var id int64
		var maximum int
		e := t.QueryRowContext(ctx, `SELECT id,max_attempts FROM design_jobs WHERE (status='queued' AND (lease_until IS NULL OR lease_until<UTC_TIMESTAMP(6))) OR (status='running' AND lease_until<UTC_TIMESTAMP(6)) ORDER BY id LIMIT 1`).Scan(&id, &maximum)
		if e != nil {
			return e
		}
		out, e = scanJob(t.QueryRowContext(ctx, `SELECT `+jobColumns+` FROM design_jobs WHERE id=?`, id))
		if e != nil {
			return e
		}
		if out.Status == "running" {
			if _, e = t.ExecContext(ctx, `UPDATE design_job_attempts SET status='expired',error_code='lease_expired',finished_at=UTC_TIMESTAMP(6) WHERE job_id=? AND status='running'`, id); e != nil {
				return e
			}
		}
		var accountStatus, providerStatus string
		if e = t.QueryRowContext(ctx, `SELECT a.status,p.status FROM design_accounts a JOIN design_providers p ON p.id=? WHERE a.id=?`, out.ProviderID, out.Owner).Scan(&accountStatus, &providerStatus); e != nil {
			return e
		}
		if out.Attempts >= maximum || accountStatus != "active" || providerStatus != "active" {
			code := "attempts_exhausted"
			if accountStatus != "active" {
				code = "account_disabled"
			}
			if providerStatus != "active" {
				code = "provider_disabled"
			}
			if _, e = t.ExecContext(ctx, `UPDATE design_jobs SET status='failed',error_code=?,lease_token=NULL,lease_until=NULL,updated_at=UTC_TIMESTAMP(6) WHERE id=?`, code, id); e != nil {
				return e
			}
			if _, e = t.ExecContext(ctx, `UPDATE design_task_quota SET reserved_units=reserved_units-? WHERE owner_id=?`, out.Units, out.Owner); e != nil {
				return e
			}
			out = Job{}
			return nil
		}
		out.Attempts++
		out.Lease = platform.Random()
		out.Status = "running"
		if e = t.QueryRowContext(ctx, `SELECT payload_json FROM design_jobs WHERE id=?`, id).Scan(&out.Payload); e != nil {
			return e
		}
		if _, e = t.ExecContext(ctx, `UPDATE design_jobs SET status='running',attempts=attempts+1,lease_token=?,lease_until=DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 120 SECOND),updated_at=UTC_TIMESTAMP(6) WHERE id=?`, out.Lease, id); e != nil {
			return e
		}
		_, e = t.ExecContext(ctx, `INSERT INTO design_job_attempts(job_id,attempt,status,started_at) VALUES(?,?,'running',UTC_TIMESTAMP(6))`, id, out.Attempts)
		return e
	})
	if errors.Is(err, sql.ErrNoRows) {
		return Job{}, nil
	}
	var pe *platform.Error
	if errors.As(err, &pe) && pe.Status == 404 {
		return Job{}, nil
	}
	return
}
func (s *Service) leased(ctx context.Context, t *sql.Tx, j Job) error {
	var n int
	return t.QueryRowContext(ctx, `SELECT 1 FROM design_jobs WHERE id=? AND status='running' AND lease_token=? AND lease_until>UTC_TIMESTAMP(6)`, j.ID, j.Lease).Scan(&n)
}
func (s *Service) renew(ctx context.Context, j Job) error {
	return platform.Tx(ctx, s.DB, func(t *sql.Tx) error {
		if e := platform.LockContent(ctx, t); e != nil {
			return e
		}
		if e := s.leased(ctx, t, j); e != nil {
			return e
		}
		var status string
		if e := t.QueryRowContext(ctx, `SELECT status FROM design_accounts WHERE id=?`, j.Owner).Scan(&status); e != nil {
			return e
		}
		if status != "active" {
			return &remoteError{"account_disabled", false}
		}
		_, e := t.ExecContext(ctx, `UPDATE design_jobs SET lease_until=DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 120 SECOND) WHERE id=?`, j.ID)
		return e
	})
}
func (s *Service) Finish(ctx context.Context, j Job, cause error) error {
	return platform.Tx(ctx, s.DB, func(t *sql.Tx) error {
		if e := platform.LockContent(ctx, t); e != nil {
			return e
		}
		if e := s.leased(ctx, t, j); e != nil {
			return e
		}
		status, code := "succeeded", ""
		retry := false
		if cause != nil {
			status = "failed"
			code = "execution_failed"
			var re *remoteError
			if errors.As(cause, &re) {
				code = re.code
				retry = re.retry
			}
			var maximum int
			if e := t.QueryRowContext(ctx, `SELECT max_attempts FROM design_jobs WHERE id=?`, j.ID).Scan(&maximum); e != nil {
				return e
			}
			retry = retry && j.Attempts < maximum
		}
		attemptStatus := status
		if retry {
			status = "queued"
		}
		if _, e := t.ExecContext(ctx, `UPDATE design_job_attempts SET status=?,error_code=?,finished_at=UTC_TIMESTAMP(6) WHERE job_id=? AND attempt=?`, attemptStatus, code, j.ID, j.Attempts); e != nil {
			return e
		}
		var next any
		if retry {
			next = time.Now().UTC().Add(time.Duration(j.Attempts*5) * time.Second)
		}
		if _, e := t.ExecContext(ctx, `UPDATE design_jobs SET status=?,error_code=?,lease_token=NULL,lease_until=?,updated_at=UTC_TIMESTAMP(6) WHERE id=?`, status, code, next, j.ID); e != nil {
			return e
		}
		if !retry {
			consumed := int64(0)
			if status == "succeeded" {
				consumed = j.Units
			}
			if _, e := t.ExecContext(ctx, `UPDATE design_task_quota SET reserved_units=reserved_units-?,consumed_units=consumed_units+? WHERE owner_id=?`, j.Units, consumed, j.Owner); e != nil {
				return e
			}
		}
		return platform.Audit(ctx, t, j.Owner, "job."+status, j.ID, "worker")
	})
}
func (s *Service) remote(ctx context.Context, p Provider, body any, key string) ([]byte, error) {
	if e := s.endpoint(p.Endpoint); e != nil {
		return nil, &remoteError{"provider_invalid", false}
	}
	secret, e := s.decrypt(p.cipher)
	if e != nil {
		return nil, &remoteError{"provider_key_invalid", false}
	}
	raw, e := json.Marshal(body)
	if e != nil {
		return nil, e
	}
	req, e := http.NewRequestWithContext(ctx, "POST", p.Endpoint, bytes.NewReader(raw))
	if e != nil {
		return nil, e
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", key)
	if secret != "" {
		req.Header.Set("Authorization", "Bearer "+secret)
	}
	client := s.httpClient()
	if s.Client == nil {
		defer client.CloseIdleConnections()
	}
	resp, e := client.Do(req)
	if e != nil {
		return nil, &remoteError{"upstream_delivery_unknown", false}
	}
	defer resp.Body.Close()
	if resp.StatusCode == 429 || resp.StatusCode == 503 {
		return nil, &remoteError{"upstream_busy", true}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &remoteError{"upstream_rejected", false}
	}
	data, e := io.ReadAll(io.LimitReader(resp.Body, 30*1024*1024+1))
	if e != nil || len(data) > 30*1024*1024 {
		return nil, &remoteError{"upstream_response_invalid", false}
	}
	var encoded string
	if p.Kind == "image" {
		var result struct {
			Data []struct {
				Image string `json:"b64_json"`
			} `json:"data"`
		}
		if json.Unmarshal(data, &result) != nil || len(result.Data) != 1 {
			return nil, &remoteError{"upstream_image_missing", false}
		}
		encoded = result.Data[0].Image
	} else {
		var result struct {
			Image string `json:"imageBase64"`
		}
		if json.Unmarshal(data, &result) != nil {
			return nil, &remoteError{"renderer_response_invalid", false}
		}
		encoded = result.Image
	}
	decoded, e := base64.StdEncoding.DecodeString(encoded)
	if e != nil || len(decoded) == 0 || len(decoded) > int(asset.MaxUpload) {
		return nil, &remoteError{"upstream_image_invalid", false}
	}
	return decoded, nil
}
func (s *Service) resources(ctx context.Context, owner int64, c canvas.Content) (map[string]any, error) {
	refs, e := canvas.Validate(c)
	if e != nil {
		return nil, e
	}
	out := map[string]any{}
	var total int64
	for _, ref := range refs {
		meta, f, e := s.Assets.Open(ctx, ref.ID, owner, false)
		if e != nil {
			return nil, e
		}
		total += meta.Size
		if total > 20*1024*1024 {
			f.Close()
			return nil, &remoteError{"renderer_input_too_large", false}
		}
		b, e := io.ReadAll(io.LimitReader(f, asset.MaxUpload+1))
		f.Close()
		if e != nil {
			return nil, e
		}
		if int64(len(b)) != meta.Size {
			return nil, &remoteError{"resource_size_changed", false}
		}
		out[meta.ID] = map[string]any{"mime": meta.MIME, "base64": base64.StdEncoding.EncodeToString(b), "fontFamily": meta.FontFamily}
	}
	return out, nil
}
func (s *Service) Execute(ctx context.Context, j Job) error {
	var data payload
	if e := json.Unmarshal(j.Payload, &data); e != nil {
		return e
	}
	count := 1
	if j.Kind == "batch-export" {
		count = len(data.Documents)
	}
	for index := 0; index < count; index++ {
		if e := s.renew(ctx, j); e != nil {
			return e
		}
		var exists int
		e := s.DB.QueryRowContext(ctx, `SELECT 1 FROM design_job_outputs WHERE job_id=? AND item_index=?`, j.ID, index).Scan(&exists)
		if e == nil {
			continue
		}
		if e != sql.ErrNoRows {
			return e
		}
		p, e := scanProvider(s.DB.QueryRowContext(ctx, `SELECT `+providerCols+` FROM design_providers WHERE id=?`, j.ProviderID))
		if e != nil {
			return e
		}
		if p.Status != "active" {
			return &remoteError{"provider_disabled", false}
		}
		var body any
		width, height := data.Request.Size, data.Request.Size
		format := "png"
		if j.Kind == "ai-image" {
			body = map[string]any{"model": p.Model, "prompt": data.Request.Prompt, "size": fmt.Sprintf("%dx%d", width, height), "n": 1, "response_format": "b64_json"}
		} else {
			c := data.Documents[index]
			resources, e := s.resources(ctx, j.Owner, c)
			if e != nil {
				return e
			}
			width, height = c.Width, c.Height
			format = data.Request.Format
			body = map[string]any{"content": c, "resources": resources, "format": format, "schemaVersion": 1}
		}
		image, e := s.remote(ctx, p, body, fmt.Sprintf("design-job-%s-item-%d", j.ID, index))
		if e != nil {
			return e
		}
		_, e = s.Assets.UploadWithCommit(ctx, j.Owner, false, "image", fmt.Sprintf("job-%s-%d.%s", j.ID, index+1, format), "", bytes.NewReader(image), "job-worker", func(t *sql.Tx, meta asset.Asset) error {
			if e := s.leased(ctx, t, j); e != nil {
				return e
			}
			if meta.Width != width || meta.Height != height {
				return &remoteError{"upstream_dimensions_invalid", false}
			}
			if meta.MIME != "image/"+format {
				return &remoteError{"renderer_format_invalid", false}
			}
			if _, e := t.ExecContext(ctx, `INSERT INTO design_job_outputs(job_id,item_index,asset_id) VALUES(?,?,?)`, j.ID, index, meta.ID); e != nil {
				return e
			}
			_, e := t.ExecContext(ctx, `INSERT INTO design_asset_references(asset_id,object_kind,object_id) VALUES(?,'job_result',?)`, meta.ID, j.ID)
			return e
		})
		if e != nil {
			return e
		}
	}
	return nil
}
func (s *Service) Run(ctx context.Context) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			claimCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
			j, e := s.Claim(claimCtx)
			cancel()
			if e != nil {
				log.Printf("job claim failed: %v", e)
				continue
			}
			if j.ID == "" {
				continue
			}
			run, stop := context.WithTimeout(ctx, 15*time.Minute)
			cause := s.Execute(run, j)
			stop()
			if ctx.Err() != nil {
				return
			}
			finish, done := context.WithTimeout(context.Background(), 15*time.Second)
			e = s.Finish(finish, j, cause)
			done()
			if e != nil {
				log.Printf("job %s finalization deferred to lease recovery", j.ID)
			}
		}
	}
}
