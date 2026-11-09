package mastodon

import (
	"net/url"
	"regexp"
	"strings"

	"golang.org/x/net/html"
)

type TextRun struct {
	LinkKind     string
	Emoji        *CustomEmoji
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
func ParseText(source string, emojis ...CustomEmoji) []TextRun {
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
			case "img":
				for _, attribute := range n.Attr {
					if attribute.Key == "alt" {
						add(attribute.Val, style)
						break
					}
				}
				return
			case "br":
				add("\n", style)
				return
			case "a":
				style.LinkKind = ""
				for _, a := range n.Attr {
					if a.Key == "class" {
						for _, class := range strings.Fields(a.Val) {
							if class == "mention" && style.LinkKind == "" {
								style.LinkKind = "mention"
							}
							if class == "hashtag" {
								style.LinkKind = "hashtag"
							}
						}
					}
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
	return ExpandEmojis(runs, emojis)
}

var emojiShortcode = regexp.MustCompile(`:[A-Za-z0-9_]+:`)

// Expand only the owning account/status's emoji dictionary, never a global one.
func ExpandEmojis(runs []TextRun, emojis []CustomEmoji) []TextRun {
	if len(emojis) == 0 {
		return runs
	}
	known := map[string]*CustomEmoji{}
	for i := range emojis {
		emoji := &emojis[i]
		if SafeURL(emoji.StaticURL) != "" || SafeURL(emoji.URL) != "" {
			known[":"+emoji.Shortcode+":"] = emoji
		}
	}
	var out []TextRun
	for _, run := range runs {
		start := 0
		for _, at := range emojiShortcode.FindAllStringIndex(run.Text, -1) {
			emoji := known[run.Text[at[0]:at[1]]]
			if emoji == nil {
				continue
			}
			if at[0] > start {
				text := run
				text.Text = run.Text[start:at[0]]
				out = append(out, text)
			}
			object := run
			object.Text = run.Text[at[0]:at[1]]
			object.Emoji = emoji
			out = append(out, object)
			start = at[1]
		}
		if start < len(run.Text) {
			text := run
			text.Text = run.Text[start:]
			out = append(out, text)
		}
	}
	return out
}
