package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"
	"github.com/romangod6/kb-crawler/internal/models"
)

type SQLiteStore struct {
	db *sql.DB
}

func NewSQLiteStore(dbPath string) (*SQLiteStore, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}

	if err := db.Ping(); err != nil {
		return nil, err
	}

	return &SQLiteStore{db: db}, nil
}

func (s *SQLiteStore) Initialize() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS categories (
            id TEXT PRIMARY KEY,
            name TEXT NOT NULL,
            description TEXT,
            parent_id TEXT,
            created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
            updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
            FOREIGN KEY(parent_id) REFERENCES categories(id)
        )`,
		`CREATE TABLE IF NOT EXISTS articles (
            id TEXT PRIMARY KEY,
            category_id TEXT,
            name TEXT NOT NULL,
            body TEXT,
            url TEXT UNIQUE NOT NULL,
            tags TEXT,
            author TEXT,
            metadata TEXT,
            created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
            updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
            FOREIGN KEY(category_id) REFERENCES categories(id)
        )`,
		`CREATE INDEX IF NOT EXISTS idx_articles_category_id ON articles(category_id)`,
		`CREATE INDEX IF NOT EXISTS idx_articles_url ON articles(url)`,
	}

	for _, query := range queries {
		if _, err := s.db.Exec(query); err != nil {
			return fmt.Errorf("error executing query %s: %w", query, err)
		}
	}

	return nil
}

func (s *SQLiteStore) CreateCategory(ctx context.Context, category *models.Category) error {
	query := `
        INSERT INTO categories (id, name, description, parent_id, created_at, updated_at)
        VALUES (?, ?, ?, ?, ?, ?)
        ON CONFLICT(id) DO UPDATE SET
            name = excluded.name,
            description = excluded.description,
            parent_id = excluded.parent_id,
            updated_at = CURRENT_TIMESTAMP
    `

	_, err := s.db.ExecContext(ctx, query,
		category.ID.String(),
		category.Name,
		category.Description,
		nilIfEmpty(category.ParentID),
		category.CreatedAt,
		category.UpdatedAt,
	)

	return err
}

func (s *SQLiteStore) CreateArticle(ctx context.Context, article *models.Article) error {
	query := `
        INSERT INTO articles (id, category_id, name, body, url, tags, author, metadata, created_at, updated_at)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
        ON CONFLICT(url) DO UPDATE SET
            category_id = excluded.category_id,
            name = excluded.name,
            body = excluded.body,
            tags = excluded.tags,
            author = excluded.author,
            metadata = excluded.metadata,
            updated_at = CURRENT_TIMESTAMP
    `

	tagsJSON, err := json.Marshal(article.Tags)
	if err != nil {
		return err
	}

	_, err = s.db.ExecContext(ctx, query,
		article.ID.String(),
		article.CategoryID.String(),
		article.Name,
		article.Body,
		article.URL,
		string(tagsJSON),
		article.Author,
		article.Metadata,
		article.CreatedAt,
		article.UpdatedAt,
	)

	return err
}

func (s *SQLiteStore) GetArticle(ctx context.Context, id uuid.UUID) (*models.Article, error) {
	query := `
        SELECT id, category_id, name, body, url, tags, author, metadata, created_at, updated_at
        FROM articles
        WHERE id = ?
    `

	article := &models.Article{}
	var tagsJSON string
	var categoryIDStr string

	err := s.db.QueryRowContext(ctx, query, id.String()).Scan(
		&categoryIDStr,
		&article.Name,
		&article.Body,
		&article.URL,
		&tagsJSON,
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

	article.ID = id
	article.CategoryID, _ = uuid.Parse(categoryIDStr)
	json.Unmarshal([]byte(tagsJSON), &article.Tags)

	return article, nil
}

func (s *SQLiteStore) GetCategory(ctx context.Context, id uuid.UUID) (*models.Category, error) {
	query := `
        SELECT id, name, description, parent_id, created_at, updated_at
        FROM categories
        WHERE id = ?
    `

	category := &models.Category{}
	var parentIDStr sql.NullString

	err := s.db.QueryRowContext(ctx, query, id.String()).Scan(
		&category.ID,
		&category.Name,
		&category.Description,
		&parentIDStr,
		&category.CreatedAt,
		&category.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}

	if err != nil {
		return nil, err
	}

	if parentIDStr.Valid {
		parentID, err := uuid.Parse(parentIDStr.String)
		if err == nil {
			category.ParentID = &parentID
		}
	}

	return category, nil
}

func (s *SQLiteStore) ListArticles(ctx context.Context, limit, offset int) ([]*models.Article, error) {
	query := `
        SELECT id, category_id, name, body, url, tags, author, metadata, created_at, updated_at
        FROM articles
        ORDER BY created_at DESC
        LIMIT ? OFFSET ?
    `

	return s.queryArticles(ctx, query, limit, offset)
}

func (s *SQLiteStore) ListCategories(ctx context.Context) ([]*models.Category, error) {
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
		var category models.Category
		var parentIDStr sql.NullString

		err := rows.Scan(
			&category.ID,
			&category.Name,
			&category.Description,
			&parentIDStr,
			&category.CreatedAt,
			&category.UpdatedAt,
		)

		if err != nil {
			return nil, err
		}

		if parentIDStr.Valid {
			parentID, err := uuid.Parse(parentIDStr.String)
			if err == nil {
				category.ParentID = &parentID
			}
		}

		categories = append(categories, &category)
	}

	return categories, nil
}

func (s *SQLiteStore) SearchArticles(ctx context.Context, searchTerm string, limit, offset int) ([]*models.Article, error) {
	query := `
        SELECT id, category_id, name, body, url, tags, author, metadata, created_at, updated_at
        FROM articles
        WHERE name LIKE ? OR body LIKE ?
        ORDER BY created_at DESC
        LIMIT ? OFFSET ?
    `

	searchPattern := "%" + searchTerm + "%"
	return s.queryArticles(ctx, query, searchPattern, searchPattern, limit, offset)
}

func (s *SQLiteStore) GetArticlesByCategory(ctx context.Context, categoryID uuid.UUID, limit, offset int) ([]*models.Article, error) {
	query := `
        SELECT id, category_id, name, body, url, tags, author, metadata, created_at, updated_at
        FROM articles
        WHERE category_id = ?
        ORDER BY created_at DESC
        LIMIT ? OFFSET ?
    `

	return s.queryArticles(ctx, query, categoryID.String(), limit, offset)
}

func (s *SQLiteStore) queryArticles(ctx context.Context, query string, args ...interface{}) ([]*models.Article, error) {
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var articles []*models.Article
	for rows.Next() {
		var article models.Article
		var idStr, categoryIDStr, tagsJSON string

		err := rows.Scan(
			&idStr,
			&categoryIDStr,
			&article.Name,
			&article.Body,
			&article.URL,
			&tagsJSON,
			&article.Author,
			&article.Metadata,
			&article.CreatedAt,
			&article.UpdatedAt,
		)

		if err != nil {
			return nil, err
		}

		article.ID, _ = uuid.Parse(idStr)
		article.CategoryID, _ = uuid.Parse(categoryIDStr)
		json.Unmarshal([]byte(tagsJSON), &article.Tags)

		articles = append(articles, &article)
	}

	return articles, nil
}

func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

func nilIfEmpty(id *uuid.UUID) interface{} {
	if id == nil {
		return nil
	}
	return id.String()
}
