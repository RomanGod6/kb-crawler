// tools/category_mapper.go
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/url"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/chromedp/chromedp"
)

type CategoryNode struct {
	Name     string                   `json:"name"`
	URL      string                   `json:"url,omitempty"`
	Path     string                   `json:"path"`
	Children map[string]*CategoryNode `json:"children,omitempty"`
}

func NewCategoryNode(name string) *CategoryNode {
	return &CategoryNode{
		Name:     name,
		Children: make(map[string]*CategoryNode),
	}
}

func main() {
	// Create root node
	root := NewCategoryNode("Datto RMM")

	// Create chromedp context
	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	// Set a timeout
	ctx, cancel = context.WithTimeout(ctx, 120*time.Second)
	defer cancel()

	var htmlContent string

	// Navigate to the page and get the full HTML
	err := chromedp.Run(ctx,
		chromedp.Navigate("https://rmm.datto.com/help/en/Content/0HOME/Home.htm"),
		// Wait until sidenav is visible
		chromedp.WaitVisible(`ul.sidenav`, chromedp.ByQuery),
		// Optional: Wait additional time for all scripts to execute
		chromedp.Sleep(2*time.Second),
		// Get the outer HTML of the sidenav
		chromedp.OuterHTML(`ul.sidenav`, &htmlContent, chromedp.ByQuery),
	)
	if err != nil {
		log.Fatalf("Failed to retrieve page: %v", err)
	}

	// Log the retrieved sidenav HTML
	log.Println("Found sidenav menu")
	log.Printf("DEBUG: sidenav HTML:\n%s", htmlContent)

	// Optionally, save the sidenav HTML to a file for inspection
	err = ioutil.WriteFile("sidenav.html", []byte(htmlContent), 0644)
	if err != nil {
		log.Printf("Failed to write sidenav HTML to file: %v", err)
	} else {
		log.Println("Saved sidenav HTML to sidenav.html")
	}

	// Parse the HTML with goquery
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlContent))
	if err != nil {
		log.Fatalf("Failed to parse HTML: %v", err)
	}

	menuCount := 0
	submenuCount := 0

	// Collect top-level menu items
	menuItems := []*CategoryNode{}

	doc.Find("li").Each(func(i int, s *goquery.Selection) {
		a := s.Find("a")
		// Extract only the direct text node, excluding spans
		clonedA := a.Clone()
		clonedA.Find("span").Remove()
		text := strings.TrimSpace(clonedA.Text())
		href, _ := a.Attr("href") // Ignoring the 'exists' flag
		classes, _ := s.Attr("class")

		log.Printf("Found menu item: Text=%q Href=%q Class=%q", text, href, classes)

		if text == "" {
			log.Println("DEBUG: Skipping LI because no text was found.")
			return
		}

		menuCount++

		// Create node
		node := NewCategoryNode(text)
		node.URL = href
		node.Path = fmt.Sprintf("Datto RMM:%s", text)
		root.Children[text] = node
		menuItems = append(menuItems, node)

		// If this is a parent node with submenu
		if strings.Contains(classes, "is-accordion-submenu-parent") {
			log.Printf("DEBUG: Detected submenu parent for %q", text)
			submenuCount++
			// Submenus are likely on separate pages
		} else {
			log.Printf("DEBUG: No submenu detected under %q", text)
		}
	})

	log.Printf("Total menu items found: %d", menuCount)
	log.Printf("Total submenu parents found: %d", submenuCount)

	// Now, for each menu item that has a valid href (not "javascript:void(0);"), scrape subcategories
	baseURL := "https://rmm.datto.com/help/en/Content/0HOME/Home.htm"

	for _, menuItem := range menuItems {
		// Skip menu items with href="javascript:void(0);" or empty href
		if menuItem.URL == "" || menuItem.URL == "javascript:void(0);" {
			log.Printf("Skipping menu item %q with href %q", menuItem.Name, menuItem.URL)
			continue
		}

		// Resolve relative URL to absolute
		absoluteURL := resolveURL(baseURL, menuItem.URL)
		if absoluteURL == "" {
			log.Printf("Failed to resolve URL for menu item %q: href=%q", menuItem.Name, menuItem.URL)
			continue
		}

		log.Printf("Scraping subcategories for menu item %q from %q", menuItem.Name, absoluteURL)

		// Scrape subcategories from the menu item's page
		subcategories, err := scrapeSubcategories(ctx, absoluteURL, menuItem.Path)
		if err != nil {
			log.Printf("Failed to scrape subcategories for %q: %v", menuItem.Name, err)
			continue
		}

		// Add subcategories as children
		for _, sub := range subcategories {
			menuItem.Children[sub.Name] = sub
		}

		log.Printf("Added %d subcategories for menu item %q", len(subcategories), menuItem.Name)
	}

	// Print the structure
	jsonData, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		log.Fatalf("Failed to marshal JSON: %v", err)
	}

	fmt.Println("\nCategory Structure:")
	fmt.Println(string(jsonData))

	// Print statistics
	printStatistics(root)
}

