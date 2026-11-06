package app

import (
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"ceramic/internal/mastodon"
)

func TestNativeNavigation(t *testing.T) {
	r := testReader(t)
	r.session = &mastodon.Session{Instance: "https://example.com", Account: json.RawMessage(`{"id":"me","display_name":"我的账号","acct":"me"}`)}
	r.applyPosts([]mastodon.Status{{ID: "3", Content: "<p>首页保留的正文</p>", Account: mastodon.Account{ID: "42", DisplayName: "林间", Acct: "forest@example.com"}, Spoiler: "内容警告"}}, false)
	home := r.feedState
	tt := ui.NewTester(r.view, 900, 820)
	if err := tt.Click("展开隐藏内容：内容警告"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(time.Second)
	if !r.posts[0].expanded {
		t.Fatal("CW closed")
	}
	if err := tt.Click("Explore"); err != nil {
		t.Fatal(err)
	}
	if r.kind != "search" || tt.HasText("补充搜索权限") || r.searchKind != "statuses" {
		t.Fatal(tt.Texts())
	}
	r.query = "forest"
	r.submitSearch()
	r.accounts = []mastodon.Account{{ID: "42", DisplayName: "林间", Acct: "forest@example.com"}}
	r.loaded = true
	r.end = true
	tt.Frame()
	savePreview(t, tt, "native-search")
	if err := tt.Click("林间"); err != nil {
		t.Fatal(err)
	}
	if r.kind != "profile" || r.profile.ID != "42" {
		t.Fatal("profile not opened")
	}
	r.profile.Note = "<p>在山间散步，记录日常。<a href=\"https://example.com\">个人网站</a></p>"
	r.profile.Followers = 17
	r.profile.Statuses = 31
	r.applyPosts([]mastodon.Status{{ID: "5", Content: "<p>个人页的帖子。</p>", Account: r.profile}}, false)
	tt.Frame()
	savePreview(t, tt, "native-profile")
	if err := tt.Click("关闭此列"); err != nil {
		t.Fatal(err)
	}
	r.navigate("search")
	tt.Frame()
	if r.kind != "search" || len(r.accounts) != 1 || r.query != "forest" {
		t.Fatal("search was lost")
	}
	r.navigate("home")
	tt.Frame()
	if r.feedState != home || len(r.posts) != 1 || !r.posts[0].expanded {
		t.Fatal("home was reset")
	}
	r.navigate("profile")
	tt.Frame()
	if r.profile.ID != "me" {
		t.Fatal("Profile should open self")
	}
	for _, size := range [][2]float32{{420, 560}, {900, 560}, {420, 820}, {900, 820}} {
		tt.SetSize(int(size[0]), int(size[1]))
		savePreview(t, tt, fmt.Sprintf("native-profile-size-%.0fx%.0f", size[0], size[1]))
	}
	tt.SetDark(true)
	savePreview(t, tt, "native-profile-dark")
}

// A response belongs to the feed that requested it, even if navigation occurs
// before it finishes. Logout invalidates every pending result.
func TestLoadKeepsResponseOnOriginPage(t *testing.T) {
	started := make(chan struct{}, 2)
	release := make(chan struct{}, 2)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started <- struct{}{}
		<-release
		io.WriteString(w, `[{"id":"9","content":"home"}]`)
	}))
	defer upstream.Close()
	r := testReader(t)
	r.session = &mastodon.Session{Instance: upstream.URL, Token: "test"}
	r.service.SetSession(r.session)
	updates := make(chan func(), 2)
	r.update = func(f func()) { updates <- f }
	r.load(false)
	<-started
	r.navigate("search")
	release <- struct{}{}
	(<-updates)()
	if len(r.searchFeed.posts) != 0 || len(r.home.posts) != 1 || r.home.busy {
		t.Fatal("response crossed navigation")
	}
	r.navigate("home")
	r.load(false)
	<-started
	r.logout()
	release <- struct{}{}
	(<-updates)()
	if r.session != nil || len(r.posts) != 0 {
		t.Fatal("logged-out response applied")
	}
}

