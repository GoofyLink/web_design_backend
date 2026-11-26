package transport

import (
	"looklook/app/design/internal/modules/catalog"
	"looklook/app/design/internal/modules/identity"
	"looklook/app/design/internal/platform"
	"net/http"
	"strings"
)

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}
type CreateUserRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Nickname string `json:"nickname,omitempty"`
}
type UpdateUserRequest struct {
	Username string `json:"username"`
	Nickname string `json:"nickname"`
}
type ResetUserPasswordRequest struct {
	NewPassword string `json:"newPassword"`
}
type ProfileRequest struct {
	Nickname string `json:"nickname"`
	Avatar   string `json:"avatar"`
}
type PasswordRequest struct {
	OldPassword string `json:"oldPassword"`
	NewPassword string `json:"newPassword"`
}
type StatusRequest struct {
	Status string `json:"status"`
}
type RoleRequest struct {
	Code        string   `json:"code"`
	Name        string   `json:"name"`
	Permissions []string `json:"permissions"`
}
type RolesRequest struct {
	RoleIDs []string `json:"roleIds"`
}
type AdminRequest struct {
	Username string   `json:"username"`
	Password string   `json:"password"`
	RoleIDs  []string `json:"roleIds"`
}
type CategoryRequest struct {
	Name   string `json:"name"`
	Sort   int    `json:"sort"`
	Status string `json:"status"`
}
type TemplateRequest struct {
	Version int64           `json:"version"`
	Content catalog.Content `json:"content"`
}
type VersionRequest struct {
	Version int64 `json:"version"`
}

