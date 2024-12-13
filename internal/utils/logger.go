package utils

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type CrawlerLogger struct {
	file       *os.File
	logger     *log.Logger
	multiWrite io.Writer
}

func NewCrawlerLogger(productName string) (*CrawlerLogger, error) {
	// Sanitize product name for file system
	sanitizedProduct := strings.ReplaceAll(strings.ToLower(productName), " ", "_")

	// Create logs directory if it doesn't exist
	logsDir := "logs"
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create logs directory: %w", err)
	}

	// Create product directory inside logs
	productDir := filepath.Join(logsDir, sanitizedProduct)
	if err := os.MkdirAll(productDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create product directory: %w", err)
	}

	// Create log file with timestamp
	timestamp := time.Now().Format("2006-01-02_15-04-05")
	logPath := filepath.Join(productDir, fmt.Sprintf("crawl_%s_%s.log", sanitizedProduct, timestamp))

	file, err := os.Create(logPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create log file: %w", err)
	}

	// Create multi-writer for both file and stdout
	multiWrite := io.MultiWriter(os.Stdout, file)
	logger := log.New(multiWrite, "", log.Ldate|log.Ltime|log.Lmicroseconds)

	return &CrawlerLogger{
		file:       file,
		logger:     logger,
		multiWrite: multiWrite,
	}, nil
}

func (cl *CrawlerLogger) LogInfo(format string, v ...interface{}) {
	cl.log("INFO", format, v...)
}

func (cl *CrawlerLogger) LogError(format string, v ...interface{}) {
	cl.log("ERROR", format, v...)
}

func (cl *CrawlerLogger) LogDebug(format string, v ...interface{}) {
	cl.log("DEBUG", format, v...)
}

func (cl *CrawlerLogger) log(level string, format string, v ...interface{}) {
	message := fmt.Sprintf(format, v...)
	cl.logger.Printf("[%s] %s", level, message)
}

func (cl *CrawlerLogger) Close() error {
	return cl.file.Close()
}
