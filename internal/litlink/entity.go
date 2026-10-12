package litlink

import (
	"net/url"
	"strings"

	"github.com/AobaIwaki123/lumi-spec/entity"
	"github.com/AobaIwaki123/lumi-spec/event"
)

// ToEntity maps the Lit.link profile response to the shared lumi entity
// contract. Lit.link-specific fields remain available from the profile API;
// this projection only exposes the stable cross-system core.
func ToEntity(username string, response Response) entity.Record {
	name := stringValue(response.Profile.Name)
	if name == "" {
		name = username
	}
	record := entity.Record{
		SchemaVersion: entity.SchemaVersion,
		ID:            username,
		Kind:          entity.KindIdol,
		Name:          name,
		SourceRefs: []event.SourceRef{{
			Source:   "lumitlink",
			RecordID: username,
			URL:      response.Source.URL,
		}},
	}

	if pictureURL := stringValue(response.Profile.PictureURL); pictureURL != "" {
		record.Assets = append(record.Assets, entity.Asset{
			Kind: entity.AssetIcon,
			URL:  pictureURL,
			Alt:  name,
		})
	}

	seenURLs := map[string]struct{}{}
	for _, link := range response.Profile.Links {
		linkURL := stringValue(link.URL)
		platform := socialPlatform(linkURL, stringValue(link.URLType))
		if platform == "" || !isCanonicalSocialLink(link, platform) {
			continue
		}
		if _, seen := seenURLs[linkURL]; seen || linkURL == "" {
			continue
		}
		seenURLs[linkURL] = struct{}{}
		record.SocialLinks = append(record.SocialLinks, entity.SocialLink{
			Platform: platform,
			URL:      linkURL,
			Label:    stringValue(link.Title),
		})
	}
	return record
}

// isCanonicalSocialLink keeps the entity projection focused on profile
// accounts. Lit.link profiles often contain many X posts, videos, and event
// announcements; those remain available from the profile endpoint and are
// handled by ToEventFeed when applicable.
func isCanonicalSocialLink(link Link, platform string) bool {
	if link.Type != "image" {
		return false
	}
	parsed, err := url.Parse(stringValue(link.URL))
	if err != nil || parsed.Hostname() == "" {
		return false
	}
	if platform == "youtube" && strings.EqualFold(strings.TrimPrefix(parsed.Hostname(), "www."), "youtu.be") {
		return false
	}
	segments := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	return len(segments) == 1 && segments[0] != ""
}

func socialPlatform(rawURL, urlType string) string {
	if rawURL == "" {
		return ""
	}
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Hostname() == "" {
		return ""
	}
	host := strings.ToLower(strings.TrimPrefix(parsed.Hostname(), "www."))
	switch {
	case host == "x.com" || host == "twitter.com":
		return "x"
	case host == "instagram.com":
		return "instagram"
	case host == "tiktok.com":
		return "tiktok"
	case host == "youtube.com" || host == "youtu.be":
		return "youtube"
	case host == "twitch.tv":
		return "twitch"
	case host == "facebook.com" || host == "fb.me":
		return "facebook"
	case host == "note.com":
		return "note"
	}
	switch strings.ToLower(strings.TrimSpace(urlType)) {
	case "x", "twitter":
		return "x"
	case "instagram", "tiktok", "youtube", "twitch", "facebook", "note":
		return strings.ToLower(strings.TrimSpace(urlType))
	default:
		return ""
	}
}
