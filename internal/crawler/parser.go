package crawler

import (
	"bytes"
	"encoding/xml"
	"io"
	"net/http"
	"strings"

	"golang.org/x/net/html"
)

type Sitemap struct {
	XMLName xml.Name `xml:"urlset"`
	URLs    []URL    `xml:"url"`
}

type URL struct {
	Loc        string `xml:"loc"`
	LastMod    string `xml:"lastmod,omitempty"`
	ChangeFreq string `xml:"changefreq,omitempty"`
	Priority   string `xml:"priority,omitempty"`
}

func parseSitemap(url string) (*Sitemap, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var sitemap Sitemap
	if err := xml.Unmarshal(body, &sitemap); err != nil {
		return nil, err
	}

	return &sitemap, nil
}

type ParsedContent struct {
	Title      string
	Content    string
	Tags       []string
	Author     string
	CategoryID string
}

func parseHTMLContent(content string) (*ParsedContent, error) {
	doc, err := html.Parse(strings.NewReader(content))
	if err != nil {
		return nil, err
	}

	parsed := &ParsedContent{
		Tags: make([]string, 0),
	}

	var f func(*html.Node)
	f = func(n *html.Node) {
		if n.Type == html.ElementNode {
			// Parse title
			if n.Data == "title" {
				parsed.Title = getNodeText(n)
			}

			// Parse meta tags
			if n.Data == "meta" {
				name := getAttr(n, "name")
				content := getAttr(n, "content")
				switch name {
				case "author":
					parsed.Author = content
				case "category-id":
					parsed.CategoryID = content
				case "keywords":
					parsed.Tags = strings.Split(content, ",")
				}
			}

			// Parse article content
			if n.Data == "article" {
				var buf bytes.Buffer
				html.Render(&buf, n)
				parsed.Content = buf.String()
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}
	}
	f(doc)

	// Clean up content
	parsed.Content = cleanHTML(parsed.Content)
	parsed.Tags = cleanTags(parsed.Tags)

	return parsed, nil
}

func getAttr(n *html.Node, key string) string {
	for _, attr := range n.Attr {
		if attr.Key == key {
			return attr.Val
		}
	}
	return ""
}

func getNodeText(n *html.Node) string {
	if n.Type == html.TextNode {
		return n.Data
	}
	var text string
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		text += getNodeText(c)
	}
	return strings.TrimSpace(text)
}

func cleanHTML(content string) string {
	// Remove script and style elements
	content = removeElements(content, "script")
	content = removeElements(content, "style")

	// Remove HTML comments
	content = removeComments(content)

	// Convert multiple spaces to single space
	content = strings.Join(strings.Fields(content), " ")

	return strings.TrimSpace(content)
}

func cleanTags(tags []string) []string {
	cleaned := make([]string, 0)
	for _, tag := range tags {
		tag = strings.TrimSpace(tag)
		if tag != "" {
			cleaned = append(cleaned, strings.ToLower(tag))
		}
	}
	return cleaned
}

func removeElements(content string, tag string) string {
	startTag := "<" + tag
	endTag := "</" + tag + ">"
	for {
		startIdx := strings.Index(content, startTag)
		if startIdx == -1 {
			break
		}
		endIdx := strings.Index(content[startIdx:], endTag)
		if endIdx == -1 {
			break
		}
		content = content[:startIdx] + content[startIdx+endIdx+len(endTag):]
	}
	return content
}

func removeComments(content string) string {
	for {
		startIdx := strings.Index(content, "<!--")
		if startIdx == -1 {
			break
		}
		endIdx := strings.Index(content[startIdx:], "-->")
		if endIdx == -1 {
			break
		}
		content = content[:startIdx] + content[startIdx+endIdx+3:]
	}
	return content
}
