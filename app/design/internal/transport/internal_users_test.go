package transport

import (
	"context"
	"golang.org/x/crypto/bcrypt"
	"strings"
	"testing"
)

func TestInternalUserProvisioning(t *testing.T) {
	a, db := contentTestAPI(t)
	a.Config.AllowPublicRegistration = false
	c := testClient{t, a.Handler()}
	input := CreateUserRequest{Username: "internal_user", Password: "Internal-user-123", Nickname: "设计同事"}
	c.expect(403, "POST", "/api/v1/auth/register", "", LoginRequest{input.Username, input.Password})
	c.expect(401, "POST", "/api/admin/v1/users", "", input)
	admin, _ := c.login("admin", "root_admin", "Strong-test-123")
	id := objectID(t, c.expect(200, "POST", "/api/admin/v1/users", admin, input))
	c.expect(409, "POST", "/api/admin/v1/users", admin, input)
	c.expect(400, "POST", "/api/admin/v1/users", admin, CreateUserRequest{"bad", "short", ""})
	var hash, kind, nickname string
	if err := db.QueryRow(`SELECT password_hash,kind,nickname FROM design_accounts WHERE id=?`, id).Scan(&hash, &kind, &nickname); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(hash, "$2") || kind != "user" || nickname != input.Nickname {
		t.Fatal("invalid account storage")
	}
	var audits int
	if err := db.QueryRow(`SELECT COUNT(*) FROM design_audit_logs WHERE action='user.create' AND target=?`, id).Scan(&audits); err != nil {
		t.Fatal(err)
	}
	if audits != 1 {
		t.Fatal("creation was not audited once")
	}
	user, _ := c.login("user", input.Username, input.Password)
	c.expect(200, "GET", "/api/v1/designs", user, nil)
	c.expect(401, "POST", "/api/admin/v1/users", user, input)
	role, err := a.Identity.SaveRole(context.Background(), 1, 0, "reader_test", "只读管理员", []string{"user:read"}, "test")
	if err != nil {
		t.Fatal(err)
	}
	var roleID int64
	if err = db.QueryRow(`SELECT id FROM design_roles WHERE id=?`, role).Scan(&roleID); err != nil {
		t.Fatal(err)
	}
	if _, err = a.Identity.CreateAdmin(context.Background(), 1, "read_admin", "Read-admin-123", []int64{roleID}, false, "test"); err != nil {
		t.Fatal(err)
	}
	reader, _ := c.login("admin", "read_admin", "Read-admin-123")
	c.expect(403, "POST", "/api/admin/v1/users", reader, input)
	edit := UpdateUserRequest{"renamed_user", "新姓名"}
	reset := ResetUserPasswordRequest{"New-password-123"}
	base := "/api/admin/v1/users/" + id
	c.expect(401, "PATCH", base, "", edit)
	c.expect(401, "PUT", base+"/password", user, reset)
	c.expect(403, "PATCH", base, reader, edit)
	c.expect(403, "PUT", base+"/password", reader, reset)
	c.expect(400, "PATCH", base, admin, UpdateUserRequest{"bad!", ""})
	c.expect(400, "PATCH", base, admin, UpdateUserRequest{input.Username, strings.Repeat("字", 65)})
	c.expect(400, "PUT", base+"/password", admin, ResetUserPasswordRequest{"short"})
	c.expect(400, "PUT", base+"/password", admin, ResetUserPasswordRequest{strings.Repeat("字", 25)})
	c.expect(404, "PATCH", "/api/admin/v1/users/1", admin, edit)
	c.expect(404, "PUT", "/api/admin/v1/users/1/password", admin, reset)
	c.expect(404, "PUT", "/api/admin/v1/users/999999/password", admin, reset)
	c.expect(200, "POST", "/api/admin/v1/users", admin, CreateUserRequest{"taken_user", "Taken-pass-123", ""})
	c.expect(409, "PATCH", base, admin, UpdateUserRequest{"taken_user", ""})
	// A name-only edit keeps sessions valid, including an unchanged save.
	c.expect(200, "PATCH", base, admin, UpdateUserRequest{input.Username, "同事"})
	c.expect(200, "PATCH", base, admin, UpdateUserRequest{input.Username, "同事"})
	c.expect(200, "GET", "/api/v1/me", user, nil)
	c.expect(200, "PATCH", base, admin, edit)
	c.expect(401, "GET", "/api/v1/me", user, nil)
	c.expect(401, "POST", "/api/v1/auth/login", "", LoginRequest{input.Username, input.Password})
	user, _ = c.login("user", edit.Username, input.Password)
	session, err := a.Identity.Login(context.Background(), "user", edit.Username, input.Password)
	if err != nil {
		t.Fatal(err)
	}
	c.expect(200, "PUT", base+"/password", admin, reset)
	c.expect(401, "GET", "/api/v1/me", user, nil)
	c.expect(401, "GET", "/api/v1/me", session.AccessToken, nil)
	if _, err := a.Identity.RefreshToken(context.Background(), "user", session.RefreshToken); err == nil {
		t.Fatal("reset did not revoke refresh token")
	}
	c.expect(401, "POST", "/api/v1/auth/login", "", LoginRequest{edit.Username, input.Password})
	user, _ = c.login("user", edit.Username, reset.NewPassword)
	if err := db.QueryRow(`SELECT COUNT(*) FROM design_audit_logs WHERE action='user.password' AND target=?`, id).Scan(&audits); err != nil || audits != 1 {
		t.Fatal("reset audit missing", err, audits)
	}
	if err := db.QueryRow(`SELECT password_hash FROM design_accounts WHERE id=?`, id).Scan(&hash); err != nil {
		t.Fatal(err)
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(reset.NewPassword)) != nil {
		t.Fatal("password was not hashed")
	}

	c.expect(200, "PATCH", "/api/admin/v1/users/"+id+"/status", admin, StatusRequest{"disabled"})
	c.expect(401, "GET", "/api/v1/designs", user, nil)
	c.expect(401, "POST", "/api/v1/auth/login", "", LoginRequest{input.Username, input.Password})
}
