package api

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/romangod6/kb-crawler/internal/crawler"
	"github.com/romangod6/kb-crawler/internal/models"
	"github.com/romangod6/kb-crawler/internal/storage"
	"github.com/romangod6/kb-crawler/internal/utils"
)

type Handler struct {
	store storage.Store
}

type ErrorResponse struct {
	Error string `json:"error"`
}

type PaginationResponse struct {
	Data       interface{} `json:"data"`
	Page       int         `json:"page"`
	Limit      int         `json:"limit"`
	TotalCount int         `json:"total_count,omitempty"`
}

func NewHandler(store storage.Store) *Handler {
	return &Handler{store: store}
}

// Existing handlers
func (h *Handler) ListArticles(c *gin.Context) {
	page, limit := getPaginationParams(c)
	offset := (page - 1) * limit

	articles, err := h.store.ListArticles(c.Request.Context(), limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Failed to fetch articles"})
		return
	}

	c.JSON(http.StatusOK, PaginationResponse{
		Data:  articles,
		Page:  page,
		Limit: limit,
	})
}

func (h *Handler) GetArticle(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid article ID"})
		return
	}

	article, err := h.store.GetArticle(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Failed to fetch article"})
		return
	}

	if article == nil {
		c.JSON(http.StatusNotFound, ErrorResponse{Error: "Article not found"})
		return
	}

	c.JSON(http.StatusOK, article)
}

func (h *Handler) SearchArticles(c *gin.Context) {
	query := c.Query("q")
	if query == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Search query is required"})
		return
	}

	page, limit := getPaginationParams(c)
	offset := (page - 1) * limit

	articles, err := h.store.SearchArticles(c.Request.Context(), query, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Failed to search articles"})
		return
	}

	c.JSON(http.StatusOK, PaginationResponse{
		Data:  articles,
		Page:  page,
		Limit: limit,
	})
}

func (h *Handler) ListCategories(c *gin.Context) {
	categories, err := h.store.ListCategories(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Failed to fetch categories"})
		return
	}

	c.JSON(http.StatusOK, categories)
}

func (h *Handler) GetCategory(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid category ID"})
		return
	}

	category, err := h.store.GetCategory(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Failed to fetch category"})
		return
	}

	if category == nil {
		c.JSON(http.StatusNotFound, ErrorResponse{Error: "Category not found"})
		return
	}

	c.JSON(http.StatusOK, category)
}

func (h *Handler) GetArticlesByCategory(c *gin.Context) {
	categoryID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid category ID"})
		return
	}

	page, limit := getPaginationParams(c)
	offset := (page - 1) * limit

	articles, err := h.store.GetArticlesByCategory(c.Request.Context(), categoryID, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Failed to fetch articles"})
		return
	}

	c.JSON(http.StatusOK, PaginationResponse{
		Data:  articles,
		Page:  page,
		Limit: limit,
	})
}

// New Crawler Config Handlers
func (h *Handler) ListCrawlerConfigs(c *gin.Context) {
	configs, err := h.store.ListCrawlerConfigs(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Failed to fetch crawler configs"})
		return
	}

	if configs == nil {
		configs = []*models.CrawlerConfig{}
	}

	c.JSON(http.StatusOK, configs)
}

func (h *Handler) GetCrawlerConfig(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid crawler config ID"})
		return
	}

	config, err := h.store.GetCrawlerConfig(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Failed to fetch crawler config"})
		return
	}

	if config == nil {
		c.JSON(http.StatusNotFound, ErrorResponse{Error: "Crawler config not found"})
		return
	}

	c.JSON(http.StatusOK, config)
}

func (h *Handler) CreateCrawlerConfig(c *gin.Context) {
	var config models.CrawlerConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid crawler config data"})
		return
	}

	// Generate new UUID if not provided
	if config.ID == uuid.Nil {
		config.ID = uuid.New()
	}

	// Set initial values for new crawler config
	now := time.Now()
	config.Status = "Running"
	config.IsFirstRun = true
	config.CreatedAt = now
	config.UpdatedAt = now

	// Calculate next scheduled run based on crawl interval
	if interval, err := time.ParseDuration(config.CrawlInterval); err == nil {
		nextRun := now.Add(interval)
		config.NextRun = &nextRun
	}

	if err := h.store.CreateCrawlerConfig(c.Request.Context(), &config); err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Failed to create crawler config"})
		return
	}

	// Start the crawler in a goroutine
	go func() {
		log.Printf("Starting crawler for config ID: %s", config.ID)
		if err := h.runCrawler(config); err != nil {
			log.Printf("Error running crawler: %v", err)
			config.Status = "Error"
			config.Errors = append(config.Errors, err.Error())
		} else {
			config.Status = "Completed"
		}

		// Update the config status
		if err := h.store.UpdateCrawlerConfig(context.Background(), &config); err != nil {
			log.Printf("Error updating crawler status: %v", err)
		}
	}()

	c.JSON(http.StatusCreated, config)
}

func (h *Handler) UpdateCrawlerConfig(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid crawler config ID"})
		return
	}

	var config models.CrawlerConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid crawler config data"})
		return
	}

	config.ID = id

	if err := h.store.UpdateCrawlerConfig(c.Request.Context(), &config); err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Failed to update crawler config"})
		return
	}

	c.JSON(http.StatusOK, config)
}

