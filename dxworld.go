package main

import (
	"encoding/xml"
	"fmt"
	"html"
	"io"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"
)

// ============================================================================
// Structures
// ============================================================================

type DXWorldItem struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	PubDate     string `xml:"pubDate"`
	Description string `xml:"description"`
	Creator     string `xml:"creator"`
	Enclosure   struct {
		URL  string `xml:"url,attr"`
		Type string `xml:"type,attr"`
	} `xml:"enclosure"`
	MediaContent struct {
		URL string `xml:"url,attr"`
	} `xml:"content"`
}

type DXWorldFeed struct {
	Items []DXWorldItem `xml:"channel>item"`
}

type DXWorldNews struct {
	Title     string `json:"title"`
	Link      string `json:"link"`
	PubDate   string `json:"pubDate"`
	Excerpt   string `json:"excerpt"`
	Creator   string `json:"creator"`
	ImageURL  string `json:"imageUrl"`
	Tag       string `json:"tag"` // "NEWS", "UPDATE", "NEW ACTIVITY", etc.
}

// ============================================================================
// Cache
// ============================================================================

type DXWorldCache struct {
	mu        sync.RWMutex
	news      []DXWorldNews
	lastFetch time.Time
}

var dxwCache = &DXWorldCache{}

func (c *DXWorldCache) Get() []DXWorldNews {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.news
}

func (c *DXWorldCache) Set(news []DXWorldNews) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.news = news
	c.lastFetch = time.Now()
}

func (c *DXWorldCache) NeedsRefresh() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return time.Since(c.lastFetch) > 30*time.Minute
}

// ============================================================================
// Fetch & Parse
// ============================================================================

var htmlTagRe    = regexp.MustCompile(`<[^>]+>`)
var multiSpaceRe = regexp.MustCompile(`\s+`)
var tagPrefixRe  = regexp.MustCompile(`(?i)^\s*\[([^\]]+)\]\s*`)

func stripHTML(s string) string {
	s = htmlTagRe.ReplaceAllString(s, " ")
	s = html.UnescapeString(s)
	s = multiSpaceRe.ReplaceAllString(s, " ")
	return strings.TrimSpace(s)
}

// extractTag pulls out the leading [TAG] from a stripped excerpt, returns (tag, remainder).
func extractTag(s string) (string, string) {
	m := tagPrefixRe.FindStringSubmatchIndex(s)
	if m == nil {
		return "", s
	}
	tag := strings.ToUpper(strings.TrimSpace(s[m[2]:m[3]]))
	rest := strings.TrimSpace(s[m[1]:])
	// Strip leading dash/dash separator if present
	rest = strings.TrimPrefix(rest, "– ")
	rest = strings.TrimPrefix(rest, "- ")
	return tag, rest
}

func truncate(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max]) + "…"
}

func parseDXWorldDate(s string) string {
	// RSS pubDate: "Thu, 17 Apr 2025 12:00:00 +0000"
	formats := []string{
		"Mon, 02 Jan 2006 15:04:05 -0700",
		"Mon, 02 Jan 2006 15:04:05 +0000",
		"Mon, 2 Jan 2006 15:04:05 -0700",
		"Mon, 2 Jan 2006 15:04:05 +0000",
	}
	s = strings.TrimSpace(s)
	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
			return t.Format("Jan 2, 2006")
		}
	}
	return s
}

func FetchDXWorld() ([]DXWorldNews, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get("https://dx-world.net/feed/")
	if err != nil {
		return nil, fmt.Errorf("fetch error: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read error: %w", err)
	}

	var feed DXWorldFeed
	if err := xml.Unmarshal(body, &feed); err != nil {
		return nil, fmt.Errorf("parse error: %w", err)
	}

	news := make([]DXWorldNews, 0, len(feed.Items))
	for _, item := range feed.Items {
		raw := stripHTML(item.Description)
		tag, body := extractTag(raw)
		excerpt := truncate(body, 200)

		imageURL := item.MediaContent.URL
		if imageURL == "" {
			imageURL = item.Enclosure.URL
		}
		// Only keep image enclosures
		if imageURL != "" && !strings.HasPrefix(item.Enclosure.Type, "image") && item.MediaContent.URL == "" {
			imageURL = ""
		}

		news = append(news, DXWorldNews{
			Title:    strings.TrimSpace(item.Title),
			Link:     strings.TrimSpace(item.Link),
			PubDate:  parseDXWorldDate(item.PubDate),
			Excerpt:  excerpt,
			Creator:  strings.TrimSpace(item.Creator),
			ImageURL: imageURL,
			Tag:      tag,
		})
	}

	return news, nil
}

// ============================================================================
// Background refresh
// ============================================================================

func StartDXWorldRefresher(broadcast chan WSMessage) {
	go func() {
		refreshDXWorld(broadcast)

		ticker := time.NewTicker(30 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			refreshDXWorld(broadcast)
		}
	}()
}

func refreshDXWorld(broadcast chan WSMessage) {
	news, err := FetchDXWorld()
	if err != nil {
		Log.Errorf("DX-World fetch error: %v", err)
		return
	}
	dxwCache.Set(news)
	Log.Infof("DX-World: %d news items loaded", len(news))

	if broadcast != nil {
		select {
		case broadcast <- WSMessage{Type: "dxworld", Data: news}:
		default:
			Log.Errorf("DX-World broadcast channel full, skipping")
		}
	}
}
