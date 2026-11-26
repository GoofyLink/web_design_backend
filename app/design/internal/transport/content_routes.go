package transport

import (
	"errors"
	"github.com/zeromicro/go-zero/rest/pathvar"
	"looklook/app/design/internal/modules/asset"
	"looklook/app/design/internal/modules/canvas"
	"looklook/app/design/internal/modules/identity"
	"looklook/app/design/internal/platform"
	"mime"
	"net/http"
)

type DesignRequest struct {
	Title   string         `json:"title,omitempty"`
	Content canvas.Content `json:"content"`
}
type SaveDesignRequest struct {
	Version int64          `json:"version"`
	Content canvas.Content `json:"content"`
}
type RenameDesignRequest struct {
	Version int64  `json:"version"`
	Title   string `json:"title"`
}
type CopyRequest struct {
	Title string `json:"title,omitempty"`
}
type UploadRequest struct {
	File    string `json:"file"`
	Kind    string `json:"kind"`
	License string `json:"license,omitempty"`
}

func (a *API) contentRoutes() []Route {
	out := []Route{}
	add := func(method, path, kind, perm, summary string, body any, f endpoint) {
		out = append(out, Route{method, path, kind, perm, summary, body, f})
	}
	for _, kind := range []string{"user", "admin"} {
		kind := kind
		admin := kind == "admin"
		base := "/api/v1"
		read, write := "", ""
		if admin {
			base = "/api/admin/v1"
			read = "asset:read"
			write = "asset:write"
		}
		add("POST", base+"/assets", kind, write, "上传图片或管理员公共字体", UploadRequest{}, func(w http.ResponseWriter, r *http.Request, p identity.Principal, rid string) (any, error) {
			r.Body = http.MaxBytesReader(w, r.Body, asset.MaxUpload+1024*1024)
			if e := r.ParseMultipartForm(1024 * 1024); e != nil {
				if r.MultipartForm != nil {
					r.MultipartForm.RemoveAll()
				}
				var large *http.MaxBytesError
				if errors.As(e, &large) {
					return nil, platform.Fail(413, "上传请求超过 21 MiB")
				}
				return nil, platform.Fail(400, "无效 multipart/form-data")
			}
			defer r.MultipartForm.RemoveAll()
			if len(r.MultipartForm.File) != 1 || len(r.MultipartForm.File["file"]) != 1 {
				return nil, platform.Fail(400, "一次仅允许一个 file 文件")
			}
			file, header, e := r.FormFile("file")
			if e != nil {
				return nil, platform.Fail(400, "缺少 file 文件")
			}
			defer file.Close()
			return a.Assets.Upload(r.Context(), p.ID, admin, r.FormValue("kind"), header.Filename, r.FormValue("license"), file, rid)
		})
		add("GET", base+"/assets", kind, read, "素材分页；scope=mine/public，kind=image/font", nil, func(w http.ResponseWriter, r *http.Request, p identity.Principal, rid string) (any, error) {
			pg, e := page(r)
			if e != nil {
				return nil, e
			}
			return a.Assets.List(r.Context(), p.ID, admin, pg, r.URL.Query().Get("scope"), r.URL.Query().Get("kind"))
		})
		add("GET", base+"/assets/:id", kind, read, "素材元数据", nil, func(w http.ResponseWriter, r *http.Request, p identity.Principal, rid string) (any, error) {
			n, e := id(r)
			if e != nil {
				return nil, e
			}
			return a.Assets.Get(r.Context(), n, p.ID, admin)
		})
		add("DELETE", base+"/assets/:id", kind, write, "删除未引用素材", nil, func(w http.ResponseWriter, r *http.Request, p identity.Principal, rid string) (any, error) {
			n, e := id(r)
			if e != nil {
				return nil, e
			}
			return nil, a.Assets.Change(r.Context(), n, p.ID, admin, "deleted", rid)
		})
		add("POST", base+"/assets/:id/access-url", kind, read, "获取 60 秒文件访问链接", nil, func(w http.ResponseWriter, r *http.Request, p identity.Principal, rid string) (any, error) {
			n, e := id(r)
			if e != nil {
				return nil, e
			}
			if _, e = a.Assets.Get(r.Context(), n, p.ID, admin); e != nil {
				return nil, e
			}
			return a.assetURL(n, p)
		})
		if admin {
			add("GET", base+"/assets/:id/references", kind, read, "查询公共素材引用", nil, func(w http.ResponseWriter, r *http.Request, p identity.Principal, rid string) (any, error) {
				n, e := id(r)
				if e != nil {
					return nil, e
				}
				return a.Assets.References(r.Context(), n, p.ID, true)
			})
			add("PATCH", base+"/assets/:id/status", kind, write, "启停未引用公共素材", StatusRequest{}, func(w http.ResponseWriter, r *http.Request, p identity.Principal, rid string) (any, error) {
				n, e := id(r)
				if e != nil {
					return nil, e
				}
				v, e := decode[StatusRequest](w, r)
				if e != nil {
					return nil, e
				}
				if v.Status != "active" && v.Status != "disabled" {
					return nil, platform.Fail(400, "状态仅支持 active/disabled")
				}
				return nil, a.Assets.Change(r.Context(), n, p.ID, true, v.Status, rid)
			})
		}
	}
	add("GET", "/api/v1/storage", "user", "", "我的存储额度", nil, func(w http.ResponseWriter, r *http.Request, p identity.Principal, rid string) (any, error) {
		return a.Assets.Usage(r.Context(), p.ID)
	})
	add("GET", "/api/v1/fonts", "", "", "可用公共字体", nil, func(w http.ResponseWriter, r *http.Request, p identity.Principal, rid string) (any, error) {
		pg, e := page(r)
		if e != nil {
			return nil, e
		}
		return a.Assets.List(r.Context(), 0, false, pg, "public", "font")
	})
	add("GET", "/api/v1/assets/:id/content", "", "", "文件内容；公共资源匿名，私有资源凭 Bearer 或 ticket", nil, func(w http.ResponseWriter, r *http.Request, p identity.Principal, rid string) (any, error) {
		n, e := id(r)
		if e != nil {
			return nil, e
		}
		p, e = a.filePrincipal(r, n)
		if e != nil {
			return nil, e
		}
		meta, f, e := a.Assets.Open(r.Context(), n, p.ID, p.Kind == "admin")
		if e != nil {
			return nil, e
		}
		defer f.Close()
		stat, e := f.Stat()
		if e != nil {
			return nil, e
		}
		w.Header().Set("Content-Type", meta.MIME)
		w.Header().Set("Content-Disposition", mime.FormatMediaType("inline", map[string]string{"filename": meta.Name}))
		http.ServeContent(w, r, meta.Name, stat.ModTime(), f)
		return rawResponse{}, nil
	})
	add("POST", "/api/v1/designs", "user", "", "创建空白作品；必填 Idempotency-Key", DesignRequest{}, func(w http.ResponseWriter, r *http.Request, p identity.Principal, rid string) (any, error) {
		v, e := decode[DesignRequest](w, r)
		if e != nil {
			return nil, e
		}
		return a.Workspace.Create(r.Context(), p.ID, "blank", 0, v.Title, v.Content, r.Header.Get("Idempotency-Key"), rid)
	})
	for _, mode := range []string{"copy", "template"} {
		mode := mode
		path := "/api/v1/designs/:id/copy"
		if mode == "template" {
			path = "/api/v1/templates/:id/use"
		}
		add("POST", path, "user", "", "复制或套用为独立作品；必填 Idempotency-Key", CopyRequest{}, func(w http.ResponseWriter, r *http.Request, p identity.Principal, rid string) (any, error) {
			n, e := id(r)
			if e != nil {
				return nil, e
			}
			v, e := decode[CopyRequest](w, r)
			if e != nil {
				return nil, e
			}
			return a.Workspace.Create(r.Context(), p.ID, mode, n, v.Title, canvas.Content{}, r.Header.Get("Idempotency-Key"), rid)
		})
	}
	for _, trash := range []bool{false, true} {
		trash := trash
		path := "/api/v1/designs"
		if trash {
			path = "/api/v1/trash"
		}
		add("GET", path, "user", "", "作品或回收站分页", nil, func(w http.ResponseWriter, r *http.Request, p identity.Principal, rid string) (any, error) {
			pg, e := page(r)
			if e != nil {
				return nil, e
			}
			return a.Workspace.List(r.Context(), p.ID, pg, trash)
		})
	}
	add("GET", "/api/v1/designs/:id", "user", "", "作品文档和版本", nil, func(w http.ResponseWriter, r *http.Request, p identity.Principal, rid string) (any, error) {
		n, e := id(r)
		if e != nil {
			return nil, e
		}
		return a.Workspace.Get(r.Context(), p.ID, n)
	})
	add("PUT", "/api/v1/designs/:id/document", "user", "", "按版本保存完整文档；冲突 409", SaveDesignRequest{}, func(w http.ResponseWriter, r *http.Request, p identity.Principal, rid string) (any, error) {
		n, e := id(r)
		if e != nil {
			return nil, e
		}
		v, e := decode[SaveDesignRequest](w, r)
		if e != nil {
			return nil, e
		}
		return a.Workspace.Save(r.Context(), p.ID, n, v.Version, v.Content, rid)
	})
	add("PATCH", "/api/v1/designs/:id", "user", "", "按版本重命名", RenameDesignRequest{}, func(w http.ResponseWriter, r *http.Request, p identity.Principal, rid string) (any, error) {
		n, e := id(r)
		if e != nil {
			return nil, e
		}
		v, e := decode[RenameDesignRequest](w, r)
		if e != nil {
			return nil, e
		}
		return a.Workspace.Rename(r.Context(), p.ID, n, v.Version, v.Title, rid)
	})
	for _, action := range []string{"delete", "restore", "purge"} {
		action := action
		method, path := "DELETE", "/api/v1/designs/:id"
		if action == "restore" {
			method = "POST"
			path += "/restore"
		}
		if action == "purge" {
			path = "/api/v1/trash/:id"
		}
		add(method, path, "user", "", "作品回收站操作："+action, VersionRequest{}, func(w http.ResponseWriter, r *http.Request, p identity.Principal, rid string) (any, error) {
			n, e := id(r)
			if e != nil {
				return nil, e
			}
			v, e := decode[VersionRequest](w, r)
			if e != nil {
				return nil, e
			}
			return nil, a.Workspace.Trash(r.Context(), p.ID, n, v.Version, action, rid)
		})
	}
	add("GET", "/api/admin/v1/templates/:id/revisions", "admin", "template:read", "模板发布修订列表", nil, func(w http.ResponseWriter, r *http.Request, p identity.Principal, rid string) (any, error) {
		n, e := id(r)
		if e != nil {
			return nil, e
		}
		pg, e := page(r)
		if e != nil {
			return nil, e
		}
		return a.Catalog.Revisions(r.Context(), n, pg)
	})
	add("GET", "/api/admin/v1/templates/:id/revisions/:revisionId", "admin", "template:read", "不可变模板修订详情", nil, func(w http.ResponseWriter, r *http.Request, p identity.Principal, rid string) (any, error) {
		n, e := id(r)
		if e != nil {
			return nil, e
		}
		v, e := platform.ParseID(pathvar.Vars(r)["revisionId"])
		if e != nil {
			return nil, e
		}
		return a.Catalog.Revision(r.Context(), n, v)
	})
	add("GET", "/api/admin/v1/templates/:id/preview", "admin", "template:read", "读取草稿和已发布文档供编辑器预览", nil, func(w http.ResponseWriter, r *http.Request, p identity.Principal, rid string) (any, error) {
		n, e := id(r)
		if e != nil {
			return nil, e
		}
		return a.Catalog.Detail(r.Context(), n, true)
	})
	return out
}
