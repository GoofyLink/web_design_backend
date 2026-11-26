package transport

import (
	"github.com/golang-jwt/jwt/v4"
	"looklook/app/design/internal/modules/canvas"
	"looklook/app/design/internal/modules/identity"
	"looklook/app/design/internal/platform"
	"net/http"
	"strings"
	"time"
)

type fileClaims struct {
	Asset   int64  `json:"asset"`
	Account int64  `json:"account"`
	Kind    string `json:"kind"`
	Session string `json:"sid"`
	jwt.RegisteredClaims
}

func (a *API) assetURL(id int64, p identity.Principal) (any, error) {
	expires := time.Now().Add(time.Minute)
	c := fileClaims{Asset: id, Account: p.ID, Kind: p.Kind, Session: p.Session, RegisteredClaims: jwt.RegisteredClaims{Issuer: "design-file", Audience: jwt.ClaimStrings{"design-file"}, ExpiresAt: jwt.NewNumericDate(expires)}}
	token, e := jwt.NewWithClaims(jwt.SigningMethodHS256, c).SignedString(a.Identity.Secret)
	return map[string]any{"url": canvas.AssetURL(id) + "?ticket=" + token, "expiresAt": expires}, e
}
func (a *API) filePrincipal(r *http.Request, id int64) (identity.Principal, error) {
	p := identity.Principal{}
	var e error
	if raw := r.URL.Query().Get("ticket"); raw != "" {
		c := &fileClaims{}
		token, err := jwt.ParseWithClaims(raw, c, func(t *jwt.Token) (any, error) { return a.Identity.Secret, nil }, jwt.WithValidMethods([]string{"HS256"}))
		if err != nil || !token.Valid || c.ExpiresAt == nil || !c.VerifyAudience("design-file", true) || c.Issuer != "design-file" || c.Asset != id || (c.Kind != "user" && c.Kind != "admin") {
			return p, platform.Fail(401, "文件访问凭据无效或过期")
		}
		var n int
		e = a.Identity.DB.QueryRowContext(r.Context(), `SELECT 1 FROM design_sessions s JOIN design_accounts a ON a.id=s.account_id WHERE s.id=? AND s.account_id=? AND s.kind=? AND a.kind=? AND a.status='active' AND s.revoked_at IS NULL AND s.expires_at>UTC_TIMESTAMP(6)`, c.Session, c.Account, c.Kind, c.Kind).Scan(&n)
		if e != nil {
			return p, platform.Fail(401, "文件访问会话失效")
		}
		p = identity.Principal{ID: c.Account, Kind: c.Kind, Session: c.Session}
	} else if header := r.Header.Get("Authorization"); header != "" {
		if !strings.HasPrefix(header, "Bearer ") {
			return p, platform.Fail(401, "无效 Bearer 凭据")
		}
		raw := strings.TrimPrefix(header, "Bearer ")
		p, e = a.Identity.Authenticate(r.Context(), "user", raw)
		if e != nil {
			p, e = a.Identity.Authenticate(r.Context(), "admin", raw)
		}
		if e != nil {
			return p, e
		}
	}
	if p.Kind == "admin" {
		e = a.Identity.Authorize(r.Context(), p, "asset:read")
	}
	return p, e
}
