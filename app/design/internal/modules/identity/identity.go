package identity

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/golang-jwt/jwt/v4"
	"golang.org/x/crypto/bcrypt"
	"looklook/app/design/internal/platform"
)

type Service struct {
	DB      *sql.DB
	Secret  []byte
	Access  time.Duration
	Refresh time.Duration
}
type Account struct {
	ID       string `json:"id"`
	Kind     string `json:"kind"`
	Username string `json:"username"`
	Nickname string `json:"nickname"`
	Avatar   string `json:"avatar"`
	Status   string `json:"status"`
}
type Principal struct {
	ID      int64
	Kind    string
	Session string
}
type Tokens struct {
	AccessToken    string    `json:"accessToken"`
	ExpiresAt      time.Time `json:"expiresAt"`
	RefreshToken   string    `json:"-"`
	RefreshExpires time.Time `json:"-"`
}
type claims struct {
	Kind    string `json:"kind"`
	Session string `json:"sid"`
	jwt.RegisteredClaims
}

var usernamePattern = regexp.MustCompile(`^[a-z0-9_]{3,32}$`)
var dummyHash, _ = bcrypt.GenerateFromPassword([]byte("not-a-real-account-password"), bcrypt.DefaultCost)

func credentials(username, password string) (string, error) {
	username = strings.ToLower(strings.TrimSpace(username))
	if !usernamePattern.MatchString(username) {
		return "", platform.Fail(400, "用户名须为 3—32 位字母、数字或下划线")
	}
	if len(password) < 8 || len(password) > 72 || !utf8.ValidString(password) {
		return "", platform.Fail(400, "密码须为 8—72 UTF-8 字节")
	}
	return username, nil
}
func lock(ctx context.Context, t *sql.Tx) error {
	var n int
	return t.QueryRowContext(ctx, `SELECT id FROM design_security_lock WHERE id=1 FOR UPDATE`).Scan(&n)
}
func hash(s string) string { b := sha256.Sum256([]byte(s)); return hex.EncodeToString(b[:]) }
func (s *Service) Register(ctx context.Context, username, password string) (string, error) {
	username, e := credentials(username, password)
	if e != nil {
		return "", e
	}
	h, e := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if e != nil {
		return "", e
	}
	r, e := s.DB.ExecContext(ctx, `INSERT INTO design_accounts(kind,username,password_hash,nickname,created_at,updated_at) VALUES('user',?,?,?,?,?)`, username, string(h), username, time.Now().UTC(), time.Now().UTC())
	if e != nil {
		return "", platform.Normalize(e)
	}
	id, e := r.LastInsertId()
	return platform.ID(id), e
}
func (s *Service) issue(ctx context.Context, t *sql.Tx, id int64, kind, sid, refresh string, expiry time.Time) (Tokens, error) {
	now := time.Now().UTC()
	expires := now.Add(s.Access)
	token, e := jwt.NewWithClaims(jwt.SigningMethodHS256, claims{Kind: kind, Session: sid, RegisteredClaims: jwt.RegisteredClaims{Issuer: "design-api", Subject: platform.ID(id), Audience: jwt.ClaimStrings{"design-" + kind}, IssuedAt: jwt.NewNumericDate(now), ExpiresAt: jwt.NewNumericDate(expires)}}).SignedString(s.Secret)
	return Tokens{token, expires, refresh, expiry}, e
}
func (s *Service) Login(ctx context.Context, kind, username, password string) (out Tokens, err error) {
	username = strings.ToLower(strings.TrimSpace(username))
	if len(password) > 72 || !usernamePattern.MatchString(username) {
		return out, platform.Fail(401, "账号或密码错误")
	}
	err = platform.Tx(ctx, s.DB, func(t *sql.Tx) error {
		if e := lock(ctx, t); e != nil {
			return e
		}
		var id int64
		var stored, status string
		e := t.QueryRowContext(ctx, `SELECT id,password_hash,status FROM design_accounts WHERE kind=? AND username=?`, kind, username).Scan(&id, &stored, &status)
		if errors.Is(e, sql.ErrNoRows) {
			bcrypt.CompareHashAndPassword(dummyHash, []byte(password))
			return platform.Fail(401, "账号或密码错误")
		}
		if e != nil {
			return e
		}
		if bcrypt.CompareHashAndPassword([]byte(stored), []byte(password)) != nil || status != "active" {
			return platform.Fail(401, "账号或密码错误")
		}
		sid, refresh := platform.Random(), platform.Random()
		expiry := time.Now().UTC().Add(s.Refresh)
		if _, e = t.ExecContext(ctx, `INSERT INTO design_sessions(id,account_id,kind,refresh_hash,expires_at,created_at) VALUES(?,?,?,?,?,?)`, sid, id, kind, hash(refresh), expiry, time.Now().UTC()); e != nil {
			return e
		}
		out, e = s.issue(ctx, t, id, kind, sid, refresh, expiry)
		return e
	})
	return
}
func (s *Service) RefreshToken(ctx context.Context, kind, raw string) (out Tokens, err error) {
	if len(raw) != 64 {
		return out, platform.Fail(401, "刷新会话失效")
	}
	err = platform.Tx(ctx, s.DB, func(t *sql.Tx) error {
		if e := lock(ctx, t); e != nil {
			return e
		}
		var id int64
		var sid string
		var expiry time.Time
		e := t.QueryRowContext(ctx, `SELECT s.id,s.account_id,s.expires_at FROM design_sessions s JOIN design_accounts a ON a.id=s.account_id WHERE s.refresh_hash=? AND s.kind=? AND a.kind=? AND a.status='active' AND s.revoked_at IS NULL AND s.expires_at>UTC_TIMESTAMP(6)`, hash(raw), kind, kind).Scan(&sid, &id, &expiry)
		if errors.Is(e, sql.ErrNoRows) {
			return platform.Fail(401, "刷新会话失效")
		}
		if e != nil {
			return e
		}
		next := platform.Random()
		if _, e = t.ExecContext(ctx, `UPDATE design_sessions SET refresh_hash=? WHERE id=?`, hash(next), sid); e != nil {
			return e
		}
		out, e = s.issue(ctx, t, id, kind, sid, next, expiry)
		return e
	})
	return
}
func (s *Service) Authenticate(ctx context.Context, kind, raw string) (Principal, error) {
	var c claims
	tok, e := jwt.ParseWithClaims(raw, &c, func(t *jwt.Token) (any, error) { return s.Secret, nil }, jwt.WithValidMethods([]string{"HS256"}))
	if e != nil || !tok.Valid || c.Kind != kind || c.ExpiresAt == nil || !c.VerifyIssuer("design-api", true) || !c.VerifyAudience("design-"+kind, true) {
		return Principal{}, platform.Fail(401, "登录已失效")
	}
	id, e := platform.ParseID(c.Subject)
	if e != nil {
		return Principal{}, platform.Fail(401, "登录已失效")
	}
	var n int
	e = s.DB.QueryRowContext(ctx, `SELECT 1 FROM design_sessions s JOIN design_accounts a ON a.id=s.account_id WHERE s.id=? AND s.account_id=? AND s.kind=? AND a.kind=? AND a.status='active' AND s.revoked_at IS NULL AND s.expires_at>UTC_TIMESTAMP(6)`, c.Session, id, kind, kind).Scan(&n)
	if errors.Is(e, sql.ErrNoRows) {
		return Principal{}, platform.Fail(401, "登录已失效")
	}
	if e != nil {
		return Principal{}, e
	}
	return Principal{id, kind, c.Session}, nil
}
func (s *Service) Logout(ctx context.Context, kind, raw string, p *Principal) error {
	return platform.Tx(ctx, s.DB, func(t *sql.Tx) error {
		if e := lock(ctx, t); e != nil {
			return e
		}
		if p != nil {
			_, e := t.ExecContext(ctx, `UPDATE design_sessions SET revoked_at=UTC_TIMESTAMP(6) WHERE id=? AND kind=?`, p.Session, kind)
			return e
		}
		_, e := t.ExecContext(ctx, `UPDATE design_sessions SET revoked_at=UTC_TIMESTAMP(6) WHERE refresh_hash=? AND kind=?`, hash(raw), kind)
		return e
	})
}
func (s *Service) Me(ctx context.Context, p Principal) (Account, error) {
	var a Account
	var id int64
	e := s.DB.QueryRowContext(ctx, `SELECT id,kind,username,nickname,avatar,status FROM design_accounts WHERE id=? AND kind=?`, p.ID, p.Kind).Scan(&id, &a.Kind, &a.Username, &a.Nickname, &a.Avatar, &a.Status)
	a.ID = platform.ID(id)
	return a, platform.Normalize(e)
}
func (s *Service) Profile(ctx context.Context, p Principal, nickname, avatar, requestID string) error {
	if len([]rune(nickname)) > 64 || len(avatar) > 512 || (avatar != "" && !strings.HasPrefix(avatar, "/api/v1/assets/")) {
		return platform.Fail(400, "昵称过长或头像不是平台资源地址")
	}
	return platform.Tx(ctx, s.DB, func(t *sql.Tx) error {
		if e := lock(ctx, t); e != nil {
			return e
		}
		if e := platform.Affected(t.ExecContext(ctx, `UPDATE design_accounts SET nickname=?,avatar=?,updated_at=UTC_TIMESTAMP(6) WHERE id=? AND kind=? AND status='active'`, nickname, avatar, p.ID, p.Kind)); e != nil {
			return e
		}
		return platform.Audit(ctx, t, p.ID, "account.profile", platform.ID(p.ID), requestID)
	})
}
func (s *Service) Password(ctx context.Context, p Principal, old, next, requestID string) error {
	if _, e := credentials("valid_user", next); e != nil {
		return e
	}
	h, e := bcrypt.GenerateFromPassword([]byte(next), bcrypt.DefaultCost)
	if e != nil {
		return e
	}
	return platform.Tx(ctx, s.DB, func(t *sql.Tx) error {
		if e := lock(ctx, t); e != nil {
			return e
		}
		var stored string
		if e := t.QueryRowContext(ctx, `SELECT password_hash FROM design_accounts WHERE id=? AND kind=? AND status='active'`, p.ID, p.Kind).Scan(&stored); e != nil {
			return e
		}
		if bcrypt.CompareHashAndPassword([]byte(stored), []byte(old)) != nil {
			return platform.Fail(400, "原密码错误")
		}
		if _, e := t.ExecContext(ctx, `UPDATE design_accounts SET password_hash=?,updated_at=UTC_TIMESTAMP(6) WHERE id=?`, string(h), p.ID); e != nil {
			return e
		}
		if _, e := t.ExecContext(ctx, `UPDATE design_sessions SET revoked_at=UTC_TIMESTAMP(6) WHERE account_id=?`, p.ID); e != nil {
			return e
		}
		return platform.Audit(ctx, t, p.ID, "account.password", platform.ID(p.ID), requestID)
	})
}
func (s *Service) Codes(ctx context.Context, id int64) ([]string, []string, error) {
	roles := []string{}
	codes := []string{}
	rows, e := s.DB.QueryContext(ctx, `SELECT r.code FROM design_roles r JOIN design_admin_roles ar ON ar.role_id=r.id WHERE ar.account_id=? ORDER BY r.code`, id)
	if e != nil {
		return nil, nil, e
	}
	super := false
	for rows.Next() {
		var v string
		if e = rows.Scan(&v); e != nil {
			rows.Close()
			return nil, nil, e
		}
		roles = append(roles, v)
		if v == "super_admin" {
			super = true
		}
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return nil, nil, e
	}
	query := `SELECT DISTINCT permission_code FROM design_role_permissions rp JOIN design_admin_roles ar ON ar.role_id=rp.role_id WHERE ar.account_id=? ORDER BY permission_code`
	args := []any{id}
	if super {
		query = `SELECT code FROM design_permissions ORDER BY code`
		args = nil
	}
	rows, e = s.DB.QueryContext(ctx, query, args...)
	if e != nil {
		return nil, nil, e
	}
	defer rows.Close()
	for rows.Next() {
		var v string
		if e = rows.Scan(&v); e != nil {
			return nil, nil, e
		}
		codes = append(codes, v)
	}
	return codes, roles, rows.Err()
}
func (s *Service) Authorize(ctx context.Context, p Principal, code string) error {
	if p.Kind != "admin" {
		return platform.Fail(403, "无管理员权限")
	}
	codes, _, e := s.Codes(ctx, p.ID)
	if e != nil {
		return e
	}
	for _, v := range codes {
		if v == code {
			return nil
		}
	}
	return platform.Fail(403, "无操作权限")
}