func TestNativeColumnsAndMedia(t *testing.T) {
	r := testReader(t)
	r.session = &mastodon.Session{Instance: "https://example.com", Scopes: mastodon.Scopes, Account: json.RawMessage(`{"id":"me","display_name":"我的账号","acct":"me"}`)}
	author := mastodon.Account{ID: "42", DisplayName: "林间散步", Acct: "forest@example.com", Note: "<p>记录旅途与日常。</p>", Followers: 127, Statuses: 83}
	picture := mastodon.Media{Type: "image", URL: "https://example.com/photo.png", Preview: "https://example.com/photo.png", Description: "媒体排版测试图"}
	img := image.NewRGBA(image.Rect(0, 0, 600, 320))
	draw.Draw(img, img.Bounds(), &image.Uniform{color.RGBA{80, 110, 94, 255}}, image.Point{}, draw.Src)
	bitmap := ui.NewBitmap(img)
	r.images[picture.URL] = &imageEntry{bitmap: bitmap, ready: true}
	author.Avatar = picture.URL
	r.home.applyPosts([]mastodon.Status{{ID: "3", Content: "<p>今天走了另一条回家的路。</p>", Account: author, Media: []mastodon.Media{picture}, Replies: 2, Favourites: 7}}, false)
	r.home.loaded = true
	r.home.end = true
	r.ensureProfile()
	r.ownProfile.profile.Note = "<p>在这里，慢慢阅读关注的人。</p>"
	r.ownProfile.loaded = true
	r.ownProfile.end = true
	r.query = "林间"
	r.searchFeed.query = r.query
	r.searchFeed.accounts = []mastodon.Account{author}
	r.searchFeed.loaded = true
	r.searchFeed.end = true
	tt := ui.NewTester(r.view, 1240, 820)
	if !tt.HasText("刷新 首页") || tt.HasText("刷新 Explore") || tt.HasText("刷新 个人资料") {
		t.Fatal("only active tab root should be visible")
	}
	savePreview(t, tt, "native-columns-light")
	tt.SetDark(true)
	savePreview(t, tt, "native-columns-dark")
	// Opening media changes only its own column.
	if err := tt.Click("媒体排版测试图"); err != nil {
		t.Fatal(err)
	}
	if r.home.preview == nil || r.searchFeed.preview != nil {
		t.Fatal("preview changed wrong column")
	}
	if err := tt.Click("关闭图片"); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("林间散步"); err != nil {
		t.Fatal(err)
	}
	if r.feedState.profile.ID != "42" || r.selected != "home" || len(r.workspaces["home"].columns) != 2 || len(r.home.posts) != 1 || len(r.searchFeed.accounts) != 1 {
		t.Fatal("profile replaced another column")
	}
	if err := tt.Click("关闭此列"); err != nil {
		t.Fatal(err)
	}
	if r.feedState != r.home {
		t.Fatal("profile back did not restore self")
	}

	// A conversation and its author extend Home, while Search keeps its own path.
	if err := tt.Click("回复 2"); err != nil {
		t.Fatal(err)
	}
	conversation := r.feedState
	if conversation.kind != "conversation" || len(r.workspaces["home"].columns) != 2 {
		t.Fatal("conversation not appended")
	}
	r.openProfileFrom(conversation, author)
	profile := r.feedState
	tt.Frame()
	savePreview(t, tt, "native-path-wide")
	tt.SetDark(false)
	savePreview(t, tt, "native-path-light")
	tt.SetDark(true)
	r.navigate("search")
	tt.Frame()
	if tt.HasText("刷新 首页") || !tt.HasText("刷新 Explore") {
		t.Fatal("tab roots mixed")
	}
	r.openProfileFrom(r.searchFeed, author)
	searchProfile := r.feedState
	r.navigate("home")
	tt.Frame()
	if r.feedState != profile || len(r.workspaces["home"].columns) != 3 {
		t.Fatal("Home path lost")
	}
	tt.SetSize(420, 560)
	if tt.HasText("刷新 首页") || !tt.HasText("刷新 林间散步") {
		t.Fatal("compact should show latest detail")
	}
	savePreview(t, tt, "native-columns-compact")
	r.closeFocused()
	tt.Frame()
	if r.feedState != conversation {
		t.Fatal("closing detail lost source column")
	}
	r.navigate("search")
	if r.feedState != searchProfile {
		t.Fatal("Search path lost")
	}
	r.navigate("home")
	r.closeFocused()
	tt.SetSize(1240, 560)
	if r.feedState != r.home || !tt.HasText("刷新 首页") {
		t.Fatal("root not restored")
	}
	savePreview(t, tt, "native-columns-short")

}

