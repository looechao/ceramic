package main

import (
	"encoding/json"
	"github.com/egoist/mygo/ui"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func testReader(t *testing.T) *reader {
	return newReader(newApp(filepath.Join(t.TempDir(), "session.json")))
}
func savePreview(t *testing.T, tt *ui.Tester, name string) {
	if os.Getenv("CERAMIC_SCREENSHOTS") == "" {
		return
	}
	if err := os.MkdirAll("build/qa", 0755); err != nil {
		t.Fatal(err)
	}
	f, err := os.Create("build/qa/" + name + ".png")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err = png.Encode(f, tt.Image()); err != nil {
		t.Fatal(err)
	}
}
func TestNativeLogin(t *testing.T) {
	r := testReader(t)
	tt := ui.NewTester(r.view, 760, 820)
	if !tt.HasText("连接 Mastodon") {
		t.Fatal(tt.Texts())
	}
	savePreview(t, tt, "native-login")
	r.instance = "https://example.com/@invalid"
	tt.Frame()
	if err := tt.Click("连接 Mastodon"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(r.message, "域名") {
		t.Fatal("missing inline validation")
	}
	tt.SetSize(420, 640)
	savePreview(t, tt, "native-login-compact")
}
func TestNativeTimeline(t *testing.T) {
	r := testReader(t)
	r.session = &session{Instance: "https://example.com", Account: json.RawMessage(`{"display_name":"测试账号"}`)}
	r.applyPosts([]status{{ID: "3", Content: "<p>这是一条用于检查原生排版的测试动态。</p><p>文字直接由 Go 组件渲染，链接也可以 <a href=\"https://example.com\">点击打开</a>。</p>", Account: account{DisplayName: "林间散步", Acct: "forest@example.com"}, URL: "https://example.com/3", CreatedAt: "2026-11-05T05:00:00Z"}, {ID: "2", Content: "<p>折叠内容测试。</p>", Spoiler: "内容警告", Account: account{DisplayName: "远山", Acct: "mountain@example.com"}, URL: "https://example.com/2"}}, false)
	tt := ui.NewTester(r.view, 760, 820)
	savePreview(t, tt, "native-timeline")
	if !tt.HasText("Home") {
		t.Fatal(tt.Texts())
	}
	if err := tt.Click("展开隐藏内容：内容警告"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(time.Second)
	if !r.posts[1].expanded {
		t.Fatal("CW did not open")
	}
	if err := tt.Click("点击打开"); err != nil {
		t.Fatal(err)
	}
	if len(tt.OpenedURLs()) != 1 {
		t.Fatal("native link not opened")
	}
	r.applyPosts([]status{{ID: "2"}, {ID: "1", Content: "更早的动态"}}, true)
	if len(r.posts) != 3 {
		t.Fatal("duplicate pagination")
	}
	tt.Frame()
	tt.SetSize(420, 640)
	savePreview(t, tt, "native-timeline-compact")
	tt.SetDark(true)
	savePreview(t, tt, "native-timeline-dark")
	if err := tt.RightClick("我的"); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem("退出登录"); err != nil {
		t.Fatal(err)
	}
	if r.session != nil || !tt.HasText("连接 Mastodon") {
		t.Fatal("logout did not reset native view")
	}
}
func TestNativeTextSafety(t *testing.T) {
	runs := nativeText(`<p>Hello <strong>world</strong> &amp; 中文<a href="javascript:alert(1)">bad</a><a href="https://example.com">safe</a><script>evil()</script><span class="invisible">hidden</span></p>`)
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

func TestConcealedPostsDoNotExposeContent(t *testing.T) {
	r := testReader(t)
	r.session = &session{Account: json.RawMessage(`{"id":"me"}`)}
	var posts []status
	err := json.Unmarshal([]byte(`[
 {"id":"warn","content":"Filtered private body","account":{"display_name":"过滤测试"},"filtered":[{"filter":{"filter_action":"warn"}}]},
 {"id":"hide","content":"Never visible","filtered":[{"filter":{"filter_action":"hide"}}]},
 {"id":"boost-hide","reblog":{"id":"inner","content":"Also hidden","filtered":[{"filter":{"filter_action":"hide"}}]}},
 {"id":"sensitive","content":"Public caption","account":{"display_name":"媒体测试"},"sensitive":true,"media_attachments":[{"type":"image","url":"https://example.com/secret.png","description":"Private ALT"}]}
 ]`), &posts)
	if err != nil {
		t.Fatal(err)
	}
	r.home.applyPosts(posts, false)
	if len(r.home.posts) != 2 {
		t.Fatal("server-hidden posts were retained")
	}
	tt := ui.NewTester(r.view, 760, 820)
	if tt.HasText("Filtered private body") || tt.HasText("Private ALT") || len(r.images) != 0 {
		t.Fatal("collapsed content exposed or fetched")
	}
	savePreview(t, tt, "native-concealed-light")
	if err := tt.Click("展开隐藏内容：已被你的过滤规则折叠"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText("Filtered private body") {
		t.Fatal("warn did not reveal")
	}
	if err := tt.Click("收起内容"); err != nil {
		t.Fatal(err)
	}
	if tt.HasText("Filtered private body") {
		t.Fatal("warn did not conceal")
	}
	if err := tt.Click("展开隐藏内容：已被你的过滤规则折叠"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText("Filtered private body") || len(r.workspaces["home"].columns) != 1 {
		t.Fatal("blur click must reveal in place without opening a column")
	}
	if err := tt.Click("收起内容"); err != nil {
		t.Fatal(err)
	}
	// Reveal the second placeholder (sensitive media) by clicking its surface.
	r.home.list.ScrollTo(1, ui.Start)
	tt.Frame()
	if err := tt.Click("展开隐藏内容：敏感媒体"); err != nil {
		t.Fatal(err)
	}
	if !r.home.posts[1].mediaExpanded || len(r.workspaces["home"].columns) != 1 {
		t.Fatal("media blur opened a column")
	}
	r.home.posts[1].mediaExpanded = false
	r.home.list.ScrollTo(0, ui.Start)
	tt.Frame()

	tt.SetDark(true)
	savePreview(t, tt, "native-concealed-dark")
	tt.SetSize(420, 560)
	savePreview(t, tt, "native-concealed-compact")
}
