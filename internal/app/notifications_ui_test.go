package app

import (
	"encoding/json"
	"fmt"
	"math"
	"testing"

	"ceramic/internal/mastodon"
	"github.com/egoist/mygo/ui"
)

func TestNativeNotificationConcealment(t *testing.T) {
	for _, tc := range []struct{ name, json string }{
		{"cw", `{"id":"s","content":"secret-body","spoiler_text":"CW: concealed"}`},
		{"hide", `{"id":"s","content":"secret-body","filtered":[{"filter":{"filter_action":"hide"}}]}`},
		{"warn", `{"id":"s","content":"secret-body","filtered":[{"filter":{"filter_action":"warn"}}]}`},
		{"nested_hide", `{"id":"s","reblog":{"id":"nested","content":"secret-body","filtered":[{"filter":{"filter_action":"hide"}}]}}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := testReader(t)
			r.session = &mastodon.Session{Account: json.RawMessage(`{"id":"me"}`)}
			var status mastodon.Status
			if err := json.Unmarshal([]byte(tc.json), &status); err != nil {
				t.Fatal(err)
			}
			r.notifications.notifications = []mastodon.Notification{{ID: "control", Type: "follow", Account: mastodon.Account{DisplayName: "Visible control"}}, {ID: "n", Type: "mention", Status: &status, Account: mastodon.Account{ID: "a", DisplayName: "Actor"}}}
			r.notifications.loaded, r.notifications.end = true, true
			r.navigate("notifications")
			tt := ui.NewTester(r.view, 760, 820)
			if !tt.HasText("关注了你") {
				t.Fatal("notification not rendered")
			}
			if tt.HasText("secret-body") {
				t.Fatal("concealed notification body is visible before explicit reveal")
			}
			savePreview(t, tt, "native-notification-"+tc.name)
			tt.SetDark(true)
			tt.SetSize(420, 560)
			savePreview(t, tt, "native-notification-"+tc.name+"-dark-compact")
			if tc.name == "cw" || tc.name == "warn" {
				label := "CW: concealed"
				if tc.name == "warn" {
					label = "已被你的过滤规则折叠"
				}
				if err := tt.Click("展开隐藏内容：" + label); err != nil {
					t.Fatal(err)
				}
				if !tt.HasText("secret-body") {
					t.Fatal("explicit reveal failed")
				}
				if len(r.workspaces["notifications"].columns) != 1 {
					t.Fatal("revealing a warning must not open a conversation")
				}
				if err := tt.Click("收起内容"); err != nil {
					t.Fatal(err)
				}
				if tt.HasText("secret-body") {
					t.Fatal("collapse failed")
				}
			}
		})
	}
}

func TestNativeNotificationReadingAnchorSurvivesPoll(t *testing.T) {
	r := testReader(t)
	r.session = &mastodon.Session{Account: json.RawMessage(`{"id":"me"}`), Scopes: mastodon.Scopes}
	f := r.notifications
	for i := 60; i > 0; i-- {
		f.notifications = append(f.notifications, mastodon.Notification{
			ID: fmt.Sprint(i), Type: "mention", Account: mastodon.Account{DisplayName: "Author"},
			Status: &mastodon.Status{ID: fmt.Sprint(i), Content: fmt.Sprintf("notice-body-%d", i)},
		})
	}
	f.loaded, f.end, f.cursor = true, true, "1"
	r.navigate("notifications")
	tt := ui.NewTester(r.view, 1240, 820)
	f.list.ScrollTo(45, ui.Start)
	tt.Frame()
	before, ok := tt.Find("notice-body-15")
	if !ok {
		t.Fatal("initial anchor missing")
	}
	f.applyNotifications([]mastodon.Notification{{ID: "61", Type: "follow", Account: mastodon.Account{DisplayName: "New follower"}}}, false)
	tt.Frame()
	after, ok := tt.Find("notice-body-15")
	if !ok || math.Abs(float64(before.Y-after.Y)) > 0.1 {
		t.Fatalf("head refresh displaced reading anchor: %v -> %v", before, after)
	}
}
