package operation

import (
	"context"
	"database/sql"
	"looklook/app/design/internal/platform"
)

func (s *Service) Cleanup(ctx context.Context) error {
	return platform.Tx(ctx, s.DB, func(t *sql.Tx) error {
		if e := platform.LockContent(ctx, t); e != nil {
			return e
		}
		_, e := t.ExecContext(ctx, `DELETE FROM design_export_checks WHERE expires_at<DATE_SUB(UTC_TIMESTAMP(6),INTERVAL 1 DAY) AND NOT EXISTS(SELECT 1 FROM design_export_events e WHERE e.check_id=design_export_checks.id) LIMIT 1000`)
		return e
	})
}
