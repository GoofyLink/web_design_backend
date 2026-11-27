package canvas

import (
	"encoding/json"
	"os"
	"testing"
)

func TestFabric6SerializedDocument(t *testing.T) {
	raw, e := os.ReadFile("testdata/fabric-6.7.1.json")
	if e != nil {
		t.Fatal(e)
	}
	c := Content{Width: 600, Height: 800, Engine: "fabric", EngineVersion: "6.7.1", SchemaVersion: 1, Document: raw}
	if _, e = Validate(c); e != nil {
		t.Fatal(e)
	}
	encoded, e := json.Marshal(c)
	if e != nil {
		t.Fatal(e)
	}
	var decoded Content
	if e = json.Unmarshal(encoded, &decoded); e != nil {
		t.Fatal(e)
	}
	var before, after any
	json.Unmarshal(c.Document, &before)
	json.Unmarshal(decoded.Document, &after)
	b, _ := json.Marshal(before)
	a, _ := json.Marshal(after)
	if string(a) != string(b) {
		t.Fatal("roundtrip changed document")
	}
}
func TestDocumentBoundaries(t *testing.T) {
	for _, raw := range []string{`null`, `{"objects":null}`, `{"objects":[null]}`, `{"objects":[{"type":"image","src":"https://example.com"}]}`, `{"objects":[{"type":"rect","fill":{"type":"pattern","source":"x"}}]}`, `{"objects":[{"type":"text","fontFamily":"unknown"}]}`, `{"objects":[{"type":"image","assetId":"1"},{"type":"text","fontAssetId":"1","fontFamily":"asset-font-1"}]}`, `{"objects":[{"type":"rect","clipPath":{"type":"image","assetId":"1","src":"data:image/png;base64,xxx"}}]}`} {
		c := Content{Width: 100, Height: 100, Engine: "fabric", EngineVersion: "6", SchemaVersion: 1, Document: json.RawMessage(raw)}
		if _, e := Validate(c); e == nil {
			t.Fatalf("unsafe document accepted: %s", raw)
		}
	}
}

func TestImageColorAdjustments(t *testing.T) {
	for _, tc := range []struct {
		raw   string
		valid bool
	}{
		{`{"objects":[{"type":"Image","assetId":"1","filters":[{"type":"Brightness","brightness":0.2},{"type":"Contrast","contrast":-0.1},{"type":"Saturation","saturation":1}]}]}`, true},
		{`{"objects":[{"type":"Image","assetId":"1","filters":[]}]}`, true},
		{`{"objects":[{"type":"Brightness","brightness":0.2}]}`, false},
		{`{"objects":[{"type":"Rect","filters":[]}]}`, false},
		{`{"objects":[{"type":"Image","assetId":"1","filters":[{"type":"Brightness","brightness":2}]}]}`, false},
		{`{"objects":[{"type":"Image","assetId":"1","filters":[{"type":"Brightness","brightness":0,"src":"https://example.com"}]}]}`, false},
		{`{"objects":[{"type":"Image","assetId":"1","filters":[{"type":"Brightness","brightness":0},{"type":"Brightness","brightness":1}]}]}`, false},
		{`{"objects":[{"type":"Image","assetId":"1","filters":[{"type":"BlendImage","image":{}}]}]}`, false},
	} {
		c := Content{Width: 100, Height: 100, Engine: "fabric", EngineVersion: "6", SchemaVersion: 1, Document: json.RawMessage(tc.raw)}
		_, err := Validate(c)
		if (err == nil) != tc.valid {
			t.Fatalf("valid=%v error=%v document=%s", tc.valid, err, tc.raw)
		}
	}
}

func TestShadowEffects(t *testing.T) {
	for _, tc := range []struct {
		raw   string
		valid bool
	}{
		{`{"objects":[{"type":"Rect","stroke":"#ff0000","strokeWidth":6,"shadow":{"type":"shadow","color":"#000000","blur":20,"offsetX":4,"offsetY":-4,"affectStroke":false,"nonScaling":false}}]}`, true},
		{`{"objects":[{"type":"Rect","shadow":null}]}`, true},
		{`{"objects":[{"type":"shadow"}]}`, false},
		{`{"objects":[{"type":"Rect","shadow":{"type":"shadow","blur":201}}]}`, false},
		{`{"objects":[{"type":"Rect","shadow":{"type":"shadow","blur":-1}}]}`, false},
		{`{"objects":[{"type":"Rect","shadow":{"color":"url(https://example.com)"}}]}`, false},
		{`{"objects":[{"type":"Rect","shadow":{"src":"https://example.com"}}]}`, false},
	} {
		c := Content{Width: 100, Height: 100, Engine: "fabric", EngineVersion: "6", SchemaVersion: 1, Document: json.RawMessage(tc.raw)}
		_, err := Validate(c)
		if (err == nil) != tc.valid {
			t.Fatalf("valid=%v err=%v document=%s", tc.valid, err, tc.raw)
		}
	}
}
