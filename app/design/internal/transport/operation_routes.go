package transport

import (
	"github.com/zeromicro/go-zero/rest/pathvar"
	"looklook/app/design/internal/modules/identity"
	"looklook/app/design/internal/modules/operation"
	"looklook/app/design/internal/platform"
	"net/http"
	"time"
)

type CoverRequest struct {
	Version int64  `json:"version"`
	AssetID string `json:"assetId"`
}
type FolderRequest struct {
	Name    string `json:"name"`
	Version int64  `json:"version"`
}
type MoveRequest struct {
	Version  int64  `json:"version"`
	FolderID string `json:"folderId"`
}

// Decode typed bodies once while preserving the same endpoint middleware.
func withBody[T any](f func(*http.Request, identity.Principal, string, T) (any, error)) endpoint {
	return func(w http.ResponseWriter, r *http.Request, p identity.Principal, rid string) (any, error) {
		v, e := decode[T](w, r)
		if e != nil {
			return nil, e
		}
		return f(r, p, rid, v)
	}
}
func withID(f func(*http.Request, identity.Principal, string, int64) (any, error)) endpoint {
	return func(w http.ResponseWriter, r *http.Request, p identity.Principal, rid string) (any, error) {
		n, e := id(r)
		if e != nil {
			return nil, e
		}
		return f(r, p, rid, n)
	}
}
func withIDBody[T any](f func(*http.Request, identity.Principal, string, int64, T) (any, error)) endpoint {
	return withBody(func(r *http.Request, p identity.Principal, rid string, v T) (any, error) {
		n, e := id(r)
		if e != nil {
			return nil, e
		}
		return f(r, p, rid, n, v)
	})
}
func (a *API) operationRoutes() []Route {
	out := []Route{}
	add := func(m, path, kind, perm, summary string, body any, h endpoint) {
		out = append(out, Route{m, path, kind, perm, summary, body, h})
	}

	add("GET", "/api/v1/canvas-presets", "", "", "启用的推荐画布尺寸", nil, func(w http.ResponseWriter, r *http.Request, p identity.Principal, rid string) (any, error) {
		return a.Operations.CanvasPresets(r.Context(), true)
	})
	add("GET", "/api/admin/v1/canvas-presets", "admin", "canvas:read", "画布推荐尺寸配置", nil, func(w http.ResponseWriter, r *http.Request, p identity.Principal, rid string) (any, error) {
		return a.Operations.CanvasPresets(r.Context(), false)
	})
	add("PUT", "/api/admin/v1/canvas-presets", "admin", "canvas:write", "按版本保存画布尺寸配置（数组顺序即展示顺序）", operation.CanvasPresets{}, withBody(func(r *http.Request, p identity.Principal, rid string, v operation.CanvasPresets) (any, error) {
		return a.Operations.SaveCanvasPresets(r.Context(), p.ID, v, rid)
	}))
	add("POST", "/api/v1/designs/:id/export-check", "user", "", "校验导出版本、资源和像素", operation.ExportRequest{}, withIDBody(func(r *http.Request, p identity.Principal, rid string, id int64, v operation.ExportRequest) (any, error) {
		return a.Operations.ExportCheck(r.Context(), p.ID, id, v)
	}))
	add("POST", "/api/v1/export-events", "user", "", "幂等记录浏览器导出结果", operation.ExportEvent{}, withBody(func(r *http.Request, p identity.Principal, rid string, v operation.ExportEvent) (any, error) {
		return nil, a.Operations.ExportEvent(r.Context(), p.ID, v, rid)
	}))
	add("POST", "/api/v1/designs/:id/cover", "user", "", "按版本替换上传成功的封面", CoverRequest{}, withIDBody(func(r *http.Request, p identity.Principal, rid string, id int64, v CoverRequest) (any, error) {
		return a.Workspace.Cover(r.Context(), p.ID, id, v.Version, v.AssetID, rid)
	}))
	add("GET", "/api/admin/v1/export-events", "admin", "operation:read", "导出结果及异常分页", nil, func(w http.ResponseWriter, r *http.Request, p identity.Principal, rid string) (any, error) {
		pg, e := page(r)
		if e != nil {
			return nil, e
		}
		return a.Operations.Exports(r.Context(), pg)
	})
	add("GET", "/api/admin/v1/statistics", "admin", "operation:read", "全局数量与区间导出统计", nil, func(w http.ResponseWriter, r *http.Request, p identity.Principal, rid string) (any, error) {
		to := time.Now().UTC()
		from := to.Add(-30 * 24 * time.Hour)
		for name, dest := range map[string]*time.Time{"from": &from, "to": &to} {
			if raw := r.URL.Query().Get(name); raw != "" {
				v, e := time.Parse(time.RFC3339, raw)
				if e != nil {
					return nil, platform.Fail(400, "from/to 必须是 RFC3339")
				}
				*dest = v
			}
		}
		return a.Operations.Stats(r.Context(), from, to)
	})
	for _, method := range []string{"PUT", "DELETE"} {
		method := method
		add(method, "/api/v1/favorites/:id", "user", "", "收藏或取消收藏模板", nil, withID(func(r *http.Request, p identity.Principal, rid string, id int64) (any, error) {
			return nil, a.Operations.Favorite(r.Context(), p.ID, id, method == "PUT")
		}))
	}
	add("GET", "/api/v1/favorites", "user", "", "当前仍可见的收藏模板", nil, func(w http.ResponseWriter, r *http.Request, p identity.Principal, rid string) (any, error) {
		pg, e := page(r)
		if e != nil {
			return nil, e
		}
		return a.Operations.Favorites(r.Context(), p.ID, pg)
	})
	add("GET", "/api/v1/folders", "user", "", "我的文件夹（最多 200 个）", nil, func(w http.ResponseWriter, r *http.Request, p identity.Principal, rid string) (any, error) {
		return a.Operations.Folders(r.Context(), p.ID)
	})
	add("POST", "/api/v1/folders", "user", "", "创建文件夹", FolderRequest{}, withBody(func(r *http.Request, p identity.Principal, rid string, v FolderRequest) (any, error) {
		return a.Operations.FolderSave(r.Context(), p.ID, 0, 0, v.Name, rid)
	}))
	add("PUT", "/api/v1/folders/:id", "user", "", "按版本修改文件夹", FolderRequest{}, withIDBody(func(r *http.Request, p identity.Principal, rid string, id int64, v FolderRequest) (any, error) {
		return a.Operations.FolderSave(r.Context(), p.ID, id, v.Version, v.Name, rid)
	}))
	add("DELETE", "/api/v1/folders/:id", "user", "", "删除文件夹，保留作品", VersionRequest{}, withIDBody(func(r *http.Request, p identity.Principal, rid string, id int64, v VersionRequest) (any, error) {
		return nil, a.Operations.FolderDelete(r.Context(), p.ID, id, v.Version, rid)
	}))
	add("GET", "/api/v1/folders/:id/designs", "user", "", "文件夹作品分页", nil, withID(func(r *http.Request, p identity.Principal, rid string, id int64) (any, error) {
		pg, e := page(r)
		if e != nil {
			return nil, e
		}
		return a.Operations.FolderItems(r.Context(), p.ID, id, pg)
	}))
	add("PUT", "/api/v1/designs/:id/folder", "user", "", "移动作品；folderId 空字符串表示移出", MoveRequest{}, withIDBody(func(r *http.Request, p identity.Principal, rid string, id int64, v MoveRequest) (any, error) {
		var folder int64
		var e error
		if v.FolderID != "" {
			folder, e = platform.ParseID(v.FolderID)
			if e != nil {
				return nil, e
			}
		}
		return a.Operations.Move(r.Context(), p.ID, id, v.Version, folder, rid)
	}))
	add("GET", "/api/v1/designs/:id/history", "user", "", "最近 20 次文档历史", nil, withID(func(r *http.Request, p identity.Principal, rid string, id int64) (any, error) {
		return a.Workspace.History(r.Context(), p.ID, id)
	}))
	add("GET", "/api/v1/designs/:id/history/:historyId", "user", "", "历史文档详情", nil, withID(func(r *http.Request, p identity.Principal, rid string, id int64) (any, error) {
		h, e := platform.ParseID(pathvar.Vars(r)["historyId"])
		if e != nil {
			return nil, e
		}
		return a.Workspace.HistoryDetail(r.Context(), p.ID, id, h)
	}))
	add("POST", "/api/v1/designs/:id/history/:historyId/restore", "user", "", "历史内容恢复为新版本", VersionRequest{}, withIDBody(func(r *http.Request, p identity.Principal, rid string, id int64, v VersionRequest) (any, error) {
		h, e := platform.ParseID(pathvar.Vars(r)["historyId"])
		if e != nil {
			return nil, e
		}
		return a.Workspace.RestoreHistory(r.Context(), p.ID, id, h, v.Version, rid)
	}))
	for _, admin := range []bool{false, true} {
		admin := admin
		base, kind, perm := "/api/v1", "", ""
		if admin {
			base, kind, perm = "/api/admin/v1", "admin", "operation:read"
		}
		add("GET", base+"/topics", kind, perm, "专题列表；slot=home/featured", nil, func(w http.ResponseWriter, r *http.Request, p identity.Principal, rid string) (any, error) {
			pg, e := page(r)
			if e != nil {
				return nil, e
			}
			return a.Operations.Topics(r.Context(), admin, pg, r.URL.Query().Get("slot"))
		})
		add("GET", base+"/topics/:id", kind, perm, "专题详情与排序模板", nil, withID(func(r *http.Request, p identity.Principal, rid string, id int64) (any, error) {
			return a.Operations.Topic(r.Context(), id, admin)
		}))
	}
	add("POST", "/api/admin/v1/topics", "admin", "operation:write", "创建专题推荐", operation.TopicInput{}, withBody(func(r *http.Request, p identity.Principal, rid string, v operation.TopicInput) (any, error) {
		return a.Operations.TopicSave(r.Context(), p.ID, 0, v, rid)
	}))
	add("PUT", "/api/admin/v1/topics/:id", "admin", "operation:write", "按版本修改专题与上下线", operation.TopicInput{}, withIDBody(func(r *http.Request, p identity.Principal, rid string, id int64, v operation.TopicInput) (any, error) {
		return a.Operations.TopicSave(r.Context(), p.ID, id, v, rid)
	}))
	add("DELETE", "/api/admin/v1/topics/:id", "admin", "operation:write", "删除已下线专题", VersionRequest{}, withIDBody(func(r *http.Request, p identity.Principal, rid string, id int64, v VersionRequest) (any, error) {
		return nil, a.Operations.TopicDelete(r.Context(), p.ID, id, v.Version, rid)
	}))
	return out
}
