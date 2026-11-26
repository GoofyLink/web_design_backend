package task

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"looklook/app/design/internal/modules/asset"
	"looklook/app/design/internal/modules/canvas"
	"looklook/app/design/internal/modules/operation"
	"looklook/app/design/internal/platform"
	"strings"
	"time"
)

type Item struct {
	DesignID string `json:"designId"`
	Version  int64  `json:"version"`
}
type CreateRequest struct {
	ProviderID string `json:"providerId"`
	Kind       string `json:"kind"`
	Prompt     string `json:"prompt,omitempty"`
	Size       int    `json:"size,omitempty"`
	Format     string `json:"format,omitempty"`
	Items      []Item `json:"items,omitempty"`
}
type payload struct {
	Request   CreateRequest    `json:"request"`
	Documents []canvas.Content `json:"documents,omitempty"`
}
type Job struct {
	CompletedItems int64         `json:"completedItems"`
	TotalItems     int64         `json:"totalItems"`
	ID             string        `json:"id"`
	OwnerID        string        `json:"ownerId"`
	Kind           string        `json:"kind"`
	ProviderID     string        `json:"providerId"`
	Status         string        `json:"status"`
	Units          int64         `json:"units"`
	Attempts       int           `json:"attempts"`
	ErrorCode      string        `json:"errorCode"`
	CreatedAt      time.Time     `json:"createdAt"`
	UpdatedAt      time.Time     `json:"updatedAt"`
	Results        []asset.Asset `json:"results,omitempty"`
	Owner          int64         `json:"-"`
	Payload        []byte        `json:"-"`
	Lease          string        `json:"-"`
}

const jobColumns = `id,owner_id,kind,provider_id,status,units,attempts,error_code,created_at,updated_at,(SELECT COUNT(*) FROM design_job_outputs WHERE job_id=design_jobs.id)`

