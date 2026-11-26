package identity

import (
	"context"
	"database/sql"
	"golang.org/x/crypto/bcrypt"
	"looklook/app/design/internal/platform"
	"strings"
	"time"
	"unicode/utf8"
)

func (s *Service) CreateUser(ctx context.Context, actor int64, username, password, nickname, requestID string) (id string, err error) {
	username, err = credentials(username, password)
	if err != nil {
		return
	}
	nickname = strings.TrimSpace(nickname)
	if !utf8.ValidString(nickname) || utf8.RuneCountInString(nickname) > 64 {
		return "", platform.Fail(400, "姓名最多 64 个字")
	}
	if nickname == "" {
		nickname = username
	}
	h, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	err = platform.Tx(ctx, s.DB, func(t *sql.Tx) error {
		if e := lock(ctx, t); e != nil {
			return e
		}
		r, e := t.ExecContext(ctx, `INSERT INTO design_accounts(kind,username,password_hash,nickname,created_at,updated_at) VALUES('user',?,?,?,?,?)`, username, string(h), nickname, time.Now().UTC(), time.Now().UTC())
		if e != nil {
			return e
		}
		n, e := r.LastInsertId()
		if e != nil {
			return e
		}
		id = platform.ID(n)
		return platform.Audit(ctx, t, actor, "user.create", id, requestID)
	})
	return
}

