package models

import (
	"time"

	"github.com/google/uuid"
)

// NewCategory creates a new category with generated UUID and timestamps
func NewCategory() *Category {
	now := time.Now()
	return &Category{
		ID:        uuid.New(),
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// IsRoot returns true if the category has no parent
func (c *Category) IsRoot() bool {
	return c.ParentID == nil
}
