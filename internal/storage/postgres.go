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
		`CREATE TABLE IF NOT EXISTS crawler_configs (
            id UUID PRIMARY KEY,
            product TEXT NOT NULL,
            sitemap_url TEXT NOT NULL,
            map_url TEXT NOT NULL,
            user_agent TEXT NOT NULL,
            crawl_interval TEXT NOT NULL,
            max_depth INTEGER NOT NULL,
            default_category TEXT NOT NULL,
            allowed_domains TEXT[],
            status TEXT NOT NULL,
            last_run TIMESTAMP,
            errors TEXT[],
            logs TEXT[],
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

// Existing methods for Article
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

// New Crawler Config Methods
func (s *PostgresStore) ListCrawlerConfigs(ctx context.Context) ([]*models.CrawlerConfig, error) {
	query := `
        SELECT id, product, sitemap_url, map_url, user_agent, crawl_interval, max_depth,
               default_category, allowed_domains, status, last_run, errors, logs,
               created_at, updated_at
        FROM crawler_configs
        ORDER BY created_at DESC
    `

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var configs []*models.CrawlerConfig
	for rows.Next() {
		config := &models.CrawlerConfig{}
		err := rows.Scan(
			&config.ID,
			&config.Product,
			&config.SitemapURL,
			&config.MapURL,
			&config.UserAgent,
			&config.CrawlInterval,
			&config.MaxDepth,
			&config.DefaultCategory,
			pq.Array(&config.AllowedDomains),
			&config.Status,
			&config.LastRun,
			pq.Array(&config.Errors),
			pq.Array(&config.Logs),
			&config.CreatedAt,
			&config.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		configs = append(configs, config)
	}

	return configs, nil
}

func (s *PostgresStore) GetCrawlerConfig(ctx context.Context, id uuid.UUID) (*models.CrawlerConfig, error) {
	query := `
        SELECT id, product, sitemap_url, map_url, user_agent, crawl_interval, max_depth,
               default_category, allowed_domains, status, last_run, errors, logs,
               created_at, updated_at
        FROM crawler_configs
        WHERE id = $1
    `

	config := &models.CrawlerConfig{}
	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&config.ID,
		&config.Product,
		&config.SitemapURL,
		&config.MapURL,
		&config.UserAgent,
		&config.CrawlInterval,
		&config.MaxDepth,
		&config.DefaultCategory,
		pq.Array(&config.AllowedDomains),
		&config.Status,
		&config.LastRun,
		pq.Array(&config.Errors),
		pq.Array(&config.Logs),
		&config.CreatedAt,
		&config.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return config, nil
}

func (s *PostgresStore) CreateCrawlerConfig(ctx context.Context, config *models.CrawlerConfig) error {
	query := `
        INSERT INTO crawler_configs (
            id, product, sitemap_url, map_url, user_agent, crawl_interval, max_depth,
            default_category, allowed_domains, status, last_run, errors, logs,
            created_at, updated_at
        ) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
    `

	_, err := s.db.ExecContext(ctx, query,
		config.ID,
		config.Product,
		config.SitemapURL,
		config.MapURL,
		config.UserAgent,
		config.CrawlInterval,
		config.MaxDepth,
		config.DefaultCategory,
		pq.Array(config.AllowedDomains),
		config.Status,
		config.LastRun,
		pq.Array(config.Errors),
		pq.Array(config.Logs),
		config.CreatedAt,
		config.UpdatedAt,
	)

	return err
}

func (s *PostgresStore) UpdateCrawlerConfig(ctx context.Context, config *models.CrawlerConfig) error {
	query := `
        UPDATE crawler_configs SET
            product = $2,
            sitemap_url = $3,
            map_url = $4,
            user_agent = $5,
            crawl_interval = $6,
            max_depth = $7,
            default_category = $8,
            allowed_domains = $9,
            status = $10,
            last_run = $11,
            errors = $12,
            logs = $13,
            updated_at = CURRENT_TIMESTAMP
        WHERE id = $1
    `

	result, err := s.db.ExecContext(ctx, query,
		config.ID,
		config.Product,
		config.SitemapURL,
		config.MapURL,
		config.UserAgent,
		config.CrawlInterval,
		config.MaxDepth,
		config.DefaultCategory,
		pq.Array(config.AllowedDomains),
		config.Status,
		config.LastRun,
		pq.Array(config.Errors),
		pq.Array(config.Logs),
	)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return sql.ErrNoRows
	}

	return nil
}

func (s *PostgresStore) DeleteCrawlerConfig(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM crawler_configs WHERE id = $1`
	result, err := s.db.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return sql.ErrNoRows
	}

	return nil
}

func (s *PostgresStore) Close() error {
	return s.db.Close()
}