// All privilege/status mutations share a database row lock. Concurrent requests
// cannot remove two final super-admins, or race a session revocation with login.
func protectSuper(ctx context.Context, t *sql.Tx) error {
	var n int
	e := t.QueryRowContext(ctx, `SELECT COUNT(DISTINCT a.id) FROM design_accounts a JOIN design_admin_roles ar ON ar.account_id=a.id JOIN design_roles r ON r.id=ar.role_id WHERE a.kind='admin' AND a.status='active' AND r.code='super_admin'`).Scan(&n)
	if e != nil {
		return e
	}
	if n == 0 {
		return platform.Fail(409, "必须保留至少一个有效超级管理员")
	}
	return nil
}
func (s *Service) CreateAdmin(ctx context.Context, actor int64, username, password string, roleIDs []int64, bootstrap bool, requestID string) (id string, err error) {
	username, err = credentials(username, password)
	if err != nil {
		return
	}
	h, e := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if e != nil {
		return "", e
	}
	err = platform.Tx(ctx, s.DB, func(t *sql.Tx) error {
		if e := lock(ctx, t); e != nil {
			return e
		}
		if bootstrap {
			var n int
			if e := t.QueryRowContext(ctx, `SELECT COUNT(*) FROM design_accounts WHERE kind='admin'`).Scan(&n); e != nil {
				return e
			}
			if n > 0 {
				return platform.Fail(409, "已有管理员，请通过管理接口创建")
			}
			var role int64
			if e := t.QueryRowContext(ctx, `SELECT id FROM design_roles WHERE code='super_admin'`).Scan(&role); e != nil {
				return e
			}
			roleIDs = []int64{role}
		}
		r, e := t.ExecContext(ctx, `INSERT INTO design_accounts(kind,username,password_hash,nickname,created_at,updated_at) VALUES('admin',?,?,?,?,?)`, username, string(h), username, time.Now().UTC(), time.Now().UTC())
		if e != nil {
			return e
		}
		n, e := r.LastInsertId()
		if e != nil {
			return e
		}
		id = platform.ID(n)
		if e = setRoles(ctx, t, n, roleIDs); e != nil {
			return e
		}
		return platform.Audit(ctx, t, actor, "admin.create", id, requestID)
	})
	return
}
func setRoles(ctx context.Context, t *sql.Tx, id int64, roles []int64) error {
	if len(roles) == 0 || len(roles) > 20 {
		return platform.Fail(400, "请选择 1—20 个角色")
	}
	if _, e := t.ExecContext(ctx, `DELETE FROM design_admin_roles WHERE account_id=?`, id); e != nil {
		return e
	}
	seen := map[int64]bool{}
	for _, role := range roles {
		if seen[role] {
			continue
		}
		seen[role] = true
		var n int
		if e := t.QueryRowContext(ctx, `SELECT 1 FROM design_roles WHERE id=?`, role).Scan(&n); e == sql.ErrNoRows {
			return platform.Fail(400, "角色不存在")
		} else if e != nil {
			return e
		}
		if _, e := t.ExecContext(ctx, `INSERT INTO design_admin_roles VALUES(?,?)`, id, role); e != nil {
			return e
		}
	}
	return nil
}
func (s *Service) SetAdminRoles(ctx context.Context, actor, target int64, roles []int64, requestID string) error {
	return platform.Tx(ctx, s.DB, func(t *sql.Tx) error {
		if e := lock(ctx, t); e != nil {
			return e
		}
		var n int
		if e := t.QueryRowContext(ctx, `SELECT 1 FROM design_accounts WHERE id=? AND kind='admin'`, target).Scan(&n); e != nil {
			return e
		}
		if e := setRoles(ctx, t, target, roles); e != nil {
			return e
		}
		if e := protectSuper(ctx, t); e != nil {
			return e
		}
		return platform.Audit(ctx, t, actor, "admin.roles", platform.ID(target), requestID)
	})
}
func (s *Service) SetStatus(ctx context.Context, actor, target int64, kind, status, requestID string) error {
	if status != "active" && status != "disabled" {
		return platform.Fail(400, "状态只能为 active 或 disabled")
	}
	return platform.Tx(ctx, s.DB, func(t *sql.Tx) error {
		if e := lock(ctx, t); e != nil {
			return e
		}
		if e := platform.Affected(t.ExecContext(ctx, `UPDATE design_accounts SET status=?,updated_at=UTC_TIMESTAMP(6) WHERE id=? AND kind=?`, status, target, kind)); e != nil {
			return e
		}
		if kind == "admin" {
			if e := protectSuper(ctx, t); e != nil {
				return e
			}
		}
		if status == "disabled" {
			if _, e := t.ExecContext(ctx, `UPDATE design_sessions SET revoked_at=UTC_TIMESTAMP(6) WHERE account_id=?`, target); e != nil {
				return e
			}
		}
		return platform.Audit(ctx, t, actor, kind+".status", platform.ID(target), requestID)
	})
}
func (s *Service) Accounts(ctx context.Context, kind string, p platform.Page) (platform.List, error) {
	if p.Status != "" && p.Status != "active" && p.Status != "disabled" {
		return platform.List{}, platform.Fail(400, "账户状态无效")
	}
	args := []any{kind, "%" + p.Keyword + "%"}
	where := ` WHERE kind=? AND username LIKE ?`
	if p.Status != "" {
		where += ` AND status=?`
		args = append(args, p.Status)
	}
	out := platform.List{Items: []Account{}, Page: p.Number, PageSize: p.Size}
	if e := s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM design_accounts`+where, args...).Scan(&out.Total); e != nil {
		return out, e
	}
	rows, e := s.DB.QueryContext(ctx, `SELECT id,kind,username,nickname,avatar,status FROM design_accounts`+where+` ORDER BY id DESC LIMIT ? OFFSET ?`, append(args, p.Size, p.Offset())...)
	if e != nil {
		return out, e
	}
	defer rows.Close()
	items := []Account{}
	for rows.Next() {
		var a Account
		var id int64
		if e = rows.Scan(&id, &a.Kind, &a.Username, &a.Nickname, &a.Avatar, &a.Status); e != nil {
			return out, e
		}
		a.ID = platform.ID(id)
		items = append(items, a)
	}
	out.Items = items
	return out, rows.Err()
}

func (s *Service) AccountDetail(ctx context.Context, kind string, id int64) (map[string]any, error) {
	a, e := s.Me(ctx, Principal{ID: id, Kind: kind})
	if e != nil {
		return nil, e
	}
	out := map[string]any{"id": a.ID, "kind": a.Kind, "username": a.Username, "nickname": a.Nickname, "avatar": a.Avatar, "status": a.Status}
	if kind == "admin" {
		rows, e := s.DB.QueryContext(ctx, `SELECT role_id FROM design_admin_roles WHERE account_id=? ORDER BY role_id`, id)
		if e != nil {
			return nil, e
		}
		defer rows.Close()
		ids := []string{}
		for rows.Next() {
			var n int64
			if e = rows.Scan(&n); e != nil {
				return nil, e
			}
			ids = append(ids, platform.ID(n))
		}
		if e = rows.Err(); e != nil {
			return nil, e
		}
		out["roleIds"] = ids
	}
	return out, nil
}

type Role struct {
	ID          string   `json:"id"`
	Code        string   `json:"code"`
	Name        string   `json:"name"`
	Permissions []string `json:"permissions"`
}

func (s *Service) Roles(ctx context.Context) ([]Role, error) {
	rows, e := s.DB.QueryContext(ctx, `SELECT id,code,name FROM design_roles ORDER BY id`)
	if e != nil {
		return nil, e
	}
	items := []Role{}
	for rows.Next() {
		var r Role
		var id int64
		if e = rows.Scan(&id, &r.Code, &r.Name); e != nil {
			rows.Close()
			return nil, e
		}
		r.ID = platform.ID(id)
		r.Permissions = []string{}
		items = append(items, r)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return nil, e
	}
	for i := range items {
		rows, e = s.DB.QueryContext(ctx, `SELECT permission_code FROM design_role_permissions WHERE role_id=? ORDER BY permission_code`, items[i].ID)
		if e != nil {
			return nil, e
		}
		for rows.Next() {
			var code string
			if e = rows.Scan(&code); e != nil {
				rows.Close()
				return nil, e
			}
			items[i].Permissions = append(items[i].Permissions, code)
		}
		e = rows.Err()
		rows.Close()
		if e != nil {
			return nil, e
		}
	}
	return items, nil
}
func (s *Service) Permissions(ctx context.Context) ([]map[string]string, error) {
	rows, e := s.DB.QueryContext(ctx, `SELECT code,name FROM design_permissions ORDER BY code`)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	items := []map[string]string{}
	for rows.Next() {
		var code, name string
		if e = rows.Scan(&code, &name); e != nil {
			return nil, e
		}
		items = append(items, map[string]string{"code": code, "name": name})
	}
	return items, rows.Err()
}
func (s *Service) SaveRole(ctx context.Context, actor, id int64, code, name string, codes []string, requestID string) (result string, err error) {
	if !usernamePattern.MatchString(code) || len([]rune(strings.TrimSpace(name))) < 1 || len([]rune(name)) > 64 || len(codes) > 100 {
		return "", platform.Fail(400, "角色代码、名称或权限数量无效")
	}
	if code == "super_admin" {
		return "", platform.Fail(403, "内置超级管理员角色不可编辑")
	}
	err = platform.Tx(ctx, s.DB, func(t *sql.Tx) error {
		if e := lock(ctx, t); e != nil {
			return e
		}
		if id == 0 {
			r, e := t.ExecContext(ctx, `INSERT INTO design_roles(code,name) VALUES(?,?)`, code, name)
			if e != nil {
				return e
			}
			id, e = r.LastInsertId()
			if e != nil {
				return e
			}
		} else {
			var current string
			if e := t.QueryRowContext(ctx, `SELECT code FROM design_roles WHERE id=?`, id).Scan(&current); e != nil {
				return e
			}
			if current == "super_admin" {
				return platform.Fail(403, "内置角色不可编辑")
			}
			if _, e := t.ExecContext(ctx, `UPDATE design_roles SET code=?,name=? WHERE id=?`, code, name, id); e != nil {
				return e
			}
		}
		if _, e := t.ExecContext(ctx, `DELETE FROM design_role_permissions WHERE role_id=?`, id); e != nil {
			return e
		}
		seen := map[string]bool{}
		for _, v := range codes {
			if seen[v] {
				continue
			}
			seen[v] = true
			var n int
			if e := t.QueryRowContext(ctx, `SELECT 1 FROM design_permissions WHERE code=?`, v).Scan(&n); e == sql.ErrNoRows {
				return platform.Fail(400, "未知权限码")
			} else if e != nil {
				return e
			}
			if _, e := t.ExecContext(ctx, `INSERT INTO design_role_permissions VALUES(?,?)`, id, v); e != nil {
				return e
			}
		}
		result = platform.ID(id)
		return platform.Audit(ctx, t, actor, "role.save", result, requestID)
	})
	return
}
func (s *Service) DeleteRole(ctx context.Context, actor, id int64, requestID string) error {
	return platform.Tx(ctx, s.DB, func(t *sql.Tx) error {
		if e := lock(ctx, t); e != nil {
			return e
		}
		var code string
		if e := t.QueryRowContext(ctx, `SELECT code FROM design_roles WHERE id=?`, id).Scan(&code); e != nil {
			return e
		}
		if code == "super_admin" {
			return platform.Fail(403, "内置角色不可删除")
		}
		var n int
		if e := t.QueryRowContext(ctx, `SELECT COUNT(*) FROM design_admin_roles WHERE role_id=?`, id).Scan(&n); e != nil {
			return e
		}
		if n > 0 {
			return platform.Fail(409, "角色仍关联管理员")
		}
		if _, e := t.ExecContext(ctx, `DELETE FROM design_role_permissions WHERE role_id=?`, id); e != nil {
			return e
		}
		if _, e := t.ExecContext(ctx, `DELETE FROM design_roles WHERE id=?`, id); e != nil {
			return e
		}
		return platform.Audit(ctx, t, actor, "role.delete", platform.ID(id), requestID)
	})
}
func (s *Service) AuditLogs(ctx context.Context, p platform.Page) (platform.List, error) {
	out := platform.List{Items: []any{}, Page: p.Number, PageSize: p.Size}
	if e := s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM design_audit_logs`).Scan(&out.Total); e != nil {
		return out, e
	}
	rows, e := s.DB.QueryContext(ctx, `SELECT id,actor_id,action,target,request_id,created_at FROM design_audit_logs ORDER BY id DESC LIMIT ? OFFSET ?`, p.Size, p.Offset())
	if e != nil {
		return out, e
	}
	defer rows.Close()
	items := []any{}
	for rows.Next() {
		var id, actor int64
		var action, target, rid string
		var at time.Time
		if e = rows.Scan(&id, &actor, &action, &target, &rid, &at); e != nil {
			return out, e
		}
		items = append(items, map[string]any{"id": platform.ID(id), "actorId": platform.ID(actor), "action": action, "target": target, "requestId": rid, "createdAt": at})
	}
	out.Items = items
	return out, rows.Err()
}

