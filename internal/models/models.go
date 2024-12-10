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

// ArticleTag represents the many-to-many relationship between articles and tags
type ArticleTag struct {
	ArticleID uuid.UUID `json:"article_id"`
	TagID     uuid.UUID `json:"tag_id"`
}
