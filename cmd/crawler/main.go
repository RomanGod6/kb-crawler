package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/romangod6/kb-crawler/config"
	"github.com/romangod6/kb-crawler/internal/api"
	"github.com/romangod6/kb-crawler/internal/crawler"
	"github.com/romangod6/kb-crawler/internal/models"
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

	// Initialize API server
	server := api.NewServer(cfg.Server.Port, store)

	// Setup periodic crawling
	ticker := time.NewTicker(cfg.GetCrawlDuration())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		for {
			select {
			case <-ticker.C:
				log.Println("Starting periodic crawl...")
				runAllCrawls(ctx, store, cfg.Crawler.MaxConcurrentCrawls)
			case <-ctx.Done():
				return
			}
		}
	}()

	// Start the API server
	go func() {
		log.Printf("Starting API server on port %d", cfg.Server.Port)
		if err := server.Start(); err != nil {
			log.Fatalf("Failed to start API server: %v", err)
		}
	}()

	// Wait for shutdown
	waitForShutdown(cancel, server)
}

func runAllCrawls(ctx context.Context, store storage.Store, maxConcurrentCrawls int) {
	// Fetch all crawler configs
	crawlerConfigs, err := store.ListCrawlerConfigs(ctx)
	if err != nil {
		log.Printf("Failed to fetch crawler configs: %v", err)
		return
	}

	if len(crawlerConfigs) == 0 {
		log.Println("No crawlers to run")
		return
	}

	now := time.Now()

	// Create a semaphore channel to limit concurrency
	semaphore := make(chan struct{}, maxConcurrentCrawls)
	wg := sync.WaitGroup{}

	for _, config := range crawlerConfigs {
		// Skip if crawler is already running
		if config.Status == "Running" {
			log.Printf("Skipping crawler %s (%s) as it's already running", config.Product, config.ID)
			continue
		}

		// Skip if it's not time to run yet
		if config.NextRun != nil && now.Before(*config.NextRun) {
			log.Printf("Skipping crawler %s (%s) as it's not scheduled yet", config.Product, config.ID)
			continue
		}

		configCopy := *config // Dereference the pointer to create a value copy
		wg.Add(1)

		// Acquire a spot in the semaphore
		semaphore <- struct{}{}

		go func(cfg models.CrawlerConfig) {
			defer wg.Done()
			defer func() { <-semaphore }() // Release the spot in the semaphore

			log.Printf("Starting crawl for %s...", cfg.SitemapURL)

			// Create and run the crawler
			c := crawler.NewCrawler(store, &crawler.CrawlerConfig{
				SitemapURL:      cfg.SitemapURL,
				UserAgent:       cfg.UserAgent,
				MaxDepth:        cfg.MaxDepth,
				AllowedDomains:  cfg.AllowedDomains,
				DefaultCategory: cfg.DefaultCategory,
			})

			cs, err := c.MapCategoryStructure(ctx)
			if err != nil {
				log.Printf("Failed to map category structure for %s: %v", cfg.SitemapURL, err)
				return
			}

			if err := c.Crawl(ctx, cs); err != nil {
				log.Printf("Crawl failed for %s: %v", cfg.SitemapURL, err)
			} else {
				log.Printf("Crawl completed for %s", cfg.SitemapURL)
			}
		}(configCopy)
	}

	wg.Wait() // Wait for all workers to finish
	log.Println("All crawls completed")
}
func waitForShutdown(cancel context.CancelFunc, server *api.Server) {
	// Handle system signals for shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	<-sigChan
	log.Println("Shutting down...")
	cancel()

	// Graceful server shutdown
	ctx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Printf("Error shutting down server: %v", err)
	}
	log.Println("Server shut down gracefully")
}