// Function to resolve relative URLs to absolute
func resolveURL(base, ref string) string {
	u, err := url.Parse(ref)
	if err != nil {
		return ""
	}
	baseU, err := url.Parse(base)
	if err != nil {
		return ""
	}
	return baseU.ResolveReference(u).String()
}

// Function to scrape subcategories from a given URL
func scrapeSubcategories(parentCtx context.Context, pageURL string, parentPath string) ([]*CategoryNode, error) {
	// Create a new chromedp context
	ctx, cancel := chromedp.NewContext(parentCtx)
	defer cancel()

	// Set a timeout
	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	var htmlContent string

	// Navigate to the page and get the sidenav
	err := chromedp.Run(ctx,
		chromedp.Navigate(pageURL),
		chromedp.WaitVisible(`ul.sidenav`, chromedp.ByQuery),
		chromedp.Sleep(1*time.Second),
		chromedp.OuterHTML(`ul.sidenav`, &htmlContent, chromedp.ByQuery),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve page: %v", err)
	}

	// Parse the HTML with goquery
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlContent))
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML: %v", err)
	}

	subcategories := []*CategoryNode{}

	doc.Find("li").Each(func(i int, s *goquery.Selection) {
		a := s.Find("a")
		// Extract only the direct text node, excluding spans
		clonedA := a.Clone()
		clonedA.Find("span").Remove()
		text := strings.TrimSpace(clonedA.Text())
		href, _ := a.Attr("href") // Ignoring the 'exists' flag
		classes, _ := s.Attr("class")

		log.Printf("  Found submenu item: Text=%q Href=%q Class=%q", text, href, classes)

		if text == "" {
			log.Println("  DEBUG: Skipping LI because no text was found.")
			return
		}

		// Create node
		node := NewCategoryNode(text)
		node.URL = href
		node.Path = fmt.Sprintf("%s:%s", parentPath, text)
		subcategories = append(subcategories, node)

		// If this is a parent node with submenu (unlikely if it's a depth-2)
		if strings.Contains(classes, "is-accordion-submenu-parent") {
			log.Printf("  DEBUG: Detected submenu parent for %q", text)
			// For simplicity, only scrape depth 2
			// Alternatively, implement recursive scraping
		} else {
			log.Printf("  DEBUG: No submenu detected under %q", text)
		}
	})

	return subcategories, nil
}

func printStatistics(root *CategoryNode) {
	totalNodes := 0
	totalLeaves := 0
	maxDepth := 0
	urlCount := 0

	var traverse func(*CategoryNode, int)
	traverse = func(node *CategoryNode, depth int) {
		totalNodes++
		if depth > maxDepth {
			maxDepth = depth
		}
		if len(node.Children) == 0 {
			totalLeaves++
		}
		if node.URL != "" && !strings.Contains(node.URL, "javascript:") {
			urlCount++
		}
		for _, child := range node.Children {
			traverse(child, depth+1)
		}
	}

	traverse(root, 0)

	fmt.Printf("\nStatistics:\n")
	fmt.Printf("Total categories: %d\n", totalNodes)
	fmt.Printf("Leaf categories: %d\n", totalLeaves)
	fmt.Printf("Maximum depth: %d\n", maxDepth)
	fmt.Printf("Categories with URLs: %d\n", urlCount)

	fmt.Printf("\nCategory Paths:\n")
	var printPaths func(*CategoryNode, string)
	printPaths = func(node *CategoryNode, prefix string) {
		path := prefix + node.Name
		if node.URL != "" && !strings.Contains(node.URL, "javascript:") {
			fmt.Printf("%s -> %s\n", path, node.URL)
		}
		for _, child := range node.Children {
			printPaths(child, path+":")
		}
	}
	printPaths(root, "")
}
