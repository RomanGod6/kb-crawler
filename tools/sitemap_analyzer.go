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

	// Print sitemap statistics
	fmt.Printf("Total URLs found: %d\n\n", len(sitemap.URLs))

	// Analyze first URL to understand structure
	if len(sitemap.URLs) > 0 {
		sampleURL := sitemap.URLs[0].Loc
		fmt.Printf("Analyzing sample URL: %s\n", sampleURL)

		doc, err := fetchAndParseHTML(sampleURL)
		if err != nil {
			log.Fatalf("Error fetching sample page: %v", err)
		}

		// Analyze HTML structure
		analyzeHTMLStructure(doc)
	}
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

func analyzeHTMLStructure(n *html.Node) {
	// Track important elements
	var analyze func(*html.Node, int)
	analyze = func(n *html.Node, depth int) {
		if n.Type == html.ElementNode {
			// Print element info with its important attributes
			attrs := make([]string, 0)
			for _, attr := range n.Attr {
				if attr.Key == "id" || attr.Key == "class" {
					attrs = append(attrs, fmt.Sprintf("%s=\"%s\"", attr.Key, attr.Val))
				}
			}

			// Only print elements with classes or IDs
			if len(attrs) > 0 {
				indent := strings.Repeat("  ", depth)
				fmt.Printf("%s<%s %s>\n", indent, n.Data, strings.Join(attrs, " "))
			}
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			analyze(c, depth+1)
		}
	}

	analyze(n, 0)
}
