package transport

import (
	"bytes"
	"encoding/json"
	"html/template"
	"net/http"
	"strings"
)

var docsPage = template.Must(template.New("docs").Parse(`<!doctype html>
<html lang="zh-CN"><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>设计平台 · 接口文档</title><style>
body{margin:0;background:#f6f8fb;color:#182438;font:15px/1.6 system-ui,sans-serif}main{max-width:1100px;margin:auto;padding:28px}h1{margin:0;font-size:26px}p{color:#526176}a{color:#2563eb}form{display:flex;gap:12px;margin:24px 0}input{flex:1;min-width:0;padding:12px;border:1px solid #ccd5e2;border-radius:6px}button{padding:8px 20px;background:#245edb;color:white;border:0;border-radius:6px;cursor:pointer}details{background:white;border:1px solid #dce3ec;border-radius:6px;margin:10px 0;padding:14px}summary{cursor:pointer;overflow-wrap:anywhere}b{display:inline-block;min-width:65px;color:#245edb}code,pre{font-family:ui-monospace,monospace}pre{padding:16px;background:#f6f8fb;overflow:auto;font-size:13px}small{color:#526176}a:focus-visible,summary:focus-visible{outline:2px solid #245edb}
</style><main><h1>设计平台 · 接口文档</h1>
<p>共 {{.Total}} 个接口 · 展示 {{len .Entries}} 个。展开查看参数、请求体、响应及权限。此页面只用于查阅。</p>
<a href="/openapi.json" target="_blank" rel="noopener">打开 OpenAPI JSON</a>
<form action="/docs" method="get"><label for="q" hidden>搜索接口</label><input id="q" name="q" value="{{.Query}}" placeholder="搜索接口路径、名称或请求方法"><button>搜索</button><a href="/docs">重置</a></form>
{{range .Entries}}<details><summary><b>{{.Method}}</b> <code>{{.Path}}</code> — {{.Summary}}</summary><pre>{{.Spec}}</pre></details>{{else}}<p>未找到匹配的接口。</p>{{end}}
<small>接口定义由后端路由自动生成。鉴权仍由每个业务接口独立校验。</small></main></html>`))

func (a *API) serveDocs(w http.ResponseWriter, r *http.Request) bool {
	if r.URL.Path == "/" && a.Config.SiteDir == "" {
		http.Redirect(w, r, "/docs", http.StatusFound)
		return true
	}
	if r.URL.Path != "/docs" && r.URL.Path != "/docs/" && r.URL.Path != "/openapi.json" {
		return false
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	spec := a.OpenAPI()
	// Relative server URL also works behind a same-origin reverse proxy.
	spec["servers"] = []any{map[string]any{"url": "/"}}
	var body []byte
	if r.URL.Path == "/openapi.json" {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		body, _ = json.MarshalIndent(spec, "", "  ")
	} else {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		type entry struct{ Method, Path, Summary, Spec string }
		entries := []entry{}
		query := strings.TrimSpace(r.URL.Query().Get("q"))
		paths := spec["paths"].(map[string]any)
		routes := a.Routes()
		for _, route := range routes {
			path := apiPath(route.Path)
			if !strings.Contains(strings.ToLower(route.Method+" "+path+" "+route.Summary), strings.ToLower(query)) {
				continue
			}
			definition := paths[path].(map[string]any)[strings.ToLower(route.Method)]
			data, _ := json.MarshalIndent(definition, "", "  ")
			entries = append(entries, entry{route.Method, path, route.Summary, string(data)})
		}
		var buf bytes.Buffer
		if err := docsPage.Execute(&buf, struct {
			Total   int
			Query   string
			Entries []entry
		}{len(routes), query, entries}); err != nil {
			http.Error(w, "文档生成失败", http.StatusInternalServerError)
			return true
		}
		body = buf.Bytes()
	}
	if r.Method != http.MethodHead {
		_, _ = w.Write(body)
	}
	return true
}
