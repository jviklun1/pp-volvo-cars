package marketpages

import (
	"encoding/json"
	"errors"
	"io"
	"net/url"
	"path"
	"strings"

	"golang.org/x/net/html"
)

type extractedPage struct {
	CanonicalURL string
	SourceID     string
	Title        string
	Body         string
}

// NormalizeSourceID derives the stable adapter identity from a canonical URL path.
func NormalizeSourceID(rawURL string) (string, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}
	if parsed.Path == "" {
		return "", errors.New("market page URL path is required")
	}
	value := path.Clean(parsed.EscapedPath())
	if !strings.HasPrefix(value, "/") {
		value = "/" + value
	}
	return value, nil
}

func extract(reader io.Reader, rawURL string) (extractedPage, error) {
	document, err := html.Parse(reader)
	if err != nil {
		return extractedPage{}, err
	}
	sourceID, err := NormalizeSourceID(rawURL)
	if err != nil {
		return extractedPage{}, err
	}
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return extractedPage{}, err
	}
	parsedURL.RawQuery = ""
	parsedURL.Fragment = ""
	parsedURL.Path, err = url.PathUnescape(sourceID)
	if err != nil {
		return extractedPage{}, err
	}
	parsedURL.RawPath = sourceID

	page := extractedPage{CanonicalURL: parsedURL.String(), SourceID: sourceID}
	var bodyParts []string
	var walk func(*html.Node, bool)
	walk = func(node *html.Node, collect bool) {
		if node.Type == html.ElementNode {
			switch node.Data {
			case "title":
				page.Title = collapse(textContent(node))
			case "main", "article":
				collect = true
			case "script":
				if attribute(node, "type") == "application/ld+json" {
					mergeStructuredData(&page, textContent(node))
				}
				return
			case "style", "nav", "header", "footer", "noscript":
				return
			}
		}
		if collect && node.Type == html.TextNode {
			if value := collapse(node.Data); value != "" {
				bodyParts = append(bodyParts, value)
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child, collect)
		}
	}
	walk(document, false)
	if page.Body == "" {
		page.Body = collapse(strings.Join(bodyParts, " "))
	}
	if page.Title == "" || page.Body == "" {
		return extractedPage{}, errors.New("market page must contain a title and body")
	}
	return page, nil
}

func mergeStructuredData(page *extractedPage, raw string) {
	var value any
	if json.Unmarshal([]byte(raw), &value) != nil {
		return
	}
	var visit func(any)
	visit = func(current any) {
		switch typed := current.(type) {
		case []any:
			for _, child := range typed {
				visit(child)
			}
		case map[string]any:
			if page.Title == "" {
				page.Title = firstString(typed, "headline", "name")
			}
			if page.Body == "" {
				page.Body = firstString(typed, "articleBody", "description")
			}
			if graph, ok := typed["@graph"]; ok {
				visit(graph)
			}
		}
	}
	visit(value)
	page.Title = collapse(page.Title)
	page.Body = collapse(page.Body)
}

func firstString(values map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := values[key].(string); ok && strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func textContent(node *html.Node) string {
	var builder strings.Builder
	var walk func(*html.Node)
	walk = func(current *html.Node) {
		if current.Type == html.TextNode {
			builder.WriteString(current.Data)
			builder.WriteByte(' ')
		}
		for child := current.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(node)
	return builder.String()
}

func attribute(node *html.Node, name string) string {
	for _, attribute := range node.Attr {
		if attribute.Key == name {
			return attribute.Val
		}
	}
	return ""
}

func collapse(value string) string {
	return strings.Join(strings.Fields(value), " ")
}
