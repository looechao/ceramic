package mastodon

import (
	"encoding/json"
	"testing"
)

// Polls must survive decoding instead of silently disappearing from statuses.
func TestStatusRetainsPoll(t *testing.T) {
	var status Status
	if err := json.Unmarshal([]byte(`{"id":"post","poll":{"id":"poll","multiple":true,"votes_count":3,"voters_count":2,"options":[{"title":"A","votes_count":2},{"title":"B","votes_count":null}]}}`), &status); err != nil {
		t.Fatal(err)
	}
	encoded, _ := json.Marshal(status)
	var fields map[string]json.RawMessage
	json.Unmarshal(encoded, &fields)
	if string(fields["poll"]) == "" || string(fields["poll"]) == "null" {
		t.Fatal("poll lost during decoding")
	}
}

// Account avatars must preserve the animated URL separately from the static fallback.
func TestAccountRetainsAnimatedAvatar(t *testing.T) {
	var account Account
	json.Unmarshal([]byte(`{"avatar":"https://example.com/moving.gif","avatar_static":"https://example.com/still.png"}`), &account)
	if account.Avatar != "https://example.com/moving.gif" {
		t.Fatalf("animated avatar replaced by %q", account.Avatar)
	}
}
