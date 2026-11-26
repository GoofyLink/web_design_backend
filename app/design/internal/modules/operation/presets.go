package operation

import (
	"context"
	"database/sql"
	"encoding/json"
	"looklook/app/design/internal/platform"
	"strings"
	"unicode/utf8"
)

type CanvasPreset struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Category string `json:"category"`
	Width    int    `json:"width"`
	Height   int    `json:"height"`
	Enabled  bool   `json:"enabled"`
}
type CanvasPresets struct {
	Version int64          `json:"version"`
	Items   []CanvasPreset `json:"items"`
}

func (s *Service) CanvasPresets(ctx context.Context, public bool) (out CanvasPresets, err error) {
	var raw []byte
	err = s.DB.QueryRowContext(ctx, `SELECT version,items FROM design_canvas_presets WHERE id=1`).Scan(&out.Version, &raw)
	if err != nil {
		return
	}
	err = json.Unmarshal(raw, &out.Items)
	if err != nil {
		return
	}
	if public {
		items := []CanvasPreset{}
		for _, v := range out.Items {
			if v.Enabled {
				items = append(items, v)
			}
		}
		out.Items = items
	}
	return
}
func (s *Service) SaveCanvasPresets(ctx context.Context, actor int64, v CanvasPresets, rid string) (CanvasPresets, error) {
	if v.Version < 1 || v.Items == nil || len(v.Items) > 100 {
		return v, platform.Fail(400, "配置版本无效或尺寸超过 100 个")
	}
	seen := map[string]bool{}
	for i := range v.Items {
		p := &v.Items[i]
		p.Name = strings.TrimSpace(p.Name)
		p.Category = strings.TrimSpace(p.Category)
		if p.ID == "" || len(p.ID) > 64 || seen[p.ID] || !utf8.ValidString(p.Name) || p.Name == "" || utf8.RuneCountInString(p.Name) > 40 || !utf8.ValidString(p.Category) || utf8.RuneCountInString(p.Category) > 20 || p.Width < 1 || p.Height < 1 || p.Width > 8192 || p.Height > 8192 || int64(p.Width)*int64(p.Height) > 40000000 {
			return v, platform.Fail(400, "尺寸名称、分类、标识或宽高无效（单边 1–8192，总像素不超过 4000 万）")
		}
		seen[p.ID] = true
	}
	raw, e := json.Marshal(v.Items)
	if e != nil {
		return v, e
	}
	e = platform.Tx(ctx, s.DB, func(t *sql.Tx) error {
		var current int64
		if e := t.QueryRowContext(ctx, `SELECT version FROM design_canvas_presets WHERE id=1 FOR UPDATE`).Scan(&current); e != nil {
			return e
		}
		if current != v.Version {
			return platform.Fail(409, "配置已被修改，请重新加载后再保存")
		}
		if _, e := t.ExecContext(ctx, `UPDATE design_canvas_presets SET items=?,version=version+1 WHERE id=1`, string(raw)); e != nil {
			return e
		}
		return platform.Audit(ctx, t, actor, "canvas.presets", "1", rid)
	})
	if e == nil {
		v.Version++
	}
	return v, e
}
