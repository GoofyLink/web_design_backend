package workspace

import (
	"context"
	"database/sql"
	"encoding/json"
	"looklook/app/design/internal/modules/canvas"
	"looklook/app/design/internal/platform"
	"time"
)

type History struct {
	ID        string          `json:"id"`
	Version   int64           `json:"version"`
	Content   *canvas.Content `json:"content,omitempty"`
	CreatedAt time.Time       `json:"createdAt"`
}

// Capture the previous committed document before changing its reference set.
func (s *Service) capture(ctx context.Context, t *sql.Tx, owner, id, version int64) (int64, error) {
	var raw []byte
	var size int64
	if e := t.QueryRowContext(ctx, `SELECT content_json,byte_size FROM design_designs WHERE id=?`, id).Scan(&raw, &size); e != nil {
		return 0, e
	}
	r, e := t.ExecContext(ctx, `INSERT INTO design_history(design_id,version,content_json,byte_size,created_at) VALUES(?,?,?,?,UTC_TIMESTAMP(6))`, id, version, string(raw), size)
	if e != nil {
		return 0, e
	}
	history, e := r.LastInsertId()
	if e != nil {
		return 0, e
	}
	if _, e = t.ExecContext(ctx, `INSERT INTO design_asset_references(asset_id,object_kind,object_id) SELECT asset_id,'design_history',? FROM design_asset_references WHERE object_kind='design' AND object_id=?`, history, id); e != nil {
		return 0, e
	}
	// Keep the latest 20 previous documents. Their bytes count against storage quota.
	rows, e := t.QueryContext(ctx, `SELECT id,byte_size FROM design_history WHERE design_id=? ORDER BY id DESC LIMIT 100000 OFFSET 20`, id)
	if e != nil {
		return 0, e
	}
	type old struct{ id, size int64 }
	out := []old{}
	for rows.Next() {
		var v old
		if e = rows.Scan(&v.id, &v.size); e != nil {
			rows.Close()
			return 0, e
		}
		out = append(out, v)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return 0, e
	}
	delta := size
	for _, v := range out {
		if _, e = t.ExecContext(ctx, `DELETE FROM design_asset_references WHERE object_kind='design_history' AND object_id=?`, v.id); e != nil {
			return 0, e
		}
		if _, e = t.ExecContext(ctx, `DELETE FROM design_history WHERE id=?`, v.id); e != nil {
			return 0, e
		}
		delta -= v.size
	}
	return delta, nil
}
func (s *Service) History(ctx context.Context, owner, id int64) ([]History, error) {
	if _, e := s.Get(ctx, owner, id); e != nil {
		return nil, e
	}
	rows, e := s.DB.QueryContext(ctx, `SELECT id,version,created_at FROM design_history WHERE design_id=? ORDER BY id DESC LIMIT 20`, id)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []History{}
	for rows.Next() {
		var v History
		var n int64
		if e = rows.Scan(&n, &v.Version, &v.CreatedAt); e != nil {
			return nil, e
		}
		v.ID = platform.ID(n)
		out = append(out, v)
	}
	return out, rows.Err()
}
func (s *Service) HistoryDetail(ctx context.Context, owner, id, h int64) (History, error) {
	var v History
	var n int64
	var raw []byte
	e := s.DB.QueryRowContext(ctx, `SELECT h.id,h.version,h.content_json,h.created_at FROM design_history h JOIN design_designs d ON d.id=h.design_id WHERE h.id=? AND d.id=? AND d.owner_id=? AND d.deleted_at IS NULL`, h, id, owner).Scan(&n, &v.Version, &raw, &v.CreatedAt)
	if e != nil {
		return v, platform.Normalize(e)
	}
	v.ID = platform.ID(n)
	e = json.Unmarshal(raw, &v.Content)
	return v, e
}
func (s *Service) RestoreHistory(ctx context.Context, owner, id, h, version int64, rid string) (Result, error) {
	v, e := s.HistoryDetail(ctx, owner, id, h)
	if e != nil {
		return Result{}, e
	}
	return s.Save(ctx, owner, id, version, *v.Content, rid)
}
func (s *Service) Cover(ctx context.Context, owner, id, version int64, assetID, rid string) (Result, error) {
	d, e := s.Get(ctx, owner, id)
	if e != nil {
		return Result{}, e
	}
	if assetID == "" {
		return Result{}, platform.Fail(400, "封面资源不能为空")
	}
	d.Content.CoverAssetID = assetID
	return s.Save(ctx, owner, id, version, *d.Content, rid)
}
