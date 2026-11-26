package workspace

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"looklook/app/design/internal/modules/asset"
	"looklook/app/design/internal/modules/canvas"
	"looklook/app/design/internal/platform"
	"regexp"
	"strings"
	"time"
)

type Service struct {
	DB        *sql.DB
	Limit     int64
	TrashDays int
}

func (s *Service) limit() int64 {
	if s.Limit > 0 {
		return s.Limit
	}
	return asset.DefaultQuota
}
func (s *Service) retention() time.Duration {
	days := s.TrashDays
	if days <= 0 {
		days = 30
	}
	return time.Duration(days) * 24 * time.Hour
}

type Design struct {
	ID               string          `json:"id"`
	Title            string          `json:"title"`
	Version          int64           `json:"version"`
	Content          *canvas.Content `json:"content,omitempty"`
	CreatedAt        time.Time       `json:"createdAt"`
	UpdatedAt        time.Time       `json:"updatedAt"`
	DeletedAt        *time.Time      `json:"deletedAt,omitempty"`
	SourceTemplateID string          `json:"sourceTemplateId,omitempty"`
}
type Result struct {
	ID      string `json:"id"`
	Version int64  `json:"version"`
}

var keyPattern = regexp.MustCompile(`^[a-zA-Z0-9._:-]{8,100}$`)

func titleValid(title string) error {
	if len([]rune(strings.TrimSpace(title))) < 1 || len([]rune(title)) > 100 {
		return platform.Fail(400, "作品标题须为 1—100 字")
	}
	return nil
}
func (s *Service) Create(ctx context.Context, owner int64, mode string, source int64, title string, c canvas.Content, key, rid string) (out Result, err error) {
	if !keyPattern.MatchString(key) {
		return out, platform.Fail(400, "创建请求必须带 8—100 位 Idempotency-Key")
	}
	if mode != "blank" && mode != "copy" && mode != "template" {
		return out, platform.Fail(400, "创建模式无效")
	}
	payload, marshalErr := json.Marshal(struct {
		Mode    string
		Source  int64
		Title   string
		Content canvas.Content
	}{mode, source, title, c})
	if marshalErr != nil {
		return out, platform.Fail(400, "文档 JSON 无效")
	}
	fingerprint := fmt.Sprintf("%x", sha256.Sum256(payload))
	scope := "design:" + mode
	err = platform.Tx(ctx, s.DB, func(t *sql.Tx) error {
		if e := platform.LockContent(ctx, t); e != nil {
			return e
		}
		var storedHash string
		var storedResult []byte
		var expires time.Time
		e := t.QueryRowContext(ctx, `SELECT request_hash,result_json,expires_at FROM design_idempotency_keys WHERE account_id=? AND scope=? AND request_key=?`, owner, scope, key).Scan(&storedHash, &storedResult, &expires)
		if e == nil && expires.After(time.Now()) {
			if storedHash != fingerprint {
				return platform.Fail(409, "相同幂等键对应不同请求")
			}
			return json.Unmarshal(storedResult, &out)
		}
		if e != nil && e != sql.ErrNoRows {
			return e
		}
		var sourceTemplate any = nil
		switch mode {
		case "template":
			var raw []byte
			if e = t.QueryRowContext(ctx, `SELECT t.published_json FROM design_templates t JOIN design_categories c ON c.id=t.published_category_id WHERE t.id=? AND t.status='published' AND c.status='active'`, source).Scan(&raw); e != nil {
				return e
			}
			var published struct {
				Title string `json:"title"`
				canvas.Content
			}
			if e = json.Unmarshal(raw, &published); e != nil {
				return e
			}
			c = published.Content
			if title == "" {
				title = published.Title
			}
			sourceTemplate = source
		case "copy":
			var raw []byte
			var oldTitle string
			var template sql.NullInt64
			if e = t.QueryRowContext(ctx, `SELECT title,content_json,source_template_id FROM design_designs WHERE id=? AND owner_id=? AND deleted_at IS NULL`, source, owner).Scan(&oldTitle, &raw, &template); e != nil {
				return e
			}
			if e = json.Unmarshal(raw, &c); e != nil {
				return e
			}
			if title == "" {
				rs := []rune(oldTitle)
				if len(rs) > 97 {
					rs = rs[:97]
				}
				title = string(rs) + " 副本"
			}
			if template.Valid {
				sourceTemplate = template.Int64
			}
		}
		if title == "" {
			title = "未命名设计"
		}
		if e = titleValid(title); e != nil {
			return e
		}
		refs, e := canvas.Validate(c)
		if e != nil {
			return e
		}
		raw, e := json.Marshal(c)
		if e != nil {
			return e
		}
		if e = platform.Quota(ctx, t, owner, int64(len(raw)), s.limit()); e != nil {
			return e
		}
		res, e := t.ExecContext(ctx, `INSERT INTO design_designs(owner_id,title,content_json,byte_size,source_template_id,created_at,updated_at) VALUES(?,?,?,?,?,UTC_TIMESTAMP(6),UTC_TIMESTAMP(6))`, owner, title, string(raw), len(raw), sourceTemplate)
		if e != nil {
			return e
		}
		id, e := res.LastInsertId()
		if e != nil {
			return e
		}
		if e = asset.SyncReferences(ctx, t, "design", id, owner, false, refs); e != nil {
			return e
		}
		out = Result{platform.ID(id), 1}
		encoded, _ := json.Marshal(out)
		if _, e = t.ExecContext(ctx, `INSERT INTO design_idempotency_keys(account_id,scope,request_key,request_hash,result_json,expires_at) VALUES(?,?,?,?,?,?) ON DUPLICATE KEY UPDATE request_hash=VALUES(request_hash),result_json=VALUES(result_json),expires_at=VALUES(expires_at)`, owner, scope, key, fingerprint, string(encoded), time.Now().UTC().Add(24*time.Hour)); e != nil {
			return e
		}
		if mode == "template" {
			if _, e = t.ExecContext(ctx, `UPDATE design_templates SET use_count=use_count+1 WHERE id=?`, source); e != nil {
				return e
			}
		}
		return platform.Audit(ctx, t, owner, "design.create."+mode, out.ID, rid)
	})
	return
}

