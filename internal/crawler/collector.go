// internal/crawler/collector.go
package crawler

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gocolly/colly/v2"
	"github.com/google/uuid"
	"github.com/romangod6/kb-crawler/internal/models"
	"github.com/romangod6/kb-crawler/internal/storage"
	"github.com/romangod6/kb-crawler/internal/utils"
)

// Crawler represents the web crawler with its dependencies.
type Crawler struct {
	collector *colly.Collector
	store     storage.Store
	config    *CrawlerConfig
}

// CrawlerConfig holds the configuration parameters for the crawler.
type CrawlerConfig struct {
	SitemapURL      string
	MapURL          string
	UserAgent       string
	MaxDepth        int
	DefaultCategory string
	AllowedDomains  []string
}

// CategoryStructure holds the mapped categories with thread-safe access.
type CategoryStructure struct {
	categories map[string]*models.Category
	mutex      sync.RWMutex
}

// NewCategoryStructure initializes a new CategoryStructure.
func NewCategoryStructure() *CategoryStructure {
	return &CategoryStructure{
		categories: make(map[string]*models.Category),
	}
}

// AddCategory adds a new category to the structure.
func (cs *CategoryStructure) AddCategory(path string, category *models.Category) {
	cs.mutex.Lock()
	defer cs.mutex.Unlock()
	cs.categories[path] = category
}

// GetCategory retrieves a category by its path.
func (cs *CategoryStructure) GetCategory(path string) (*models.Category, bool) {
	cs.mutex.RLock()
	defer cs.mutex.RUnlock()
	cat, exists := cs.categories[path]
	return cat, exists
}

// ContextKey is a type for context keys to avoid collisions.
type ContextKey string

const tagsContextKey = "crawler_product_feature_tags"

// NewCrawler initializes and returns a new Crawler instance.
func NewCrawler(store storage.Store, config *CrawlerConfig) *Crawler {
	logger, _ := utils.NewCrawlerLogger(config.DefaultCategory)

	// Create collector with extended configuration
	c := colly.NewCollector(
		colly.UserAgent(config.UserAgent),
		colly.MaxDepth(config.MaxDepth),
		colly.AllowedDomains(config.AllowedDomains...),
		colly.AllowURLRevisit(),
		colly.Async(true),
	)

	// Configure transport
	c.WithTransport(&http.Transport{
		DisableKeepAlives: true,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	})

	// Set timeouts
	c.SetRequestTimeout(30 * time.Second)

	// Debug callback to see what we're receiving
	c.OnResponse(func(r *colly.Response) {
		logger.LogDebug("Raw HTML preview for %s:\n%s", r.Request.URL, string(r.Body[:min(1000, len(r.Body))]))
	})

	// Error handling
	c.OnError(func(r *colly.Response, err error) {
		logger.LogError("Error on %v: %v", r.Request.URL, err)
		if r != nil {
			logger.LogError("Response headers: %v", r.Headers)
		}
	})

	// Set up rate limiting
	c.Limit(&colly.LimitRule{
		DomainGlob:  "*",
		RandomDelay: 2 * time.Second,
		Parallelism: 2,
	})

	return &Crawler{
		collector: c,
		store:     store,
		config:    config,
	}
}

