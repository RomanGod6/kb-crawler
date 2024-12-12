package api

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/romangod6/kb-crawler/internal/crawler"
	"github.com/romangod6/kb-crawler/internal/models"
	"github.com/romangod6/kb-crawler/internal/storage"
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

	// Set default status if not provided
	if config.Status == "" {
		config.Status = "stopped"
	}

	if err := h.store.CreateCrawlerConfig(c.Request.Context(), &config); err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Failed to create crawler config"})
		return
	}

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
	crawlerInstance := crawler.NewCrawler(h.store, &crawler.CrawlerConfig{
		SitemapURL:      config.SitemapURL,
		UserAgent:       config.UserAgent,
		MaxDepth:        config.MaxDepth,
		AllowedDomains:  config.AllowedDomains,
		DefaultCategory: config.DefaultCategory,
	})

	categoryStructure, err := crawlerInstance.MapCategoryStructure(context.Background())
	if err != nil {
		return fmt.Errorf("failed to map category structure: %w", err)
	}

	err = crawlerInstance.Crawl(context.Background(), categoryStructure)
	if err != nil {
		return fmt.Errorf("crawl failed: %w", err)
	}

	return nil
}
