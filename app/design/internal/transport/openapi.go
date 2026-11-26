package transport

import (
	"encoding/json"
	"reflect"
	"strings"
)

func schema(t reflect.Type) map[string]any {
	if t == reflect.TypeOf(json.RawMessage{}) {
		return map[string]any{"type": "object", "additionalProperties": true}
	}
	switch t.Kind() {
	case reflect.String:
		return map[string]any{"type": "string"}
	case reflect.Int, reflect.Int64:
		return map[string]any{"type": "integer", "format": "int64"}
	case reflect.Bool:
		return map[string]any{"type": "boolean"}
	case reflect.Slice:
		return map[string]any{"type": "array", "items": schema(t.Elem())}
	case reflect.Struct:
		properties := map[string]any{}
		required := []string{}
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			tag := strings.Split(f.Tag.Get("json"), ",")
			name := tag[0]
			if name == "-" {
				continue
			}
			if name == "" {
				name = f.Name
			}
			properties[name] = schema(f.Type)
			if name == "apiKey" {
				properties[name].(map[string]any)["writeOnly"] = true
				properties[name].(map[string]any)["format"] = "password"
			}
			if len(tag) == 1 && (name != "version" || t.Name() != "TemplateRequest") {
				required = append(required, name)
			}
		}
		return map[string]any{"type": "object", "properties": properties, "required": required, "additionalProperties": false}
	}
	return map[string]any{}
}

