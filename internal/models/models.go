package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type Category struct {
	ID          uuid.UUID  `json:"id"`
	Name        string     `json:"name"`
	Description string     `json:"description,omitempty"`
	ParentID    *uuid.UUID `json:"parent_id,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

type Article struct {
	ID         uuid.UUID        `json:"id"`
	CategoryID uuid.UUID        `json:"category_id"`
	Name       string           `json:"name"`
	Body       string           `json:"body"`
	URL        string           `json:"url"`
	Tags       []string         `json:"tags"`
	Author     string           `json:"author"`
	Metadata   *json.RawMessage `json:"metadata,omitempty"`
	CreatedAt  time.Time        `json:"created_at"`
	UpdatedAt  time.Time        `json:"updated_at"`
}

type Tag struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

type ArticleTag struct {
	ArticleID uuid.UUID `json:"article_id"`
	TagID     uuid.UUID `json:"tag_id"`
}

type CrawlerConfig struct {
	ID              uuid.UUID  `json:"id"`
	Product         string     `json:"product"`
	SitemapURL      string     `json:"sitemapUrl"`
	MapURL          string     `json:"mapUrl"`
	UserAgent       string     `json:"userAgent"`
	CrawlInterval   string     `json:"crawlInterval"`
	MaxDepth        int        `json:"maxDepth"`
	DefaultCategory string     `json:"defaultCategory"`
	AllowedDomains  []string   `json:"allowedDomains"`
	Status          string     `json:"status"` // "Running", "Stopped", "Error", "Completed", "Scheduled"
	IsFirstRun      bool       `json:"isFirstRun"`
	LastRun         *time.Time `json:"lastRun,omitempty"`
	NextRun         *time.Time `json:"nextRun,omitempty"`
	Errors          []string   `json:"errors,omitempty"`
	Logs            []string   `json:"logs,omitempty"`
	CreatedAt       time.Time  `json:"createdAt"`
	UpdatedAt       time.Time  `json:"updatedAt"`
}
