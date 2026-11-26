package task

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"io"
	"looklook/app/design/internal/modules/asset"
	"looklook/app/design/internal/platform"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Service struct {
	DB           *sql.DB
	Assets       *asset.Service
	Secret       []byte
	AllowPrivate bool
	Client       *http.Client
}
type ProviderInput struct {
	Name     string `json:"name"`
	Kind     string `json:"kind"`
	Endpoint string `json:"endpoint"`
	Model    string `json:"model,omitempty"`
	APIKey   string `json:"apiKey,omitempty"`
	ClearKey bool   `json:"clearKey,omitempty"`
	Status   string `json:"status"`
	Version  int64  `json:"version"`
}
type Provider struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Kind     string `json:"kind"`
	Endpoint string `json:"endpoint"`
	Model    string `json:"model"`
	HasKey   bool   `json:"hasKey"`
	Status   string `json:"status"`
	Version  int64  `json:"version"`
	cipher   string
}

func (s *Service) crypt() (cipher.AEAD, error) {
	key := sha256.Sum256(append([]byte("design-provider-key:v1:"), s.Secret...))
	block, e := aes.NewCipher(key[:])
	if e != nil {
		return nil, e
	}
	return cipher.NewGCM(block)
}
func (s *Service) encrypt(raw string) (string, error) {
	if raw == "" {
		return "", nil
	}
	g, e := s.crypt()
	if e != nil {
		return "", e
	}
	nonce := make([]byte, g.NonceSize())
	if _, e = io.ReadFull(rand.Reader, nonce); e != nil {
		return "", e
	}
	return base64.StdEncoding.EncodeToString(g.Seal(nonce, nonce, []byte(raw), nil)), nil
}
func (s *Service) decrypt(raw string) (string, error) {
	if raw == "" {
		return "", nil
	}
	g, e := s.crypt()
	if e != nil {
		return "", e
	}
	b, e := base64.StdEncoding.DecodeString(raw)
	if e != nil || len(b) < g.NonceSize() {
		return "", platform.Fail(503, "服务商密钥无法解密")
	}
	v, e := g.Open(nil, b[:g.NonceSize()], b[g.NonceSize():], nil)
	return string(v), e
}
func (s *Service) endpoint(raw string) error {
	u, e := url.Parse(raw)
	if e != nil || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || len(raw) > 1000 {
		return platform.Fail(400, "服务地址须为不带凭据或查询参数的完整 URL")
	}
	if u.Scheme != "https" && !(s.AllowPrivate && u.Scheme == "http") {
		return platform.Fail(400, "服务地址必须使用 HTTPS")
	}
	return nil
}
func scanProvider(row interface{ Scan(...any) error }) (Provider, error) {
	var p Provider
	var id int64
	e := row.Scan(&id, &p.Name, &p.Kind, &p.Endpoint, &p.Model, &p.cipher, &p.Status, &p.Version)
	p.ID = platform.ID(id)
	p.HasKey = p.cipher != ""
	return p, platform.Normalize(e)
}

const providerCols = `id,name,kind,endpoint,model,key_cipher,status,version`

