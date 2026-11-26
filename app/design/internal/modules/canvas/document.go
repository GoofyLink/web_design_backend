// Package canvas validates asset-ID based Fabric documents without fetching URLs.
package canvas

import (
	"encoding/json"
	"fmt"
	"looklook/app/design/internal/platform"
	"strconv"
	"strings"
)

type Reference struct {
	ID   int64
	Kind string
}
type Content struct {
	Width         int             `json:"width"`
	Height        int             `json:"height"`
	Engine        string          `json:"engine"`
	EngineVersion string          `json:"engineVersion"`
	SchemaVersion int             `json:"schemaVersion"`
	Document      json.RawMessage `json:"document"`
	CoverAssetID  string          `json:"coverAssetId,omitempty"`
}

func Validate(c Content) ([]Reference, error) {
	if c.Width < 1 || c.Height < 1 || c.Width > 8192 || c.Height > 8192 || int64(c.Width)*int64(c.Height) > 40000000 {
		return nil, platform.Fail(400, "画布尺寸超限")
	}
	if c.Engine != "fabric" || c.EngineVersion == "" || len(c.EngineVersion) > 40 || c.SchemaVersion != 1 {
		return nil, platform.Fail(400, "仅支持带引擎版本的 fabric schemaVersion=1 文档")
	}
	if len(c.Document) > 5*1024*1024 {
		return nil, platform.Fail(413, "文档超过 5 MiB")
	}
	var root map[string]any
	if json.Unmarshal(c.Document, &root) != nil || root == nil {
		return nil, platform.Fail(400, "文档必须是 JSON 对象")
	}
	objects, ok := root["objects"].([]any)
	if !ok {
		return nil, platform.Fail(400, "文档必须包含 objects 数组")
	}
	refs := map[int64]string{}
	count := 0
	var walk func(any, int) error
	walk = func(value any, depth int) error {
		if depth > 32 {
			return platform.Fail(400, "文档嵌套过深")
		}
		switch v := value.(type) {
		case map[string]any:
			typ, _ := v["type"].(string)
			typ = strings.ToLower(typ)
			if typ != "" {
				switch typ {
				case "rect", "circle", "ellipse", "triangle", "line", "path", "polygon", "polyline", "text", "textbox", "i-text", "itext", "image", "group":
					count++
					if count > 2000 {
						return platform.Fail(400, "对象超过 2000 个")
					}
				case "linear", "radial":
				case "layoutmanager":
					if v["strategy"] != "fit-content" && v["strategy"] != "fixed" && v["strategy"] != "clip-path" {
						return platform.Fail(400, "布局策略无效")
					}
				case "pattern":
					return platform.Fail(400, "暂不支持 Pattern 图片填充，请使用图片对象")
				default:
					return platform.Fail(400, "未支持的画布对象类型: "+typ)
				}
			}
			if typ == "image" {
				raw, ok := v["assetId"].(string)
				if !ok {
					return platform.Fail(400, "图片对象须使用 assetId")
				}
				id, e := platform.ParseID(raw)
				if e != nil {
					return e
				}
				if previous, ok := refs[id]; ok && previous != "image" {
					return platform.Fail(400, "资源类型冲突")
				}
				refs[id] = "image"
				if src, exists := v["src"]; exists && src != "" && src != "asset:"+raw {
					return platform.Fail(400, "src 只能为空或 asset:ID")
				}
			}
			if raw, exists := v["fontAssetId"]; exists {
				str, ok := raw.(string)
				if !ok {
					return platform.Fail(400, "fontAssetId 必须为字符串")
				}
				id, e := platform.ParseID(str)
				if e != nil {
					return e
				}
				if previous, ok := refs[id]; ok && previous != "font" {
					return platform.Fail(400, "资源类型冲突")
				}
				refs[id] = "font"
				if v["fontFamily"] != "asset-font-"+str {
					return platform.Fail(400, "上传字体的 fontFamily 应为 asset-font-ID")
				}
			}
			if family, exists := v["fontFamily"]; exists {
				f, ok := family.(string)
				if !ok {
					return platform.Fail(400, "fontFamily 无效")
				}
				if _, custom := v["fontAssetId"]; !custom {
					switch strings.ToLower(f) {
					case "sans-serif", "serif", "monospace", "arial", "times new roman", "courier new":
					default:
						return platform.Fail(400, "非系统字体须提供 fontAssetId")
					}
				}
			}
			for k, x := range v {
				lower := strings.ToLower(k)
				if lower == "__proto__" || lower == "constructor" || lower == "prototype" || lower == "source" || lower == "href" || strings.Contains(lower, "url") {
					return platform.Fail(400, "文档包含不支持的资源属性")
				}
				if k == "assetId" && typ != "image" {
					return platform.Fail(400, "assetId 仅用于图片对象")
				}
				if k == "src" {
					if typ != "image" {
						return platform.Fail(400, "src 仅用于资源图片")
					}
					continue
				}
				if k == "objects" {
					items, ok := x.([]any)
					if !ok {
						return platform.Fail(400, "objects 必须是数组")
					}
					for _, item := range items {
						m, ok := item.(map[string]any)
						if !ok {
							return platform.Fail(400, "objects 元素必须是对象")
						}
						typ, _ := m["type"].(string)
						if strings.TrimSpace(typ) == "" {
							return platform.Fail(400, "对象缺少 type")
						}
					}
				}
				if k == "text" {
					str, ok := x.(string)
					if !ok || len(str) > 100000 {
						return platform.Fail(400, "文字内容超限")
					}
					continue
				}
				if e := walk(x, depth+1); e != nil {
					return e
				}
			}
		case []any:
			if len(v) > 100000 {
				return platform.Fail(400, "数组超限")
			}
			for _, x := range v {
				if e := walk(x, depth+1); e != nil {
					return e
				}
			}
		case string:
			if len(v) > 100000 {
				return platform.Fail(400, "字符串超限")
			}
			lower := strings.ToLower(v)
			for _, prefix := range []string{"http:", "https:", "data:", "blob:", "file:", "javascript:", "url("} {
				if strings.Contains(lower, prefix) {
					return platform.Fail(400, "文档不能保存外链或临时地址")
				}
			}
		case float64:
			if v < -1e9 || v > 1e9 {
				return platform.Fail(400, "数值超限")
			}
		case nil, bool:
		default:
			return platform.Fail(400, "文档值无效")
		}
		return nil
	}
	for _, o := range objects {
		m, ok := o.(map[string]any)
		typ, _ := m["type"].(string)
		if !ok || strings.TrimSpace(typ) == "" {
			return nil, platform.Fail(400, "objects 元素须包含 type")
		}
	}
	if e := walk(root, 0); e != nil {
		return nil, e
	}
	if c.CoverAssetID != "" {
		id, e := platform.ParseID(c.CoverAssetID)
		if e != nil {
			return nil, e
		}
		if refs[id] == "font" {
			return nil, platform.Fail(400, "封面不能使用字体")
		}
		refs[id] = "image"
	}
	out := []Reference{}
	for id, kind := range refs {
		out = append(out, Reference{id, kind})
	}
	return out, nil
}
func AssetURL(id int64) string {
	return fmt.Sprintf("/api/v1/assets/%s/content", strconv.FormatInt(id, 10))
}