// Generated directly from the runtime route registry to prevent missing endpoints.
func (a *API) OpenAPI() map[string]any {
	paths := map[string]any{}
	for _, r := range a.Routes() {
		path := apiPath(r.Path)
		entry, ok := paths[path].(map[string]any)
		if !ok {
			entry = map[string]any{}
			paths[path] = entry
		}
		response := map[string]any{"type": "object", "required": []string{"code", "message", "data", "requestId"}, "properties": map[string]any{"code": map[string]any{"type": "integer"}, "message": map[string]any{"type": "string"}, "data": map[string]any{"nullable": true}, "requestId": map[string]any{"type": "string"}}}
		op := map[string]any{"summary": r.Summary, "operationId": strings.ToLower(r.Method) + strings.NewReplacer("/", "_", ":", "").Replace(r.Path), "responses": map[string]any{"200": map[string]any{"description": "成功，code=0；详情响应见 API.md", "content": map[string]any{"application/json": map[string]any{"schema": response}}}, "default": map[string]any{"description": "400/401/403/404/409/413/429/500/503，统一错误结构", "content": map[string]any{"application/json": map[string]any{"schema": response}}}}}
		if r.Kind != "" {
			op["security"] = []any{map[string]any{"bearerAuth": []string{}}}
			op["x-audience"] = r.Kind
		}
		if r.Permission != "" {
			op["x-permission"] = r.Permission
		}
		params := []any{}
		for _, segment := range strings.Split(r.Path, "/") {
			if !strings.HasPrefix(segment, ":") {
				continue
			}
			name := strings.TrimPrefix(segment, ":")
			{
				params = append(params, map[string]any{"name": name, "in": "path", "required": true, "schema": map[string]any{"type": "string", "pattern": "^[1-9][0-9]*$"}})
			}
		}
		if r.Method == "GET" && (strings.HasSuffix(path, "/templates") || strings.HasSuffix(path, "/users") || strings.HasSuffix(path, "/admins") || strings.HasSuffix(path, "/audit-logs") || strings.HasSuffix(path, "/assets") || strings.HasSuffix(path, "/fonts") || strings.HasSuffix(path, "/designs") || strings.HasSuffix(path, "/trash") || strings.HasSuffix(path, "/revisions") || strings.HasSuffix(path, "/jobs") || strings.HasSuffix(path, "/export-events") || strings.HasSuffix(path, "/favorites") || strings.HasSuffix(path, "/topics")) {
			for _, name := range []string{"page", "pageSize", "keyword", "status"} {
				params = append(params, map[string]any{"name": name, "in": "query", "schema": map[string]any{"type": "string"}})
			}
			if strings.HasSuffix(path, "/templates") {
				for _, name := range []string{"categoryId", "sort", "ratio"} {
					params = append(params, map[string]any{"name": name, "in": "query", "schema": map[string]any{"type": "string"}})
				}
			}
		}
		if strings.HasSuffix(path, "/auth/refresh") || strings.HasSuffix(path, "/auth/logout") {
			params = append(params, map[string]any{"name": "Origin", "in": "header", "required": true, "schema": map[string]any{"type": "string"}})
			op["description"] = "同时携带对应 design_user_refresh 或 design_admin_refresh HttpOnly Cookie；Cookie 由登录返回。"
		}
		if r.Method == "POST" && (strings.HasSuffix(path, "/designs") || strings.HasSuffix(path, "/copy") || strings.HasSuffix(path, "/use") || strings.HasSuffix(path, "/jobs")) {
			params = append(params, map[string]any{"name": "Idempotency-Key", "in": "header", "required": true, "schema": map[string]any{"type": "string", "minLength": 8, "maxLength": 100}})
		}
		if strings.HasSuffix(path, "/assets") && r.Method == "GET" {
			for _, name := range []string{"scope", "kind"} {
				params = append(params, map[string]any{"name": name, "in": "query", "schema": map[string]any{"type": "string"}})
			}
		}
		if strings.HasSuffix(path, "/statistics") {
			for _, name := range []string{"from", "to"} {
				params = append(params, map[string]any{"name": name, "in": "query", "schema": map[string]any{"type": "string", "format": "date-time"}})
			}
		}
		if strings.HasSuffix(path, "/topics") && r.Method == "GET" {
			params = append(params, map[string]any{"name": "slot", "in": "query", "schema": map[string]any{"type": "string", "enum": []string{"home", "featured"}}})
		}
		if len(params) > 0 {
			op["parameters"] = params
		}
		if r.Body != nil {
			op["requestBody"] = map[string]any{"required": true, "content": map[string]any{"application/json": map[string]any{"schema": requestSchema(r.Method, reflect.TypeOf(r.Body))}}}
		}
		if _, ok := r.Body.(UploadRequest); ok {
			op["requestBody"] = map[string]any{"required": true, "content": map[string]any{"multipart/form-data": map[string]any{"schema": map[string]any{"type": "object", "required": []string{"file", "kind"}, "properties": map[string]any{"file": map[string]any{"type": "string", "format": "binary"}, "kind": map[string]any{"type": "string", "enum": []string{"image", "font"}}, "license": map[string]any{"type": "string"}}}}}}
		}
		if strings.HasSuffix(path, "/content") {
			op["responses"] = map[string]any{"200": map[string]any{"description": "二进制文件", "content": map[string]any{"application/octet-stream": map[string]any{"schema": map[string]any{"type": "string", "format": "binary"}}}}, "206": map[string]any{"description": "文件范围响应"}, "default": map[string]any{"description": "鉴权或资源错误，统一 JSON 错误结构"}}
			op["parameters"] = append(params, map[string]any{"name": "ticket", "in": "query", "schema": map[string]any{"type": "string"}})
		}
		if strings.HasSuffix(path, "/download") {
			op["responses"] = map[string]any{"200": map[string]any{"description": "ZIP 文件", "content": map[string]any{"application/zip": map[string]any{"schema": map[string]any{"type": "string", "format": "binary"}}}}, "default": map[string]any{"description": "统一 JSON 错误响应"}}
		}
		if strings.HasSuffix(path, ".svg") {
			op["responses"] = map[string]any{"200": map[string]any{"description": "内置 SVG 图片", "content": map[string]any{"image/svg+xml": map[string]any{"schema": map[string]any{"type": "string"}}}}}
		}
		entry[strings.ToLower(r.Method)] = op
	}
	return map[string]any{"openapi": "3.0.3", "info": map[string]any{"title": "Design API Internal B1-B6 B8", "version": "0.3.0"}, "servers": []any{map[string]any{"url": "http://127.0.0.1:8888"}}, "paths": paths, "components": map[string]any{"securitySchemes": map[string]any{"bearerAuth": map[string]any{"type": "http", "scheme": "bearer", "bearerFormat": "JWT"}}}}
}

func apiPath(path string) string {
	parts := strings.Split(path, "/")
	for i, p := range parts {
		if strings.HasPrefix(p, ":") {
			parts[i] = "{" + strings.TrimPrefix(p, ":") + "}"
		}
	}
	return strings.Join(parts, "/")
}

func requestSchema(method string, t reflect.Type) map[string]any {
	out := schema(t)
	if method == "POST" && (t.Name() == "FolderRequest" || t.Name() == "TopicInput" || t.Name() == "ProviderInput") {
		required, _ := out["required"].([]string)
		filtered := []string{}
		for _, v := range required {
			if v != "version" {
				filtered = append(filtered, v)
			}
		}
		out["required"] = filtered
	}
	return out
}
