package migrations

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"embed"
	"fmt"
	"strings"
)

//go:embed *.sql
var files embed.FS

const Version = 6

var names = []string{"001_initial.sql", "002_assets_workspace.sql", "003_operations_jobs.sql", "004_internal_users.sql", "005_user_management.sql", "006_canvas_presets.sql"}

func Apply(ctx context.Context, db *sql.DB) error {
	conn, e := db.Conn(ctx)
	if e != nil {
		return e
	}
	defer conn.Close()
	var locked int
	if e = conn.QueryRowContext(ctx, `SELECT GET_LOCK(CONCAT(DATABASE(),':design:migrate'),10)`).Scan(&locked); e != nil {
		return e
	}
	if locked != 1 {
		return fmt.Errorf("migration already running")
	}
	defer conn.ExecContext(context.Background(), `SELECT RELEASE_LOCK(CONCAT(DATABASE(),':design:migrate'))`)
	if _, e = conn.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS design_schema_migrations(version INT PRIMARY KEY,checksum CHAR(64) NOT NULL,applied_at DATETIME(6) NOT NULL)`); e != nil {
		return e
	}
	var max int
	if e = conn.QueryRowContext(ctx, `SELECT COALESCE(MAX(version),0) FROM design_schema_migrations`).Scan(&max); e != nil {
		return e
	}
	var count int
	if e = conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM design_schema_migrations`).Scan(&count); e != nil {
		return e
	}
	if count != max {
		return fmt.Errorf("migration history has gaps")
	}
	if max > Version {
		return fmt.Errorf("database schema is newer than this binary")
	}
	for i, name := range names {
		b, e := files.ReadFile(name)
		if e != nil {
			return e
		}
		sum := fmt.Sprintf("%x", sha256.Sum256(b))
		var existing string
		e = conn.QueryRowContext(ctx, `SELECT checksum FROM design_schema_migrations WHERE version=?`, i+1).Scan(&existing)
		if e == nil {
			if existing != sum {
				return fmt.Errorf("migration %s checksum mismatch", name)
			}
			continue
		}
		if e != sql.ErrNoRows {
			return e
		}
		for _, statement := range strings.Split(string(b), ";") {
			if strings.TrimSpace(statement) == "" {
				continue
			}
			if _, e = conn.ExecContext(ctx, statement); e != nil {
				return fmt.Errorf("migration %s: %w", name, e)
			}
		}
		if _, e = conn.ExecContext(ctx, `INSERT INTO design_schema_migrations VALUES(?,?,UTC_TIMESTAMP(6))`, i+1, sum); e != nil {
			return e
		}
	}
	return nil
}
func Check(ctx context.Context, db *sql.DB) error {
	rows, e := db.QueryContext(ctx, `SELECT version,checksum FROM design_schema_migrations ORDER BY version`)
	if e != nil {
		return fmt.Errorf("run migrate first: %w", e)
	}
	defer rows.Close()
	n := 0
	for rows.Next() {
		var version int
		var checksum string
		if e = rows.Scan(&version, &checksum); e != nil {
			return e
		}
		if version != n+1 || version > Version {
			return fmt.Errorf("incompatible migration version")
		}
		b, _ := files.ReadFile(names[n])
		if checksum != fmt.Sprintf("%x", sha256.Sum256(b)) {
			return fmt.Errorf("migration checksum mismatch")
		}
		n++
	}
	if e = rows.Err(); e != nil {
		return e
	}
	if n != Version {
		return fmt.Errorf("run migrate: expected schema %d, got %d", Version, n)
	}
	return nil
}
