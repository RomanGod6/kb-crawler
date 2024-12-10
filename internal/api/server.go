package api

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/romangod6/kb-crawler/internal/storage"
)

type Server struct {
	router *gin.Engine
	port   int
	server *http.Server
}

func NewServer(port int, store storage.Store) *Server {
	router := gin.Default()

	// Setup CORS
	router.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	// Create handler
	handler := NewHandler(store)

	// Setup routes
	api := router.Group("/api")
	{
		// Health check
		api.GET("/health", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"status": "healthy"})
		})

		// Articles routes
		articles := api.Group("/articles")
		{
			articles.GET("", handler.ListArticles)
			articles.GET("/:id", handler.GetArticle)
			articles.GET("/search", handler.SearchArticles)
		}

		// Categories routes
		categories := api.Group("/categories")
		{
			categories.GET("", handler.ListCategories)
			categories.GET("/:id", handler.GetCategory)
			categories.GET("/:id/articles", handler.GetArticlesByCategory)
		}
	}

	return &Server{
		router: router,
		port:   port,
	}
}

func (s *Server) Start() error {
	s.server = &http.Server{
		Addr:         fmt.Sprintf(":%d", s.port),
		Handler:      s.router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	return s.server.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	if s.server != nil {
		return s.server.Shutdown(ctx)
	}
	return nil
}