func (h *Handler) DeleteCrawlerConfig(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid crawler config ID"})
		return
	}

	if err := h.store.DeleteCrawlerConfig(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Failed to delete crawler config"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}

// Utility functions
func getPaginationParams(c *gin.Context) (page, limit int) {
	page, _ = strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ = strconv.Atoi(c.DefaultQuery("limit", "10"))

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 10
	}

	return page, limit
}
func (h *Handler) StartCrawl(c *gin.Context) {
	var crawlConfig models.CrawlerConfig
	if err := c.ShouldBindJSON(&crawlConfig); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid request payload"})
		return
	}

	// Generate new UUID for the crawl
	crawlConfig.ID = uuid.New()
	crawlConfig.Status = "Running"
	crawlConfig.CreatedAt = time.Now()
	crawlConfig.UpdatedAt = time.Now()

	// Save the crawl config in the database
	if err := h.store.CreateCrawlerConfig(c.Request.Context(), &crawlConfig); err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Failed to save crawl configuration"})
		return
	}

	// Start the crawl in a Goroutine
	go func(config models.CrawlerConfig) {
		log.Printf("Starting crawl for %s...", config.SitemapURL)

		// Update logs and status during crawling
		err := h.runCrawler(config)
		if err != nil {
			log.Printf("Crawl failed for %s: %v", config.SitemapURL, err)
			config.Status = "Error"
			config.Errors = append(config.Errors, err.Error())
		} else {
			log.Printf("Crawl completed for %s", config.SitemapURL)
			config.Status = "Stopped"
		}

		config.UpdatedAt = time.Now()
		if updateErr := h.store.UpdateCrawlerConfig(context.Background(), &config); updateErr != nil {
			log.Printf("Failed to update crawl config: %v", updateErr)
		}
	}(crawlConfig)

	c.JSON(http.StatusAccepted, crawlConfig)
}
func (h *Handler) runCrawler(config models.CrawlerConfig) error {
	// Create logger for this crawl
	logger, err := utils.NewCrawlerLogger(config.Product)
	if err != nil {
		log.Printf("Failed to create logger: %v", err)
		return fmt.Errorf("failed to create logger: %w", err)
	}
	defer logger.Close()

	logger.LogInfo("Starting crawler for %s (ID: %s)", config.Product, config.ID)
	logger.LogInfo("Configuration details:")
	logger.LogInfo("  Sitemap URL: %s", config.SitemapURL)
	logger.LogInfo("  Map URL: %s", config.MapURL)
	logger.LogInfo("  Max Depth: %d", config.MaxDepth)
	logger.LogInfo("  User Agent: %s", config.UserAgent)
	logger.LogInfo("  Crawl Interval: %s", config.CrawlInterval)
	logger.LogInfo("  Default Category: %s", config.DefaultCategory)
	logger.LogInfo("  Allowed Domains: %v", config.AllowedDomains)

	// Set default MapURL if not provided
	if config.MapURL == "" {
		config.MapURL = strings.Replace(config.SitemapURL, "Sitemap.xml", "0HOME/Home.htm", 1)
		logger.LogInfo("Set default Map URL to: %s", config.MapURL)
	}

	crawlerInstance := crawler.NewCrawler(h.store, &crawler.CrawlerConfig{
		SitemapURL:      config.SitemapURL,
		MapURL:          config.MapURL,
		UserAgent:       config.UserAgent,
		MaxDepth:        config.MaxDepth,
		AllowedDomains:  config.AllowedDomains,
		DefaultCategory: config.DefaultCategory,
	})

	// Update status to Running
	config.Status = "Running"
	if err := h.store.UpdateCrawlerConfig(context.Background(), &config); err != nil {
		logger.LogError("Failed to update crawler status: %v", err)
		return fmt.Errorf("failed to update crawler status: %w", err)
	}

	logger.LogInfo("Beginning category structure mapping...")
	categoryStructure, err := crawlerInstance.MapCategoryStructure(context.Background())
	if err != nil {
		config.Status = "Error"
		config.Errors = append(config.Errors, err.Error())
		h.store.UpdateCrawlerConfig(context.Background(), &config)
		logger.LogError("Failed to map category structure: %v", err)
		return fmt.Errorf("failed to map category structure: %w", err)
	}
	logger.LogInfo("Category structure mapping completed successfully")

	logger.LogInfo("Starting crawl process...")
	err = crawlerInstance.Crawl(context.Background(), categoryStructure)
	now := time.Now()

	if err != nil {
		config.Status = "Error"
		config.Errors = append(config.Errors, err.Error())
		logger.LogError("Crawl failed with error: %v", err)
	} else {
		config.Status = "Completed"
		config.LastRun = &now

		// Calculate next run time
		if interval, err := time.ParseDuration(config.CrawlInterval); err == nil {
			nextRun := now.Add(interval)
			config.NextRun = &nextRun
			logger.LogInfo("Next scheduled run: %v", nextRun)
		}
		logger.LogInfo("Crawl completed successfully")
	}

	config.IsFirstRun = false
	config.UpdatedAt = now

	if updateErr := h.store.UpdateCrawlerConfig(context.Background(), &config); updateErr != nil {
		logger.LogError("Error updating crawler status: %v", updateErr)
	}

	logger.LogInfo("Crawler execution finished. Status: %s", config.Status)
	if len(config.Errors) > 0 {
		logger.LogError("Errors encountered during crawl:")
		for _, err := range config.Errors {
			logger.LogError("  - %s", err)
		}
	}

	if err != nil {
		return fmt.Errorf("crawl failed: %w", err)
	}

	return nil
}
