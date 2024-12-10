package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/romangod6/kb-crawler/config"
	"github.com/romangod6/kb-crawler/internal/api"
	"github.com/romangod6/kb-crawler/internal/crawler"
	"github.com/romangod6/kb-crawler/internal/storage"
)

func main() {
	// Load configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Initialize storage
	store, err := storage.NewPostgresStore(cfg.Database.URL)
	if err != nil {
		log.Fatalf("Failed to initialize storage: %v", err)
	}
	defer store.Close()

	// Initialize database tables
	if err := store.Initialize(); err != nil {
		log.Fatalf("Failed to initialize database tables: %v", err)
	}

	// Initialize crawler
	crawlerConfig := &crawler.CrawlerConfig{
		SitemapURL:     cfg.Crawler.SitemapURL,
		UserAgent:      cfg.Crawler.UserAgent,
		MaxDepth:       cfg.Crawler.MaxDepth,
		AllowedDomains: cfg.Crawler.AllowedDomains,
	}

	c := crawler.NewCrawler(store, crawlerConfig)

	// Initialize API server
	server := api.NewServer(cfg.Server.Port, store)

	// Setup graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start API server in a goroutine
	go func() {
		log.Printf("Starting API server on port %d", cfg.Server.Port)
		if err := server.Start(); err != nil && err != http.ErrServerClosed {
			log.Printf("Server error: %v", err)
			cancel()
		}
	}()

	// Start crawler in a goroutine
	ticker := time.NewTicker(cfg.GetCrawlDuration())
	go func() {
		// Initial crawl
		if err := c.Crawl(ctx); err != nil {
			log.Printf("Initial crawl error: %v", err)
		}

		for {
			select {
			case <-ticker.C:
				if err := c.Crawl(ctx); err != nil {
					log.Printf("Crawl error: %v", err)
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	// Wait for shutdown signal
	<-sigChan
	log.Println("Shutting down...")

	// Cleanup
	ticker.Stop()
	cancel()

	// Give the server a chance to shutdown gracefully
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("Server shutdown error: %v", err)
	}

	log.Println("Shutdown complete")
}
