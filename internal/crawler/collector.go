package crawler

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/gocolly/colly/v2"
	"github.com/google/uuid"
	"github.com/romangod6/kb-crawler/internal/models"
	"github.com/romangod6/kb-crawler/internal/storage"
)

type Crawler struct {
	collector *colly.Collector
	store     storage.Store
	config    *CrawlerConfig
}

type CrawlerConfig struct {
	SitemapURL      string
	MapURL          string
	UserAgent       string
	MaxDepth        int
	DefaultCategory string
	AllowedDomains  []string
}

// CategoryStructure holds our mapped categories
type CategoryStructure struct {
	categories map[string]*models.Category
	mutex      sync.RWMutex
}

func NewCategoryStructure() *CategoryStructure {
	return &CategoryStructure{
		categories: make(map[string]*models.Category),
	}
}

func (cs *CategoryStructure) AddCategory(path string, category *models.Category) {
	cs.mutex.Lock()
	defer cs.mutex.Unlock()
	cs.categories[path] = category
}

func (cs *CategoryStructure) GetCategory(path string) (*models.Category, bool) {
	cs.mutex.RLock()
	defer cs.mutex.RUnlock()
	cat, exists := cs.categories[path]
	return cat, exists
}

func NewCrawler(store storage.Store, config *CrawlerConfig) *Crawler {
	c := colly.NewCollector(
		colly.UserAgent(config.UserAgent),
		colly.MaxDepth(config.MaxDepth),
		colly.AllowedDomains(config.AllowedDomains...),
	)

	// Set reasonable limits
	c.Limit(&colly.LimitRule{
		DomainGlob:  "*",
		Parallelism: 2,
		RandomDelay: 5 * time.Second,
	})

	return &Crawler{
		collector: c,
		store:     store,
		config:    config,
	}
}

