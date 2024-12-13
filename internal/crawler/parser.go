// internal/crawler/parser.go
package crawler

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/gocolly/colly/v2"
	"github.com/romangod6/kb-crawler/internal/utils"
	"golang.org/x/net/html"
)

// ParsedContent holds the extracted content from an HTML page.
type ParsedContent struct {
	Title      string
	Content    string
	Tags       []string
	Author     string
	CategoryID string
}

// ExtractProductFeatureTags extracts the ProductFeatureTags from the <meta> tags in the <head> section.
func ExtractProductFeatureTags(e *colly.HTMLElement, tags *[]string, logger *utils.CrawlerLogger) {
	e.DOM.Find("meta[name='ProductFeatureTags']").Each(func(i int, s *goquery.Selection) {
		content, exists := s.Attr("content")
		if exists {
			pftags := strings.Split(content, ",")
			for _, pftag := range pftags {
				pftag = strings.TrimSpace(pftag)
				if pftag != "" {
					logger.LogDebug("Found ProductFeatureTag: %s", pftag)
					*tags = append(*tags, pftag)
				}
			}
		}
	})

	if len(*tags) > 0 {
		logger.LogInfo("Found ProductFeatureTags: %v", *tags)
	} else {
		logger.LogDebug("No ProductFeatureTags found in meta elements")
	}
}

// ParseHTMLContent parses the raw HTML content and extracts relevant information.
func ParseHTMLContent(content string) (*ParsedContent, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(content))
	if err != nil {
		return nil, fmt.Errorf("error parsing HTML: %w", err)
	}

	parsed := &ParsedContent{
		Tags: make([]string, 0),
	}

	// Extract title
	parsed.Title = strings.TrimSpace(doc.Find("title").First().Text())

	// Extract author
	doc.Find("meta[name='author']").Each(func(i int, s *goquery.Selection) {
		if content, exists := s.Attr("content"); exists {
			parsed.Author = strings.TrimSpace(content)
		}
	})

	// Extract category-id
	doc.Find("meta[name='category-id']").Each(func(i int, s *goquery.Selection) {
		if content, exists := s.Attr("content"); exists {
			parsed.CategoryID = strings.TrimSpace(content)
		}
	})

	// Extract keywords as tags
	doc.Find("meta[name='keywords']").Each(func(i int, s *goquery.Selection) {
		if content, exists := s.Attr("content"); exists {
			keywords := strings.Split(content, ",")
			for _, kw := range keywords {
				kw = strings.TrimSpace(kw)
				if kw != "" {
					parsed.Tags = append(parsed.Tags, strings.ToLower(kw))
				}
			}
		}
	})

	// Extract ProductFeatureTags using the dedicated function
	doc.Find("meta[name='ProductFeatureTags']").Each(func(i int, s *goquery.Selection) {
		if content, exists := s.Attr("content"); exists {
			pftags := strings.Split(content, ",")
			for _, pftag := range pftags {
				pftag = strings.TrimSpace(pftag)
				if pftag != "" {
					parsed.Tags = append(parsed.Tags, strings.ToLower(pftag))
				}
			}
		}
	})

	// Extract main content from the <article> tag or other selectors
	var mainContent string
	if article := doc.Find("article"); article.Length() > 0 {
		mainContent, _ = article.Html()
	} else {
		// Fallback to the entire body
		mainContent, _ = doc.Find("body").Html()
	}

	parsed.Content = cleanHTML(mainContent)

	return parsed, nil
}

// cleanHTML cleans the HTML content by removing scripts, styles, comments, and unnecessary whitespace.
func cleanHTML(content string) string {
	// Parse the HTML content
	doc, err := html.Parse(strings.NewReader(content))
	if err != nil {
		return content // Return original content if parsing fails
	}

	// Remove script and style elements
	var removeNodes func(*html.Node)
	removeNodes = func(n *html.Node) {
		if n.Type == html.ElementNode && (n.Data == "script" || n.Data == "style") {
			n.Parent.RemoveChild(n)
			return
		}
		// Remove comments
		if n.Type == html.CommentNode {
			n.Parent.RemoveChild(n)
			return
		}
		for c := n.FirstChild; c != nil; {
			next := c.NextSibling
			removeNodes(c)
			c = next
		}
	}
	removeNodes(doc)

	// Render the cleaned HTML back to a string
	var buf bytes.Buffer
	if err := html.Render(&buf, doc); err != nil {
		return content // Return original content if rendering fails
	}

	// Convert multiple spaces to single space
	cleaned := strings.Join(strings.Fields(buf.String()), " ")

	return strings.TrimSpace(cleaned)
}
