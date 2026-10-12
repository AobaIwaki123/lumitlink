package litlink

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/AobaIwaki123/lumi-spec/event"
)

var (
	eventDatePrefix = regexp.MustCompile(`^\s*(\d{1,2})/(\d{1,2})(?:\s*\([^)]*\))?\s*(.*)$`)
	timeRange       = regexp.MustCompile(`(\d{1,2}):(\d{2})(?:\s*[-〜～~]\s*(\d{1,2}):(\d{2}))?`)
)

// ToEventFeed projects the date-prefixed event buttons on a Lit.link profile
// into the shared event-feed contract. Non-event profile links are ignored.
func ToEventFeed(username string, response Response) event.Feed {
	return ToEventFeedAt(username, response, time.Now())
}

// ToEventFeedAt is deterministic for tests and for callers that have a source
// update timestamp available.
func ToEventFeedAt(username string, response Response, now time.Time) event.Feed {
	feed := event.Feed{
		SchemaVersion: event.FeedSchemaVersion,
		Source:        "lumitlink",
		GeneratedAt:   now.UTC().Format(time.RFC3339Nano),
		Events:        make([]event.Candidate, 0),
	}
	for _, link := range response.Profile.Links {
		candidate, ok := linkToEvent(username, response, link, now)
		if ok {
			feed.Events = append(feed.Events, candidate)
		}
	}
	return feed
}

func linkToEvent(username string, response Response, link Link, now time.Time) (event.Candidate, bool) {
	if link.Type != "button" {
		return event.Candidate{}, false
	}
	rawTitle := stringValue(link.Title)
	matches := eventDatePrefix.FindStringSubmatch(rawTitle)
	if len(matches) != 4 {
		return event.Candidate{}, false
	}
	month, monthErr := strconv.Atoi(matches[1])
	day, dayErr := strconv.Atoi(matches[2])
	if monthErr != nil || dayErr != nil {
		return event.Candidate{}, false
	}
	eventDate, ok := inferredEventDate(now, month, day)
	if !ok {
		return event.Candidate{}, false
	}

	title := strings.TrimSpace(matches[3])
	if title == "" {
		title = rawTitle
	}
	description := stringValue(link.DescriptionText)
	sourceURL := stringValue(link.URL)
	if sourceURL == "" {
		sourceURL = response.Source.URL
	}
	recordID := stringValue(link.ID)
	if recordID == "" {
		recordID = fmt.Sprintf("%s-%02d-%02d", username, month, day)
	}

	candidate := event.Candidate{
		SchemaVersion: event.CandidateSchemaVersion,
		ID:            "lumitlink:" + recordID,
		SourceRef: event.SourceRef{
			Source:       "lumitlink",
			RecordID:     recordID,
			URL:          sourceURL,
			CollectionID: username,
		},
		Title:     title,
		LocalDate: eventDate.Format("2006-01-02"),
		Timezone:  "Asia/Tokyo",
		Extensions: map[string]any{
			"source_title":       rawTitle,
			"source_description": description,
			"source_link_type":   link.Type,
			"date_inferred":      true,
			"source_profile":     response.Source.URL,
		},
	}
	if name := stringValue(response.Profile.Name); name != "" {
		candidate.Performers = []string{name}
		candidate.PerformerRefs = []event.EntityRef{{ID: username, Name: name}}
	}

	venue := extractVenue(description)
	if venue != "" {
		candidate.Venue = venue
	}
	activities := extractActivities(description, eventDate)
	if len(activities) > 0 {
		candidate.Extensions["activities"] = activities
		candidate.StartsAt = activities[0]["starts_at"].(string)
		for _, activity := range activities {
			if end, ok := activity["ends_at"].(string); ok && end != "" {
				candidate.EndsAt = end
			}
		}
	}
	return candidate, true
}

func inferredEventDate(now time.Time, month, day int) (time.Time, bool) {
	location := time.FixedZone("Asia/Tokyo", 9*60*60)
	current := now.In(location)
	year := current.Year()
	if month < int(current.Month()) {
		year++
	}
	date := time.Date(year, time.Month(month), day, 0, 0, 0, 0, location)
	if date.Month() != time.Month(month) || date.Day() != day {
		return time.Time{}, false
	}
	return date, true
}

func extractVenue(description string) string {
	marker := strings.Index(description, "📍")
	if marker < 0 {
		return ""
	}
	value := strings.TrimSpace(description[marker+len("📍"):])
	stop := len(value)
	for _, token := range []string{"🎤", "📸", "📷", "🎫", "OPEN", "START"} {
		if index := strings.Index(value, token); index >= 0 && index < stop {
			stop = index
		}
	}
	return strings.TrimSpace(value[:stop])
}

func extractActivities(description string, date time.Time) []map[string]any {
	matches := timeRange.FindAllStringIndex(description, -1)
	activities := make([]map[string]any, 0, len(matches))
	for _, match := range matches {
		value := description[match[0]:match[1]]
		parts := timeRange.FindStringSubmatch(value)
		if len(parts) < 3 {
			continue
		}
		start, ok := eventTime(date, parts[1], parts[2])
		if !ok {
			continue
		}
		kind := activityKind(description, match[0])
		activity := map[string]any{
			"kind":        kind,
			"starts_at":   start.Format(time.RFC3339),
			"source_text": value,
		}
		if parts[3] != "" {
			if end, valid := eventTime(date, parts[3], parts[4]); valid {
				activity["ends_at"] = end.Format(time.RFC3339)
			}
		}
		activities = append(activities, activity)
	}
	return activities
}

func activityKind(description string, start int) string {
	prefixStart := start - 8
	if prefixStart < 0 {
		prefixStart = 0
	}
	prefix := description[prefixStart:start]
	switch {
	case strings.Contains(prefix, "📸"), strings.Contains(prefix, "📷"):
		return "photo_session"
	case strings.Contains(prefix, "🎤"), strings.Contains(prefix, "START"):
		return "live"
	case strings.Contains(prefix, "OPEN"):
		return "doors"
	default:
		return "schedule"
	}
}

func eventTime(date time.Time, hourText, minuteText string) (time.Time, bool) {
	hour, hourErr := strconv.Atoi(hourText)
	minute, minuteErr := strconv.Atoi(minuteText)
	if hourErr != nil || minuteErr != nil || hour > 23 || minute > 59 {
		return time.Time{}, false
	}
	return time.Date(date.Year(), date.Month(), date.Day(), hour, minute, 0, 0, date.Location()), true
}
