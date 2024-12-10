package storage

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/romangod6/kb-crawler/internal/models"
)

type PostgresStore struct {
	db *sql.DB
}

func NewPostgresStore(connStr string) (*PostgresStore, error) {
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, err
	}

	if err := db.Ping(); err != nil {
		return nil, err
	}

	return &PostgresStore{db: db}, nil
}

func (s *PostgresStore) Initialize() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS categories (
            id UUID PRIMARY KEY,
            name VARCHAR(255) NOT NULL,
            description TEXT,
            parent_id UUID REFERENCES categories(id),
            created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
            updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
        )`,
		`CREATE TABLE IF NOT EXISTS articles (
            id UUID PRIMARY KEY,
            category_id UUID REFERENCES categories(id),
            name VARCHAR(255) NOT NULL,
            body TEXT,
            url VARCHAR(2048) UNIQUE NOT NULL,
            tags TEXT[],
            author VARCHAR(255),
            metadata JSONB,
            created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
            updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
        )`,
		`CREATE INDEX IF NOT EXISTS idx_articles_category_id ON articles(category_id)`,
		`CREATE INDEX IF NOT EXISTS idx_articles_url ON articles(url)`,
		`CREATE INDEX IF NOT EXISTS idx_articles_tags ON articles USING GIN(tags)`,
		`CREATE INDEX IF NOT EXISTS idx_articles_body_fts ON articles USING GIN (to_tsvector('english', body))`,
	}

	for _, query := range queries {
		if _, err := s.db.Exec(query); err != nil {
			return fmt.Errorf("error executing query %s: %w", query, err)
		}
	}

	return nil
}

func (s *PostgresStore) CreateCategory(ctx context.Context, category *models.Category) error {
	query := `
        INSERT INTO categories (id, name, description, parent_id, created_at, updated_at)
        VALUES ($1, $2, $3, $4, $5, $6)
        ON CONFLICT (id) DO UPDATE SET
            name = EXCLUDED.name,
            description = EXCLUDED.description,
            parent_id = EXCLUDED.parent_id,
            updated_at = CURRENT_TIMESTAMP
    `

	_, err := s.db.ExecContext(ctx, query,
		category.ID,
		category.Name,
		category.Description,
		category.ParentID,
		category.CreatedAt,
		category.UpdatedAt,
	)

	return err
}

func (s *PostgresStore) CreateArticle(ctx context.Context, article *models.Article) error {
	query := `
        INSERT INTO articles (id, category_id, name, body, url, tags, author, metadata, created_at, updated_at)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
        ON CONFLICT (url) DO UPDATE SET
            category_id = EXCLUDED.category_id,
            name = EXCLUDED.name,
            body = EXCLUDED.body,
            tags = EXCLUDED.tags,
            author = EXCLUDED.author,
            metadata = EXCLUDED.metadata,
            updated_at = CURRENT_TIMESTAMP
    `

	_, err := s.db.ExecContext(ctx, query,
		article.ID,
		article.CategoryID,
		article.Name,
		article.Body,
		article.URL,
		pq.Array(article.Tags),
		article.Author,
		article.Metadata,
		article.CreatedAt,
		article.UpdatedAt,
	)

	return err
}

func (s *PostgresStore) GetArticle(ctx context.Context, id uuid.UUID) (*models.Article, error) {
	query := `
        SELECT id, category_id, name, body, url, tags, author, metadata, created_at, updated_at
        FROM articles
        WHERE id = $1
    `

	article := &models.Article{}
	var tags []string

	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&article.ID,
		&article.CategoryID,
		&article.Name,
		&article.Body,
		&article.URL,
		pq.Array(&tags),
		&article.Author,
		&article.Metadata,
		&article.CreatedAt,
		&article.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}

	if err != nil {
		return nil, err
	}

	article.Tags = tags
	return article, nil
}

func (s *PostgresStore) GetCategory(ctx context.Context, id uuid.UUID) (*models.Category, error) {
	query := `
        SELECT id, name, description, parent_id, created_at, updated_at
        FROM categories
        WHERE id = $1
    `

	category := &models.Category{}
	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&category.ID,
		&category.Name,
		&category.Description,
		&category.ParentID,
		&category.CreatedAt,
		&category.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}

	if err != nil {
		return nil, err
	}

	return category, nil
}

func (s *PostgresStore) ListArticles(ctx context.Context, limit, offset int) ([]*models.Article, error) {
	query := `
        SELECT id, category_id, name, body, url, tags, author, metadata, created_at, updated_at
        FROM articles
        ORDER BY created_at DESC
        LIMIT $1 OFFSET $2
    `

	rows, err := s.db.QueryContext(ctx, query, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var articles []*models.Article
	for rows.Next() {
		article := &models.Article{}
		var tags []string

		err := rows.Scan(
			&article.ID,
			&article.CategoryID,
			&article.Name,
			&article.Body,
			&article.URL,
			pq.Array(&tags),
			&article.Author,
			&article.Metadata,
			&article.CreatedAt,
			&article.UpdatedAt,
		)

		if err != nil {
			return nil, err
		}

		article.Tags = tags
		articles = append(articles, article)
	}

	return articles, nil
}

func (s *PostgresStore) ListCategories(ctx context.Context) ([]*models.Category, error) {
	query := `
        SELECT id, name, description, parent_id, created_at, updated_at
        FROM categories
        ORDER BY name
    `

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var categories []*models.Category
	for rows.Next() {
		category := &models.Category{}
		err := rows.Scan(
			&category.ID,
			&category.Name,
			&category.Description,
			&category.ParentID,
			&category.CreatedAt,
			&category.UpdatedAt,
		)

		if err != nil {
			return nil, err
		}

		categories = append(categories, category)
	}

	return categories, nil
}

func (s *PostgresStore) GetArticlesByCategory(ctx context.Context, categoryID uuid.UUID, limit, offset int) ([]*models.Article, error) {
	query := `
        SELECT id, category_id, name, body, url, tags, author, metadata, created_at, updated_at
        FROM articles
        WHERE category_id = $1
        ORDER BY created_at DESC
        LIMIT $2 OFFSET $3
    `

	rows, err := s.db.QueryContext(ctx, query, categoryID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var articles []*models.Article
	for rows.Next() {
		article := &models.Article{}
		var tags []string

		err := rows.Scan(
			&article.ID,
			&article.CategoryID,
			&article.Name,
			&article.Body,
			&article.URL,
			pq.Array(&tags),
			&article.Author,
			&article.Metadata,
			&article.CreatedAt,
			&article.UpdatedAt,
		)

		if err != nil {
			return nil, err
		}

		article.Tags = tags
		articles = append(articles, article)
	}

	return articles, nil
}

func (s *PostgresStore) SearchArticles(ctx context.Context, query string, limit, offset int) ([]*models.Article, error) {
	sqlQuery := `
        SELECT id, category_id, name, body, url, tags, author, metadata, created_at, updated_at
        FROM articles
        WHERE to_tsvector('english', body) @@ plainto_tsquery('english', $1)
        ORDER BY ts_rank(to_tsvector('english', body), plainto_tsquery('english', $1)) DESC
        LIMIT $2 OFFSET $3
    `

	rows, err := s.db.QueryContext(ctx, sqlQuery, query, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var articles []*models.Article
	for rows.Next() {
		article := &models.Article{}
		var tags []string

		err := rows.Scan(
			&article.ID,
			&article.CategoryID,
			&article.Name,
			&article.Body,
			&article.URL,
			pq.Array(&tags),
			&article.Author,
			&article.Metadata,
			&article.CreatedAt,
			&article.UpdatedAt,
		)

		if err != nil {
			return nil, err
		}

		article.Tags = tags
		articles = append(articles, article)
	}

	return articles, nil
}

func (s *PostgresStore) Close() error {
	return s.db.Close()
}