func TestNativeReadingPositionSurvivesNavigation(t *testing.T) {
	r := testReader(t)
	r.session = &mastodon.Session{Account: json.RawMessage(`{"id":"me"}`)}
	for i := 0; i < 60; i++ {
		r.home.applyPosts([]mastodon.Status{{ID: fmt.Sprint(i), Content: fmt.Sprintf("reading-row-%d", i), Account: mastodon.Account{DisplayName: "测试作者"}}}, true)
	}
	r.home.loaded = true
	tt := ui.NewTester(r.view, 760, 820)
	r.home.list.ScrollTo(30, ui.Start)
	tt.Frame()
	before, ok := tt.Find("reading-row-30")
	if !ok {
		t.Fatal("scroll target missing")
	}
	r.navigate("search")
	tt.Frame()
	r.navigate("home")
	tt.Frame()
	after, ok := tt.Find("reading-row-30")
	if !ok || math.Abs(float64(before.Y-after.Y)) > 0.1 {
		t.Fatalf("reading position changed: %v -> %v", before, after)
	}
}

func TestImageCacheEvictsLeastRecentlyUsed(t *testing.T) {
	r := testReader(t)
	for i := 0; i < 130; i++ {
		r.images[fmt.Sprint(i)] = &imageEntry{ready: true, failed: true, used: uint64(i)}
	}
	r.trimImages()
	if len(r.images) != 128 || r.images["0"] != nil || r.images["1"] != nil || r.images["129"] == nil {
		t.Fatal("cache did not evict oldest entries")
	}
}

// The system material must show only through the rail, never through posts.
func TestSidebarMaterialKeepsReadingColumnsOpaque(t *testing.T) {
	r := testReader(t)
	r.session = &mastodon.Session{Account: json.RawMessage(`{"id":"me"}`)}
	tt := ui.NewTester(r.view, 1240, 820)
	tt.SetVibrancy(true)
	_, _, _, railAlpha := tt.Image().At(68, 400).RGBA()
	_, _, _, bodyAlpha := tt.Image().At(200, 400).RGBA()
	_, _, _, headerAlpha := tt.Image().At(300, 35).RGBA()
	if headerAlpha != 65535 {
		t.Fatal("column toolbar must remain opaque")
	}
	for _, point := range [][2]int{{94, 35}, {900, 35}, {300, 5}, {300, 815}, {101, 13}, {101, 806}} {
		_, _, _, a := tt.Image().At(point[0], point[1]).RGBA()
		if a != 65535 {
			t.Fatalf("workspace leaked material at %v", point)
		}
	}
	if railAlpha != 0 || bodyAlpha != 65535 {
		t.Fatalf("unexpected alpha: rail=%d body=%d", railAlpha, bodyAlpha)
	}
	tt.SetVibrancy(false)
	_, _, _, railAlpha = tt.Image().At(68, 400).RGBA()
	if railAlpha != 65535 {
		t.Fatal("missing fallback sidebar background")
	}
}

