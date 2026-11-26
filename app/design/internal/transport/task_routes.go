package transport

import (
	"archive/zip"
	"fmt"
	"io"
	"looklook/app/design/internal/modules/identity"
	"looklook/app/design/internal/modules/task"
	"looklook/app/design/internal/platform"
	"net/http"
	"os"
)

type QuotaRequest struct {
	LimitUnits int64 `json:"limitUnits"`
}

func (a *API) taskRoutes() []Route {
	out := []Route{}
	add := func(m, path, kind, perm, summary string, body any, h endpoint) {
		out = append(out, Route{m, path, kind, perm, summary, body, h})
	}
	add("GET", "/api/v1/job-capabilities", "user", "", "已启用服务商能力（无密钥与地址）", nil, func(w http.ResponseWriter, r *http.Request, p identity.Principal, rid string) (any, error) {
		providers, e := a.Tasks.Providers(r.Context())
		if e != nil {
			return nil, e
		}
		items := []any{}
		for _, v := range providers {
			if v.Status == "active" && (v.Kind != "image" || v.HasKey) {
				items = append(items, map[string]any{"id": v.ID, "name": v.Name, "kind": v.Kind, "model": v.Model})
			}
		}
		return map[string]any{"providers": items, "billingEnabled": false}, nil
	})
	add("GET", "/api/v1/task-quota", "user", "", "我的内部任务额度", nil, func(w http.ResponseWriter, r *http.Request, p identity.Principal, rid string) (any, error) {
		return a.Tasks.Quota(r.Context(), p.ID)
	})
	add("POST", "/api/v1/jobs", "user", "", "创建 AI 图片或批量导出任务；需要幂等键", task.CreateRequest{}, withBody(func(r *http.Request, p identity.Principal, rid string, v task.CreateRequest) (any, error) {
		return a.Tasks.Create(r.Context(), p.ID, v, r.Header.Get("Idempotency-Key"), rid)
	}))
	for _, admin := range []bool{false, true} {
		admin := admin
		base, kind, perm := "/api/v1", "user", ""
		if admin {
			base, kind, perm = "/api/admin/v1", "admin", "job:read"
		}
		add("GET", base+"/jobs", kind, perm, "任务分页；管理员仅元信息", nil, func(w http.ResponseWriter, r *http.Request, p identity.Principal, rid string) (any, error) {
			pg, e := page(r)
			if e != nil {
				return nil, e
			}
			return a.Tasks.List(r.Context(), p.ID, admin, pg)
		})
		add("GET", base+"/jobs/:id", kind, perm, "任务详情；用户含输出素材，管理员不含私有内容", nil, withID(func(r *http.Request, p identity.Principal, rid string, id int64) (any, error) {
			return a.Tasks.Get(r.Context(), p.ID, id, admin)
		}))
		for _, action := range []string{"cancel", "retry"} {
			action := action
			write := ""
			if admin {
				write = "job:write"
			}
			add("POST", base+"/jobs/:id/"+action, kind, write, "任务操作："+action, nil, withID(func(r *http.Request, p identity.Principal, rid string, id int64) (any, error) {
				return nil, a.Tasks.Action(r.Context(), p.ID, id, admin, action, rid)
			}))
		}
	}
	add("DELETE", "/api/v1/jobs/:id", "user", "", "删除已结束任务并释放快照；输出保留在素材库", nil, withID(func(r *http.Request, p identity.Principal, rid string, id int64) (any, error) {
		return nil, a.Tasks.Action(r.Context(), p.ID, id, false, "delete", rid)
	}))
	add("GET", "/api/admin/v1/jobs/:id/attempts", "admin", "job:read", "任务执行与重试记录", nil, withID(func(r *http.Request, p identity.Principal, rid string, id int64) (any, error) {
		if _, e := a.Tasks.Get(r.Context(), p.ID, id, true); e != nil {
			return nil, e
		}
		return a.Tasks.Attempts(r.Context(), id)
	}))
	add("GET", "/api/admin/v1/providers", "admin", "provider:write", "服务商配置；密钥仅返回 hasKey", nil, func(w http.ResponseWriter, r *http.Request, p identity.Principal, rid string) (any, error) {
		return a.Tasks.Providers(r.Context())
	})
	add("POST", "/api/admin/v1/providers", "admin", "provider:write", "创建服务商配置；apiKey 只写并加密存储", task.ProviderInput{}, withBody(func(r *http.Request, p identity.Principal, rid string, v task.ProviderInput) (any, error) {
		return a.Tasks.ProviderSave(r.Context(), p.ID, 0, v, rid)
	}))
	add("PUT", "/api/admin/v1/providers/:id", "admin", "provider:write", "按版本更新服务商及启停；空密钥保持原值", task.ProviderInput{}, withIDBody(func(r *http.Request, p identity.Principal, rid string, id int64, v task.ProviderInput) (any, error) {
		return a.Tasks.ProviderSave(r.Context(), p.ID, id, v, rid)
	}))
	add("GET", "/api/admin/v1/users/:id/task-quota", "admin", "job:read", "用户任务额度", nil, withID(func(r *http.Request, p identity.Principal, rid string, id int64) (any, error) {
		if _, e := a.Identity.AccountDetail(r.Context(), "user", id); e != nil {
			return nil, e
		}
		return a.Tasks.Quota(r.Context(), id)
	}))
	add("PUT", "/api/admin/v1/users/:id/task-quota", "admin", "job:write", "分配累计任务额度；不含会员支付", QuotaRequest{}, withIDBody(func(r *http.Request, p identity.Principal, rid string, id int64, v QuotaRequest) (any, error) {
		return nil, a.Tasks.SetQuota(r.Context(), p.ID, id, v.LimitUnits, rid)
	}))
	add("GET", "/api/v1/jobs/:id/download", "user", "", "下载成功批量任务的 ZIP 文件", nil, func(w http.ResponseWriter, r *http.Request, p identity.Principal, rid string) (any, error) {
		id, e := id(r)
		if e != nil {
			return nil, e
		}
		j, e := a.Tasks.Get(r.Context(), p.ID, id, false)
		if e != nil {
			return nil, e
		}
		if j.Kind != "batch-export" || j.Status != "succeeded" || len(j.Results) == 0 {
			return nil, platform.Fail(409, "批量导出尚未成功")
		}
		files := []*os.File{}
		defer func() {
			for _, f := range files {
				f.Close()
			}
		}()
		for _, v := range j.Results {
			n, e := platform.ParseID(v.ID)
			if e != nil {
				return nil, e
			}
			_, f, e := a.Assets.Open(r.Context(), n, p.ID, false)
			if e != nil {
				return nil, e
			}
			files = append(files, f)
		}
		w.Header().Set("Content-Type", "application/zip")
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="design-job-%d.zip"`, id))
		archive := zip.NewWriter(w)
		for i, f := range files {
			entry, e := archive.Create(j.Results[i].Name)
			if e != nil {
				return rawResponse{}, nil
			}
			if _, e = io.Copy(entry, f); e != nil {
				return rawResponse{}, nil
			}
		}
		archive.Close()
		return rawResponse{}, nil
	})
	return out
}
