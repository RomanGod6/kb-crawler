package crawler

import (
	"context"
	"log"
	"strings"
	"time"

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
	SitemapURL     string
	UserAgent      string
	MaxDepth       int
	AllowedDomains []string
}

func NewCrawler(store storage.Store, config *CrawlerConfig) *Crawler {
	c := colly.NewCollector(
		colly.UserAgent(config.UserAgent),
		colly.MaxDepth(config.MaxDepth),
		colly.AllowedDomains(config.AllowedDomains...),
	)

	// Set reasonable limits for Datto's site
	c.Limit(&colly.LimitRule{
		DomainGlob:  "*",
		Parallelism: 2,
		RandomDelay: 5 * time.Second,
	})

	crawler := &Crawler{
		collector: c,
		store:     store,
		config:    config,
	}

	crawler.setupHandlers()
	return crawler
}

func (c *Crawler) setupHandlers() {
	// Handle article pages
	c.collector.OnHTML("#mc-main-content", func(e *colly.HTMLElement) {
		// Debug logging
		log.Printf("Processing URL: %s", e.Request.URL.String())

		// Extract title
		title := e.ChildText("p.Title-bar")
		if title == "" {
			title = e.ChildText("p.tile-title")
		}
		log.Printf("Found title: %s", title)

		// Extract content from tiles or main content
		var content string
		e.ForEach("p.tile-content", func(_ int, s *colly.HTMLElement) {
			if content != "" {
				content += "\n\n"
			}
			content += s.Text
		})

		// If no tile content, try to get regular content
		if content == "" {
			content = e.Text
		}
		log.Printf("Content length: %d characters", len(content))

		// Extract category from navigation or sidenav
		categoryName := ""
		e.ForEach("ul.navigation li", func(_ int, s *colly.HTMLElement) {
			if s.Attr("class") == "selected" {
				categoryName = s.Text
			}
		})
		if categoryName == "" {
			categoryName = e.ChildText("#skin-heading")
		}
		if categoryName == "" {
			categoryName = "General"
		}
		log.Printf("Found category: %s", categoryName)

		// Create category
		category := &models.Category{
			ID:          uuid.New(),
			Name:        categoryName,
			Description: "",
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		}

		if err := c.store.CreateCategory(context.Background(), category); err != nil {
			log.Printf("Error saving category: %v", err)
			return
		}

		// Extract tags
		var tags []string

		// Add section as a tag
		section := e.ChildText("#skin-heading")
		if section != "" {
			tags = append(tags, section)
		}

		// Extract keywords from meta tags
		e.ForEach("meta[name=keywords]", func(_ int, s *colly.HTMLElement) {
			keywords := strings.Split(s.Attr("content"), ",")
			for _, keyword := range keywords {
				if keyword = strings.TrimSpace(keyword); keyword != "" {
					tags = append(tags, keyword)
				}
			}
		})

		// Create article
		article := &models.Article{
			ID:         uuid.New(),
			CategoryID: category.ID,
			Name:       title,
			Body:       content,
			URL:        e.Request.URL.String(),
			Tags:       tags,
			CreatedAt:  time.Now(),
			UpdatedAt:  time.Now(),
		}

		if err := c.store.CreateArticle(context.Background(), article); err != nil {
			log.Printf("Error saving article: %v", err)
		} else {
			log.Printf("Successfully saved article: %s", title)
		}
	})

	// Handle errors
	c.collector.OnError(func(r *colly.Response, err error) {
		log.Printf("Error crawling %s: %v", r.Request.URL, err)
	})
}

func (c *Crawler) Crawl(ctx context.Context) error {
	// Parse sitemap
	sitemap, err := parseSitemap(c.config.SitemapURL)
	if err != nil {
		return err
	}

	log.Printf("Found %d URLs in sitemap", len(sitemap.URLs))

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
