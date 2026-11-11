package app

import (
	"ceramic/internal/mastodon"
	"ceramic/locales"
	"github.com/egoist/mygo/ui"
	"testing"
)

func TestNativeProfileLoading(t *testing.T) {
	r := testReader(t)
	f := &feedState{kind: "profile", busy: true, profile: mastodon.Account{DisplayName: "looe", Acct: "looe"}}
	tt := ui.NewTester(func(c *ui.Context) { r.profileHeader(c, f, f.profile) }, 420, 560)
	if tt.HasText(locales.PostCount(0)) || tt.HasText(locales.FollowerCount(0)) {
		t.Fatal("unfetched counts displayed as zero")
	}
	if _, ok := tt.Find(locales.LoadingAvatar("looe")); !ok {
		t.Fatal("avatar loading missing")
	}
	savePreview(t, tt, "native-profile-loading")
	tt.SetDark(true)
	savePreview(t, tt, "native-profile-loading-dark")
	f.loaded, f.busy = true, false
	tt.Frame()
	if !tt.HasText(locales.PostCount(0)) {
		t.Fatal("confirmed zero count missing")
	}
	if _, ok := tt.Find(locales.LoadingAvatar("looe")); ok {
		t.Fatal("avatar loading remained after completion")
	}
	f.busy = true
	tt.Frame()
	if !tt.HasText(locales.PostCount(0)) {
		t.Fatal("refresh hid loaded profile")
	}
}
