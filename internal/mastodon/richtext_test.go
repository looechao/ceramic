package mastodon

import (
	"strings"
	"testing"
)

func TestParseTextSafety(t *testing.T) {
	runs := ParseText(`<p>Hello <strong>world</strong> &amp; 中文<a href="javascript:alert(1)">bad</a><a href="https://example.com">safe</a><script>evil()</script><span class="invisible">hidden</span></p>`)
	var text string
	links := 0
	bold := false
	for _, r := range runs {
		text += r.Text
		if r.URL != "" {
			links++
			if r.URL != "https://example.com" {
				t.Fatal(r.URL)
			}
		}
		bold = bold || r.Bold
	}
	if strings.Contains(text, "evil") || strings.Contains(text, "hidden") || links != 1 || !bold || !strings.Contains(text, "& 中文") {
		t.Fatal(runs)
	}
}
