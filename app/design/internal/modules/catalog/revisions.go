package catalog

import (
	"context"
	"encoding/json"
	"looklook/app/design/internal/platform"
	"time"
)

type Revision struct {
	ID        string    `json:"id"`
	Version   int64     `json:"version"`
	Content   *Content  `json:"content,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
}

func (s *Service) Revisions(ctx context.Context, id int64, p platform.Page) (platform.List, error) {
	out := platform.List{Items: []Revision{}, Page: p.Number, PageSize: p.Size}
	if _, e := s.Detail(ctx, id, true); e != nil {
		return out, e
	}
	if e := s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM design_template_revisions WHERE template_id=?`, id).Scan(&out.Total); e != nil {
		return out, e
	}
	rows, e := s.DB.QueryContext(ctx, `SELECT id,version,created_at FROM design_template_revisions WHERE template_id=? ORDER BY id DESC LIMIT ? OFFSET ?`, id, p.Size, p.Offset())
	if e != nil {
		return out, e
	}
	defer rows.Close()
	items := []Revision{}
	for rows.Next() {
		var v Revision
		var n int64
		if e = rows.Scan(&n, &v.Version, &v.CreatedAt); e != nil {
			return out, e
		}
		v.ID = platform.ID(n)
		items = append(items, v)
	}
	out.Items = items
	return out, rows.Err()
}
func (s *Service) Revision(ctx context.Context, id, revision int64) (Revision, error) {
	var v Revision
	var n int64
	var raw []byte
	e := s.DB.QueryRowContext(ctx, `SELECT id,version,content_json,created_at FROM design_template_revisions WHERE template_id=? AND id=?`, id, revision).Scan(&n, &v.Version, &raw, &v.CreatedAt)
	if e != nil {
		return v, platform.Normalize(e)
	}
	v.ID = platform.ID(n)
	e = json.Unmarshal(raw, &v.Content)
	return v, e
}