const columns = `id,title,content_json,version,source_template_id,created_at,updated_at,deleted_at`

func scan(row interface{ Scan(...any) error }) (Design, error) {
	var d Design
	var id int64
	var source sql.NullInt64
	var raw []byte
	var deleted sql.NullTime
	e := row.Scan(&id, &d.Title, &raw, &d.Version, &source, &d.CreatedAt, &d.UpdatedAt, &deleted)
	if e != nil {
		return d, platform.Normalize(e)
	}
	d.ID = platform.ID(id)
	if source.Valid {
		d.SourceTemplateID = platform.ID(source.Int64)
	}
	if deleted.Valid {
		d.DeletedAt = &deleted.Time
	}
	e = json.Unmarshal(raw, &d.Content)
	return d, e
}
func (s *Service) Get(ctx context.Context, owner, id int64) (Design, error) {
	return scan(s.DB.QueryRowContext(ctx, `SELECT `+columns+` FROM design_designs WHERE id=? AND owner_id=? AND deleted_at IS NULL`, id, owner))
}
func (s *Service) List(ctx context.Context, owner int64, p platform.Page, trash bool) (platform.List, error) {
	where := ` WHERE owner_id=? AND deleted_at IS NULL`
	if trash {
		where = ` WHERE owner_id=? AND deleted_at IS NOT NULL`
	}
	args := []any{owner}
	if p.Keyword != "" {
		where += ` AND title LIKE ?`
		args = append(args, "%"+p.Keyword+"%")
	}
	out := platform.List{Items: []Design{}, Page: p.Number, PageSize: p.Size}
	if e := s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM design_designs`+where, args...).Scan(&out.Total); e != nil {
		return out, e
	}
	rows, e := s.DB.QueryContext(ctx, `SELECT id,title,JSON_REMOVE(content_json,'$.document'),version,source_template_id,created_at,updated_at,deleted_at FROM design_designs`+where+` ORDER BY updated_at DESC,id DESC LIMIT ? OFFSET ?`, append(args, p.Size, p.Offset())...)
	if e != nil {
		return out, e
	}
	defer rows.Close()
	items := []Design{}
	for rows.Next() {
		d, e := scan(rows)
		if e != nil {
			return out, e
		}
		d.Content.Document = nil
		items = append(items, d)
	}
	out.Items = items
	return out, rows.Err()
}
func (s *Service) Save(ctx context.Context, owner, id, version int64, c canvas.Content, rid string) (out Result, err error) {
	refs, e := canvas.Validate(c)
	if e != nil {
		return out, e
	}
	raw, e := json.Marshal(c)
	if e != nil {
		return out, e
	}
	err = platform.Tx(ctx, s.DB, func(t *sql.Tx) error {
		if e := platform.LockContent(ctx, t); e != nil {
			return e
		}
		var oldSize, current int64
		if e := t.QueryRowContext(ctx, `SELECT version,byte_size FROM design_designs WHERE id=? AND owner_id=? AND deleted_at IS NULL FOR UPDATE`, id, owner).Scan(&current, &oldSize); e != nil {
			return e
		}
		if version != current {
			return platform.Fail(409, "作品版本冲突，请重新加载或另存副本")
		}
		historyDelta, e := s.capture(ctx, t, owner, id, current)
		if e != nil {
			return e
		}
		if e := asset.SyncReferences(ctx, t, "design", id, owner, false, refs); e != nil {
			return e
		}
		if e := platform.Quota(ctx, t, owner, int64(len(raw))-oldSize+historyDelta, s.limit()); e != nil {
			return e
		}
		if _, e := t.ExecContext(ctx, `UPDATE design_designs SET content_json=?,byte_size=?,version=version+1,updated_at=UTC_TIMESTAMP(6) WHERE id=?`, string(raw), len(raw), id); e != nil {
			return e
		}
		out = Result{platform.ID(id), current + 1}
		return platform.Audit(ctx, t, owner, "design.save", out.ID, rid)
	})
	return
}
func (s *Service) Rename(ctx context.Context, owner, id, version int64, title, rid string) (out Result, err error) {
	if e := titleValid(title); e != nil {
		return out, e
	}
	err = platform.Tx(ctx, s.DB, func(t *sql.Tx) error {
		if e := platform.LockContent(ctx, t); e != nil {
			return e
		}
		var current int64
		if e := t.QueryRowContext(ctx, `SELECT version FROM design_designs WHERE id=? AND owner_id=? AND deleted_at IS NULL FOR UPDATE`, id, owner).Scan(&current); e != nil {
			return e
		}
		if current != version {
			return platform.Fail(409, "作品版本冲突")
		}
		if _, e := t.ExecContext(ctx, `UPDATE design_designs SET title=?,version=version+1,updated_at=UTC_TIMESTAMP(6) WHERE id=?`, strings.TrimSpace(title), id); e != nil {
			return e
		}
		out = Result{platform.ID(id), current + 1}
		return platform.Audit(ctx, t, owner, "design.rename", out.ID, rid)
	})
	return
}
func (s *Service) Trash(ctx context.Context, owner, id, version int64, action, rid string) error {
	return platform.Tx(ctx, s.DB, func(t *sql.Tx) error {
		if e := platform.LockContent(ctx, t); e != nil {
			return e
		}
		var current, size int64
		var deleted sql.NullTime
		if e := t.QueryRowContext(ctx, `SELECT version,byte_size,deleted_at FROM design_designs WHERE id=? AND owner_id=? FOR UPDATE`, id, owner).Scan(&current, &size, &deleted); e != nil {
			return e
		}
		if current != version {
			return platform.Fail(409, "作品版本冲突")
		}
		switch action {
		case "delete":
			if deleted.Valid {
				return platform.Fail(409, "作品已在回收站")
			}
			if _, e := t.ExecContext(ctx, `UPDATE design_designs SET deleted_at=UTC_TIMESTAMP(6),updated_at=UTC_TIMESTAMP(6),version=version+1 WHERE id=?`, id); e != nil {
				return e
			}
		case "restore":
			if !deleted.Valid || deleted.Time.Before(time.Now().Add(-s.retention())) {
				return platform.Fail(409, "作品不在可恢复期限内")
			}
			if _, e := t.ExecContext(ctx, `UPDATE design_designs SET deleted_at=NULL,updated_at=UTC_TIMESTAMP(6),version=version+1 WHERE id=?`, id); e != nil {
				return e
			}
		case "purge":
			if !deleted.Valid {
				return platform.Fail(409, "只能彻底删除回收站作品")
			}
			if e := purge(ctx, t, owner, id, size, s.limit()); e != nil {
				return e
			}
		default:
			return platform.Fail(400, "无效回收站操作")
		}
		return platform.Audit(ctx, t, owner, "design."+action, platform.ID(id), rid)
	})
}
func purge(ctx context.Context, t *sql.Tx, owner, id, size, limit int64) error {
	var historySize int64
	if e := t.QueryRowContext(ctx, `SELECT COALESCE(SUM(byte_size),0) FROM design_history WHERE design_id=?`, id).Scan(&historySize); e != nil {
		return e
	}
	size += historySize
	if _, e := t.ExecContext(ctx, `DELETE r FROM design_asset_references r JOIN design_history h ON r.object_kind='design_history' AND r.object_id=h.id WHERE h.design_id=?`, id); e != nil {
		return e
	}
	if _, e := t.ExecContext(ctx, `DELETE FROM design_history WHERE design_id=?`, id); e != nil {
		return e
	}
	if _, e := t.ExecContext(ctx, `DELETE FROM design_folder_items WHERE design_id=?`, id); e != nil {
		return e
	}

	if _, e := t.ExecContext(ctx, `DELETE FROM design_asset_references WHERE object_kind='design' AND object_id=?`, id); e != nil {
		return e
	}
	if e := platform.Quota(ctx, t, owner, -size, limit); e != nil {
		return e
	}
	_, e := t.ExecContext(ctx, `DELETE FROM design_designs WHERE id=?`, id)
	return e
}
func (s *Service) Cleanup(ctx context.Context) (int, error) {
	count := 0
	e := platform.Tx(ctx, s.DB, func(t *sql.Tx) error {
		if e := platform.LockContent(ctx, t); e != nil {
			return e
		}
		rows, e := t.QueryContext(ctx, `SELECT id,owner_id,byte_size FROM design_designs WHERE deleted_at<? ORDER BY id LIMIT 100`, time.Now().UTC().Add(-s.retention()))
		if e != nil {
			return e
		}
		type item struct{ id, owner, size int64 }
		items := []item{}
		for rows.Next() {
			var i item
			if e = rows.Scan(&i.id, &i.owner, &i.size); e != nil {
				rows.Close()
				return e
			}
			items = append(items, i)
		}
		e = rows.Err()
		rows.Close()
		if e != nil {
			return e
		}
		for _, i := range items {
			if e = purge(ctx, t, i.owner, i.id, i.size, s.limit()); e != nil {
				return e
			}
			if e = platform.Audit(ctx, t, 0, "design.expire", platform.ID(i.id), "maintenance"); e != nil {
				return e
			}
			count++
		}
		_, e = t.ExecContext(ctx, `DELETE FROM design_idempotency_keys WHERE expires_at<UTC_TIMESTAMP(6) LIMIT 1000`)
		return e
	})
	return count, e
}
