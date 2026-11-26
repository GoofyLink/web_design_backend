package operation

import (
	"context"
	"database/sql"
	"encoding/json"
	"looklook/app/design/internal/modules/asset"
	"looklook/app/design/internal/modules/canvas"
	"looklook/app/design/internal/platform"
	"regexp"
	"time"
)

type Service struct{ DB *sql.DB }

var KeyPattern = regexp.MustCompile(`^[a-zA-Z0-9._:-]{8,100}$`)

var errorCodePattern = regexp.MustCompile(`^[a-zA-Z0-9_.:-]{0,64}$`)

type ExportRequest struct {
	Version int64  `json:"version"`
	Format  string `json:"format"`
	Scale   int    `json:"scale"`
}
type ExportEvent struct {
	EventID   string `json:"eventId"`
	CheckID   string `json:"checkId"`
	Status    string `json:"status"`
	ErrorCode string `json:"errorCode,omitempty"`
}

func (s *Service) ExportCheck(ctx context.Context, owner, id int64, v ExportRequest) (out any, err error) {
	if (v.Format != "png" && v.Format != "jpeg") || v.Scale < 1 || v.Scale > 4 {
		return nil, platform.Fail(400, "导出格式仅 png/jpeg，scale 为 1—4")
	}
	err = platform.Tx(ctx, s.DB, func(t *sql.Tx) error {
		if e := platform.LockContent(ctx, t); e != nil {
			return e
		}
		var current int64
		var raw []byte
		if e := t.QueryRowContext(ctx, `SELECT version,content_json FROM design_designs WHERE id=? AND owner_id=? AND deleted_at IS NULL`, id, owner).Scan(&current, &raw); e != nil {
			return e
		}
		if v.Version != current {
			return platform.Fail(409, "作品版本已改变")
		}
		var c canvas.Content
		if e := json.Unmarshal(raw, &c); e != nil {
			return e
		}
		refs, e := canvas.Validate(c)
		if e != nil {
			return e
		}
		if int64(c.Width)*int64(c.Height)*int64(v.Scale*v.Scale) > 40000000 || c.Width*v.Scale > 8192 || c.Height*v.Scale > 8192 {
			return platform.Fail(400, "导出像素超限")
		}
		if e = asset.SyncReferences(ctx, t, "design", id, owner, false, refs); e != nil {
			return e
		}
		check := platform.Random()
		expires := time.Now().UTC().Add(10 * time.Minute)
		if _, e = t.ExecContext(ctx, `INSERT INTO design_export_checks(id,owner_id,design_id,version,format,width,height,expires_at) VALUES(?,?,?,?,?,?,?,?)`, check, owner, id, current, v.Format, c.Width*v.Scale, c.Height*v.Scale, expires); e != nil {
			return e
		}
		out = map[string]any{"checkId": check, "designId": platform.ID(id), "version": current, "format": v.Format, "width": c.Width * v.Scale, "height": c.Height * v.Scale, "expiresAt": expires, "renderMode": "browser", "watermark": false}
		return nil
	})
	return
}
func (s *Service) ExportEvent(ctx context.Context, owner int64, v ExportEvent, rid string) error {
	if !KeyPattern.MatchString(v.EventID) || len(v.CheckID) != 64 || (v.Status != "succeeded" && v.Status != "failed") || !errorCodePattern.MatchString(v.ErrorCode) || (v.Status == "succeeded" && v.ErrorCode != "") {
		return platform.Fail(400, "导出事件字段无效")
	}
	return platform.Tx(ctx, s.DB, func(t *sql.Tx) error {
		if e := platform.LockContent(ctx, t); e != nil {
			return e
		}
		var check, status, code string
		e := t.QueryRowContext(ctx, `SELECT check_id,status,error_code FROM design_export_events WHERE owner_id=? AND event_key=?`, owner, v.EventID).Scan(&check, &status, &code)
		if e == nil {
			if check == v.CheckID && status == v.Status && code == v.ErrorCode {
				return nil
			}
			return platform.Fail(409, "事件 ID 已用于不同结果")
		}
		if e != sql.ErrNoRows {
			return e
		}
		var expires time.Time
		if e = t.QueryRowContext(ctx, `SELECT expires_at FROM design_export_checks WHERE id=? AND owner_id=?`, v.CheckID, owner).Scan(&expires); e != nil {
			return e
		}
		if time.Now().After(expires) {
			return platform.Fail(409, "导出检查已过期，请重新检查")
		}
		if _, e = t.ExecContext(ctx, `INSERT INTO design_export_events(owner_id,event_key,check_id,status,error_code,created_at) VALUES(?,?,?,?,?,UTC_TIMESTAMP(6))`, owner, v.EventID, v.CheckID, v.Status, v.ErrorCode); e != nil {
			return e
		}
		return platform.Audit(ctx, t, owner, "export."+v.Status, v.CheckID, rid)
	})
}
func (s *Service) Exports(ctx context.Context, p platform.Page) (platform.List, error) {
	where := " WHERE 1=1"
	args := []any{}
	if p.Status != "" {
		if p.Status != "succeeded" && p.Status != "failed" {
			return platform.List{}, platform.Fail(400, "状态无效")
		}
		where += " AND e.status=?"
		args = append(args, p.Status)
	}
	out := platform.List{Items: []any{}, Page: p.Number, PageSize: p.Size}
	if e := s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM design_export_events e`+where, args...).Scan(&out.Total); e != nil {
		return out, e
	}
	rows, e := s.DB.QueryContext(ctx, `SELECT e.id,e.owner_id,c.design_id,c.version,c.format,e.status,e.error_code,e.created_at FROM design_export_events e JOIN design_export_checks c ON c.id=e.check_id`+where+` ORDER BY e.id DESC LIMIT ? OFFSET ?`, append(args, p.Size, p.Offset())...)
	if e != nil {
		return out, e
	}
	defer rows.Close()
	items := []any{}
	for rows.Next() {
		var id, owner, design, version int64
		var format, status, code string
		var at time.Time
		if e = rows.Scan(&id, &owner, &design, &version, &format, &status, &code, &at); e != nil {
			return out, e
		}
		items = append(items, map[string]any{"id": platform.ID(id), "ownerId": platform.ID(owner), "designId": platform.ID(design), "version": version, "format": format, "status": status, "errorCode": code, "createdAt": at})
	}
	out.Items = items
	return out, rows.Err()
}
func (s *Service) Stats(ctx context.Context, from, to time.Time) (map[string]any, error) {
	if !to.After(from) || to.Sub(from) > 366*24*time.Hour {
		return nil, platform.Fail(400, "统计区间须为 1—366 天以内的有效区间")
	}
	out := map[string]any{"from": from, "to": to, "exportSource": "client-reported", "interval": "[from,to)"}
	queries := map[string]string{"users": `SELECT COUNT(*) FROM design_accounts WHERE kind='user'`, "designs": `SELECT COUNT(*) FROM design_designs WHERE deleted_at IS NULL`, "publishedTemplates": `SELECT COUNT(*) FROM design_templates WHERE status='published'`, "favorites": `SELECT COUNT(*) FROM design_favorites`}
	for k, q := range queries {
		var n int64
		if e := s.DB.QueryRowContext(ctx, q).Scan(&n); e != nil {
			return nil, e
		}
		out[k] = n
	}
	for _, status := range []string{"succeeded", "failed"} {
		var n int64
		if e := s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM design_export_events WHERE status=? AND created_at>=? AND created_at<?`, status, from, to).Scan(&n); e != nil {
			return nil, e
		}
		out["exports_"+status] = n
	}
	rows, e := s.DB.QueryContext(ctx, `SELECT DATE_FORMAT(created_at,'%Y-%m-%d'),SUM(status='succeeded'),SUM(status='failed') FROM design_export_events WHERE created_at>=? AND created_at<? GROUP BY DATE_FORMAT(created_at,'%Y-%m-%d') ORDER BY 1`, from, to)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	series := []any{}
	for rows.Next() {
		var day string
		var succeeded, failed int64
		if e = rows.Scan(&day, &succeeded, &failed); e != nil {
			return nil, e
		}
		series = append(series, map[string]any{"date": day, "succeeded": succeeded, "failed": failed})
	}
	if e = rows.Err(); e != nil {
		return nil, e
	}
	out["dailyExportsUTC"] = series
	return out, nil
}
