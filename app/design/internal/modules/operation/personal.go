package operation

import (
	"context"
	"database/sql"
	"looklook/app/design/internal/platform"
	"strings"
)

type Folder struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Version int64  `json:"version"`
	Count   int64  `json:"count"`
}

func (s *Service) Favorite(ctx context.Context, owner, template int64, add bool) error {
	return platform.Tx(ctx, s.DB, func(t *sql.Tx) error {
		if e := platform.LockContent(ctx, t); e != nil {
			return e
		}
		if !add {
			_, e := t.ExecContext(ctx, `DELETE FROM design_favorites WHERE owner_id=? AND template_id=?`, owner, template)
			return e
		}
		var n int
		if e := t.QueryRowContext(ctx, `SELECT 1 FROM design_templates t JOIN design_categories c ON c.id=t.published_category_id WHERE t.id=? AND t.status='published' AND c.status='active'`, template).Scan(&n); e != nil {
			return e
		}
		_, e := t.ExecContext(ctx, `INSERT IGNORE INTO design_favorites(owner_id,template_id,created_at) VALUES(?,?,UTC_TIMESTAMP(6))`, owner, template)
		return e
	})
}
func (s *Service) Favorites(ctx context.Context, owner int64, p platform.Page) (platform.List, error) {
	out := platform.List{Page: p.Number, PageSize: p.Size, Items: []any{}}
	from := ` FROM design_favorites f JOIN design_templates t ON t.id=f.template_id AND t.status='published' JOIN design_categories c ON c.id=t.published_category_id AND c.status='active' WHERE f.owner_id=?`
	if e := s.DB.QueryRowContext(ctx, `SELECT COUNT(*)`+from, owner).Scan(&out.Total); e != nil {
		return out, e
	}
	rows, e := s.DB.QueryContext(ctx, `SELECT t.id,JSON_UNQUOTE(JSON_EXTRACT(t.published_json,'$.title')),JSON_UNQUOTE(JSON_EXTRACT(t.published_json,'$.cover'))`+from+` ORDER BY f.created_at DESC,t.id DESC LIMIT ? OFFSET ?`, owner, p.Size, p.Offset())
	if e != nil {
		return out, e
	}
	defer rows.Close()
	items := []any{}
	for rows.Next() {
		var id int64
		var title, cover string
		if e = rows.Scan(&id, &title, &cover); e != nil {
			return out, e
		}
		items = append(items, map[string]any{"id": platform.ID(id), "title": title, "cover": cover})
	}
	out.Items = items
	return out, rows.Err()
}
func (s *Service) Folders(ctx context.Context, owner int64) ([]Folder, error) {
	rows, e := s.DB.QueryContext(ctx, `SELECT f.id,f.name,f.version,(SELECT COUNT(*) FROM design_folder_items i JOIN design_designs d ON d.id=i.design_id WHERE i.folder_id=f.id AND d.deleted_at IS NULL) FROM design_folders f WHERE owner_id=? ORDER BY id DESC LIMIT 200`, owner)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []Folder{}
	for rows.Next() {
		var v Folder
		var n int64
		if e = rows.Scan(&n, &v.Name, &v.Version, &v.Count); e != nil {
			return nil, e
		}
		v.ID = platform.ID(n)
		out = append(out, v)
	}
	return out, rows.Err()
}
func (s *Service) FolderSave(ctx context.Context, owner, id, version int64, name, rid string) (out any, err error) {
	name = strings.TrimSpace(name)
	if name == "" || len([]rune(name)) > 100 {
		return nil, platform.Fail(400, "文件夹名称须为 1—100 字")
	}
	err = platform.Tx(ctx, s.DB, func(t *sql.Tx) error {
		if e := platform.LockContent(ctx, t); e != nil {
			return e
		}
		if id == 0 {
			var count int
			if e := t.QueryRowContext(ctx, `SELECT COUNT(*) FROM design_folders WHERE owner_id=?`, owner).Scan(&count); e != nil {
				return e
			}
			if count >= 200 {
				return platform.Fail(409, "最多 200 个文件夹")
			}
			r, e := t.ExecContext(ctx, `INSERT INTO design_folders(owner_id,name) VALUES(?,?)`, owner, name)
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
			if e := t.QueryRowContext(ctx, `SELECT version FROM design_folders WHERE id=? AND owner_id=?`, id, owner).Scan(&current); e != nil {
				return e
			}
			if version != current {
				return platform.Fail(409, "文件夹版本冲突")
			}
			if _, e := t.ExecContext(ctx, `UPDATE design_folders SET name=?,version=version+1 WHERE id=?`, name, id); e != nil {
				return e
			}
			version++
		}
		out = map[string]any{"id": platform.ID(id), "version": version}
		return platform.Audit(ctx, t, owner, "folder.save", platform.ID(id), rid)
	})
	return
}
func (s *Service) FolderDelete(ctx context.Context, owner, id, version int64, rid string) error {
	return platform.Tx(ctx, s.DB, func(t *sql.Tx) error {
		if e := platform.LockContent(ctx, t); e != nil {
			return e
		}
		var current int64
		if e := t.QueryRowContext(ctx, `SELECT version FROM design_folders WHERE id=? AND owner_id=?`, id, owner).Scan(&current); e != nil {
			return e
		}
		if version != current {
			return platform.Fail(409, "文件夹版本冲突")
		}
		if _, e := t.ExecContext(ctx, `DELETE FROM design_folder_items WHERE folder_id=?`, id); e != nil {
			return e
		}
		if _, e := t.ExecContext(ctx, `DELETE FROM design_folders WHERE id=?`, id); e != nil {
			return e
		}
		return platform.Audit(ctx, t, owner, "folder.delete", platform.ID(id), rid)
	})
}
func (s *Service) Move(ctx context.Context, owner, design, version, folder int64, rid string) (any, error) {
	err := platform.Tx(ctx, s.DB, func(t *sql.Tx) error {
		if e := platform.LockContent(ctx, t); e != nil {
			return e
		}
		var current int64
		if e := t.QueryRowContext(ctx, `SELECT version FROM design_designs WHERE id=? AND owner_id=? AND deleted_at IS NULL`, design, owner).Scan(&current); e != nil {
			return e
		}
		if version != current {
			return platform.Fail(409, "作品版本冲突")
		}
		if folder > 0 {
			var n int
			if e := t.QueryRowContext(ctx, `SELECT 1 FROM design_folders WHERE id=? AND owner_id=?`, folder, owner).Scan(&n); e != nil {
				return e
			}
			if _, e := t.ExecContext(ctx, `INSERT INTO design_folder_items(design_id,folder_id) VALUES(?,?) ON DUPLICATE KEY UPDATE folder_id=VALUES(folder_id)`, design, folder); e != nil {
				return e
			}
		} else {
			if _, e := t.ExecContext(ctx, `DELETE FROM design_folder_items WHERE design_id=?`, design); e != nil {
				return e
			}
		}
		if _, e := t.ExecContext(ctx, `UPDATE design_designs SET version=version+1,updated_at=UTC_TIMESTAMP(6) WHERE id=?`, design); e != nil {
			return e
		}
		return platform.Audit(ctx, t, owner, "design.move", platform.ID(design), rid)
	})
	return map[string]any{"id": platform.ID(design), "version": version + 1}, err
}
func (s *Service) FolderItems(ctx context.Context, owner, folder int64, p platform.Page) (platform.List, error) {
	out := platform.List{Page: p.Number, PageSize: p.Size, Items: []any{}}
	var n int
	if e := s.DB.QueryRowContext(ctx, `SELECT 1 FROM design_folders WHERE id=? AND owner_id=?`, folder, owner).Scan(&n); e != nil {
		return out, platform.Normalize(e)
	}
	from := ` FROM design_folder_items i JOIN design_designs d ON d.id=i.design_id WHERE i.folder_id=? AND d.owner_id=? AND d.deleted_at IS NULL`
	if e := s.DB.QueryRowContext(ctx, `SELECT COUNT(*)`+from, folder, owner).Scan(&out.Total); e != nil {
		return out, e
	}
	rows, e := s.DB.QueryContext(ctx, `SELECT d.id,d.title,d.version`+from+` ORDER BY d.updated_at DESC,d.id DESC LIMIT ? OFFSET ?`, folder, owner, p.Size, p.Offset())
	if e != nil {
		return out, e
	}
	defer rows.Close()
	items := []any{}
	for rows.Next() {
		var id, version int64
		var title string
		if e = rows.Scan(&id, &title, &version); e != nil {
			return out, e
		}
		items = append(items, map[string]any{"id": platform.ID(id), "title": title, "version": version})
	}
	out.Items = items
	return out, rows.Err()
}
