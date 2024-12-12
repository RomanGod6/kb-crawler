package storage

import (
	"context"

	"github.com/google/uuid"
	"github.com/romangod6/kb-crawler/internal/models"
)

type Store interface {
	Initialize() error
	Close() error

	// Category operations
	CreateCategory(ctx context.Context, category *models.Category) error
	GetCategory(ctx context.Context, id uuid.UUID) (*models.Category, error)
	ListCategories(ctx context.Context) ([]*models.Category, error)

	// Article operations
	CreateArticle(ctx context.Context, article *models.Article) error
	GetArticle(ctx context.Context, id uuid.UUID) (*models.Article, error)
	ListArticles(ctx context.Context, limit, offset int) ([]*models.Article, error)
	SearchArticles(ctx context.Context, query string, limit, offset int) ([]*models.Article, error)
	GetArticlesByCategory(ctx context.Context, categoryID uuid.UUID, limit, offset int) ([]*models.Article, error)

	// Crawler Config operations
	ListCrawlerConfigs(ctx context.Context) ([]*models.CrawlerConfig, error)
	GetCrawlerConfig(ctx context.Context, id uuid.UUID) (*models.CrawlerConfig, error)
	CreateCrawlerConfig(ctx context.Context, config *models.CrawlerConfig) error
	UpdateCrawlerConfig(ctx context.Context, config *models.CrawlerConfig) error
	DeleteCrawlerConfig(ctx context.Context, id uuid.UUID) error
}