// MapCategoryStructure maps the category structure from the MapURL.
func (c *Crawler) MapCategoryStructure(ctx context.Context) (*CategoryStructure, error) {
	logger, _ := utils.NewCrawlerLogger(c.config.DefaultCategory)
	cs := NewCategoryStructure()

	// Create root category
	rootCat := &models.Category{
		ID:          uuid.New(),
		Name:        c.config.DefaultCategory,
		Description: "Root category",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if err := c.store.CreateCategory(ctx, rootCat); err != nil {
		logger.LogError("Failed to create root category: %v", err)
		return nil, fmt.Errorf("failed to create root category: %w", err)
	}

	cs.AddCategory(c.config.DefaultCategory, rootCat)

	// Create a new collector specifically for structure mapping
	mapper := colly.NewCollector(
		colly.AllowedDomains(c.config.AllowedDomains...),
		colly.UserAgent(c.config.UserAgent),
		colly.AllowURLRevisit(),
	)

	// Configure transport for mapper
	mapper.WithTransport(&http.Transport{
		DisableKeepAlives: true,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	})

	// Set timeouts for mapper
	mapper.SetRequestTimeout(30 * time.Second)

	// Debug response
	mapper.OnResponse(func(r *colly.Response) {
		logger.LogDebug("Mapping response from %s", r.Request.URL)
		logger.LogDebug("Response body preview: %s", string(r.Body[:min(1000, len(r.Body))]))
	})

	mapper.OnHTML("nav.sidebarNav", func(e *colly.HTMLElement) {
		logger.LogInfo("Found navigation structure")

		e.ForEach("li", func(_ int, el *colly.HTMLElement) {
			text := strings.TrimSpace(el.Text)
			if text == "" {
				return
			}

			// Build category path
			var pathParts []string
			pathParts = append(pathParts, c.config.DefaultCategory)

			// Traverse up to find parent categories
			current := el.DOM
			for parent := current.Parent(); parent != nil; parent = parent.Parent() {
				// Check if this parent is a list item
				if parent.Is("li") {
					parentText := strings.TrimSpace(parent.Text())
					if parentText != "" {
						pathParts = append([]string{parentText}, pathParts...)
					}
				}
			}

			pathParts = append(pathParts, text)
			categoryPath := strings.Join(pathParts, ":")

			logger.LogInfo("Found category: %s", categoryPath)

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
				logger.LogError("Error creating category %s: %v", categoryPath, err)
				return
			}

			cs.AddCategory(categoryPath, cat)
			logger.LogInfo("Added category to structure: %s", categoryPath)
		})
	})

	// Visit the map URL
	logger.LogInfo("Visiting map URL: %s", c.config.MapURL)
	if err := mapper.Visit(c.config.MapURL); err != nil {
		logger.LogError("Failed to visit map URL: %v", err)
		return nil, fmt.Errorf("failed to map structure: %w", err)
	}

	// Wait for async operations to finish
	mapper.Wait()

	logger.LogInfo("Category mapping completed. Total categories: %d", len(cs.categories))
	return cs, nil
}

// Crawl starts the crawling process using the mapped category structure.
func (c *Crawler) Crawl(ctx context.Context, cs *CategoryStructure) error {
	// Setup content handlers first
	c.setupHandlers(cs)

	logger, err := utils.NewCrawlerLogger(c.config.DefaultCategory)
	if err != nil {
		log.Printf("Failed to create logger: %v", err)
	} else {
		defer logger.Close()
	}

	logMsg := func(level string, format string, v ...interface{}) {
		if logger != nil {
			switch level {
			case "error":
				logger.LogError(format, v...)
			case "debug":
				logger.LogDebug(format, v...)
			default:
				logger.LogInfo(format, v...)
			}
		} else {
			log.Printf(format, v...)
		}
	}

	// Add collector callbacks for logging
	c.collector.OnRequest(func(r *colly.Request) {
		logMsg("info", "Visiting: %s", r.URL.String())
	})

	c.collector.OnError(func(r *colly.Response, err error) {
		logMsg("error", "Error visiting %s: %v", r.Request.URL.String(), err)
	})

	c.collector.OnResponse(func(r *colly.Response) {
		logMsg("info", "Received response from %s: Status %d", r.Request.URL.String(), r.StatusCode)
	})

	// Parse sitemap
	sitemap, err := parseSitemap(c.config.SitemapURL)
	if err != nil {
		logMsg("error", "Failed to parse sitemap: %v", err)
		return err
	}

	logMsg("info", "Successfully parsed sitemap, found %d URLs", len(sitemap.URLs))

	// Visit each URL from sitemap
	for idx, url := range sitemap.URLs {
		select {
		case <-ctx.Done():
			logMsg("info", "Context cancelled, stopping crawl")
			return ctx.Err()
		default:
			logMsg("info", "Processing URL %d/%d: %s", idx+1, len(sitemap.URLs), url.Loc)
			if err := c.collector.Visit(url.Loc); err != nil {
				logMsg("error", "Error visiting %s: %v", url.Loc, err)
			}
		}
	}

	// Wait for async operations to finish
	c.collector.Wait()

	logMsg("info", "Crawl completed successfully")
	return nil
}

const TagsContextKey = "crawler_product_feature_tags"

