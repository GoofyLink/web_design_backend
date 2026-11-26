package transport

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/rest/pathvar"
	"github.com/zeromicro/go-zero/rest/router"
	"looklook/app/design/internal/config"
	"looklook/app/design/internal/modules/asset"
	"looklook/app/design/internal/modules/catalog"
	"looklook/app/design/internal/modules/identity"
	"looklook/app/design/internal/modules/operation"
	"looklook/app/design/internal/modules/task"
	"looklook/app/design/internal/modules/workspace"
	"looklook/app/design/internal/platform"
	"looklook/app/design/migrations"
)

type API struct {
	Tasks      *task.Service
	Operations *operation.Service
	Assets     *asset.Service
	Workspace  *workspace.Service
	Config     config.Config
	Identity   *identity.Service
	Catalog    *catalog.Service
	mu         sync.Mutex
	attempts   map[string]attempt
}
type attempt struct {
	Count int
	Until time.Time
}
type endpoint func(http.ResponseWriter, *http.Request, identity.Principal, string) (any, error)
type Route struct {
	Method     string `json:"method"`
	Path       string `json:"path"`
	Kind       string `json:"audience,omitempty"`
	Permission string `json:"permission,omitempty"`
	Summary    string `json:"summary"`
	Body       any    `json:"-"`
	handler    endpoint
}
type Envelope struct {
	Code      int    `json:"code"`
	Message   string `json:"message"`
	Data      any    `json:"data"`
	RequestID string `json:"requestId"`
}

func New(c config.Config, i *identity.Service, cat *catalog.Service) *API {
	if i == nil {
		i = &identity.Service{}
	}
	a := &API{Operations: &operation.Service{DB: i.DB}, Config: c, Identity: i, Catalog: cat, Assets: &asset.Service{DB: i.DB, Root: c.StorageDir, Limit: c.StorageQuotaBytes}, Workspace: &workspace.Service{DB: i.DB, Limit: c.StorageQuotaBytes, TrashDays: c.TrashDays}, attempts: map[string]attempt{}}
	a.Tasks = &task.Service{DB: i.DB, Assets: a.Assets, Secret: i.Secret, AllowPrivate: c.AllowPrivateProviders}
	return a
}
func write(w http.ResponseWriter, status int, data any, err error, rid string) {
	out := Envelope{Code: 0, Message: "ok", Data: data, RequestID: rid}
	if err != nil {
		err = platform.Normalize(err)
		var e *platform.Error
		if errors.As(err, &e) {
			status = e.Status
			out.Code = e.Code
			out.Message = e.Message
		} else {
			status = 500
			out.Code = 50000
			out.Message = "服务器内部错误"
			log.Printf("requestId=%s error=%v", rid, err)
		}
		out.Data = nil
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(out)
}
func decode[T any](w http.ResponseWriter, r *http.Request) (v T, err error) {
	if !strings.HasPrefix(strings.ToLower(r.Header.Get("Content-Type")), "application/json") {
		return v, platform.Fail(400, "Content-Type 必须为 application/json")
	}
	d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 6*1024*1024))
	d.DisallowUnknownFields()
	var decoded *T
	if e := d.Decode(&decoded); e != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(e, &tooLarge) {
			return v, platform.Fail(413, "请求体超限")
		}
		return v, platform.Fail(400, "JSON 格式或字段错误")
	}
	if decoded == nil {
		return v, platform.Fail(400, "请求体必须是 JSON 对象")
	}
	v = *decoded
	var trailing any
	if d.Decode(&trailing) != io.EOF {
		return v, platform.Fail(400, "只允许一个 JSON 对象")
	}
	return v, nil
}
func page(r *http.Request) (platform.Page, error) {
	p := platform.Page{Number: 1, Size: 20, Keyword: strings.TrimSpace(r.URL.Query().Get("keyword")), Status: r.URL.Query().Get("status")}
	for name, dest := range map[string]*int{"page": &p.Number, "pageSize": &p.Size} {
		if raw := r.URL.Query().Get(name); raw != "" {
			n, e := strconv.Atoi(raw)
			if e != nil {
				return p, platform.Fail(400, "分页参数无效")
			}
			*dest = n
		}
	}
	if p.Number < 1 || p.Number > 100000 || p.Size < 1 || p.Size > 100 || len(p.Keyword) > 200 {
		return p, platform.Fail(400, "分页或关键词超限")
	}
	return p, nil
}
func id(r *http.Request) (int64, error) { return platform.ParseID(pathvar.Vars(r)["id"]) }
func (a *API) allowed(origin string) bool {
	for _, o := range a.Config.AllowedOrigins {
		if o == origin {
			return true
		}
	}
	return false
}
func (a *API) allowAttempt(r *http.Request) bool {
	host, _, e := net.SplitHostPort(r.RemoteAddr)
	if e != nil {
		host = r.RemoteAddr
	}
	key := host + ":" + r.URL.Path
	now := time.Now()
	a.mu.Lock()
	defer a.mu.Unlock()
	for k, v := range a.attempts {
		if now.After(v.Until) {
			delete(a.attempts, k)
		}
	}
	v, exists := a.attempts[key]
	if !exists {
		if len(a.attempts) >= 10000 {
			return false
		}
		v = attempt{Until: now.Add(time.Minute)}
	}
	v.Count++
	a.attempts[key] = v
	return v.Count <= 20
}
func (a *API) wrap(route Route) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rid := platform.Random()[:24]
		w.Header().Set("X-Request-ID", rid)
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		defer func() {
			if v := recover(); v != nil {
				log.Printf("requestId=%s panic=%v", rid, v)
				write(w, 500, nil, platform.Fail(500, "服务器内部错误"), rid)
			}
		}()
		origin := r.Header.Get("Origin")
		if origin != "" {
			w.Header().Add("Vary", "Origin")
			if !a.allowed(origin) {
				write(w, 403, nil, platform.Fail(403, "不允许的请求来源"), rid)
				return
			}
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
		}
		if r.Method == http.MethodOptions {
			w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,PATCH,DELETE,OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type,Authorization,Idempotency-Key")
			w.WriteHeader(204)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 55*time.Second)
		defer cancel()
		r = r.WithContext(ctx)
		if strings.HasSuffix(route.Path, "/auth/login") || strings.HasSuffix(route.Path, "/auth/register") || strings.HasSuffix(route.Path, "/auth/refresh") {
			if !a.allowAttempt(r) {
				w.Header().Set("Retry-After", "60")
				write(w, 429, nil, platform.Fail(429, "请求过于频繁"), rid)
				return
			}
		}
		var p identity.Principal
		if route.Kind != "" {
			if !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
				write(w, 401, nil, platform.Fail(401, "缺少 Bearer 登录凭据"), rid)
				return
			}
			raw := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
			var e error
			p, e = a.Identity.Authenticate(ctx, route.Kind, raw)
			if e != nil {
				write(w, 401, nil, e, rid)
				return
			}
			if route.Permission != "" {
				if e = a.Identity.Authorize(ctx, p, route.Permission); e != nil {
					write(w, 403, nil, e, rid)
					return
				}
			}
		}
		data, e := route.handler(w, r, p, rid)
		if e != nil {
			write(w, 500, nil, e, rid)
			return
		}
		if _, ok := data.(rawResponse); ok {
			return
		}
		write(w, 200, data, nil, rid)
	}
}

