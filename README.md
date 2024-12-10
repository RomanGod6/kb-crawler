# kb-crawler# Knowledge Base Crawler

A Go-based web crawler specifically designed for crawling and indexing knowledge base content. It includes a REST API for accessing the crawled content.

## Features

- Web crawling using Colly framework
- Sitemap-based URL discovery
- PostgreSQL storage backend
- RESTful API with Gin framework
- Category and article management
- Full-text search capabilities
- Configurable crawling intervals
- Rate limiting and polite crawling

## Prerequisites

- Go 1.21 or higher
- PostgreSQL 12 or higher
- Git (optional)

## Installation

1. Clone the repository:
```bash
git clone https://github.com/yourusername/kb-crawler.git
cd kb-crawler
```

2. Install dependencies:
```bash
go mod download
```

3. Create a config.yaml file in the config directory:
```yaml
database:
  host: "localhost"
  port: 5432
  user: "your_user"
  password: "your_password"
  dbname: "kb_crawler"
  sslmode: "disable"

server:
  port: 8080

crawler:
  sitemapURL: "https://example.com/sitemap.xml"
  userAgent: "KB Crawler Bot v1.0"
  crawlInterval: "24h"
  maxDepth: 10
  allowedDomains:
    - "example.com"
```

## Usage

1. Start the application:
```bash
go run cmd/crawler/main.go
```

2. The API will be available at `http://localhost:8080`

## API Endpoints

- `GET /api/articles` - List all articles (paginated)
- `GET /api/articles/:id` - Get specific article
- `GET /api/articles/search` - Search articles
- `GET /api/categories` - List all categories
- `GET /api/categories/:id` - Get specific category
- `GET /api/categories/:id/articles` - Get articles in category

## Configuration

The application can be configured using environment variables or a config.yaml file. See the config.example.yaml for available options.

## Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## License

This project is licensed under the MIT License - see the LICENSE file for details.