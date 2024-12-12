package config

import (
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	Database struct {
		URL string
	}
	Server struct {
		Port int
	}
	Crawler struct {
		SitemapURL          string
		MapURL              string
		UserAgent           string
		CrawlInterval       string
		MaxDepth            int
		DefaultCategory     string
		AllowedDomains      []string
		MaxConcurrentCrawls int // Add this
	}
}

func LoadConfig() (*Config, error) {
	viper.SetDefault("crawler.maxconcurrentcrawls", 5)
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	viper.AddConfigPath("./config")

	// Default values
	viper.SetDefault("server.port", 8080)
	viper.SetDefault("crawler.useragent", "KB Crawler Bot v1.0")
	viper.SetDefault("crawler.maxdepth", 10)
	viper.SetDefault("crawler.crawlinterval", "24h")
	viper.SetDefault("crawler.defaultcategory", "Datto RMM")

	if err := viper.ReadInConfig(); err != nil {
		return nil, err
	}

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, err
	}

	return &config, nil
}

func (c *Config) GetCrawlDuration() time.Duration {
	duration, err := time.ParseDuration(c.Crawler.CrawlInterval)
	if err != nil {
		return 24 * time.Hour
	}
	return duration
}
