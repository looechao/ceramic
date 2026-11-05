package mastodon

import (
	"net/url"
	"strings"

	"golang.org/x/net/html"
)

type TextRun struct {
	Text, URL    string
	Bold, Italic bool
}

func SafeURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Host == "" {
		return ""
	}
	return u.String()
}

// Decode Mastodon HTML into native text runs; no HTML renderer executes it.
func ParseText(source string) []TextRun {
	doc, err := html.Parse(strings.NewReader(source))
	if err != nil {
		return []TextRun{{Text: source}}
	}
	var runs []TextRun
	add := func(text string, style TextRun) {
		if text == "" {
			return
		}
		style.Text = text
		runs = append(runs, style)
	}
	var walk func(*html.Node, TextRun)
	walk = func(n *html.Node, style TextRun) {
		if n.Type == html.TextNode {
			add(n.Data, style)
			return
		}
		if n.Type == html.ElementNode {
			switch n.Data {
			case "script", "style", "iframe", "svg", "math", "object":
				return
			case "strong", "b":
				style.Bold = true
			case "em", "i":
				style.Italic = true
			case "br":
				add("\n", style)
				return
			case "a":
				for _, a := range n.Attr {
					if a.Key == "href" {
						style.URL = SafeURL(a.Val)
					}
				}
			case "li":
				add("• ", style)
			}
			for _, a := range n.Attr {
				if a.Key == "class" && strings.Contains(" "+a.Val+" ", " invisible ") {
					return
				}
			}
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child, style)
		}
		if n.Type == html.ElementNode && (n.Data == "p" || n.Data == "div" || n.Data == "blockquote" || n.Data == "li") {
			add("\n\n", TextRun{})
		}
	}
	walk(doc, TextRun{})
	for len(runs) > 0 {
		last := &runs[len(runs)-1]
		last.Text = strings.TrimRight(last.Text, "\n")
		if last.Text != "" {
			break
		}
		runs = runs[:len(runs)-1]
	}
	return runs
}
