package platform

import (
	"context"
	"database/sql"
)

// Shared DB lock keeps quota/reference mutations atomic across API processes.
func LockContent(ctx context.Context, t *sql.Tx) error {
	var n int
	return t.QueryRowContext(ctx, `SELECT id FROM design_content_lock WHERE id=1 FOR UPDATE`).Scan(&n)
}
func Quota(ctx context.Context, t *sql.Tx, account, delta, limit int64) error {
	if _, e := t.ExecContext(ctx, `INSERT IGNORE INTO design_storage_usage(account_id,used_bytes) VALUES(?,0)`, account); e != nil {
		return e
	}
	var used int64
	if e := t.QueryRowContext(ctx, `SELECT used_bytes FROM design_storage_usage WHERE account_id=? FOR UPDATE`, account).Scan(&used); e != nil {
		return e
	}
	if delta > 0 && used+delta > limit {
		return Fail(413, "存储额度不足")
	}
	if used+delta < 0 {
		return Fail(409, "存储计数不一致")
	}
	_, e := t.ExecContext(ctx, `UPDATE design_storage_usage SET used_bytes=used_bytes+? WHERE account_id=?`, delta, account)
	return e
}
