package asset

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"golang.org/x/image/font/sfnt"
	_ "golang.org/x/image/webp"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"looklook/app/design/internal/modules/canvas"
	"looklook/app/design/internal/platform"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const MaxUpload = int64(20 * 1024 * 1024)
const DefaultQuota = int64(200 * 1024 * 1024)

type Service struct {
	DB    *sql.DB
	Root  string
	Limit int64
	once  sync.Once
	slots chan struct{}
}

func (s *Service) QuotaLimit() int64 {
	if s.Limit > 0 {
		return s.Limit
	}
	return DefaultQuota
}

type Asset struct {
	ID         string    `json:"id"`
	OwnerID    string    `json:"ownerId"`
	Visibility string    `json:"visibility"`
	Kind       string    `json:"kind"`
	Name       string    `json:"name"`
	MIME       string    `json:"mime"`
	Size       int64     `json:"size"`
	Hash       string    `json:"sha256"`
	Width      int       `json:"width"`
	Height     int       `json:"height"`
	License    string    `json:"license"`
	Status     string    `json:"status"`
	CreatedAt  time.Time `json:"createdAt"`
	Key        string    `json:"-"`
	FontFamily string    `json:"fontFamily,omitempty"`
	URL        string    `json:"contentUrl"`
}

const columns = `id,owner_id,visibility,kind,name,mime,size_bytes,sha256,width,height,license_note,status,created_at,storage_key`

func scan(row interface{ Scan(...any) error }) (Asset, error) {
	var a Asset
	var id, owner int64
	e := row.Scan(&id, &owner, &a.Visibility, &a.Kind, &a.Name, &a.MIME, &a.Size, &a.Hash, &a.Width, &a.Height, &a.License, &a.Status, &a.CreatedAt, &a.Key)
	a.ID = platform.ID(id)
	a.OwnerID = platform.ID(owner)
	a.URL = canvas.AssetURL(id)
	if a.Kind == "font" {
		a.FontFamily = "asset-font-" + a.ID
	}
	return a, platform.Normalize(e)
}
func (s *Service) filePath(key string) (string, error) {
	if len(key) != 64 {
		return "", fmt.Errorf("invalid storage key")
	}
	if _, e := hex.DecodeString(key); e != nil {
		return "", e
	}
	return filepath.Join(s.Root, "files", key), nil
}
func (s *Service) Upload(ctx context.Context, owner int64, admin bool, kind, name, license string, input io.Reader, rid string) (Asset, error) {
	return s.UploadWithCommit(ctx, owner, admin, kind, name, license, input, rid, nil)
}