func TestPostClickAndColumnScrolling(t *testing.T) {
	r := testReader(t)
	r.session = &mastodon.Session{Account: json.RawMessage(`{"id":"me"}`)}
	author := mastodon.Account{ID: "author", DisplayName: "作者"}
	r.home.applyPosts([]mastodon.Status{{ID: "post", Content: `点击正文打开讨论 <a href="https://example.com">外部链接</a>`, Account: author}}, false)
	r.home.loaded = true
	tt := ui.NewTester(r.view, 1240, 820)
	if tt.HasText("关闭此列") {
		t.Fatal("root has close control")
	}
	ws := r.workspaces["home"]
	r.closeColumn(ws, r.home)
	if len(ws.columns) != 1 {
		t.Fatal("root closed")
	}
	if err := tt.Click("外部链接"); err != nil {
		t.Fatal(err)
	}
	if len(ws.columns) != 1 {
		t.Fatal("link click opened discussion")
	}
	if err := tt.Click("点击正文打开讨论 外部链接"); err != nil {
		t.Fatal(err)
	}
	if len(ws.columns) != 2 || ws.columns[1].kind != "conversation" {
		t.Fatal("body click did not open discussion")
	}
	if !tt.HasText("关闭此列") {
		t.Fatal("detail missing close control")
	}
	r.openProfileFrom(r.feedState, author)
	tt.Frame()
	before := ws.scroll.X
	tt.Scroll(900, 300, -200, 0)
	if ws.scroll.X >= before {
		t.Fatalf("touchpad horizontal scrolling failed: %v -> %v", before, ws.scroll.X)
	}
	before = ws.scroll.X
	// Tester.Wheel cannot carry modifiers; exercise the horizontal wheel delta
	// produced by MyGo after its built-in Shift axis conversion.
	tt.Wheel(900, 300, -100, 0)
	tt.Advance(time.Second)
	if ws.scroll.X >= before {
		t.Fatalf("horizontal wheel failed: %v -> %v", before, ws.scroll.X)
	}
}

func TestSidebarAccountAndWindowControls(t *testing.T) {
	r := testReader(t)
	tt := ui.NewTester(r.view, 420, 560)
	tt.SetTitleBar(ui.TitleBar{Height: 52, Left: 88})
	if !tt.HasText("我的") || !tt.HasText("首页") || !tt.HasText("Explore") {
		t.Fatal("signed-out sidebar missing")
	}
	mine, ok := tt.Find("我的")
	home, _ := tt.Find("首页")
	if !ok || mine.Y >= home.Y || mine.Y < 52 {
		t.Fatal("sidebar overlaps controls or account is not above navigation")
	}
	r.session = &mastodon.Session{Account: json.RawMessage(`{"id":"me","display_name":"Me"}`)}
	tt.Frame()
	if err := tt.Click("我的"); err != nil {
		t.Fatal(err)
	}
	if r.selected != "profile" {
		t.Fatal("account did not open profile")
	}
}

func TestFocusedColumnsCloseIndependently(t *testing.T) {
	r := testReader(t)
	r.session = &mastodon.Session{Account: json.RawMessage(`{"id":"me"}`)}
	closed := 0
	r.closeWindow = func() { closed++ }
	tt := ui.NewTester(r.view, 1240, 820)
	ws := r.workspaces["home"]
	r.openProfileFrom(r.home, mastodon.Account{ID: "one", DisplayName: "First"})
	first := r.feedState
	r.openProfileFrom(first, mastodon.Account{ID: "two", DisplayName: "Second"})
	second := r.feedState
	tt.Frame()
	if ws.focused != second {
		t.Fatal("new column not focused")
	}
	// The title is inside the column; clicking it must select the column.
	if err := tt.Click("First"); err != nil {
		t.Fatal(err)
	}
	if ws.focused != first {
		t.Fatal("click did not focus first detail")
	}
	r.navigate("search")
	r.navigate("home")
	if r.feedState != first {
		t.Fatal("tab lost focus")
	}
	tt.Frame()
	tt.Key(ui.Cmd, ui.KeyW)
	if closed != 0 || len(ws.columns) != 2 || ws.columns[1] != second || ws.focused != r.home {
		t.Fatal("close removed wrong columns or window")
	}
	tt.Frame()
	// Clicking a nested refresh button also selects its originating column.
	if err := tt.Click("刷新 Second"); err != nil {
		t.Fatal(err)
	}
	if ws.focused != second {
		t.Fatal("nested control did not focus column")
	}
	tt.Key(ui.Cmd, ui.KeyW)
	if len(ws.columns) != 1 || closed != 0 {
		t.Fatal("detail close closed root")
	}
	tt.Key(ui.Cmd, ui.KeyW)
	if closed != 1 || len(ws.columns) != 1 {
		t.Fatal("root Cmd-W should close window")
	}
}

