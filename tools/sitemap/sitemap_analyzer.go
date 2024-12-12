package main

import (
	"encoding/xml"
	"fmt"
	"io"
	"log"
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

func main() {
	sitemapURL := "https://rmm.datto.com/help/en/Sitemap.xml"

	// Fetch and parse sitemap
	sitemap, err := fetchSitemap(sitemapURL)
	if err != nil {
		log.Fatalf("Error fetching sitemap: %v", err)
	}

	fmt.Printf("Total URLs found: %d\n\n", len(sitemap.URLs))

	// Analyze multiple URLs to get a better sample
	samplesToAnalyze := 3
	if len(sitemap.URLs) > 0 {
		for i := 0; i < samplesToAnalyze && i < len(sitemap.URLs); i++ {
			sampleURL := sitemap.URLs[i].Loc
			fmt.Printf("\n=== Analyzing URL %d/%d: %s ===\n", i+1, samplesToAnalyze, sampleURL)

			doc, err := fetchAndParseHTML(sampleURL)
			if err != nil {
				log.Printf("Error fetching page: %v", err)
				continue
			}

			// Analyze navigation structure
			fmt.Println("\n--- Navigation Elements ---")
			analyzeNavigation(doc)

			// Analyze breadcrumb structure
			fmt.Println("\n--- Breadcrumb Elements ---")
			analyzeBreadcrumbs(doc)

			// Analyze content structure
			fmt.Println("\n--- Content Structure ---")
			analyzeContent(doc)
		}
	}
}

func analyzeNavigation(n *html.Node) {
	var f func(*html.Node)
	f = func(n *html.Node) {
		if n.Type == html.ElementNode {
			// Check for navigation elements
			classes := getAttr(n, "class")
			if strings.Contains(classes, "sidenav") ||
				strings.Contains(classes, "navigation") ||
				strings.Contains(classes, "nav") {
				fmt.Printf("Found navigation element: <%s> class='%s'\n", n.Data, classes)
				// Print child items
				printNavItems(n, 1)
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}
	}
	f(n)
}

func analyzeBreadcrumbs(n *html.Node) {
	var f func(*html.Node)
	f = func(n *html.Node) {
		if n.Type == html.ElementNode {
			classes := getAttr(n, "class")
			if strings.Contains(classes, "breadcrumb") ||
				strings.Contains(classes, "breadcrumbs") {
				fmt.Printf("Found breadcrumb element: <%s> class='%s'\n", n.Data, classes)
				printNavItems(n, 1)
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}
	}
	f(n)
}

func analyzeContent(n *html.Node) {
	var f func(*html.Node)
	f = func(n *html.Node) {
		if n.Type == html.ElementNode {
			id := getAttr(n, "id")
			classes := getAttr(n, "class")
			if id == "mc-main-content" ||
				strings.Contains(classes, "article-content") ||
				strings.Contains(classes, "topic-content") {
				fmt.Printf("Found content container: <%s> id='%s' class='%s'\n", n.Data, id, classes)
				analyzeTitleAndMetadata(n)
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}
	}
	f(n)
}

func analyzeTitleAndMetadata(n *html.Node) {
	var f func(*html.Node)
	f = func(n *html.Node) {
		if n.Type == html.ElementNode {
			// Look for title elements
			if n.Data == "title" ||
				strings.Contains(getAttr(n, "class"), "title") ||
				strings.Contains(getAttr(n, "class"), "heading") {
				fmt.Printf("  Found title element: <%s> class='%s' text='%s'\n",
					n.Data, getAttr(n, "class"), getNodeText(n))
			}

			// Look for metadata elements
			if n.Data == "meta" {
				name := getAttr(n, "name")
				content := getAttr(n, "content")
				if name != "" && content != "" {
					fmt.Printf("  Found metadata: %s = '%s'\n", name, content)
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}
	}
	f(n)
}

func printNavItems(n *html.Node, depth int) {
	var f func(*html.Node, int)
	f = func(n *html.Node, depth int) {
		if n.Type == html.ElementNode && (n.Data == "li" || n.Data == "a") {
			indent := strings.Repeat("  ", depth)
			classes := getAttr(n, "class")
			text := strings.TrimSpace(getNodeText(n))
			if text != "" {
				fmt.Printf("%s%s (class='%s') -> %s\n", indent, n.Data, classes, text)
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c, depth+1)
		}
	}
	f(n, depth)
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
		return strings.TrimSpace(n.Data)
	}
	var text string
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		text += getNodeText(c)
	}
	return strings.TrimSpace(text)
}

func fetchSitemap(url string) (*Sitemap, error) {
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

func fetchAndParseHTML(url string) (*html.Node, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	doc, err := html.Parse(resp.Body)
	if err != nil {
		return nil, err
	}

	return doc, nil
}