func roleIDs(values []string) ([]int64, error) {
	out := []int64{}
	for _, v := range values {
		n, e := platform.ParseID(v)
		if e != nil {
			return nil, e
		}
		out = append(out, n)
	}
	return out, nil
}
func (a *API) Routes() []Route {
	out := []Route{}
	add := func(method, path, kind, permission, summary string, body any, f endpoint) {
		out = append(out, Route{method, path, kind, permission, summary, body, f})
	}
	add("GET", "/healthz", "", "", "进程存活", nil, func(w http.ResponseWriter, r *http.Request, p identity.Principal, rid string) (any, error) {
		return map[string]string{"status": "ok"}, nil
	})
	add("GET", "/readyz", "", "", "数据库和存储就绪", nil, func(w http.ResponseWriter, r *http.Request, p identity.Principal, rid string) (any, error) {
		return map[string]string{"status": "ready"}, a.Ready(r.Context())
	})
	add("POST", "/api/v1/auth/register", "", "", "用户注册", LoginRequest{}, func(w http.ResponseWriter, r *http.Request, p identity.Principal, rid string) (any, error) {
		if !a.Config.AllowPublicRegistration {
			return nil, platform.Fail(403, "公司内部使用，请联系管理员开通账号")
		}
		v, e := decode[LoginRequest](w, r)
		if e != nil {
			return nil, e
		}
		id, e := a.Identity.Register(r.Context(), v.Username, v.Password)
		return map[string]string{"id": id}, e
	})
	for _, kind := range []string{"user", "admin"} {
		kind := kind
		base := "/api/v1"
		if kind == "admin" {
			base = "/api/admin/v1"
		}
		add("POST", base+"/auth/login", "", "", kind+" 登录", LoginRequest{}, func(w http.ResponseWriter, r *http.Request, p identity.Principal, rid string) (any, error) {
			v, e := decode[LoginRequest](w, r)
			if e != nil {
				return nil, e
			}
			t, e := a.Identity.Login(r.Context(), kind, v.Username, v.Password)
			if e == nil {
				a.setCookie(w, kind, t)
			}
			return t, e
		})
		add("POST", base+"/auth/refresh", "", "", kind+" 刷新会话", nil, func(w http.ResponseWriter, r *http.Request, p identity.Principal, rid string) (any, error) {
			if !a.allowed(r.Header.Get("Origin")) {
				return nil, platform.Fail(403, "刷新请求须提供受信 Origin")
			}
			t, e := a.Identity.RefreshToken(r.Context(), kind, cookie(r, kind))
			if e == nil {
				a.setCookie(w, kind, t)
			} else {
				a.clearCookie(w, kind)
			}
			return t, e
		})
		add("POST", base+"/auth/logout", "", "", kind+" 退出会话", nil, func(w http.ResponseWriter, r *http.Request, p identity.Principal, rid string) (any, error) {
			if !a.allowed(r.Header.Get("Origin")) {
				return nil, platform.Fail(403, "退出请求须提供受信 Origin")
			}
			var principal *identity.Principal
			if header := r.Header.Get("Authorization"); header != "" {
				current, e := a.Identity.Authenticate(r.Context(), kind, strings.TrimPrefix(header, "Bearer "))
				if e == nil {
					principal = &current
				} else if cookie(r, kind) == "" {
					return nil, e
				}
			}
			e := a.Identity.Logout(r.Context(), kind, cookie(r, kind), principal)
			if e == nil {
				a.clearCookie(w, kind)
			}
			return nil, e
		})
		add("GET", base+"/me", kind, "", "当前账户", nil, func(w http.ResponseWriter, r *http.Request, p identity.Principal, rid string) (any, error) {
			return a.Identity.Me(r.Context(), p)
		})
		add("PATCH", base+"/me", kind, "", "修改个人资料", ProfileRequest{}, func(w http.ResponseWriter, r *http.Request, p identity.Principal, rid string) (any, error) {
			v, e := decode[ProfileRequest](w, r)
			if e != nil {
				return nil, e
			}
			return nil, a.Identity.Profile(r.Context(), p, v.Nickname, v.Avatar, rid)
		})
		add("PUT", base+"/me/password", kind, "", "修改密码并撤销全部会话", PasswordRequest{}, func(w http.ResponseWriter, r *http.Request, p identity.Principal, rid string) (any, error) {
			v, e := decode[PasswordRequest](w, r)
			if e != nil {
				return nil, e
			}
			e = a.Identity.Password(r.Context(), p, v.OldPassword, v.NewPassword, rid)
			if e == nil {
				a.clearCookie(w, kind)
			}
			return nil, e
		})
	}
	admin := "/api/admin/v1"
	add("POST", admin+"/users", "admin", "user:create", "管理员创建内部用户", CreateUserRequest{}, func(w http.ResponseWriter, r *http.Request, p identity.Principal, rid string) (any, error) {
		v, e := decode[CreateUserRequest](w, r)
		if e != nil {
			return nil, e
		}
		id, e := a.Identity.CreateUser(r.Context(), p.ID, v.Username, v.Password, v.Nickname, rid)
		return map[string]string{"id": id}, e
	})
	add("PATCH", admin+"/users/:id", "admin", "user:update", "编辑内部用户账号和姓名", UpdateUserRequest{}, func(w http.ResponseWriter, r *http.Request, p identity.Principal, rid string) (any, error) {
		n, e := id(r)
		if e != nil {
			return nil, e
		}
		v, e := decode[UpdateUserRequest](w, r)
		if e != nil {
			return nil, e
		}
		return nil, a.Identity.UpdateUser(r.Context(), p.ID, n, v.Username, v.Nickname, rid)
	})
	add("PUT", admin+"/users/:id/password", "admin", "user:password", "重置内部用户密码并撤销全部会话", ResetUserPasswordRequest{}, func(w http.ResponseWriter, r *http.Request, p identity.Principal, rid string) (any, error) {
		n, e := id(r)
		if e != nil {
			return nil, e
		}
		v, e := decode[ResetUserPasswordRequest](w, r)
		if e != nil {
			return nil, e
		}
		return nil, a.Identity.ResetUserPassword(r.Context(), p.ID, n, v.NewPassword, rid)
	})

	add("GET", admin+"/user/info", "admin", "", "Vben 当前管理员资料", nil, func(w http.ResponseWriter, r *http.Request, p identity.Principal, rid string) (any, error) {
		u, e := a.Identity.Me(r.Context(), p)
		if e != nil {
			return nil, e
		}
		_, roles, e := a.Identity.Codes(r.Context(), p.ID)
		return map[string]any{"userId": u.ID, "username": u.Username, "realName": u.Nickname, "avatar": u.Avatar, "roles": roles, "homePath": "/"}, e
	})
	add("GET", admin+"/auth/codes", "admin", "", "当前管理员权限码", nil, func(w http.ResponseWriter, r *http.Request, p identity.Principal, rid string) (any, error) {
		codes, _, e := a.Identity.Codes(r.Context(), p.ID)
		return codes, e
	})
	for _, kind := range []string{"user", "admin"} {
		kind := kind
		path := admin + "/users"
		perm := "user:read"
		writePerm := "user:status"
		if kind == "admin" {
			path = admin + "/admins"
			perm = "admin:read"
			writePerm = "admin:write"
		}
		add("GET", path, "admin", perm, "账户分页列表", nil, func(w http.ResponseWriter, r *http.Request, p identity.Principal, rid string) (any, error) {
			pg, e := page(r)
			if e != nil {
				return nil, e
			}
			return a.Identity.Accounts(r.Context(), kind, pg)
		})
		add("GET", path+"/:id", "admin", perm, "账户详情（管理员包含 roleIds）", nil, func(w http.ResponseWriter, r *http.Request, p identity.Principal, rid string) (any, error) {
			n, e := id(r)
			if e != nil {
				return nil, e
			}
			return a.Identity.AccountDetail(r.Context(), kind, n)
		})
		add("PATCH", path+"/:id/status", "admin", writePerm, "启停账户并撤销会话", StatusRequest{}, func(w http.ResponseWriter, r *http.Request, p identity.Principal, rid string) (any, error) {
			n, e := id(r)
			if e != nil {
				return nil, e
			}
			v, e := decode[StatusRequest](w, r)
			if e != nil {
				return nil, e
			}
			return nil, a.Identity.SetStatus(r.Context(), p.ID, n, kind, v.Status, rid)
		})
	}
	add("POST", admin+"/admins", "admin", "admin:write", "创建管理员", AdminRequest{}, func(w http.ResponseWriter, r *http.Request, p identity.Principal, rid string) (any, error) {
		v, e := decode[AdminRequest](w, r)
		if e != nil {
			return nil, e
		}
		roles, e := roleIDs(v.RoleIDs)
		if e != nil {
			return nil, e
		}
		id, e := a.Identity.CreateAdmin(r.Context(), p.ID, v.Username, v.Password, roles, false, rid)
		return map[string]string{"id": id}, e
	})
	add("PUT", admin+"/admins/:id/roles", "admin", "admin:write", "分配管理员角色", RolesRequest{}, func(w http.ResponseWriter, r *http.Request, p identity.Principal, rid string) (any, error) {
		n, e := id(r)
		if e != nil {
			return nil, e
		}
		v, e := decode[RolesRequest](w, r)
		if e != nil {
			return nil, e
		}
		roles, e := roleIDs(v.RoleIDs)
		if e != nil {
			return nil, e
		}
		return nil, a.Identity.SetAdminRoles(r.Context(), p.ID, n, roles, rid)
	})
	add("GET", admin+"/roles", "admin", "role:read", "角色列表及权限", nil, func(w http.ResponseWriter, r *http.Request, p identity.Principal, rid string) (any, error) {
		return a.Identity.Roles(r.Context())
	})
	add("GET", admin+"/permissions", "admin", "role:read", "全部可分配权限", nil, func(w http.ResponseWriter, r *http.Request, p identity.Principal, rid string) (any, error) {
		return a.Identity.Permissions(r.Context())
	})
	for _, method := range []string{"POST", "PUT"} {
		method := method
		path := admin + "/roles"
		if method == "PUT" {
			path += "/:id"
		}
		add(method, path, "admin", "role:write", "保存角色与权限", RoleRequest{}, func(w http.ResponseWriter, r *http.Request, p identity.Principal, rid string) (any, error) {
			var n int64
			var e error
			if method == "PUT" {
				n, e = id(r)
				if e != nil {
					return nil, e
				}
			}
			v, e := decode[RoleRequest](w, r)
			if e != nil {
				return nil, e
			}
			result, e := a.Identity.SaveRole(r.Context(), p.ID, n, v.Code, v.Name, v.Permissions, rid)
			return map[string]string{"id": result}, e
		})
	}
	add("DELETE", admin+"/roles/:id", "admin", "role:write", "删除未使用角色", nil, func(w http.ResponseWriter, r *http.Request, p identity.Principal, rid string) (any, error) {
		n, e := id(r)
		if e != nil {
			return nil, e
		}
		return nil, a.Identity.DeleteRole(r.Context(), p.ID, n, rid)
	})
	add("GET", admin+"/audit-logs", "admin", "audit:read", "审计日志分页", nil, func(w http.ResponseWriter, r *http.Request, p identity.Principal, rid string) (any, error) {
		pg, e := page(r)
		if e != nil {
			return nil, e
		}
		return a.Identity.AuditLogs(r.Context(), pg)
	})
	for _, isAdmin := range []bool{false, true} {
		isAdmin := isAdmin
		base := "/api/v1"
		kind, catPerm, tplPerm := "", "", ""
		if isAdmin {
			base = admin
			kind = "admin"
			catPerm = "category:read"
			tplPerm = "template:read"
		}
		add("GET", base+"/categories", kind, catPerm, "分类列表", nil, func(w http.ResponseWriter, r *http.Request, p identity.Principal, rid string) (any, error) {
			return a.Catalog.Categories(r.Context(), isAdmin)
		})
		add("GET", base+"/templates", kind, tplPerm, "模板搜索与分页", nil, func(w http.ResponseWriter, r *http.Request, p identity.Principal, rid string) (any, error) {
			pg, e := page(r)
			if e != nil {
				return nil, e
			}
			q := r.URL.Query()
			return a.Catalog.Templates(r.Context(), isAdmin, pg, q.Get("categoryId"), q.Get("sort"), q.Get("ratio"))
		})
		add("GET", base+"/templates/:id", kind, tplPerm, "模板详情", nil, func(w http.ResponseWriter, r *http.Request, p identity.Principal, rid string) (any, error) {
			n, e := id(r)
			if e != nil {
				return nil, e
			}
			return a.Catalog.Detail(r.Context(), n, isAdmin)
		})
	}
	add("GET", "/api/v1/home", "", "", "首页分类与最新模板", nil, func(w http.ResponseWriter, r *http.Request, p identity.Principal, rid string) (any, error) {
		cats, e := a.Catalog.Categories(r.Context(), false)
		if e != nil {
			return nil, e
		}
		templates, e := a.Catalog.Templates(r.Context(), false, platform.Page{Number: 1, Size: 12}, "", "latest", "")
		if e != nil {
			return nil, e
		}
		topics, e := a.Operations.Topics(r.Context(), false, platform.Page{Number: 1, Size: 12}, "home")
		return map[string]any{"categories": cats, "templates": templates, "topics": topics}, e
	})
	for _, method := range []string{"POST", "PUT"} {
		method := method
		path := admin + "/categories"
		if method == "PUT" {
			path += "/:id"
		}
		add(method, path, "admin", "category:write", "创建或修改分类", CategoryRequest{}, func(w http.ResponseWriter, r *http.Request, p identity.Principal, rid string) (any, error) {
			var n int64
			var e error
			if method == "PUT" {
				n, e = id(r)
				if e != nil {
					return nil, e
				}
			}
			v, e := decode[CategoryRequest](w, r)
			if e != nil {
				return nil, e
			}
			result, e := a.Catalog.SaveCategory(r.Context(), p.ID, n, catalog.Category{Name: v.Name, Sort: v.Sort, Status: v.Status}, rid)
			return map[string]string{"id": result}, e
		})
	}
	add("DELETE", admin+"/categories/:id", "admin", "category:write", "删除未被引用分类", nil, func(w http.ResponseWriter, r *http.Request, p identity.Principal, rid string) (any, error) {
		n, e := id(r)
		if e != nil {
			return nil, e
		}
		return nil, a.Catalog.DeleteCategory(r.Context(), p.ID, n, rid)
	})
	for _, method := range []string{"POST", "PUT"} {
		method := method
		path := admin + "/templates"
		if method == "PUT" {
			path += "/:id"
		}
		add(method, path, "admin", "template:write", "创建或更新模板草稿", TemplateRequest{}, func(w http.ResponseWriter, r *http.Request, p identity.Principal, rid string) (any, error) {
			var n int64
			var e error
			if method == "PUT" {
				n, e = id(r)
				if e != nil {
					return nil, e
				}
			}
			v, e := decode[TemplateRequest](w, r)
			if e != nil {
				return nil, e
			}
			return a.Catalog.SaveTemplate(r.Context(), p.ID, n, v.Version, v.Content, rid)
		})
	}
	for _, action := range []string{"publish", "offline", "delete"} {
		action := action
		method := "POST"
		path := admin + "/templates/:id/" + action
		perm := "template:publish"
		if action == "delete" {
			method = "DELETE"
			path = admin + "/templates/:id"
			perm = "template:write"
		}
		add(method, path, "admin", perm, "模板状态操作 "+action, VersionRequest{}, func(w http.ResponseWriter, r *http.Request, p identity.Principal, rid string) (any, error) {
			n, e := id(r)
			if e != nil {
				return nil, e
			}
			v, e := decode[VersionRequest](w, r)
			if e != nil {
				return nil, e
			}
			return nil, a.Catalog.Transition(r.Context(), p.ID, n, v.Version, action, rid)
		})
	}
	add("GET", "/api/v1/seed-assets/poster.svg", "", "", "自制示例模板封面", nil, func(w http.ResponseWriter, r *http.Request, p identity.Principal, rid string) (any, error) {
		w.Header().Set("Content-Type", "image/svg+xml")
		w.Header().Set("Cache-Control", "public, max-age=3600")
		w.Write([]byte(`<svg xmlns="http://www.w3.org/2000/svg" width="600" height="800"><rect width="600" height="800" fill="#e7eef8"/><rect x="40" y="80" width="520" height="640" rx="24" fill="#3568d4"/><text x="300" y="400" fill="white" font-size="48" font-family="sans-serif" text-anchor="middle">DESIGN DEMO</text></svg>`))
		return rawResponse{}, nil
	})
	out = append(out, a.contentRoutes()...)
	out = append(out, a.operationRoutes()...)
	return append(out, a.taskRoutes()...)
}