func (s *Service) Providers(ctx context.Context) ([]Provider, error) {
	rows, e := s.DB.QueryContext(ctx, `SELECT `+providerCols+` FROM design_providers ORDER BY id DESC LIMIT 100`)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []Provider{}
	for rows.Next() {
		p, e := scanProvider(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
func (s *Service) ProviderSave(ctx context.Context, actor, id int64, v ProviderInput, rid string) (out any, err error) {
	if v.ClearKey && v.APIKey != "" {
		return nil, platform.Fail(400, "不能同时写入和清除密钥")
	}
	if strings.TrimSpace(v.Name) == "" || len([]rune(v.Name)) > 100 || (v.Kind != "image" && v.Kind != "renderer") || (v.Status != "active" && v.Status != "disabled") || len(v.Model) > 100 || len(v.APIKey) > 4096 || strings.ContainsAny(v.APIKey, "\r\n") {
		return nil, platform.Fail(400, "服务商配置字段无效")
	}
	if e := s.endpoint(v.Endpoint); e != nil {
		return nil, e
	}
	if v.Kind == "image" && v.Model == "" {
		return nil, platform.Fail(400, "图片模型不能为空")
	}
	encrypted, e := s.encrypt(v.APIKey)
	if e != nil {
		return nil, e
	}
	err = platform.Tx(ctx, s.DB, func(t *sql.Tx) error {
		if e := platform.LockContent(ctx, t); e != nil {
			return e
		}
		if id == 0 {
			var n int
			if e := t.QueryRowContext(ctx, `SELECT COUNT(*) FROM design_providers`).Scan(&n); e != nil {
				return e
			}
			if n >= 100 {
				return platform.Fail(409, "服务商数量已达上限")
			}
			r, e := t.ExecContext(ctx, `INSERT INTO design_providers(name,kind,endpoint,model,key_cipher,status) VALUES(?,?,?,?,?,?)`, v.Name, v.Kind, v.Endpoint, v.Model, encrypted, v.Status)
			if e != nil {
				return e
			}
			id, e = r.LastInsertId()
			if e != nil {
				return e
			}
			v.Version = 1
		} else {
			old, e := scanProvider(t.QueryRowContext(ctx, `SELECT `+providerCols+` FROM design_providers WHERE id=?`, id))
			if e != nil {
				return e
			}
			if old.Version != v.Version {
				return platform.Fail(409, "服务商配置版本冲突")
			}
			var pending int
			if e = t.QueryRowContext(ctx, `SELECT COUNT(*) FROM design_jobs WHERE provider_id=? AND status IN ('queued','running')`, id).Scan(&pending); e != nil {
				return e
			}
			if pending > 0 && (old.Kind != v.Kind || old.Endpoint != v.Endpoint || old.Model != v.Model) {
				return platform.Fail(409, "有未完成任务，不能更换服务类型、地址或模型")
			}
			if v.APIKey == "" && !v.ClearKey {
				encrypted = old.cipher
			}
			if _, e = t.ExecContext(ctx, `UPDATE design_providers SET name=?,kind=?,endpoint=?,model=?,key_cipher=?,status=?,version=version+1 WHERE id=?`, v.Name, v.Kind, v.Endpoint, v.Model, encrypted, v.Status, id); e != nil {
				return e
			}
			v.Version++
		}
		out = map[string]any{"id": platform.ID(id), "version": v.Version}
		return platform.Audit(ctx, t, actor, "provider.save", platform.ID(id), rid)
	})
	return
}
func (s *Service) httpClient() *http.Client {
	if s.Client != nil {
		return s.Client
	}
	dial := &net.Dialer{Timeout: 10 * time.Second}
	tr := &http.Transport{TLSHandshakeTimeout: 10 * time.Second, ResponseHeaderTimeout: 70 * time.Second}
	tr.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, e := net.SplitHostPort(address)
		if e != nil {
			return nil, e
		}
		ips, e := net.DefaultResolver.LookupIPAddr(ctx, host)
		if e != nil {
			return nil, e
		}
		for _, ip := range ips {
			if !s.AllowPrivate && (ip.IP.IsPrivate() || ip.IP.IsLoopback() || ip.IP.IsLinkLocalUnicast() || ip.IP.IsLinkLocalMulticast() || ip.IP.IsUnspecified() || ip.IP.IsMulticast()) {
				return nil, platform.Fail(503, "服务地址解析到受限网络")
			}
		}
		if len(ips) == 0 {
			return nil, platform.Fail(503, "服务地址无解析记录")
		}
		return dial.DialContext(ctx, network, net.JoinHostPort(ips[0].IP.String(), port))
	}
	return &http.Client{Transport: tr, Timeout: 75 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
}
