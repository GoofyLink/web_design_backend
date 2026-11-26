package transport

import (
	"encoding/json"
	"looklook/app/design/internal/modules/operation"
	"testing"
)

func TestCanvasPresets(t *testing.T) {
	a, _ := contentTestAPI(t)
	c := testClient{t, a.Handler()}
	admin, _ := c.login("admin", "root_admin", "Strong-test-123")
	var initial operation.CanvasPresets
	if e := json.Unmarshal(c.expect(200, "GET", "/api/v1/canvas-presets", "", nil), &initial); e != nil {
		t.Fatal(e)
	}
	if len(initial.Items) != 6 {
		t.Fatal("missing defaults")
	}
	input := operation.CanvasPresets{Version: initial.Version, Items: []operation.CanvasPreset{{ID: "custom", Name: "公司海报", Category: "海报", Width: 1200, Height: 1600, Enabled: true}, {ID: "hidden", Name: "隐藏", Width: 300, Height: 300, Enabled: false}}}
	c.expect(401, "PUT", "/api/admin/v1/canvas-presets", "", input)
	c.expect(200, "PUT", "/api/admin/v1/canvas-presets", admin, input)
	c.expect(409, "PUT", "/api/admin/v1/canvas-presets", admin, input)
	var public operation.CanvasPresets
	json.Unmarshal(c.expect(200, "GET", "/api/v1/canvas-presets", "", nil), &public)
	if len(public.Items) != 1 || public.Items[0].Name != "公司海报" {
		t.Fatal("enabled filter failed")
	}
	input.Version = public.Version
	input.Items[0].Width = 8193
	c.expect(400, "PUT", "/api/admin/v1/canvas-presets", admin, input)
	input.Items[0].Width = 8000
	input.Items[0].Height = 8000
	c.expect(400, "PUT", "/api/admin/v1/canvas-presets", admin, input)
	input.Items = []operation.CanvasPreset{}
	c.expect(200, "PUT", "/api/admin/v1/canvas-presets", admin, input)
}
