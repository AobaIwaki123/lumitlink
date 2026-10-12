// Package litlink fetches and normalizes public Lit.link profiles.
package litlink

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode"

	"golang.org/x/net/html"
)

const (
	BaseURL             = "https://lit.link"
	DefaultUserAgent    = "lumipage/0.1 (+https://lit.link/)"
	DefaultTimeout      = 10 * time.Second
	MaxResponseBytes    = 5 * 1024 * 1024
	profilePathPrefix   = "/api/v1/profiles/"
	flightPayloadMarker = "self.__next_f.push([1,"
)

var usernamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$`)

// FetchError is returned when Lit.link cannot provide a usable public page.
type FetchError struct {
	HTTPStatus int
	Message    string
	Err        error
}

func (e *FetchError) Error() string {
	if e.Err == nil {
		return e.Message
	}
	return fmt.Sprintf("%s: %v", e.Message, e.Err)
}

func (e *FetchError) Unwrap() error { return e.Err }

// Page is the raw HTTP response used by the Lit.link adapter.
type Page struct {
	URL        string
	Body       []byte
	Status     int
	Headers    http.Header
	DurationMS int64
}

// Client is a browserless HTTP client for public Lit.link pages.
type Client struct {
	HTTPClient *http.Client
	Timeout    time.Duration
	UserAgent  string
}

// NewClient creates a client with a host-locked redirect policy.
func NewClient(timeout time.Duration, userAgent string) *Client {
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	if strings.TrimSpace(userAgent) == "" {
		userAgent = DefaultUserAgent
	}
	return &Client{
		HTTPClient: &http.Client{
			Timeout: timeout,
			CheckRedirect: func(req *http.Request, _ []*http.Request) error {
				if strings.EqualFold(req.URL.Hostname(), "lit.link") {
					return nil
				}
				return http.ErrUseLastResponse
			},
		},
		Timeout:   timeout,
		UserAgent: userAgent,
	}
}

// ValidateUsername validates the public Lit.link path segment.
func ValidateUsername(username string) (string, error) {
	normalized := strings.TrimSpace(username)
	if !usernamePattern.MatchString(normalized) {
		return "", errors.New("username must contain 1-64 ASCII letters, digits, '_' or '-'")
	}
	return normalized, nil
}

// FetchPage performs a plain HTTP GET. It never starts a browser or executes JavaScript.
func (c *Client) FetchPage(ctx context.Context, username string) (Page, error) {
	validated, err := ValidateUsername(username)
	if err != nil {
		return Page{}, err
	}
	pageURL := BaseURL + "/" + url.PathEscape(validated)
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, pageURL, nil)
	if err != nil {
		return Page{}, &FetchError{HTTPStatus: http.StatusBadGateway, Message: "could not create Lit.link request", Err: err}
	}
	request.Header.Set("Accept", "text/html,application/xhtml+xml")
	request.Header.Set("User-Agent", c.UserAgent)

	started := time.Now()
	response, err := c.HTTPClient.Do(request)
	if err != nil {
		return Page{}, &FetchError{HTTPStatus: http.StatusBadGateway, Message: "could not fetch lit.link", Err: err}
	}
	defer func() { _ = response.Body.Close() }()

	body, readErr := io.ReadAll(io.LimitReader(response.Body, MaxResponseBytes+1))
	duration := time.Since(started).Milliseconds()
	if readErr != nil {
		return Page{}, &FetchError{HTTPStatus: http.StatusBadGateway, Message: "could not read lit.link response", Err: readErr}
	}
	if len(body) > MaxResponseBytes {
		return Page{}, &FetchError{HTTPStatus: http.StatusBadGateway, Message: "lit.link response exceeded the size limit"}
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		status := response.StatusCode
		if status >= http.StatusInternalServerError {
			status = http.StatusBadGateway
		}
		return Page{}, &FetchError{HTTPStatus: status, Message: fmt.Sprintf("lit.link returned HTTP %d", response.StatusCode)}
	}

	return Page{
		URL:        pageURL,
		Body:       body,
		Status:     response.StatusCode,
		Headers:    response.Header.Clone(),
		DurationMS: duration,
	}, nil
}

// Link is one normalized profile link or media item.
type Link struct {
	ID                 any    `json:"id"`
	Type               string `json:"type"`
	Title              any    `json:"title"`
	Description        any    `json:"description"`
	DescriptionText    any    `json:"description_text"`
	URL                any    `json:"url"`
	URLType            any    `json:"url_type"`
	Provider           any    `json:"provider,omitempty"`
	ImageURL           any    `json:"image_url,omitempty"`
	PublicationStartAt any    `json:"publication_start_at"`
	PublicationEndAt   any    `json:"publication_end_at"`
}

// Category is a Lit.link SNS activity category.
type Category struct {
	Genre    any `json:"genre"`
	Category any `json:"category"`
}

// Profile contains normalized fields from Lit.link's SSR payload.
type Profile struct {
	Name        any        `json:"name"`
	Catchphrase any        `json:"catchphrase"`
	ProfileText any        `json:"profile_text"`
	URLPath     any        `json:"url_path"`
	PictureURL  any        `json:"picture_url"`
	Layout      any        `json:"layout,omitempty"`
	Categories  []Category `json:"categories,omitempty"`
	Links       []Link     `json:"links"`
}

// Source describes the upstream response and the cache state visible to callers.
type Source struct {
	URL             string   `json:"url"`
	HTTPStatus      int      `json:"http_status"`
	ContentType     string   `json:"content_type,omitempty"`
	ContentLength   int      `json:"content_length"`
	ETag            string   `json:"etag,omitempty"`
	DurationMS      int64    `json:"duration_ms"`
	Cached          bool     `json:"cached"`
	CacheAgeSeconds *float64 `json:"cache_age_seconds,omitempty"`
}

// Response is the public JSON API payload.
type Response struct {
	PageTitle any               `json:"page_title"`
	Metadata  map[string]string `json:"metadata"`
	Profile   Profile           `json:"profile"`
	Stats     Stats             `json:"stats"`
	Source    Source            `json:"source"`
	Cached    bool              `json:"cached"`
}

// Stats contains parser output counts useful for debugging upstream changes.
type Stats struct {
	ProfileBlocks int `json:"profile_blocks"`
	Links         int `json:"links"`
}

type pageParser struct {
	title   string
	meta    map[string]string
	scripts []string
	anchors []anchor
}

type anchor struct {
	href string
	text string
}

func parsePageDocument(body []byte) (pageParser, error) {
	root, err := html.Parse(strings.NewReader(string(body)))
	if err != nil {
		return pageParser{}, fmt.Errorf("could not parse lit.link HTML: %w", err)
	}
	doc := pageParser{meta: make(map[string]string)}
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node.Type == html.ElementNode {
			switch strings.ToLower(node.Data) {
			case "title":
				doc.title = normalizeText(textContent(node))
			case "meta":
				key := attr(node, "property")
				if key == "" {
					key = attr(node, "name")
				}
				value := attr(node, "content")
				if key != "" && value != "" {
					doc.meta[strings.ToLower(key)] = value
				}
			case "script":
				doc.scripts = append(doc.scripts, textContent(node))
			case "a":
				doc.anchors = append(doc.anchors, anchor{href: attr(node, "href"), text: normalizeText(textContent(node))})
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(root)
	return doc, nil
}

func attr(node *html.Node, key string) string {
	for _, attribute := range node.Attr {
		if strings.EqualFold(attribute.Key, key) {
			return attribute.Val
		}
	}
	return ""
}

func textContent(node *html.Node) string {
	var builder strings.Builder
	var walk func(*html.Node)
	walk = func(current *html.Node) {
		if current.Type == html.TextNode {
			builder.WriteString(current.Data)
		}
		for child := current.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(node)
	return builder.String()
}

func normalizeText(value string) string {
	return strings.Join(strings.FieldsFunc(value, unicode.IsSpace), " ")
}

func decodeFlightPayload(script string) (string, bool) {
	position := strings.Index(script, flightPayloadMarker)
	if position < 0 {
		return "", false
	}
	decoder := json.NewDecoder(strings.NewReader(script[position+len(flightPayloadMarker):]))
	var payload string
	if err := decoder.Decode(&payload); err != nil {
		return "", false
	}
	return payload, true
}

func extractProfile(scripts []string) (map[string]any, bool) {
	const marker = `"profile":`
	for _, script := range scripts {
		payload, ok := decodeFlightPayload(script)
		if !ok {
			continue
		}
		for offset := 0; offset < len(payload); {
			position := strings.Index(payload[offset:], marker)
			if position < 0 {
				break
			}
			position += offset + len(marker)
			for position < len(payload) && unicode.IsSpace(rune(payload[position])) {
				position++
			}
			decoder := json.NewDecoder(strings.NewReader(payload[position:]))
			var candidate any
			if err := decoder.Decode(&candidate); err == nil {
				if profile, ok := candidate.(map[string]any); ok {
					if _, hasLinks := profile["profileLinks"]; hasLinks {
						return profile, true
					}
				}
			}
			offset = position
		}
	}
	return nil, false
}

func stringValue(value any) string {
	text, ok := value.(string)
	if !ok || text == "$undefined" {
		return ""
	}
	return text
}

func optional(value any) any {
	if value == nil || value == "$undefined" {
		return nil
	}
	return value
}

func htmlToText(value any) any {
	text := stringValue(value)
	if text == "" {
		return nil
	}
	root, err := html.Parse(strings.NewReader("<body>" + text + "</body>"))
	if err != nil {
		return normalizeText(text)
	}
	return normalizeText(blockTextContent(root))
}

func blockTextContent(node *html.Node) string {
	var builder strings.Builder
	var walk func(*html.Node)
	walk = func(current *html.Node) {
		if current.Type == html.ElementNode {
			switch strings.ToLower(current.Data) {
			case "br", "div", "li", "p", "section", "article", "h1", "h2", "h3", "h4", "h5", "h6":
				builder.WriteByte(' ')
			}
		}
		if current.Type == html.TextNode {
			builder.WriteString(current.Data)
		}
		for child := current.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
		if current.Type == html.ElementNode {
			switch strings.ToLower(current.Data) {
			case "div", "li", "p", "section", "article", "h1", "h2", "h3", "h4", "h5", "h6":
				builder.WriteByte(' ')
			}
		}
	}
	walk(node)
	return builder.String()
}

func object(value any) map[string]any {
	result, _ := value.(map[string]any)
	return result
}

func array(value any) []any {
	result, _ := value.([]any)
	return result
}

func sourceCommon(block, source map[string]any, linkType string) Link {
	return Link{
		ID:                 optional(firstValue(block, source, "id")),
		Type:               linkType,
		Title:              optional(source["title"]),
		Description:        optional(source["description"]),
		DescriptionText:    htmlToText(source["description"]),
		URL:                optional(source["url"]),
		URLType:            optional(source["urlType"]),
		PublicationStartAt: optional(block["publicationStartAt"]),
		PublicationEndAt:   optional(block["publicationEndAt"]),
	}
}

func firstValue(first, second map[string]any, key string) any {
	if value, ok := first[key]; ok && value != nil && value != "$undefined" {
		return value
	}
	return second[key]
}

func flattenProfileLinks(profile map[string]any) []Link {
	fields := []string{"buttonLink", "textLink", "imageLink", "movieLink", "musicLink", "shopLink", "calendarLink"}
	var links []Link
	for _, rawBlock := range array(profile["profileLinks"]) {
		block := object(rawBlock)
		if block == nil {
			continue
		}
		linkType := stringValue(block["linkType"])
		if linkType == "" {
			linkType = "unknown"
		}
		var source map[string]any
		for _, field := range fields {
			candidate := object(block[field])
			if candidate != nil {
				source = candidate
				if field == "imageLink" {
					for _, rawImage := range array(source["profileImages"]) {
						image := object(rawImage)
						if image == nil {
							continue
						}
						item := sourceCommon(block, image, linkType)
						item.ImageURL = optional(image["imageUrl"])
						links = append(links, item)
					}
				} else {
					item := sourceCommon(block, source, linkType)
					providerKey := map[string]string{"movie": "movieType", "music": "musicType", "calendar": "type"}[linkType]
					if providerKey != "" {
						item.Provider = optional(source[providerKey])
					}
					links = append(links, item)
				}
				break
			}
		}
	}
	return links
}

func fallbackLinks(anchors []anchor) []Link {
	links := make([]Link, 0, len(anchors))
	seen := make(map[string]struct{})
	for _, item := range anchors {
		if !strings.HasPrefix(item.href, "http://") && !strings.HasPrefix(item.href, "https://") {
			continue
		}
		if strings.Contains(strings.ToLower(item.href), "lit.link") {
			continue
		}
		key := item.href + "\x00" + item.text
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		title := item.text
		if title == "" {
			title = item.href
		}
		links = append(links, Link{Type: "anchor", Title: title, URL: item.href})
	}
	return links
}

func categories(profile map[string]any) []Category {
	var result []Category
	for _, raw := range array(profile["creatorSnsActivityCategories"]) {
		item := object(raw)
		if item == nil {
			continue
		}
		genre := object(item["snsActivityGenre"])
		category := object(item["snsActivityCategory"])
		result = append(result, Category{Genre: optional(genre["name"]), Category: optional(category["name"])})
	}
	return result
}

// Parse normalizes an HTML page without executing its JavaScript.
func Parse(body []byte, page *Page) (Response, error) {
	document, err := parsePageDocument(body)
	if err != nil {
		return Response{}, &FetchError{HTTPStatus: http.StatusBadGateway, Message: "could not parse lit.link HTML", Err: err}
	}
	profile, hasProfile := extractProfile(document.scripts)
	links := fallbackLinks(document.anchors)
	if hasProfile {
		links = flattenProfileLinks(profile)
	}
	metadata := make(map[string]string)
	for _, key := range []string{"description", "og:title", "og:description", "og:url", "og:image", "twitter:title", "twitter:description", "twitter:image"} {
		if value, ok := document.meta[key]; ok {
			metadata[key] = value
		}
	}
	profileData := Profile{Links: links}
	if hasProfile {
		profileData.Name = optional(profile["name"])
		profileData.Catchphrase = optional(profile["catchphrase"])
		profileData.ProfileText = optional(profile["profileText"])
		profileData.URLPath = optional(profile["urlPath"])
		profileData.PictureURL = optional(profile["pictureUrl"])
		profileData.Layout = optional(profile["creatorDetailLayout"])
		profileData.Categories = categories(profile)
	} else {
		profileData.Catchphrase = metadata["description"]
		profileData.ProfileText = metadata["og:description"]
	}
	result := Response{
		PageTitle: document.title,
		Metadata:  metadata,
		Profile:   profileData,
		Stats:     Stats{ProfileBlocks: len(array(profile["profileLinks"])), Links: len(links)},
	}
	if document.title == "" {
		result.PageTitle = nil
	}
	if !hasProfile {
		result.Stats.ProfileBlocks = len(links)
	}
	if page != nil {
		result.Source = Source{
			URL:           page.URL,
			HTTPStatus:    page.Status,
			ContentType:   page.Headers.Get("Content-Type"),
			ContentLength: len(page.Body),
			ETag:          page.Headers.Get("ETag"),
			DurationMS:    page.DurationMS,
		}
	}
	return result, nil
}

// FetchProfile fetches and parses one public profile.
func (c *Client) FetchProfile(ctx context.Context, username string) (Response, error) {
	page, err := c.FetchPage(ctx, username)
	if err != nil {
		return Response{}, err
	}
	return Parse(page.Body, &page)
}
