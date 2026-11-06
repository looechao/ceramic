package mastodon

import "testing"

// Shortcodes resolve only in their owning dictionary and preserve link/style data.
func TestCustomEmojiRuns(t *testing.T) {
	emoji := CustomEmoji{Shortcode: "wave", StaticURL: "https://example.com/wave.png"}
	runs := ParseText(`<p>Hello <strong>:wave:</strong> <a href="https://example.com">:wave:</a> :unknown: 🙂</p>`, emoji)
	found := 0
	for _, run := range runs {
		if run.Emoji != nil {
			found++
			if run.Text != ":wave:" || run.Emoji.StaticURL != emoji.StaticURL {
				t.Fatal(run)
			}
			if found == 1 && !run.Bold {
				t.Fatal("emoji lost bold context")
			}
			if found == 2 && run.URL != "https://example.com" {
				t.Fatal("emoji lost its link")
			}
		}
	}
	if found != 2 {
		t.Fatalf("found %d emojis", found)
	}
	unscoped := ParseText(":wave:")
	if unscoped[0].Emoji != nil || unscoped[0].Text != ":wave:" {
		t.Fatal("emoji leaked between instances")
	}
	image := ParseText(`<img class="emoji" alt=":wave:" src="https://untrusted.example/other.png">`, emoji)
	if len(image) != 1 || image[0].Emoji == nil || image[0].Emoji.StaticURL != emoji.StaticURL {
		t.Fatal("inline image ignored trusted dictionary")
	}
}
