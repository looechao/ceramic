package app

import (
	"ceramic/internal/mastodon"
	"ceramic/locales"
	"github.com/egoist/mygo/ui"
	"strings"
	"testing"
)

func TestNativeBoostSource(t *testing.T) {
	r := testReader(t)
	r.session = &mastodon.Session{Scopes: mastodon.Scopes}
	author := mastodon.Account{ID: "author", DisplayName: "林间", Acct: "forest@example.social"}
	booster := mastodon.Account{ID: "booster", DisplayName: "小满", Acct: "xiaoman@example.social"}
	original := mastodon.Status{ID: "original", Account: author, Visibility: "public", Content: "雨停了，沿着河边走了一小段。"}
	r.home.applyPosts([]mastodon.Status{{ID: "boost", Account: booster, Reblog: &original}}, false)
	r.home.loaded, r.home.end = true, true
	r.navigate("home")
	tt := ui.NewTester(r.view, 760, 560)
	label := locales.Boosted(booster.Name())
	if !tt.HasText(label) || !tt.HasText(author.Name()) {
		t.Fatal("boost source or original author missing")
	}
	savePreview(t, tt, "native-boost-source")
	source, _ := tt.Find(label)
	tt.Move(source.X+source.W/2, source.Y+source.H/2)
	tt.Advance(listHoverDuration * 2)
	savePreview(t, tt, "native-boost-source-hover")
	tt.Move(0, 0)
	tt.Advance(listHoverDuration * 2)
	tt.SetSize(420, 560)
	tt.SetDark(true)
	savePreview(t, tt, "native-boost-source-dark-compact")
	if err := tt.Click(label); err != nil {
		t.Fatal(err)
	}
	if r.feedState.kind != "profile" || r.feedState.profile.ID != booster.ID {
		t.Fatal("boost source opened original post or wrong profile")
	}
	// A long source name must not push the pill outside its compact column.
	r.navigate("home")
	ws := r.workspaces["home"]
	r.closeColumn(ws, ws.columns[1])
	booster.DisplayName = strings.Repeat("Long Display Name ", 8)
	r.home.posts[0].status.Account = booster
	tt.Frame()
	b, ok := tt.Find(locales.Boosted(booster.Name()))
	if !ok || b.X+b.W > 420 {
		t.Fatalf("boost source overflow: %v", b)
	}
}

func TestNativePrivateMentionEmphasis(t *testing.T) {
	for _, tag := range []string{"zh-Hans", "de"} {
		t.Run(tag, func(t *testing.T) {
			t.Setenv("MYGO_LOCALE", tag)
			r := testReader(t)
			r.session = &mastodon.Session{Scopes: mastodon.Scopes}
			actor := mastodon.Account{ID: "actor", DisplayName: "小满", Acct: "xiaoman@example.social"}
			s := mastodon.Status{ID: "post", Visibility: "direct", Content: "下次读书会见。", Account: actor, Spoiler: "关于下次聚会"}
			r.notifications.notifications = []mastodon.Notification{{ID: "private", Type: "mention", Account: actor, Status: &s}}
			r.notifications.loaded, r.notifications.end = true, true
			r.navigate("notifications")
			tt := ui.NewTester(r.view, 420, 560)
			tt.SetLocale(tag)
			badge, ok := tt.Find(locales.PrivateMentions())
			name, hasName := tt.Find(actor.Name())
			if !ok || !hasName || badge.Y >= name.Y || badge.X+badge.W > 420 {
				t.Fatal("private mention label is not visible above author")
			}
			if tt.HasText(locales.MentionedYou()) || tt.HasText(s.Content) {
				t.Fatal("duplicate mention label or concealed content leaked")
			}
			savePreview(t, tt, "native-private-mention-"+tag)
			tt.SetDark(true)
			savePreview(t, tt, "native-private-mention-dark-"+tag)
			for _, visibility := range []string{"private", "public", ""} {
				s.Visibility = visibility
				tt.Frame()
				if tt.HasText(locales.PrivateMentions()) || !tt.HasText(locales.MentionedYou()) {
					t.Fatalf("%q mislabeled as private mention", visibility)
				}
			}
		})
	}
}
