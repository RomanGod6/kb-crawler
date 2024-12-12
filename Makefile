.PHONY: watch analyze clean map help install dev-backend dev-frontend dev

# Default target
all: help

# Install dependencies
install:
	go mod download
	cd frontend && npm install

# Run the backend server
dev-backend:
	export CGO_ENABLED=0 && go run cmd/crawler/main.go

# Run the frontend development server
dev-frontend:
	cd frontend && npm run dev

# Run both frontend and backend
watch:
	make dev-backend & make dev-frontend

# Run sitemap analyzer
analyze:
	go run tools/sitemap/sitemap_analyzer.go

# Run category mapper
map:
	go run tools/category/category_mapper.go

# Clean generated files
clean:
	go clean
	rm -f kb_crawler.db
	cd frontend && rm -rf node_modules dist

# Help information
help:
	@echo "Available commands:"
	@echo "  make install     - Install all dependencies (Go and Node.js)"
	@echo "  make dev        - Run both frontend and backend"
	@echo "  make dev-backend - Run only the backend server"
	@echo "  make dev-frontend - Run only the frontend server"
	@echo "  make analyze    - Run the sitemap analyzer"
	@echo "  make map        - Run the category structure mapper"
	@echo "  make clean      - Clean up generated files"