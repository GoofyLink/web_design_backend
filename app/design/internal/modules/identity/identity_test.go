package identity

import (
	"context"
	"github.com/golang-jwt/jwt/v4"
	"looklook/app/design/internal/platform"
	"strings"
	"testing"
	"time"
)

func TestRejectInvalidTokenBeforeDatabase(t *testing.T) {
	secret := []byte(strings.Repeat("s", 32))
	service := &Service{Secret: secret}
	cases := []struct {
		name     string
		audience string
		kind     string
		expiry   time.Time
		key      []byte
		method   jwt.SigningMethod
	}{
		{"expired", "design-user", "user", time.Now().Add(-time.Minute), secret, jwt.SigningMethodHS256},
		{"wrong audience", "design-admin", "user", time.Now().Add(time.Minute), secret, jwt.SigningMethodHS256},
		{"wrong kind", "design-user", "admin", time.Now().Add(time.Minute), secret, jwt.SigningMethodHS256},
		{"wrong secret", "design-user", "user", time.Now().Add(time.Minute), []byte(strings.Repeat("x", 32)), jwt.SigningMethodHS256},
		{"wrong algorithm", "design-user", "user", time.Now().Add(time.Minute), secret, jwt.SigningMethodHS384},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			raw, e := jwt.NewWithClaims(tt.method, claims{Kind: tt.kind, Session: "session", RegisteredClaims: jwt.RegisteredClaims{Issuer: "design-api", Subject: "1", Audience: jwt.ClaimStrings{tt.audience}, ExpiresAt: jwt.NewNumericDate(tt.expiry)}}).SignedString(tt.key)
			if e != nil {
				t.Fatal(e)
			}
			_, e = service.Authenticate(context.Background(), "user", raw)
			p, ok := e.(*platform.Error)
			if !ok || p.Status != 401 {
				t.Fatalf("want unauthorized, got %v", e)
			}
		})
	}
}
