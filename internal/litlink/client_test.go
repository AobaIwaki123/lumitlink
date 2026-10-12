package litlink

import (
	"encoding/json"
	"testing"
)

func testRSCHTML(t *testing.T, profile map[string]any) []byte {
	t.Helper()
	payload, err := json.Marshal(map[string]any{"profile": profile})
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(string(payload))
	if err != nil {
		t.Fatal(err)
	}
	return []byte("<html><head><title>テストプロフィール</title><meta name=\"description\" content=\"テスト\"></head><body><script>self.__next_f.push([1," + string(encoded) + "])</script></body></html>")
}

func TestParseExtractsProfileAndStructuredLinks(t *testing.T) {
	body := testRSCHTML(t, map[string]any{
		"name":        "雨宮 サク",
		"catchphrase": "アイドル",
		"profileText": "東京拠点\nアイドル",
		"urlPath":     "amamiyasaku",
		"pictureUrl":  "https://cdn.example/icon.jpg",
		"profileLinks": []any{
			map[string]any{
				"id":       "social-block",
				"linkType": "image",
				"imageLink": map[string]any{
					"profileImages": []any{map[string]any{
						"title":    "X",
						"url":      "https://x.com/example",
						"urlType":  "x",
						"imageUrl": "https://cdn.example/x.png",
					}},
				},
			},
			map[string]any{
				"id":       "button-block",
				"linkType": "button",
				"buttonLink": map[string]any{
					"title":       "公式サイト",
					"description": "見てね",
					"url":         "https://example.com",
					"urlType":     "others",
				},
			},
			map[string]any{
				"id":       "text-block",
				"linkType": "text",
				"textLink": map[string]any{
					"title":       "お知らせ",
					"description": "<p>更新しました</p><p>よろしくお願いします</p>",
				},
			},
		},
	})
	result, err := Parse(body, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := result.Profile.Name; got != "雨宮 サク" {
		t.Fatalf("name = %v, want 雨宮 サク", got)
	}
	if got := result.Profile.URLPath; got != "amamiyasaku" {
		t.Fatalf("url path = %v, want amamiyasaku", got)
	}
	if got := len(result.Profile.Links); got != 3 {
		t.Fatalf("links = %d, want 3", got)
	}
	if got := result.Profile.Links[0].URL; got != "https://x.com/example" {
		t.Fatalf("first URL = %v", got)
	}
	if got := result.Profile.Links[2].DescriptionText; got != "更新しました よろしくお願いします" {
		t.Fatalf("description text = %v", got)
	}
}

func TestParseFallsBackToAnchorLinks(t *testing.T) {
	body := []byte(`<html><head><meta property="og:title" content="Fallback"></head><body><a href="https://example.com">Example</a><a href="https://lit.link/other">Internal</a></body></html>`)
	result, err := Parse(body, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.PageTitle != nil {
		t.Fatalf("page title = %v, want nil", result.PageTitle)
	}
	if result.Metadata["og:title"] != "Fallback" {
		t.Fatalf("og:title = %q", result.Metadata["og:title"])
	}
	if len(result.Profile.Links) != 1 || result.Profile.Links[0].Title != "Example" {
		t.Fatalf("fallback links = %#v", result.Profile.Links)
	}
}

func TestValidateUsername(t *testing.T) {
	if _, err := ValidateUsername("amamiyasaku"); err != nil {
		t.Fatal(err)
	}
	if _, err := ValidateUsername("../private"); err == nil {
		t.Fatal("expected invalid username")
	}
}

func TestToEntityProjectsIconAndSNS(t *testing.T) {
	response := Response{
		Profile: Profile{
			Name:       "雨宮サク",
			PictureURL: "https://cdn.example/icon.jpg",
			Links: []Link{
				{Type: "image", Title: "X", URL: "https://x.com/amamiyasaku", URLType: "x"},
				{Title: "公式サイト", URL: "https://example.com"},
			},
		},
		Source: Source{URL: "https://lit.link/amamiyasaku"},
	}
	record := ToEntity("amamiyasaku", response)
	if record.SchemaVersion != "lumi.entity.v1" {
		t.Fatalf("schema version = %q", record.SchemaVersion)
	}
	if record.Kind != "idol" || record.Name != "雨宮サク" {
		t.Fatalf("record identity = %#v", record)
	}
	if len(record.Assets) != 1 || record.Assets[0].Kind != "icon" {
		t.Fatalf("assets = %#v", record.Assets)
	}
	if len(record.SocialLinks) != 1 || record.SocialLinks[0].Platform != "x" {
		t.Fatalf("social links = %#v", record.SocialLinks)
	}
}
