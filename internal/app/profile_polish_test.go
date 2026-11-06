package app

import (
	"encoding/json"
	"testing"

	"ceramic/internal/mastodon"
	"github.com/egoist/mygo/ui"
)

func TestNativeProfileIntroduction(t *testing.T) {
	r := testReader(t)
	r.session = &mastodon.Session{Scopes: mastodon.Scopes, Account: json.RawMessage(`{"id":"me"}`)}
	var person mastodon.Account
	if err := json.Unmarshal([]byte(`{"id":"author","display_name":"林间 · Forest","acct":"forest@example.social","note":"<p>写代码，也记录日常。喜欢摄影、散步和慢慢读完一本书。</p><p>在这里分享 <strong>开源项目</strong> 与路上的见闻。<br>更多作品：<a href=\"https://example.com\">我的网站</a></p>","fields":[{"name":"网站","value":"<a href=\"https://example.com\">example.com</a>","verified_at":"2026-11-01T00:00:00Z"},{"name":"关于","value":"独立开发者 / 摄影爱好者","verified_at":null}],"statuses_count":128,"following_count":256,"followers_count":1024}`), &person); err != nil {
		t.Fatal(err)
	}
	r.navigate("profile")
	r.ownProfile.profile = person
	r.ownProfile.loaded, r.ownProfile.end = true, true
	tt := ui.NewTester(r.view, 1240, 820)
	for _, text := range []string{"网站", "example.com", "独立开发者 / 摄影爱好者", "256 正在关注"} {
		if !tt.HasText(text) {
			t.Fatalf("missing %s", text)
		}
	}
	if err := tt.Click("example.com"); err != nil {
		t.Fatal(err)
	}
	if len(r.workspaces["profile"].columns) != 1 {
		t.Fatal("bio link opened a conversation")
	}
	savePreview(t, tt, "native-profile-introduction")
	tt.SetSize(420, 560)
	savePreview(t, tt, "native-profile-introduction-compact")
	tt.SetDark(true)
	savePreview(t, tt, "native-profile-introduction-dark")
}

func TestNativePostActionHover(t *testing.T) {
	r := testReader(t)
	r.session = &mastodon.Session{Scopes: mastodon.Scopes, Account: json.RawMessage(`{"id":"me"}`)}
	post := mastodon.Status{ID: "42", Visibility: "public", Content: "悬停操作，点赞后显示红心。", Favourited: true, Favourites: 12, Reblogs: 3, URL: "https://example.com/post", Account: mastodon.Account{DisplayName: "林间"}}
	r.home.applyPosts([]mastodon.Status{post}, false)
	r.home.loaded, r.home.end = true, true
	tt := ui.NewTester(r.view, 760, 560)
	savePreview(t, tt, "native-post-action-rest")
	b, ok := tt.Find("取消喜欢 12")
	if !ok {
		t.Fatal("like action missing")
	}
	tt.Move(b.X+b.W/2, b.Y+b.H/2)
	savePreview(t, tt, "native-post-action-hover")
	tt.Press(b.X+b.W/2, b.Y+b.H/2)
	savePreview(t, tt, "native-post-action-pressed")
	tt.Release(b.X+b.W/2, b.Y+b.H/2)
	tt.SetDark(true)
	savePreview(t, tt, "native-post-action-dark")
	tt.SetSize(420, 560)
	savePreview(t, tt, "native-post-action-compact")
}