func TestAutomaticPaginationPausesOnError(t *testing.T) {
	requests := make(chan string, 8)
	responses := make(chan int, 8)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		requests <- req.URL.Query().Get("max_id")
		code := <-responses
		w.WriteHeader(code)
		io.WriteString(w, "[]")
	}))
	defer server.Close()
	r := testReader(t)
	r.session = &mastodon.Session{Instance: server.URL, Token: "test", Account: json.RawMessage(`{"id":"me"}`)}
	r.service.SetSession(r.session)
	r.home.applyPosts([]mastodon.Status{{ID: "123", Content: "Page one"}}, false)
	r.home.loaded = true
	updates := make(chan func(), 8)
	r.update = func(f func()) { updates <- f }
	tt := ui.NewTester(r.view, 760, 820)
	tt.Frame()
	select {
	case cursor := <-requests:
		if cursor != "123" {
			t.Fatal(cursor)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("visible footer did not load next page")
	}
	for i := 0; i < 5; i++ {
		tt.Frame()
	}
	select {
	case <-requests:
		t.Fatal("duplicate request while busy")
	default:
	}
	responses <- 500
	select {
	case f := <-updates:
		f()
	case <-time.After(3 * time.Second):
		t.Fatal("missing error")
	}
	for i := 0; i < 5; i++ {
		tt.Frame()
	}
	select {
	case <-requests:
		t.Fatal("error caused automatic retry")
	default:
	}
	if err := tt.Click("重试"); err != nil {
		t.Fatal(err)
	}
	select {
	case cursor := <-requests:
		if cursor != "123" {
			t.Fatal("retry refreshed instead of paginating", cursor)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("retry not sent")
	}
	responses <- 200
	select {
	case f := <-updates:
		f()
	case <-time.After(3 * time.Second):
		t.Fatal("missing result")
	}
	tt.Frame()
	if !r.home.end || len(r.home.posts) != 1 || tt.HasText("加载更多") {
		t.Fatal("end state incorrect")
	}
	select {
	case <-requests:
		t.Fatal("requested after end")
	default:
	}
}

func TestConversationReusesExistingColumn(t *testing.T) {
	r := testReader(t)
	r.session = &mastodon.Session{Account: json.RawMessage(`{"id":"me"}`)}
	post := mastodon.Status{ID: "original", Content: "Original post"}
	r.openConversation(r.home, post)
	ws := r.workspaces["home"]
	original := ws.focused
	original.scroll.Y = 120
	r.openConversation(r.home, mastodon.Status{ID: "another", Content: "Another post"})
	other := ws.focused
	for _, source := range []*feedState{r.home, other} {
		r.openConversation(source, mastodon.Status{ID: "boost", Reblog: &post})
		if len(ws.columns) != 3 || ws.focused != original || r.feedState != original {
			t.Fatal("reopening a post must focus its existing conversation")
		}
		if original.scroll.Y != 120 || !ws.revealFocus || ws.reveal {
			t.Fatal("reopening must preserve reading position and reveal the existing column")
		}
	}
	r.closeColumn(ws, original)
	r.openConversation(r.home, post)
	if len(ws.columns) != 3 || ws.focused == original || ws.focused.focalID != post.ID {
		t.Fatal("closed conversations must be openable again")
	}
}
