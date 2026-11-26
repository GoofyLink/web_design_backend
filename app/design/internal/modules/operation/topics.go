package operation

import (
	"context"
	"database/sql"
	"looklook/app/design/internal/platform"
	"strings"
	"time"
)

type TopicInput struct {
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Status      string   `json:"status"`
	Slot        string   `json:"slot"`
	Sort        int      `json:"sort"`
	StartsAt    string   `json:"startsAt"`
	EndsAt      string   `json:"endsAt"`
	TemplateIDs []string `json:"templateIds"`
	Version     int64    `json:"version"`
}
type Topic struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Status      string    `json:"status"`
	Slot        string    `json:"slot"`
	Sort        int       `json:"sort"`
	StartsAt    time.Time `json:"startsAt"`
	EndsAt      time.Time `json:"endsAt"`
	Version     int64     `json:"version"`
	Templates   []any     `json:"templates,omitempty"`
}

const topicColumns = `id,title,description,status,slot,sort_order,starts_at,ends_at,version`

func scanTopic(row interface{ Scan(...any) error }) (Topic, error) {
	var v Topic
	var n int64
	e := row.Scan(&n, &v.Title, &v.Description, &v.Status, &v.Slot, &v.Sort, &v.StartsAt, &v.EndsAt, &v.Version)
	v.ID = platform.ID(n)
	return v, platform.Normalize(e)
}
func (s *Service) TopicSave(ctx context.Context, actor, id int64, v TopicInput, rid string) (out any, err error) {
	start, e := time.Parse(time.RFC3339, v.StartsAt)
	if e != nil {
		return nil, platform.Fail(400, "startsAt 必须是 RFC3339")
	}
	end, e := time.Parse(time.RFC3339, v.EndsAt)
	if e != nil || !end.After(start) {
		return nil, platform.Fail(400, "专题起止时间无效")
	}
	if strings.TrimSpace(v.Title) == "" || len([]rune(v.Title)) > 100 || len([]rune(v.Description)) > 2000 || (v.Status != "draft" && v.Status != "published" && v.Status != "offline") || (v.Slot != "home" && v.Slot != "featured" && v.Slot != "") || len(v.TemplateIDs) > 100 || v.Sort < -100000 || v.Sort > 100000 {
		return nil, platform.Fail(400, "专题字段或数量无效")
	}
	ids := []int64{}
	seen := map[int64]bool{}
	for _, raw := range v.TemplateIDs {
		n, e := platform.ParseID(raw)
		if e != nil {
			return nil, e
		}
		if seen[n] {
			return nil, platform.Fail(400, "专题模板重复")
		}
		seen[n] = true
		ids = append(ids, n)
	}
	if v.Status == "published" && len(ids) == 0 {
		return nil, platform.Fail(400, "发布专题至少需要一个模板")
	}
	err = platform.Tx(ctx, s.DB, func(t *sql.Tx) error {
		if e := platform.LockContent(ctx, t); e != nil {
			return e
		}
		for _, n := range ids {
			var state string
			if e := t.QueryRowContext(ctx, `SELECT t.status FROM design_templates t JOIN design_categories c ON c.id=t.category_id WHERE t.id=?`, n).Scan(&state); e != nil {
				return e
			}
			if v.Status == "published" {
				var available int
				if e := t.QueryRowContext(ctx, `SELECT COUNT(*) FROM design_templates t JOIN design_categories c ON c.id=t.published_category_id WHERE t.id=? AND t.status='published' AND c.status='active'`, n).Scan(&available); e != nil {
					return e
				}
				if available != 1 {
					return platform.Fail(409, "专题包含未发布或分类停用的模板")
				}
			}
		}
		if id == 0 {
			r, e := t.ExecContext(ctx, `INSERT INTO design_topics(title,description,status,slot,sort_order,starts_at,ends_at) VALUES(?,?,?,?,?,?,?)`, v.Title, v.Description, v.Status, v.Slot, v.Sort, start.UTC(), end.UTC())
			if e != nil {
				return e
			}
			id, e = r.LastInsertId()
			if e != nil {
				return e
			}
			v.Version = 1
		} else {
			var current int64
			if e := t.QueryRowContext(ctx, `SELECT version FROM design_topics WHERE id=?`, id).Scan(&current); e != nil {
				return e
			}
			if current != v.Version {
				return platform.Fail(409, "专题版本冲突")
			}
			if _, e := t.ExecContext(ctx, `UPDATE design_topics SET title=?,description=?,status=?,slot=?,sort_order=?,starts_at=?,ends_at=?,version=version+1 WHERE id=?`, v.Title, v.Description, v.Status, v.Slot, v.Sort, start.UTC(), end.UTC(), id); e != nil {
				return e
			}
			v.Version++
		}
		if _, e := t.ExecContext(ctx, `DELETE FROM design_topic_items WHERE topic_id=?`, id); e != nil {
			return e
		}
		for index, n := range ids {
			if _, e := t.ExecContext(ctx, `INSERT INTO design_topic_items VALUES(?,?,?)`, id, n, index); e != nil {
				return e
			}
		}
		out = map[string]any{"id": platform.ID(id), "version": v.Version}
		return platform.Audit(ctx, t, actor, "topic.save", platform.ID(id), rid)
	})
	return
}
func (s *Service) Topics(ctx context.Context, admin bool, p platform.Page, slot string) (platform.List, error) {
	out := platform.List{Page: p.Number, PageSize: p.Size, Items: []Topic{}}
	where := ` WHERE 1=1`
	args := []any{}
	if !admin {
		where += ` AND status='published' AND starts_at<=UTC_TIMESTAMP(6) AND ends_at>UTC_TIMESTAMP(6)`
	} else if p.Status != "" {
		if p.Status != "draft" && p.Status != "published" && p.Status != "offline" {
			return out, platform.Fail(400, "状态无效")
		}
		where += ` AND status=?`
		args = append(args, p.Status)
	}
	if slot != "" {
		if slot != "home" && slot != "featured" {
			return out, platform.Fail(400, "推荐位无效")
		}
		where += ` AND slot=?`
		args = append(args, slot)
	}
	if p.Keyword != "" {
		where += ` AND title LIKE ?`
		args = append(args, "%"+p.Keyword+"%")
	}
	if e := s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM design_topics`+where, args...).Scan(&out.Total); e != nil {
		return out, e
	}
	rows, e := s.DB.QueryContext(ctx, `SELECT `+topicColumns+` FROM design_topics`+where+` ORDER BY sort_order,id DESC LIMIT ? OFFSET ?`, append(args, p.Size, p.Offset())...)
	if e != nil {
		return out, e
	}
	defer rows.Close()
	items := []Topic{}
	for rows.Next() {
		v, e := scanTopic(rows)
		if e != nil {
			return out, e
		}
		items = append(items, v)
	}
	out.Items = items
	return out, rows.Err()
}
func (s *Service) Topic(ctx context.Context, id int64, admin bool) (Topic, error) {
	q := `SELECT ` + topicColumns + ` FROM design_topics WHERE id=?`
	if !admin {
		q += ` AND status='published' AND starts_at<=UTC_TIMESTAMP(6) AND ends_at>UTC_TIMESTAMP(6)`
	}
	v, e := scanTopic(s.DB.QueryRowContext(ctx, q, id))
	if e != nil {
		return v, e
	}
	q = `SELECT t.id,JSON_UNQUOTE(JSON_EXTRACT(COALESCE(t.published_json,t.draft_json),'$.title')),JSON_UNQUOTE(JSON_EXTRACT(COALESCE(t.published_json,t.draft_json),'$.cover')) FROM design_topic_items i JOIN design_templates t ON t.id=i.template_id LEFT JOIN design_categories c ON c.id=t.published_category_id WHERE i.topic_id=?`
	if !admin {
		q += ` AND t.status='published' AND c.status='active'`
	}
	q += ` ORDER BY i.sort_order,t.id`
	rows, e := s.DB.QueryContext(ctx, q, id)
	if e != nil {
		return v, e
	}
	defer rows.Close()
	v.Templates = []any{}
	for rows.Next() {
		var n int64
		var title, cover sql.NullString
		if e = rows.Scan(&n, &title, &cover); e != nil {
			return v, e
		}
		v.Templates = append(v.Templates, map[string]any{"id": platform.ID(n), "title": title.String, "cover": cover.String})
	}
	return v, rows.Err()
}
func (s *Service) TopicDelete(ctx context.Context, actor, id, version int64, rid string) error {
	return platform.Tx(ctx, s.DB, func(t *sql.Tx) error {
		if e := platform.LockContent(ctx, t); e != nil {
			return e
		}
		var current int64
		var status string
		if e := t.QueryRowContext(ctx, `SELECT version,status FROM design_topics WHERE id=?`, id).Scan(&current, &status); e != nil {
			return e
		}
		if current != version || status == "published" {
			return platform.Fail(409, "版本冲突或专题尚未下架")
		}
		if _, e := t.ExecContext(ctx, `DELETE FROM design_topic_items WHERE topic_id=?`, id); e != nil {
			return e
		}
		if _, e := t.ExecContext(ctx, `DELETE FROM design_topics WHERE id=?`, id); e != nil {
			return e
		}
		return platform.Audit(ctx, t, actor, "topic.delete", platform.ID(id), rid)
	})
}