func (c *Crawler) MapCategoryStructure(ctx context.Context) (*CategoryStructure, error) {
	cs := NewCategoryStructure()

	// Create root category first
	rootCat := &models.Category{
		ID:          uuid.New(),
		Name:        c.config.DefaultCategory,
		Description: "Root category",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if err := c.store.CreateCategory(ctx, rootCat); err != nil {
		return nil, fmt.Errorf("failed to create root category: %v", err)
	}

	cs.AddCategory(c.config.DefaultCategory, rootCat)

	// Create a new collector for structure mapping
	mapper := colly.NewCollector(
		colly.AllowedDomains(c.config.AllowedDomains...),
		colly.UserAgent(c.config.UserAgent),
	)

	mapper.OnHTML("nav.sidenav-wrapper", func(e *colly.HTMLElement) {
		log.Println("Mapping navigation structure...")

		// Process each level of navigation
		e.ForEach("ul.sidenav li", func(_ int, s *colly.HTMLElement) {
			text := strings.TrimSpace(s.ChildText("a"))

			// Skip empty items
			if text == "" {
				return
			}

			// Build category path
			var pathParts []string
			pathParts = append(pathParts, c.config.DefaultCategory)

			// Find all parent categories
			parentElements := s.DOM.Parents().Filter("li")
			parentElements.Each(func(i int, parent *goquery.Selection) {
				if parentText := strings.TrimSpace(parent.Find("a").First().Text()); parentText != "" {
					// Insert at beginning since we're going from child to parent
					pathParts = append([]string{parentText}, pathParts...)
				}
			})

			pathParts = append(pathParts, text)

			// Create category path
			categoryPath := strings.Join(pathParts, ":")
			log.Printf("Found category: %s", categoryPath)

			// Create category
			cat := &models.Category{
				ID:          uuid.New(),
				Name:        text,
				ParentID:    &rootCat.ID,
				Description: fmt.Sprintf("Category: %s", categoryPath),
				CreatedAt:   time.Now(),
				UpdatedAt:   time.Now(),
			}

			if err := c.store.CreateCategory(ctx, cat); err != nil {
				log.Printf("Error creating category %s: %v", categoryPath, err)
				return
			}

			cs.AddCategory(categoryPath, cat)
		})
	})
	// Visit the map URL
	if err := mapper.Visit(c.config.MapURL); err != nil {
		return nil, fmt.Errorf("failed to map structure: %v", err)
	}

	return cs, nil
}

func (c *Crawler) setupHandlers(cs *CategoryStructure) {
	// Handle article pages
	c.collector.OnHTML("html", func(e *colly.HTMLElement) {
		log.Printf("Processing URL: %s", e.Request.URL.String())

		// Extract meta tags first
		var tags []string
		e.ForEach("meta[name=ProductFeatureTags]", func(_ int, s *colly.HTMLElement) {
			content := s.Attr("content")
			if content != "" {
				// Split content by comma and clean up each tag
				tagList := strings.Split(content, ",")
				for _, tag := range tagList {
					tag = strings.TrimSpace(tag)
					if tag != "" {
						tags = append(tags, tag)
					}
				}
				log.Printf("Found meta tags: %v", tags)
			}
		})

		// Process main content
		e.ForEach("#mc-main-content", func(_ int, el *colly.HTMLElement) {
			// Extract path components
			var categoryPath []string
			categoryPath = append(categoryPath, c.config.DefaultCategory)

			// Try to get category from navigation
			e.ForEach(".sidenav li.is-selected", func(_ int, s *colly.HTMLElement) {
				if text := strings.TrimSpace(s.Text); text != "" {
					categoryPath = append(categoryPath, text)
				}
			})

			// Create category string
			categoryString := strings.Join(categoryPath, ":")
			log.Printf("Category path: %s", categoryString)

			// Get category from mapped structure
			category, exists := cs.GetCategory(categoryString)
			if !exists {
				log.Printf("Category not found for path: %s, using default", categoryString)
				category, _ = cs.GetCategory(c.config.DefaultCategory)
			}

			// Extract title and content
			title := el.ChildText("p.Title-bar")
			if title == "" {
				title = el.ChildText("p.tile-title")
			}
			if title == "" {
				title = el.ChildText("h1")
			}

			// Extract content
			var content string
			el.ForEach(".sidenav_content", func(_ int, s *colly.HTMLElement) {
				if content != "" {
					content += "\n\n"
				}
				content += s.Text
			})

			if content == "" {
				content = el.Text
			}

			// Create metadata with additional meta tag info
			metadata := map[string]interface{}{
				"categoryPath":       categoryPath,
				"fullCategoryString": categoryString,
				"url":                e.Request.URL.String(),
				"version":            el.ChildText(".mc-variable.General\\.Version"),
				"metaTags":           tags,
			}

			metadataJSON, err := json.Marshal(metadata)
			if err != nil {
				log.Printf("Error marshaling metadata: %v", err)
			}

			// Create article
			article := &models.Article{
				ID:         uuid.New(),
				CategoryID: category.ID,
				Name:       title,
				Body:       content,
				URL:        e.Request.URL.String(),
				Tags:       tags, // Use meta tags instead of category path
				Metadata:   (*json.RawMessage)(&metadataJSON),
				CreatedAt:  time.Now(),
				UpdatedAt:  time.Now(),
			}

			if err := c.store.CreateArticle(context.Background(), article); err != nil {
				log.Printf("Error saving article: %v", err)
			} else {
				log.Printf("Successfully saved article: %s with category path: %s and tags: %v",
					title, categoryString, tags)
			}
		})
	})
}

func (c *Crawler) Crawl(ctx context.Context, cs *CategoryStructure) error {
	c.setupHandlers(cs)

	// Parse sitemap
	sitemap, err := parseSitemap(c.config.SitemapURL)
	if err != nil {
		return err
	}

	// Create a new context with timeout
	crawlCtx, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()

	// Create a channel to signal completion
	done := make(chan bool)

	go func() {
		// Visit each URL from sitemap
		for idx, url := range sitemap.URLs {
			select {
			case <-crawlCtx.Done():
				return
			default:
				log.Printf("Processing URL %d/%d: %s", idx+1, len(sitemap.URLs), url.Loc)
				if err := c.collector.Visit(url.Loc); err != nil {
					log.Printf("Error visiting %s: %v", url.Loc, err)
				}
			}
		}
		done <- true
	}()

	// Wait for either completion or context cancellation
	select {
	case <-crawlCtx.Done():
		return crawlCtx.Err()
	case <-done:
		return nil
	}
}
