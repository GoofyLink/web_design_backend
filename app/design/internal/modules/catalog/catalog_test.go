package catalog

import (
	"encoding/json"
	"testing"
)

func TestPublishValidation(t *testing.T) {
	valid := Content{Title: "Demo", CategoryID: "1", Tags: []string{}, Width: 600, Height: 800, Cover: "/api/v1/seed-assets/poster.svg", Engine: "fabric", EngineVersion: "6", SchemaVersion: 1, Document: json.RawMessage(`{"objects":[{"type":"text","text":"Hello","fontFamily":"sans-serif"}]}`)}
	if e := ValidateContent(valid, true); e != nil {
		t.Fatal(e)
	}
	cases := []struct {
		name   string
		change func(*Content)
	}{
		{"root resource", func(c *Content) {
			c.Document = json.RawMessage(`{"backgroundImage":{"src":"http://127.0.0.1"},"objects":[{"type":"rect"}]}`)
		}},
		{"missing document", func(c *Content) { c.Document = nil }},
		{"remote image", func(c *Content) {
			c.Document = json.RawMessage(`{"objects":[{"type":"image","src":"http://127.0.0.1/admin"}]}`)
		}},
		{"nested resource", func(c *Content) {
			c.Document = json.RawMessage(`{"objects":[{"type":"rect","fill":{"source":"http://example.com"}}]}`)
		}},
		{"unknown font", func(c *Content) {
			c.Document = json.RawMessage(`{"objects":[{"type":"text","fontFamily":"commercial-font"}]}`)
		}},
		{"pixel limit", func(c *Content) { c.Width = 8192; c.Height = 8192 }},
		{"invalid cover", func(c *Content) { c.Cover = "https://example.com/cover.jpg" }},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			c := valid
			tt.change(&c)
			if ValidateContent(c, true) == nil {
				t.Fatal("invalid document accepted")
			}
		})
	}
}
