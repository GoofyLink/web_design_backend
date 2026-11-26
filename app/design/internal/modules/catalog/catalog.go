package catalog

import (
	"context"
	"database/sql"
	"encoding/json"
	"looklook/app/design/internal/modules/asset"
	"looklook/app/design/internal/modules/canvas"
	"looklook/app/design/internal/platform"
	"strings"
	"time"
)

type Service struct{ DB *sql.DB }
type Category struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Sort   int    `json:"sort"`
	Status string `json:"status"`
}
type Content struct {
	CoverAssetID  string          `json:"coverAssetId,omitempty"`
	Title         string          `json:"title"`
	CategoryID    string          `json:"categoryId"`
	Tags          []string        `json:"tags"`
	Width         int             `json:"width"`
	Height        int             `json:"height"`
	Cover         string          `json:"cover"`
	Engine        string          `json:"engine"`
	EngineVersion string          `json:"engineVersion"`
	SchemaVersion int             `json:"schemaVersion"`
	Document      json.RawMessage `json:"document,omitempty"`
}
type Template struct {
	ID        string    `json:"id"`
	Status    string    `json:"status"`
	Version   int64     `json:"version"`
	Draft     *Content  `json:"draft,omitempty"`
	Published *Content  `json:"published,omitempty"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func validCategory(c Category) error {
	if len([]rune(strings.TrimSpace(c.Name))) < 1 || len([]rune(c.Name)) > 64 || (c.Status != "active" && c.Status != "disabled") || c.Sort < -100000 || c.Sort > 100000 {
		return platform.Fail(400, "分类名称、排序或状态无效")
	}
	return nil
}
func (s *Service) Categories(ctx context.Context, admin bool) ([]Category, error) {
	query := `SELECT id,name,sort_order,status FROM design_categories`
	if !admin {
		query += ` WHERE status='active'`
	}
	query += ` ORDER BY sort_order,id`
	rows, e := s.DB.QueryContext(ctx, query)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []Category{}
	for rows.Next() {
		var c Category
		var id int64
		if e = rows.Scan(&id, &c.Name, &c.Sort, &c.Status); e != nil {
			return nil, e
		}
		c.ID = platform.ID(id)
		out = append(out, c)
	}
	return out, rows.Err()
}
func (s *Service) SaveCategory(ctx context.Context, actor, id int64, c Category, rid string) (result string, err error) {
	if e := validCategory(c); e != nil {
		return "", e
	}
	err = platform.Tx(ctx, s.DB, func(t *sql.Tx) error {
		if e := platform.LockContent(ctx, t); e != nil {
			return e
		}
		if id == 0 {
			r, e := t.ExecContext(ctx, `INSERT INTO design_categories(name,sort_order,status) VALUES(?,?,?)`, strings.TrimSpace(c.Name), c.Sort, c.Status)
			if e != nil {
				return e
			}
			id, e = r.LastInsertId()
			if e != nil {
				return e
			}
		} else {
			if e := platform.Affected(t.ExecContext(ctx, `UPDATE design_categories SET name=?,sort_order=?,status=? WHERE id=?`, strings.TrimSpace(c.Name), c.Sort, c.Status, id)); e != nil {
				return e
			}
		}
		result = platform.ID(id)
		return platform.Audit(ctx, t, actor, "category.save", result, rid)
	})
	return
}
func (s *Service) DeleteCategory(ctx context.Context, actor, id int64, rid string) error {
	return platform.Tx(ctx, s.DB, func(t *sql.Tx) error {
		if e := platform.LockContent(ctx, t); e != nil {
			return e
		}
		var n int
		if e := t.QueryRowContext(ctx, `SELECT id FROM design_categories WHERE id=? FOR UPDATE`, id).Scan(&n); e != nil {
			return e
		}
		if e := t.QueryRowContext(ctx, `SELECT COUNT(*) FROM design_templates WHERE category_id=? OR published_category_id=?`, id, id).Scan(&n); e != nil {
			return e
		}
		if n > 0 {
			return platform.Fail(409, "分类仍被模板引用，请先迁移或删除模板")
		}
		if e := platform.Affected(t.ExecContext(ctx, `DELETE FROM design_categories WHERE id=?`, id)); e != nil {
			return e
		}
		return platform.Audit(ctx, t, actor, "category.delete", platform.ID(id), rid)
	})
}
func ValidateContent(c Content, publish bool) error {
	if len([]rune(strings.TrimSpace(c.Title))) < 1 || len([]rune(c.Title)) > 100 {
		return platform.Fail(400, "模板标题须为 1—100 字")
	}
	if _, e := platform.ParseID(c.CategoryID); e != nil {
		return platform.Fail(400, "分类 ID 无效")
	}
	if c.Width < 1 || c.Height < 1 || c.Width > 8192 || c.Height > 8192 || int64(c.Width)*int64(c.Height) > 40000000 {
		return platform.Fail(400, "画布尺寸超限")
	}
	if len(c.Tags) > 20 {
		return platform.Fail(400, "标签不能超过 20 个")
	}
	for _, tag := range c.Tags {
		if len([]rune(tag)) < 1 || len([]rune(tag)) > 32 {
			return platform.Fail(400, "标签长度须为 1—32 字")
		}
	}
	if len(c.Cover) > 512 || len(c.EngineVersion) > 40 || len(c.Document) > 5*1024*1024 {
		return platform.Fail(413, "模板数据超限")
	}
	if c.CoverAssetID != "" {
		n, e := platform.ParseID(c.CoverAssetID)
		if e != nil {
			return e
		}
		if c.Cover != canvas.AssetURL(n) {
			return platform.Fail(400, "封面地址须匹配 coverAssetId")
		}
	} else if c.Cover != "" && c.Cover != "/api/v1/seed-assets/poster.svg" {
		return platform.Fail(400, "封面须使用已上传图片")
	}
	if publish && c.Cover == "" {
		return platform.Fail(400, "发布需要封面")
	}
	if !publish && len(c.Document) == 0 {
		return nil
	}
	_, e := canvas.Validate(c.Canvas())
	return e
}
func (c Content) Canvas() canvas.Content {
	return canvas.Content{Width: c.Width, Height: c.Height, Engine: c.Engine, EngineVersion: c.EngineVersion, SchemaVersion: c.SchemaVersion, Document: c.Document, CoverAssetID: c.CoverAssetID}
}
func (c Content) references() ([]canvas.Reference, error) {
	if len(c.Document) > 0 {
		return canvas.Validate(c.Canvas())
	}
	if c.CoverAssetID != "" {
		n, e := platform.ParseID(c.CoverAssetID)
		return []canvas.Reference{{ID: n, Kind: "image"}}, e
	}
	return nil, nil
}
func (s *Service) SaveTemplate(ctx context.Context, actor, id, version int64, c Content, rid string) (result map[string]any, err error) {
	if e := ValidateContent(c, false); e != nil {
		return nil, e
	}
	if c.Tags == nil {
		c.Tags = []string{}
	}
	data, e := json.Marshal(c)
	if e != nil {
		return nil, e
	}
	err = platform.Tx(ctx, s.DB, func(t *sql.Tx) error {
		if e := platform.LockContent(ctx, t); e != nil {
			return e
		}
		var n int
		if e := t.QueryRowContext(ctx, `SELECT id FROM design_categories WHERE id=? LOCK IN SHARE MODE`, c.CategoryID).Scan(&n); e == sql.ErrNoRows {
			return platform.Fail(400, "分类不存在")
		} else if e != nil {
			return e
		}
		if id == 0 {
			r, e := t.ExecContext(ctx, `INSERT INTO design_templates(category_id,draft_json,created_at,updated_at) VALUES(?,?,UTC_TIMESTAMP(6),UTC_TIMESTAMP(6))`, c.CategoryID, string(data))
			if e != nil {
				return e
			}
			id, e = r.LastInsertId()
			if e != nil {
				return e
			}
			version = 1
		} else {
			var current int64
			if e := t.QueryRowContext(ctx, `SELECT version FROM design_templates WHERE id=? FOR UPDATE`, id).Scan(&current); e != nil {
				return e
			}
			if current != version {
				return platform.Fail(409, "模板版本冲突，请重新加载")
			}
			if _, e := t.ExecContext(ctx, `UPDATE design_templates SET category_id=?,draft_json=?,version=version+1,updated_at=UTC_TIMESTAMP(6) WHERE id=?`, c.CategoryID, string(data), id); e != nil {
				return e
			}
			version++
		}
		refs, e := c.references()
		if e != nil {
			return e
		}
		if e = asset.SyncReferences(ctx, t, "template_draft", id, actor, true, refs); e != nil {
			return e
		}
		result = map[string]any{"id": platform.ID(id), "version": version}
		return platform.Audit(ctx, t, actor, "template.save", platform.ID(id), rid)
	})
	return
}
func (s *Service) Transition(ctx context.Context, actor, id, version int64, action, rid string) error {
	return platform.Tx(ctx, s.DB, func(t *sql.Tx) error {
		if e := platform.LockContent(ctx, t); e != nil {
			return e
		}
		var current int64
		var state string
		var raw []byte
		if e := t.QueryRowContext(ctx, `SELECT version,status,draft_json FROM design_templates WHERE id=? FOR UPDATE`, id).Scan(&current, &state, &raw); e != nil {
			return e
		}
		if current != version {
			return platform.Fail(409, "模板版本冲突，请重新加载")
		}
		switch action {
		case "publish":
			var c Content
			if e := json.Unmarshal(raw, &c); e != nil {
				return e
			}
			if e := ValidateContent(c, true); e != nil {
				return e
			}
			var status string
			if e := t.QueryRowContext(ctx, `SELECT status FROM design_categories WHERE id=? LOCK IN SHARE MODE`, c.CategoryID).Scan(&status); e != nil {
				return e
			}
			if status != "active" {
				return platform.Fail(409, "分类未启用")
			}
			if _, e := t.ExecContext(ctx, `UPDATE design_templates SET status='published',published_json=draft_json,published_category_id=category_id,version=version+1,published_at=UTC_TIMESTAMP(6),updated_at=UTC_TIMESTAMP(6) WHERE id=?`, id); e != nil {
				return e
			}
			refs, e := c.references()
			if e != nil {
				return e
			}
			if e = asset.SyncReferences(ctx, t, "template_published", id, actor, true, refs); e != nil {
				return e
			}
			result, e := t.ExecContext(ctx, `INSERT INTO design_template_revisions(template_id,version,content_json,actor_id,created_at) VALUES(?,?,?,?,UTC_TIMESTAMP(6))`, id, current+1, string(raw), actor)
			if e != nil {
				return e
			}
			revision, e := result.LastInsertId()
			if e != nil {
				return e
			}
			if e = asset.SyncReferences(ctx, t, "template_revision", revision, actor, true, refs); e != nil {
				return e
			}
		case "offline":
			if state != "published" {
				return platform.Fail(409, "仅已发布模板可以下架")
			}
			if _, e := t.ExecContext(ctx, `UPDATE design_templates SET status='offline',version=version+1,updated_at=UTC_TIMESTAMP(6) WHERE id=?`, id); e != nil {
				return e
			}
		case "delete":
			if state == "published" {
				return platform.Fail(409, "请先下架模板")
			}
			if _, e := t.ExecContext(ctx, `DELETE r FROM design_asset_references r JOIN design_template_revisions v ON r.object_kind='template_revision' AND r.object_id=v.id WHERE v.template_id=?`, id); e != nil {
				return e
			}
			if _, e := t.ExecContext(ctx, `DELETE FROM design_asset_references WHERE object_kind IN ('template_draft','template_published') AND object_id=?`, id); e != nil {
				return e
			}
			if _, e := t.ExecContext(ctx, `DELETE FROM design_template_revisions WHERE template_id=?`, id); e != nil {
				return e
			}
			for _, table := range []string{"design_favorites", "design_topic_items"} {
				if _, e := t.ExecContext(ctx, `DELETE FROM `+table+` WHERE template_id=?`, id); e != nil {
					return e
				}
			}
			if _, e := t.ExecContext(ctx, `DELETE FROM design_templates WHERE id=?`, id); e != nil {
				return e
			}
		default:
			return platform.Fail(400, "未知状态操作")
		}
		return platform.Audit(ctx, t, actor, "template."+action, platform.ID(id), rid)
	})
}
func scanTemplate(row interface{ Scan(...any) error }) (Template, error) {
	var t Template
	var id int64
	var draft, published []byte
	e := row.Scan(&id, &t.Status, &t.Version, &draft, &published, &t.UpdatedAt)
	if e != nil {
		return t, platform.Normalize(e)
	}
	t.ID = platform.ID(id)
	if e = json.Unmarshal(draft, &t.Draft); e != nil {
		return t, e
	}
	if len(published) > 0 {
		if e = json.Unmarshal(published, &t.Published); e != nil {
			return t, e
		}
	}
	return t, nil
}
func publicTemplate(t Template) map[string]any {
	c := *t.Published
	return map[string]any{"id": t.ID, "title": c.Title, "categoryId": c.CategoryID, "tags": c.Tags, "width": c.Width, "height": c.Height, "cover": c.Cover}
}
func (s *Service) Detail(ctx context.Context, id int64, admin bool) (any, error) {
	query := `SELECT t.id,t.status,t.version,t.draft_json,t.published_json,t.updated_at FROM design_templates t`
	if !admin {
		query += ` JOIN design_categories c ON c.id=t.published_category_id AND c.status='active'`
	}
	query += ` WHERE t.id=?`
	if !admin {
		query += ` AND t.status='published'`
	}
	t, e := scanTemplate(s.DB.QueryRowContext(ctx, query, id))
	if e != nil {
		return nil, e
	}
	if admin {
		return t, nil
	}
	return publicTemplate(t), nil
}
func (s *Service) Templates(ctx context.Context, admin bool, p platform.Page, category, sort, ratio string) (platform.List, error) {
	if admin && p.Status != "" && p.Status != "draft" && p.Status != "published" && p.Status != "offline" {
		return platform.List{}, platform.Fail(400, "模板状态无效")
	}
	if sort != "" && sort != "latest" && sort != "popular" {
		return platform.List{}, platform.Fail(400, "sort 仅支持 latest/popular")
	}
	from := ` FROM design_templates t JOIN design_categories c ON c.id=t.category_id`
	field := "draft_json"
	where := ` WHERE 1=1`
	args := []any{}
	if !admin {
		from = ` FROM design_templates t JOIN design_categories c ON c.id=t.published_category_id`
		field = "published_json"
		where += ` AND t.status='published' AND c.status='active'`
	} else if p.Status != "" {
		where += ` AND t.status=?`
		args = append(args, p.Status)
	}
	if category != "" {
		if _, e := platform.ParseID(category); e != nil {
			return platform.List{}, e
		}
		where += ` AND c.id=?`
		args = append(args, category)
	}
	if p.Keyword != "" {
		where += ` AND (JSON_UNQUOTE(JSON_EXTRACT(t.` + field + `,'$.title')) LIKE ? OR JSON_EXTRACT(t.` + field + `,'$.tags') LIKE ? OR c.name LIKE ?)`
		for i := 0; i < 3; i++ {
			args = append(args, "%"+p.Keyword+"%")
		}
	}
	if ratio != "" {
		op := ""
		switch ratio {
		case "square":
			op = "="
		case "landscape":
			op = ">"
		case "portrait":
			op = "<"
		default:
			return platform.List{}, platform.Fail(400, "ratio 仅支持 square/landscape/portrait")
		}
		where += ` AND CAST(JSON_EXTRACT(t.` + field + `,'$.width') AS UNSIGNED)` + op + `CAST(JSON_EXTRACT(t.` + field + `,'$.height') AS UNSIGNED)`
	}
	out := platform.List{Items: []any{}, Page: p.Number, PageSize: p.Size}
	if e := s.DB.QueryRowContext(ctx, `SELECT COUNT(*)`+from+where, args...).Scan(&out.Total); e != nil {
		return out, e
	}
	order := ` ORDER BY t.published_at DESC,t.id DESC`
	if admin {
		order = ` ORDER BY t.updated_at DESC,t.id DESC`
	}
	if sort == "popular" {
		order = ` ORDER BY t.use_count DESC,t.id DESC`
	}
	rows, e := s.DB.QueryContext(ctx, `SELECT t.id,t.status,t.version,JSON_REMOVE(t.draft_json,'$.document'),JSON_REMOVE(t.published_json,'$.document'),t.updated_at`+from+where+order+` LIMIT ? OFFSET ?`, append(args, p.Size, p.Offset())...)
	if e != nil {
		return out, e
	}
	defer rows.Close()
	items := []any{}
	for rows.Next() {
		t, e := scanTemplate(rows)
		if e != nil {
			return out, e
		}
		if admin {
			t.Draft.Document = nil
			if t.Published != nil {
				t.Published.Document = nil
			}
			items = append(items, t)
		} else {
			items = append(items, publicTemplate(t))
		}
	}
	out.Items = items
	return out, rows.Err()
}
