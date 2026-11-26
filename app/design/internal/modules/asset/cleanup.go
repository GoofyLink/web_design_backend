package asset

import (
	"context"
	"database/sql"
	"looklook/app/design/internal/platform"
	"os"
	"path/filepath"
	"time"
)

// Reclaim only known tombstones or old orphaned files, never a referenced file.
func (s *Service) Cleanup(ctx context.Context) (int, error) {
	count := 0
	err := platform.Tx(ctx, s.DB, func(t *sql.Tx) error {
		if e := platform.LockContent(ctx, t); e != nil {
			return e
		}
		for _, dir := range []string{"files", "tmp"} {
			folder := filepath.Join(s.Root, dir)
			f, e := os.Open(folder)
			if os.IsNotExist(e) {
				continue
			}
			if e != nil {
				return e
			}
			entries, e := f.ReadDir(-1)
			f.Close()
			if e != nil {
				return e
			}
			for _, entry := range entries {
				if count >= 100 {
					return nil
				}
				if e = ctx.Err(); e != nil {
					return e
				}
				if !entry.Type().IsRegular() {
					continue
				}
				info, e := entry.Info()
				if e != nil {
					return e
				}
				remove := false
				if dir == "files" {
					if _, err := s.filePath(entry.Name()); err != nil {
						continue
					}
					var status string
					e = t.QueryRowContext(ctx, `SELECT status FROM design_assets WHERE storage_key=?`, entry.Name()).Scan(&status)
					if e != nil && e != sql.ErrNoRows {
						return e
					}
					remove = status == "deleted" || (e == sql.ErrNoRows && info.ModTime().Before(time.Now().Add(-24*time.Hour)))
				} else {
					remove = info.ModTime().Before(time.Now().Add(-24 * time.Hour))
				}
				if remove {
					if e = os.Remove(filepath.Join(folder, entry.Name())); e != nil && !os.IsNotExist(e) {
						return e
					}
					count++
				}
			}
		}
		return nil
	})
	return count, err
}