// UpdateUser changes internal user identity; administrator accounts cannot be targeted.
func (s *Service) UpdateUser(ctx context.Context, actor, target int64, username, nickname, requestID string) error {
	username = strings.ToLower(strings.TrimSpace(username))
	if !usernamePattern.MatchString(username) {
		return platform.Fail(400, "用户名须为 3—32 位字母、数字或下划线")
	}
	nickname = strings.TrimSpace(nickname)
	if !utf8.ValidString(nickname) || utf8.RuneCountInString(nickname) > 64 {
		return platform.Fail(400, "姓名最多 64 个字")
	}
	if nickname == "" {
		nickname = username
	}
	return platform.Tx(ctx, s.DB, func(t *sql.Tx) error {
		if e := lock(ctx, t); e != nil {
			return e
		}
		var previous string
		if e := t.QueryRowContext(ctx, `SELECT username FROM design_accounts WHERE id=? AND kind='user'`, target).Scan(&previous); e != nil {
			return e
		}
		if _, e := t.ExecContext(ctx, `UPDATE design_accounts SET username=?,nickname=?,updated_at=UTC_TIMESTAMP(6) WHERE id=? AND kind='user'`, username, nickname, target); e != nil {
			return e
		}
		if previous != username {
			if _, e := t.ExecContext(ctx, `UPDATE design_sessions SET revoked_at=UTC_TIMESTAMP(6) WHERE account_id=?`, target); e != nil {
				return e
			}
		}
		return platform.Audit(ctx, t, actor, "user.update", platform.ID(target), requestID)
	})
}

func (s *Service) ResetUserPassword(ctx context.Context, actor, target int64, password, requestID string) error {
	if _, e := credentials("valid_user", password); e != nil {
		return e
	}
	h, e := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if e != nil {
		return e
	}
	return platform.Tx(ctx, s.DB, func(t *sql.Tx) error {
		if e := lock(ctx, t); e != nil {
			return e
		}
		if e := platform.Affected(t.ExecContext(ctx, `UPDATE design_accounts SET password_hash=?,updated_at=UTC_TIMESTAMP(6) WHERE id=? AND kind='user'`, string(h), target)); e != nil {
			return e
		}
		if _, e := t.ExecContext(ctx, `UPDATE design_sessions SET revoked_at=UTC_TIMESTAMP(6) WHERE account_id=?`, target); e != nil {
			return e
		}
		return platform.Audit(ctx, t, actor, "user.password", platform.ID(target), requestID)
	})
}
