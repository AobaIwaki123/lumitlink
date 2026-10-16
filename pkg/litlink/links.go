package litlink

import (
	"net/url"
	"regexp"
	"strings"

	"github.com/AobaIwaki123/lumi-spec/event"
)

var eventURLPattern = regexp.MustCompile(`https?://[^\s<>"'。、！？（）「」『』【】]+`)

// extractEventLinks collects URLs attached to a Lit.link event button. The
// profile button URL and URLs written in its description are kept separately
// from SourceRef.URL so callers can distinguish source identity from useful
// destinations such as ticket pages.
func extractEventLinks(link Link, sourceURL string) []event.Link {
	links := make([]event.Link, 0, 1)
	seen := make(map[string]struct{})
	add := func(rawURL, label, origin string) {
		rawURL = cleanURL(rawURL)
		if rawURL == "" || rawURL == sourceURL {
			return
		}
		parsed, err := url.Parse(rawURL)
		if err != nil || parsed.Scheme != "http" && parsed.Scheme != "https" || parsed.Hostname() == "" {
			return
		}
		key := strings.ToLower(rawURL)
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		links = append(links, event.Link{
			Kind:   classifyEventLink(rawURL, label),
			URL:    rawURL,
			Label:  strings.TrimSpace(label),
			Origin: origin,
		})
	}

	label := stringValue(link.Title)
	if urlType := stringValue(link.URLType); urlType != "" {
		label = strings.TrimSpace(label + " " + urlType)
	}
	add(stringValue(link.URL), label, "button")
	description := stringValue(link.DescriptionText)
	for _, rawURL := range eventURLPattern.FindAllString(description, -1) {
		add(rawURL, descriptionContext(description, rawURL), "description")
	}
	return links
}

func descriptionContext(description, rawURL string) string {
	index := strings.Index(description, rawURL)
	if index < 0 {
		return ""
	}
	start := index - 20
	if start < 0 {
		start = 0
	}
	return strings.TrimSpace(description[start:index])
}

func cleanURL(rawURL string) string {
	return strings.TrimRight(strings.TrimSpace(rawURL), ".,!?;:。、)]}〉》」』】")
}

func classifyEventLink(rawURL, label string) string {
	parsed, _ := url.Parse(rawURL)
	host := strings.ToLower(strings.TrimPrefix(parsed.Hostname(), "www."))
	text := strings.ToLower(label + " " + rawURL)
	if strings.Contains(text, "ticket") || strings.Contains(text, "チケット") || strings.Contains(text, "受付") || strings.Contains(text, "販売") || strings.Contains(text, "先行") || strings.Contains(text, "一般発売") {
		return event.LinkKindTicket
	}
	switch host {
	case "ticketdive.com", "t-dv.com", "tiget.net", "eplus.jp", "l-tike.com", "livepocket.jp", "passmarket.yahoo.co.jp", "ticketpay.jp", "teket.jp":
		return event.LinkKindTicket
	case "x.com", "twitter.com", "instagram.com", "tiktok.com", "youtube.com", "youtu.be", "twitch.tv":
		return event.LinkKindSocial
	}
	if strings.Contains(text, "公式") || strings.Contains(text, "official") || strings.Contains(text, "ホームページ") || strings.Contains(text, "website") {
		return event.LinkKindOfficial
	}
	return event.LinkKindRelated
}
