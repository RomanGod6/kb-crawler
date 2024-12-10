package api

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
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