// UploadWithCommit atomically links an uploaded asset to a leased job before commit.
func (s *Service) UploadWithCommit(ctx context.Context, owner int64, admin bool, kind, name, license string, input io.Reader, rid string, hook func(*sql.Tx, Asset) error) (out Asset, err error) {
	if kind != "image" && kind != "font" {
		return out, platform.Fail(400, "kind 仅支持 image/font")
	}
	if kind == "font" && !admin {
		return out, platform.Fail(403, "仅管理员可以上传字体")
	}
	name = filepath.Base(strings.ReplaceAll(name, "\\", "/"))
	if name == "." || name == "" || len([]rune(name)) > 128 || len([]rune(license)) > 1000 {
		return out, platform.Fail(400, "文件名或授权说明无效")
	}
	if admin && strings.TrimSpace(license) == "" {
		return out, platform.Fail(400, "公共图片和字体必须填写授权说明")
	}
	s.once.Do(func() { s.slots = make(chan struct{}, 2) })
	select {
	case s.slots <- struct{}{}:
		defer func() { <-s.slots }()
	case <-ctx.Done():
		return out, ctx.Err()
	}
	if e := os.MkdirAll(filepath.Join(s.Root, "files"), 0700); e != nil {
		return out, e
	}
	if e := os.MkdirAll(filepath.Join(s.Root, "tmp"), 0700); e != nil {
		return out, e
	}
	tmp, e := os.CreateTemp(filepath.Join(s.Root, "tmp"), "upload-*")
	if e != nil {
		return out, e
	}
	defer func() { tmp.Close(); os.Remove(tmp.Name()) }()
	digest := sha256.New()
	size, e := io.Copy(io.MultiWriter(tmp, digest), io.LimitReader(input, MaxUpload+1))
	if e != nil {
		return out, e
	}
	if size == 0 {
		return out, platform.Fail(400, "空文件")
	}
	if size > MaxUpload {
		return out, platform.Fail(413, "单文件不能超过 20 MiB")
	}
	if e = tmp.Sync(); e != nil {
		return out, e
	}
	if _, e = tmp.Seek(0, 0); e != nil {
		return out, e
	}
	mime := ""
	width, height := 0, 0
	if kind == "image" {
		cfg, format, e := image.DecodeConfig(tmp)
		if e != nil {
			return out, platform.Fail(400, "无法识别图片内容")
		}
		switch format {
		case "png":
			mime = "image/png"
		case "jpeg":
			mime = "image/jpeg"
		case "webp":
			mime = "image/webp"
		default:
			return out, platform.Fail(400, "只接受 PNG/JPEG/WebP")
		}
		if cfg.Width < 1 || cfg.Height < 1 || cfg.Width > 8192 || cfg.Height > 8192 || int64(cfg.Width)*int64(cfg.Height) > 40000000 {
			return out, platform.Fail(400, "图片尺寸超限")
		}
		width, height = cfg.Width, cfg.Height
		if _, e = tmp.Seek(0, 0); e != nil {
			return out, e
		}
		if _, _, e = image.Decode(tmp); e != nil {
			return out, platform.Fail(400, "图片数据损坏")
		}
	} else {
		b, e := io.ReadAll(tmp)
		if e != nil {
			return out, e
		}
		font, e := sfnt.Parse(b)
		if e != nil || font.NumGlyphs() == 0 {
			return out, platform.Fail(400, "仅支持有效的单字体 TTF/OTF 文件")
		}
		mime = "font/ttf"
		if len(b) >= 4 && string(b[:4]) == "OTTO" {
			mime = "font/otf"
		}
	}
	if e = tmp.Close(); e != nil {
		return out, e
	}
	key := platform.Random()
	dest, e := s.filePath(key)
	if e != nil {
		return out, e
	}
	if e = os.Rename(tmp.Name(), dest); e != nil {
		return out, e
	}
	// A crash between rename and commit leaves an unreferenced file, reclaimed by maintenance.
	committed := false
	defer func() {
		if !committed {
			os.Remove(dest)
		}
	}()
	visibility := "private"
	if admin {
		visibility = "public"
	}
	err = platform.Tx(ctx, s.DB, func(t *sql.Tx) error {
		if e := platform.LockContent(ctx, t); e != nil {
			return e
		}
		if e := platform.Quota(ctx, t, owner, size, s.QuotaLimit()); e != nil {
			return e
		}
		res, e := t.ExecContext(ctx, `INSERT INTO design_assets(owner_id,visibility,kind,name,storage_key,mime,size_bytes,sha256,width,height,license_note,created_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,UTC_TIMESTAMP(6))`, owner, visibility, kind, name, key, mime, size, hex.EncodeToString(digest.Sum(nil)), width, height, license)
		if e != nil {
			return e
		}
		id, e := res.LastInsertId()
		if e != nil {
			return e
		}
		out, e = scan(t.QueryRowContext(ctx, `SELECT `+columns+` FROM design_assets WHERE id=?`, id))
		if e != nil {
			return e
		}
		if hook != nil {
			if e = hook(t, out); e != nil {
				return e
			}
		}
		return platform.Audit(ctx, t, owner, "asset.upload", platform.ID(id), rid)
	})
	if err == nil {
		committed = true
	}
	return
}
func (s *Service) Get(ctx context.Context, id, owner int64, admin bool) (Asset, error) {
	a, e := scan(s.DB.QueryRowContext(ctx, `SELECT `+columns+` FROM design_assets WHERE id=? AND status<>'deleted'`, id))
	if e != nil {
		return a, e
	}
	if a.Visibility == "public" && (a.Status == "active" || admin) {
		return a, nil
	}
	if a.Visibility == "private" && !admin && a.OwnerID == platform.ID(owner) && a.Status == "active" {
		return a, nil
	}
	return Asset{}, platform.Fail(404, "资源不存在")
}
func (s *Service) Open(ctx context.Context, id, owner int64, admin bool) (Asset, *os.File, error) {
	a, e := s.Get(ctx, id, owner, admin)
	if e != nil {
		return a, nil, e
	}
	path, e := s.filePath(a.Key)
	if e != nil {
		return a, nil, e
	}
	file, e := os.Open(path)
	if os.IsNotExist(e) {
		return a, nil, platform.Fail(404, "资源文件不存在")
	}
	return a, file, e
}
func (s *Service) List(ctx context.Context, owner int64, admin bool, p platform.Page, scope, kind string) (platform.List, error) {
	if kind != "" && kind != "image" && kind != "font" {
		return platform.List{}, platform.Fail(400, "素材类型无效")
	}
	where := ` WHERE status<>'deleted'`
	args := []any{}
	if admin {
		where += ` AND visibility='public'`
		if p.Status != "" {
			if p.Status != "active" && p.Status != "disabled" {
				return platform.List{}, platform.Fail(400, "素材状态无效")
			}
			where += ` AND status=?`
			args = append(args, p.Status)
		}
	} else if scope == "public" {
		where += ` AND visibility='public' AND status='active'`
	} else if scope == "" || scope == "mine" {
		where += ` AND owner_id=? AND visibility='private' AND status='active'`
		args = append(args, owner)
	} else {
		return platform.List{}, platform.Fail(400, "scope 仅支持 mine/public")
	}
	if kind != "" {
		where += ` AND kind=?`
		args = append(args, kind)
	}
	if p.Keyword != "" {
		where += ` AND name LIKE ?`
		args = append(args, "%"+p.Keyword+"%")
	}
	out := platform.List{Items: []Asset{}, Page: p.Number, PageSize: p.Size}
	if e := s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM design_assets`+where, args...).Scan(&out.Total); e != nil {
		return out, e
	}
	rows, e := s.DB.QueryContext(ctx, `SELECT `+columns+` FROM design_assets`+where+` ORDER BY id DESC LIMIT ? OFFSET ?`, append(args, p.Size, p.Offset())...)
	if e != nil {
		return out, e
	}
	defer rows.Close()
	items := []Asset{}
	for rows.Next() {
		a, e := scan(rows)
		if e != nil {
			return out, e
		}
		items = append(items, a)
	}
	out.Items = items
	return out, rows.Err()
}
func (s *Service) Usage(ctx context.Context, owner int64) (map[string]int64, error) {
	var used int64
	e := s.DB.QueryRowContext(ctx, `SELECT used_bytes FROM design_storage_usage WHERE account_id=?`, owner).Scan(&used)
	if e == sql.ErrNoRows {
		e = nil
	}
	return map[string]int64{"usedBytes": used, "limitBytes": s.QuotaLimit()}, e
}
func (s *Service) References(ctx context.Context, id, owner int64, admin bool) ([]map[string]string, error) {
	if _, e := s.Get(ctx, id, owner, admin); e != nil {
		return nil, e
	}
	rows, e := s.DB.QueryContext(ctx, `SELECT object_kind,object_id FROM design_asset_references WHERE asset_id=? ORDER BY object_kind,object_id`, id)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []map[string]string{}
	for rows.Next() {
		var kind string
		var n int64
		if e = rows.Scan(&kind, &n); e != nil {
			return nil, e
		}
		out = append(out, map[string]string{"kind": kind, "id": platform.ID(n)})
	}
	return out, rows.Err()
}
func (s *Service) Change(ctx context.Context, id, actor int64, admin bool, status, rid string) error {
	if status != "active" && status != "disabled" && status != "deleted" {
		return platform.Fail(400, "素材状态无效")
	}
	if !admin && status != "deleted" {
		return platform.Fail(403, "用户仅可删除自己的素材")
	}
	return platform.Tx(ctx, s.DB, func(t *sql.Tx) error {
		if e := platform.LockContent(ctx, t); e != nil {
			return e
		}
		a, e := scan(t.QueryRowContext(ctx, `SELECT `+columns+` FROM design_assets WHERE id=? AND status<>'deleted' FOR UPDATE`, id))
		if e != nil {
			return e
		}
		if (admin && a.Visibility != "public") || (!admin && (a.OwnerID != platform.ID(actor) || a.Visibility != "private")) {
			return platform.Fail(404, "资源不存在")
		}
		if status != "active" {
			var n int
			if e = t.QueryRowContext(ctx, `SELECT COUNT(*) FROM design_asset_references WHERE asset_id=?`, id).Scan(&n); e != nil {
				return e
			}
			if n > 0 {
				return platform.Fail(409, "素材仍被作品或模板引用，请先查看引用并解除")
			}
		}
		if status == "deleted" {
			owner, _ := platform.ParseID(a.OwnerID)
			if e = platform.Quota(ctx, t, owner, -a.Size, s.QuotaLimit()); e != nil {
				return e
			}
			_, e = t.ExecContext(ctx, `UPDATE design_assets SET status='deleted',deleted_at=UTC_TIMESTAMP(6) WHERE id=?`, id)
		} else {
			_, e = t.ExecContext(ctx, `UPDATE design_assets SET status=? WHERE id=?`, status, id)
		}
		if e != nil {
			return e
		}
		return platform.Audit(ctx, t, actor, "asset."+status, platform.ID(id), rid)
	})
}

// Call while holding LockContent, inside the same transaction as document changes.
func SyncReferences(ctx context.Context, t *sql.Tx, kind string, id, owner int64, publicOnly bool, refs []canvas.Reference) error {
	seen := map[int64]bool{}
	for _, ref := range refs {
		if seen[ref.ID] {
			continue
		}
		seen[ref.ID] = true
		var aOwner int64
		var visibility, aKind, status string
		if e := t.QueryRowContext(ctx, `SELECT owner_id,visibility,kind,status FROM design_assets WHERE id=? LOCK IN SHARE MODE`, ref.ID).Scan(&aOwner, &visibility, &aKind, &status); e == sql.ErrNoRows {
			return platform.Fail(400, "文档引用不存在的资源")
		} else if e != nil {
			return e
		}
		if status != "active" || aKind != ref.Kind {
			return platform.Fail(400, "资源未启用或类型不匹配")
		}
		if visibility != "public" && (publicOnly || aOwner != owner) {
			return platform.Fail(404, "资源不存在或不可访问")
		}
	}
	if _, e := t.ExecContext(ctx, `DELETE FROM design_asset_references WHERE object_kind=? AND object_id=?`, kind, id); e != nil {
		return e
	}
	for ref := range seen {
		if _, e := t.ExecContext(ctx, `INSERT INTO design_asset_references(asset_id,object_kind,object_id) VALUES(?,?,?)`, ref, kind, id); e != nil {
			return e
		}
	}
	return nil
}