func scanJob(row interface{ Scan(...any) error }) (Job, error) {
	var j Job
	var id, provider int64
	e := row.Scan(&id, &j.Owner, &j.Kind, &provider, &j.Status, &j.Units, &j.Attempts, &j.ErrorCode, &j.CreatedAt, &j.UpdatedAt, &j.CompletedItems)
	j.TotalItems = j.Units
	j.ID = platform.ID(id)
	j.OwnerID = platform.ID(j.Owner)
	j.ProviderID = platform.ID(provider)
	return j, platform.Normalize(e)
}
func (s *Service) Quota(ctx context.Context, owner int64) (map[string]int64, error) {
	out := map[string]int64{"limitUnits": 0, "reservedUnits": 0, "consumedUnits": 0}
	var limit, reserved, consumed int64
	e := s.DB.QueryRowContext(ctx, `SELECT limit_units,reserved_units,consumed_units FROM design_task_quota WHERE owner_id=?`, owner).Scan(&limit, &reserved, &consumed)
	if e == sql.ErrNoRows {
		e = nil
	}
	out["limitUnits"], out["reservedUnits"], out["consumedUnits"] = limit, reserved, consumed
	out["availableUnits"] = limit - reserved - consumed
	return out, e
}
func (s *Service) SetQuota(ctx context.Context, actor, owner, limit int64, rid string) error {
	if limit < 0 || limit > 1000000000 {
		return platform.Fail(400, "额度须为 0—10 亿单位")
	}
	return platform.Tx(ctx, s.DB, func(t *sql.Tx) error {
		if e := platform.LockContent(ctx, t); e != nil {
			return e
		}
		var n int
		if e := t.QueryRowContext(ctx, `SELECT 1 FROM design_accounts WHERE id=? AND kind='user'`, owner).Scan(&n); e != nil {
			return e
		}
		if _, e := t.ExecContext(ctx, `INSERT IGNORE INTO design_task_quota(owner_id) VALUES(?)`, owner); e != nil {
			return e
		}
		var used int64
		if e := t.QueryRowContext(ctx, `SELECT reserved_units+consumed_units FROM design_task_quota WHERE owner_id=?`, owner).Scan(&used); e != nil {
			return e
		}
		if limit < used {
			return platform.Fail(409, "新额度不能小于已用加预留")
		}
		if _, e := t.ExecContext(ctx, `UPDATE design_task_quota SET limit_units=? WHERE owner_id=?`, limit, owner); e != nil {
			return e
		}
		return platform.Audit(ctx, t, actor, "task.quota", platform.ID(owner), rid)
	})
}
func reserve(ctx context.Context, t *sql.Tx, owner, units int64) error {
	r, e := t.ExecContext(ctx, `UPDATE design_task_quota SET reserved_units=reserved_units+? WHERE owner_id=? AND limit_units-consumed_units-reserved_units>=?`, units, owner, units)
	if e != nil {
		return e
	}
	n, e := r.RowsAffected()
	if e != nil {
		return e
	}
	if n != 1 {
		return platform.Fail(409, "任务额度不足，请联系管理员分配")
	}
	return nil
}
func (s *Service) Create(ctx context.Context, owner int64, v CreateRequest, key, rid string) (out Job, err error) {
	if !operation.KeyPattern.MatchString(key) {
		return out, platform.Fail(400, "需要 8—100 位 Idempotency-Key")
	}
	provider, e := platform.ParseID(v.ProviderID)
	if e != nil {
		return out, e
	}
	if v.Kind != "ai-image" && v.Kind != "batch-export" {
		return out, platform.Fail(400, "任务类型仅 ai-image/batch-export")
	}
	if v.Kind == "ai-image" {
		if strings.TrimSpace(v.Prompt) == "" || len([]rune(v.Prompt)) > 2000 || len(v.Items) > 0 || (v.Size != 256 && v.Size != 512 && v.Size != 1024) || v.Format != "" {
			return out, platform.Fail(400, "图片任务需 prompt 和 size=256/512/1024")
		}
	} else {
		if len(v.Items) < 1 || len(v.Items) > 10 || v.Prompt != "" || v.Size != 0 || (v.Format != "png" && v.Format != "jpeg") {
			return out, platform.Fail(400, "批量导出需 1—10 个作品及 png/jpeg 格式")
		}
	}
	raw, _ := json.Marshal(v)
	fingerprint := fmt.Sprintf("%x", sha256.Sum256(raw))
	err = platform.Tx(ctx, s.DB, func(t *sql.Tx) error {
		if e := platform.LockContent(ctx, t); e != nil {
			return e
		}
		var previous, hash string
		e := t.QueryRowContext(ctx, `SELECT CAST(id AS CHAR),request_hash FROM design_jobs WHERE owner_id=? AND request_key=?`, owner, key).Scan(&previous, &hash)
		if e == nil {
			if hash != fingerprint {
				return platform.Fail(409, "幂等键对应不同任务参数")
			}
			out, e = scanJob(t.QueryRowContext(ctx, `SELECT `+jobColumns+` FROM design_jobs WHERE id=?`, previous))
			return e
		}
		if e != sql.ErrNoRows {
			return e
		}
		p, e := scanProvider(t.QueryRowContext(ctx, `SELECT `+providerCols+` FROM design_providers WHERE id=?`, provider))
		if e != nil {
			return e
		}
		if p.Status != "active" {
			return platform.Fail(503, "服务商未启用")
		}
		if (v.Kind == "ai-image" && p.Kind != "image") || (v.Kind == "batch-export" && p.Kind != "renderer") {
			return platform.Fail(400, "服务商类型不匹配")
		}
		if p.Kind == "image" && !p.HasKey {
			return platform.Fail(503, "图片服务尚未配置密钥")
		}
		var pending int
		if e = t.QueryRowContext(ctx, `SELECT COUNT(*) FROM design_jobs WHERE owner_id=? AND status IN ('queued','running')`, owner).Scan(&pending); e != nil {
			return e
		}
		if pending >= 10 {
			return platform.Fail(429, "最多同时保留 10 个未完成任务")
		}
		data := payload{Request: v}
		refs := []canvas.Reference{}
		seen := map[int64]bool{}
		for _, item := range v.Items {
			id, e := platform.ParseID(item.DesignID)
			if e != nil {
				return e
			}
			if seen[id] {
				return platform.Fail(400, "批量任务作品重复")
			}
			seen[id] = true
			var version int64
			var doc []byte
			if e = t.QueryRowContext(ctx, `SELECT version,content_json FROM design_designs WHERE id=? AND owner_id=? AND deleted_at IS NULL`, id, owner).Scan(&version, &doc); e != nil {
				return e
			}
			if version != item.Version {
				return platform.Fail(409, "批量作品版本冲突")
			}
			var c canvas.Content
			if e = json.Unmarshal(doc, &c); e != nil {
				return e
			}
			r, e := canvas.Validate(c)
			if e != nil {
				return e
			}
			refs = append(refs, r...)
			data.Documents = append(data.Documents, c)
		}
		encoded, e := json.Marshal(data)
		if e != nil {
			return e
		}
		if len(encoded) > 5*1024*1024 {
			return platform.Fail(413, "任务快照总量不能超过 5 MiB")
		}
		units := int64(1)
		if v.Kind == "batch-export" {
			units = int64(len(v.Items))
		}
		if e = reserve(ctx, t, owner, units); e != nil {
			return e
		}
		if e = platform.Quota(ctx, t, owner, int64(len(encoded)), s.Assets.QuotaLimit()); e != nil {
			return e
		}
		r, e := t.ExecContext(ctx, `INSERT INTO design_jobs(owner_id,kind,provider_id,request_key,request_hash,payload_json,byte_size,units,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,UTC_TIMESTAMP(6),UTC_TIMESTAMP(6))`, owner, v.Kind, provider, key, fingerprint, string(encoded), len(encoded), units)
		if e != nil {
			return e
		}
		id, e := r.LastInsertId()
		if e != nil {
			return e
		}
		if e = asset.SyncReferences(ctx, t, "job_input", id, owner, false, refs); e != nil {
			return e
		}
		out, e = scanJob(t.QueryRowContext(ctx, `SELECT `+jobColumns+` FROM design_jobs WHERE id=?`, id))
		if e != nil {
			return e
		}
		return platform.Audit(ctx, t, owner, "job.create", out.ID, rid)
	})
	return
}
func (s *Service) List(ctx context.Context, owner int64, admin bool, p platform.Page) (platform.List, error) {
	where := ` WHERE 1=1`
	args := []any{}
	if !admin {
		where += ` AND owner_id=?`
		args = append(args, owner)
	}
	if p.Status != "" {
		switch p.Status {
		case "queued", "running", "succeeded", "failed", "cancelled":
		default:
			return platform.List{}, platform.Fail(400, "任务状态无效")
		}
		where += ` AND status=?`
		args = append(args, p.Status)
	}
	out := platform.List{Page: p.Number, PageSize: p.Size, Items: []Job{}}
	if e := s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM design_jobs`+where, args...).Scan(&out.Total); e != nil {
		return out, e
	}
	rows, e := s.DB.QueryContext(ctx, `SELECT `+jobColumns+` FROM design_jobs`+where+` ORDER BY id DESC LIMIT ? OFFSET ?`, append(args, p.Size, p.Offset())...)
	if e != nil {
		return out, e
	}
	defer rows.Close()
	items := []Job{}
	for rows.Next() {
		j, e := scanJob(rows)
		if e != nil {
			return out, e
		}
		items = append(items, j)
	}
	out.Items = items
	return out, rows.Err()
}
func (s *Service) Get(ctx context.Context, owner, id int64, admin bool) (Job, error) {
	q := `SELECT ` + jobColumns + ` FROM design_jobs WHERE id=?`
	args := []any{id}
	if !admin {
		q += ` AND owner_id=?`
		args = append(args, owner)
	}
	j, e := scanJob(s.DB.QueryRowContext(ctx, q, args...))
	if e != nil || admin {
		return j, e
	}
	rows, e := s.DB.QueryContext(ctx, `SELECT asset_id FROM design_job_outputs WHERE job_id=? ORDER BY item_index`, id)
	if e != nil {
		return j, e
	}
	ids := []int64{}
	for rows.Next() {
		var n int64
		if e = rows.Scan(&n); e != nil {
			rows.Close()
			return j, e
		}
		ids = append(ids, n)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return j, e
	}
	for _, n := range ids {
		v, e := s.Assets.Get(ctx, n, owner, false)
		if e != nil {
			return j, e
		}
		j.Results = append(j.Results, v)
	}
	return j, nil
}
func (s *Service) Action(ctx context.Context, actor, id int64, admin bool, action, rid string) error {
	return platform.Tx(ctx, s.DB, func(t *sql.Tx) error {
		if e := platform.LockContent(ctx, t); e != nil {
			return e
		}
		j, e := scanJob(t.QueryRowContext(ctx, `SELECT `+jobColumns+` FROM design_jobs WHERE id=?`, id))
		if e != nil {
			return e
		}
		if !admin && j.Owner != actor {
			return platform.Fail(404, "任务不存在")
		}
		switch action {
		case "cancel":
			if j.Status == "cancelled" {
				return nil
			}
			if j.Status != "queued" && j.Status != "running" {
				return platform.Fail(409, "已结束任务不能取消")
			}
			if _, e = t.ExecContext(ctx, `UPDATE design_jobs SET status='cancelled',lease_token=NULL,lease_until=NULL,updated_at=UTC_TIMESTAMP(6) WHERE id=?`, id); e != nil {
				return e
			}
			if _, e = t.ExecContext(ctx, `UPDATE design_job_attempts SET status='cancelled',finished_at=UTC_TIMESTAMP(6) WHERE job_id=? AND status='running'`, id); e != nil {
				return e
			}
			if _, e = t.ExecContext(ctx, `UPDATE design_task_quota SET reserved_units=reserved_units-? WHERE owner_id=?`, j.Units, j.Owner); e != nil {
				return e
			}
		case "retry":
			if j.Status != "failed" {
				return platform.Fail(409, "只允许重试失败任务")
			}
			var status string
			if e = t.QueryRowContext(ctx, `SELECT status FROM design_providers WHERE id=?`, j.ProviderID).Scan(&status); e != nil {
				return e
			}
			if status != "active" {
				return platform.Fail(503, "服务商未启用")
			}
			var accountStatus string
			if e = t.QueryRowContext(ctx, `SELECT status FROM design_accounts WHERE id=?`, j.Owner).Scan(&accountStatus); e != nil {
				return e
			}
			if accountStatus != "active" {
				return platform.Fail(409, "任务所属账号已停用")
			}
			var pending int
			if e = t.QueryRowContext(ctx, `SELECT COUNT(*) FROM design_jobs WHERE owner_id=? AND status IN ('queued','running')`, j.Owner).Scan(&pending); e != nil {
				return e
			}
			if pending >= 10 {
				return platform.Fail(429, "未完成任务数已达上限")
			}
			if e = reserve(ctx, t, j.Owner, j.Units); e != nil {
				return e
			}
			if _, e = t.ExecContext(ctx, `UPDATE design_jobs SET status='queued',max_attempts=attempts+3,error_code='',lease_token=NULL,lease_until=NULL,updated_at=UTC_TIMESTAMP(6) WHERE id=?`, id); e != nil {
				return e
			}
		case "delete":
			if admin {
				return platform.Fail(403, "管理员不能删除用户任务内容")
			}
			if j.Status == "queued" || j.Status == "running" {
				return platform.Fail(409, "请先取消未完成任务")
			}
			var size int64
			if e = t.QueryRowContext(ctx, `SELECT byte_size FROM design_jobs WHERE id=?`, id).Scan(&size); e != nil {
				return e
			}
			if e = platform.Quota(ctx, t, j.Owner, -size, s.Assets.QuotaLimit()); e != nil {
				return e
			}
			if _, e = t.ExecContext(ctx, `DELETE FROM design_asset_references WHERE object_kind IN ('job_input','job_result') AND object_id=?`, id); e != nil {
				return e
			}
			for _, table := range []string{"design_job_outputs", "design_job_attempts"} {
				if _, e = t.ExecContext(ctx, `DELETE FROM `+table+` WHERE job_id=?`, id); e != nil {
					return e
				}
			}
			if _, e = t.ExecContext(ctx, `DELETE FROM design_jobs WHERE id=?`, id); e != nil {
				return e
			}
		default:
			return platform.Fail(400, "未知任务操作")
		}
		return platform.Audit(ctx, t, actor, "job."+action, platform.ID(id), rid)
	})
}
func (s *Service) Attempts(ctx context.Context, id int64) ([]any, error) {
	rows, e := s.DB.QueryContext(ctx, `SELECT attempt,status,error_code,started_at,finished_at FROM design_job_attempts WHERE job_id=? ORDER BY attempt DESC LIMIT 100`, id)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []any{}
	for rows.Next() {
		var attempt int
		var status, code string
		var start time.Time
		var end sql.NullTime
		if e = rows.Scan(&attempt, &status, &code, &start, &end); e != nil {
			return nil, e
		}
		var finish any
		if end.Valid {
			finish = end.Time
		}
		out = append(out, map[string]any{"attempt": attempt, "status": status, "errorCode": code, "startedAt": start, "finishedAt": finish})
	}
	return out, rows.Err()
}