type rawResponse struct{}

func (a *API) RestRoutes() []rest.Route {
	out := []rest.Route{}
	paths := map[string]bool{}
	for _, r := range a.Routes() {
		out = append(out, rest.Route{Method: r.Method, Path: r.Path, Handler: a.wrap(r)})
		if !paths[r.Path] {
			paths[r.Path] = true
			out = append(out, rest.Route{Method: "OPTIONS", Path: r.Path, Handler: a.wrap(r)})
		}
	}
	return out
}

// Handler uses the same routing and application middleware for integration tests.
func (a *API) Handler() http.Handler {
	r := router.NewRouter()
	r.SetNotFoundHandler(a.NotFoundHandler())
	r.SetNotAllowedHandler(a.ErrorHandler(405, "请求方法不允许"))
	for _, route := range a.RestRoutes() {
		if e := r.Handle(route.Method, route.Path, route.Handler); e != nil {
			panic(e)
		}
	}
	return r
}
func (a *API) ErrorHandler(status int, message string) http.Handler {
	return a.wrap(Route{handler: func(w http.ResponseWriter, r *http.Request, p identity.Principal, rid string) (any, error) {
		return nil, platform.Fail(status, message)
	}})
}
func (a *API) Ready(ctx context.Context) error {
	if e := a.Identity.DB.PingContext(ctx); e != nil {
		return platform.Fail(503, "数据库不可用")
	}
	if e := migrations.Check(ctx, a.Identity.DB); e != nil {
		return platform.Fail(503, "数据库迁移版本不匹配")
	}
	f, e := os.CreateTemp(a.Config.StorageDir, ".ready-*")
	if e != nil {
		return platform.Fail(503, "存储目录不可写")
	}
	name := f.Name()
	f.Close()
	if e = os.Remove(name); e != nil {
		return platform.Fail(503, "存储目录不可清理")
	}
	return nil
}
func (a *API) setCookie(w http.ResponseWriter, kind string, t identity.Tokens) {
	path := "/api/v1/auth"
	if kind == "admin" {
		path = "/api/admin/v1/auth"
	}
	http.SetCookie(w, &http.Cookie{Name: "design_" + kind + "_refresh", Value: t.RefreshToken, Path: path, HttpOnly: true, Secure: a.Config.CookieSecure, SameSite: http.SameSiteLaxMode, Expires: t.RefreshExpires, MaxAge: int(time.Until(t.RefreshExpires).Seconds())})
}
func (a *API) clearCookie(w http.ResponseWriter, kind string) {
	path := "/api/v1/auth"
	if kind == "admin" {
		path = "/api/admin/v1/auth"
	}
	http.SetCookie(w, &http.Cookie{Name: "design_" + kind + "_refresh", Value: "", Path: path, HttpOnly: true, Secure: a.Config.CookieSecure, SameSite: http.SameSiteLaxMode, MaxAge: -1})
}
func cookie(r *http.Request, kind string) string {
	c, e := r.Cookie("design_" + kind + "_refresh")
	if e != nil {
		return ""
	}
	return c.Value
}