// setupHandlers sets up the HTML handlers for the collector.
func (c *Crawler) setupHandlers(cs *CategoryStructure) {
	logger, _ := utils.NewCrawlerLogger(c.config.DefaultCategory)

	// Handler for the <head> section to extract meta tags
	c.collector.OnHTML("head", func(e *colly.HTMLElement) {
		logger.LogDebug("Processing <head> section for meta tags")
		var tags []string

		// Extract ProductFeatureTags using parser.go's function
		ExtractProductFeatureTags(e, &tags, logger)

		// Store tags in request context
		e.Request.Ctx.Put("tags", tags)
	})

	// Handler for main content selectors
	pageHandlers := []string{"#mc-main-content", "div[role='main']", "body"}
	for _, selector := range pageHandlers {
		c.collector.OnHTML(selector, func(e *colly.HTMLElement) {
			logger.LogDebug("Processing selector: %s", selector)

			// Initialize empty tags slice
			tags := make([]string, 0)

			// Get tags from context if they exist
			if storedTags := e.Request.Ctx.GetAny("tags"); storedTags != nil {
				if tagList, ok := storedTags.([]string); ok {
					tags = tagList
					logger.LogDebug("Retrieved %d tags from context", len(tags))
				}
			}

			// Extract and parse the raw HTML content
			rawHTMLBytes := e.Response.Body
			parsedContent, err := ParseHTMLContent(string(rawHTMLBytes))
			if err != nil {
				logger.LogError("Error parsing HTML content: %v", err)
				return
			}

			// Assign the extracted tags if not already assigned
			if len(parsedContent.Tags) > 0 && len(tags) == 0 {
				tags = parsedContent.Tags
			}

			// Determine the category path
			var categoryPath []string
			categoryPath = append(categoryPath, c.config.DefaultCategory)

			navSelectors := []string{
				".sidenav li.is-selected",
				".breadcrumbs li",
				".navigation .selected",
				"nav .mc-breadcrumb li",
			}

			for _, navSelector := range navSelectors {
				e.ForEach(navSelector, func(_ int, s *colly.HTMLElement) {
					if text := strings.TrimSpace(s.Text); text != "" {
						categoryPath = append(categoryPath, text)
						logger.LogInfo("Found category component: %s", text)
					}
				})
			}

			// Construct the category string
			categoryString := strings.Join(categoryPath, ":")
			logger.LogInfo("Category path: %s", categoryString)

			// Retrieve the category from the structure
			category, exists := cs.GetCategory(categoryString)
			if !exists {
				logger.LogInfo("Category not found for path: %s, using default", categoryString)
				category, exists = cs.GetCategory(c.config.DefaultCategory)
				if !exists {
					logger.LogError("Default category not found!")
					return
				}
			}

			if parsedContent.Title == "" || parsedContent.Content == "" {
				logger.LogError("Missing required content - Title found: %v, Content found: %v",
					parsedContent.Title != "", parsedContent.Content != "")
				return
			}

			// Create metadata
			metadata := map[string]interface{}{
				"categoryPath":       categoryPath,
				"fullCategoryString": categoryString,
				"url":                e.Request.URL.String(),
				"metaTags":           tags,
			}

			metadataJSON, err := json.Marshal(metadata)
			if err != nil {
				logger.LogError("Error marshaling metadata: %v", err)
				return
			}

			// Create article
			article := &models.Article{
				ID:         uuid.New(),
				CategoryID: category.ID,
				Name:       parsedContent.Title,
				Body:       parsedContent.Content,
				URL:        e.Request.URL.String(),
				Tags:       tags,
				Metadata:   (*json.RawMessage)(&metadataJSON),
				CreatedAt:  time.Now(),
				UpdatedAt:  time.Now(),
			}

			logger.LogInfo("Attempting to save article: %s", parsedContent.Title)
			if err := c.store.CreateArticle(context.Background(), article); err != nil {
				logger.LogError("Error saving article: %v", err)
			} else {
				logger.LogInfo("Successfully saved article: %s with category path: %s and tags: %v",
					parsedContent.Title, categoryString, tags)
			}
		})
	}
}

// min returns the smaller of two integers.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// runCrawler is the main entry point to start the crawling process.
func (h *Crawler) runCrawler(config models.CrawlerConfig) error {

	crawler := NewCrawler(h.store, &CrawlerConfig{
		SitemapURL:      config.SitemapURL,
		UserAgent:       config.UserAgent,
		MaxDepth:        config.MaxDepth,
		AllowedDomains:  config.AllowedDomains,
		DefaultCategory: config.DefaultCategory,
	})

	categoryStructure, err := crawler.MapCategoryStructure(context.Background())
	if err != nil {
		return fmt.Errorf("failed to map category structure: %w", err)
	}

	// Start crawling
	err = crawler.Crawl(context.Background(), categoryStructure)
	if err != nil {
		return fmt.Errorf("crawl failed: %w", err)
	}

	return nil
}

// parseSitemap parses the sitemap XML from the given URL and returns a Sitemap structure.
func parseSitemap(url string) (*models.Sitemap, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch sitemap: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("sitemap fetch returned status: %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read sitemap body: %w", err)
	}

	var sitemap models.Sitemap
	if err := xml.Unmarshal(body, &sitemap); err != nil {
		return nil, fmt.Errorf("failed to parse sitemap XML: %w", err)
	}

	return &sitemap, nil
}
