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
		if platform == "" {
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
