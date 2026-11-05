package main

import (
	"golang.org/x/net/html"
	"net/url"
	"strings"
)

type account struct {
	ID          string `json:"id"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	Acct        string `json:"acct"`
	Avatar      string `json:"avatar_static"`
	Note        string `json:"note"`
	URL         string `json:"url"`
	Followers   int    `json:"followers_count"`
	Following   int    `json:"following_count"`
	Statuses    int    `json:"statuses_count"`
}

func (a account) name() string {
	if a.DisplayName != "" {
		return a.DisplayName
	}
	return a.Username
}

type media struct {
	Type        string `json:"type"`
	URL         string `json:"url"`
	Preview     string `json:"preview_url"`
	Description string `json:"description"`
}
type linkCard struct {
	URL         string `json:"url"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Image       string `json:"image"`
	Provider    string `json:"provider_name"`
}
type status struct {
	Card *linkCard `json:"card"`

	ID         string  `json:"id"`
	Content    string  `json:"content"`
	URL        string  `json:"url"`
	CreatedAt  string  `json:"created_at"`
	Account    account `json:"account"`
	Reblog     *status `json:"reblog"`
	Spoiler    string  `json:"spoiler_text"`
	Sensitive  bool    `json:"sensitive"`
	Media      []media `json:"media_attachments"`
	Replies    int     `json:"replies_count"`
	Reblogs    int     `json:"reblogs_count"`
	Favourites int     `json:"favourites_count"`
	ReplyTo    string  `json:"in_reply_to_id"`
	Filtered   []struct {
		Filter struct {
			Action string `json:"filter_action"`
		} `json:"filter"`
	} `json:"filtered"`
}
type textRun struct {
	Text, URL    string
	Bold, Italic bool
}

func safeURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Host == "" {
		return ""
	}
	return u.String()
}

// Decode Mastodon HTML into native text runs; no HTML renderer executes it.
func nativeText(source string) []textRun {
	doc, err := html.Parse(strings.NewReader(source))
	if err != nil {
		return []textRun{{Text: source}}
	}
	var runs []textRun
	add := func(text string, style textRun) {
		if text == "" {
			return
		}
		style.Text = text
		runs = append(runs, style)
	}
	var walk func(*html.Node, textRun)
	walk = func(n *html.Node, style textRun) {
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
						style.URL = safeURL(a.Val)
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
			add("\n\n", textRun{})
		}
	}
	walk(doc, textRun{})
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
