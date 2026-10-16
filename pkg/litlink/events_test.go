package litlink

import (
	"testing"
	"time"
)

func TestToEventFeedExtractsDatePrefixedButtons(t *testing.T) {
	now := time.Date(2026, time.October, 12, 12, 0, 0, 0, time.FixedZone("Asia/Tokyo", 9*60*60))
	feed := ToEventFeedAt("amamiyasaku", Response{
		Profile: Profile{
			Name: "雨宮 サク",
			Links: []Link{
				{Type: "button", ID: "photo", Title: "10/18(日)撮影会📸"},
				{Type: "button", ID: "live", Title: "10/22(木) プリュきたる", DescriptionText: "📍渋谷WOMB 🎤19:55-20:15 📷20:40-21:50（終演後）", URL: "https://example.test/ticket"},
				{Type: "button", ID: "not-event", Title: "公式サイト", URL: "https://example.test"},
				{Type: "image", ID: "social", Title: "X", URL: "https://x.com/example"},
			},
		},
		Source: Source{URL: "https://lit.link/amamiyasaku"},
	}, now)

	if len(feed.Events) != 2 {
		t.Fatalf("events = %d, want 2", len(feed.Events))
	}
	if feed.Events[0].LocalDate != "2026-10-18" || feed.Events[0].Title != "撮影会📸" {
		t.Fatalf("photo event = %#v", feed.Events[0])
	}
	if feed.Events[1].Venue != "渋谷WOMB" || feed.Events[1].StartsAt != "2026-10-22T19:55:00+09:00" {
		t.Fatalf("live event = %#v", feed.Events[1])
	}
	if len(feed.Events[1].Links) != 1 || feed.Events[1].Links[0].Kind != "ticket" || feed.Events[1].Links[0].URL != "https://example.test/ticket" {
		t.Fatalf("event links = %#v", feed.Events[1].Links)
	}
	activities, ok := feed.Events[1].Extensions["activities"].([]map[string]any)
	if !ok || len(activities) != 2 || activities[1]["kind"] != "photo_session" {
		t.Fatalf("activities = %#v", feed.Events[1].Extensions["activities"])
	}
}

func TestToEventFeedExtractsDescriptionURLs(t *testing.T) {
	now := time.Date(2026, time.October, 12, 12, 0, 0, 0, time.FixedZone("Asia/Tokyo", 9*60*60))
	feed := ToEventFeedAt("example", Response{
		Profile: Profile{Links: []Link{{
			Type:            "button",
			Title:           "10/18(日)ライブ",
			DescriptionText: "🎫 チケット https://tiget.net/events/123。公式 https://example.com/info。",
		}}},
		Source: Source{URL: "https://lit.link/example"},
	}, now)
	if len(feed.Events) != 1 || len(feed.Events[0].Links) != 2 {
		t.Fatalf("events = %#v", feed.Events)
	}
	if feed.Events[0].Links[0].Kind != "ticket" || feed.Events[0].Links[1].Kind != "official" {
		t.Fatalf("links = %#v", feed.Events[0].Links)
	}
}

func TestToEntityOmitsXPostsAndKeepsProfileAccounts(t *testing.T) {
	record := ToEntity("amamiyasaku", Response{Profile: Profile{Links: []Link{
		{Type: "image", Title: "X", URL: "https://x.com/rookey_sakusabu", URLType: "x"},
		{Type: "button", Title: "イベント", URL: "https://x.com/rookeyrookeys/status/123", URLType: "others"},
		{Type: "button", Title: "ダイジェスト", URL: "https://x.com/rookeyrookeys/status/456", URLType: "others"},
	}}})
	if len(record.SocialLinks) != 1 || record.SocialLinks[0].URL != "https://x.com/rookey_sakusabu" {
		t.Fatalf("social links = %#v", record.SocialLinks)
	}
}
