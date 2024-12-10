package models

import (
	"time"

	"github.com/google/uuid"
)

// NewArticle creates a new article with generated UUID and timestamps
func NewArticle() *Article {
	now := time.Now()
	return &Article{
		ID:        uuid.New(),
		CreatedAt: now,
		UpdatedAt: now,
	}
}
