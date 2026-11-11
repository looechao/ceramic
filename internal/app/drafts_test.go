package app

import (
	"bytes"
	"encoding/json"
	"image"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"ceramic/internal/mastodon"
	"github.com/egoist/mygo/ui"
)

func draftReader(t *testing.T, dir, account string) *reader {
	t.Helper()
	r := testReader(t)
	r.preferencesPath = filepath.Join(dir, "preferences.json")
	r.session = &mastodon.Session{Instance: "https://example.social", Scopes: mastodon.Scopes, Account: json.RawMessage(`{"id":"` + account + `","acct":"reader"}`)}
	return r
}

func TestDraftPersistenceAndIsolation(t *testing.T) {
	dir := t.TempDir()
	r := draftReader(t, dir, "one")
	r.showComposer()
	d := r.composer
	d.text, d.spoiler = "保存的文字", "内容警告"
	d.pollEnabled, d.pollMultiple = true, true
	d.pollOptions = []string{"山里", "海边"}
	r.closeComposer(d)
	if d.open || len(r.drafts) != 1 {
		t.Fatal("close did not save")
	}
	id := d.draftID
	again := draftReader(t, dir, "one")
	again.showComposer()
	if len(again.drafts) != 1 {
		t.Fatal("restart lost draft")
	}
	again.restoreDraft(again.composer, again.drafts[0])
	if again.composer.text != d.text || again.composer.spoiler != d.spoiler || !again.composer.pollMultiple || len(again.composer.pollOptions) != 2 {
		t.Fatal("restored draft differs")
	}
	again.composer.pollOptions[0] = "edited"
	if again.drafts[0].Options[0] != "山里" {
		t.Fatal("snapshot aliases editor")
	}
	other := draftReader(t, dir, "two")
	other.showComposer()
	if len(other.drafts) != 0 {
		t.Fatal("draft leaked to another account")
	}
	if !again.deleteDraft(again.composer, id) {
		t.Fatal("delete failed")
	}
	reloaded := draftReader(t, dir, "one")
	reloaded.showComposer()
	if len(reloaded.drafts) != 0 {
		t.Fatal("deleted draft returned")
	}
}

func TestDraftImageRoundTripAndSaveFailure(t *testing.T) {
	dir := t.TempDir()
	r := draftReader(t, dir, "one")
	r.showComposer()
	d := r.composer
	var buf bytes.Buffer
	png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 2, 2)))
	d.attachments = []composeAttachment{{name: "image.png", description: "海边", data: buf.Bytes(), digest: "fixture"}}
	d.close()
	restored := draftReader(t, dir, "one")
	restored.showComposer()
	restored.restoreDraft(restored.composer, restored.drafts[0])
	got := restored.composer.attachments
	if len(got) != 1 || got[0].description != "海边" || r.composeBitmap(got[0]) == nil || !bytes.Equal(got[0].data, buf.Bytes()) {
		t.Fatal("image data lost")
	}
	info, err := os.Stat(filepath.Join(r.draftDirectory(), d.draftID+".json"))
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatal("draft file must be private")
	}
	bad := draftReader(t, t.TempDir(), "one")
	bad.showComposer()
	bad.composer.text = "do not lose me"
	// A regular file occupies the storage directory, guaranteeing a write failure.
	os.WriteFile(filepath.Join(filepath.Dir(bad.preferencesPath), "drafts"), []byte("blocked"), 0600)
	bad.closeComposer(bad.composer)
	if !bad.composer.open || bad.composer.text == "" || bad.composer.message == "" {
		t.Fatal("failed save discarded draft")
	}
}

func TestNativeDraftShelf(t *testing.T) {
	r := draftReader(t, t.TempDir(), "one")
	r.home.loaded = true
	r.openWriting = r.showComposer
	tt := ui.NewTester(r.view, 900, 820)
	tt.Click("写帖子")
	d := r.composer
	tt.Click("草稿")
	if len(r.drafts) != 0 {
		t.Fatal("blank editor created a draft")
	}
	savePreview(t, tt, "native-draft-empty")
	tt.Click("返回编辑")
	d.text = "周末想去海边散步，拍一些照片。"
	tt.Frame()
	savePreview(t, tt, "native-compose-post-button")
	tt.Click("草稿")
	if len(r.drafts) != 1 || !d.draftsOpen {
		t.Fatal("draft shelf did not save current editor")
	}
	savePreview(t, tt, "native-draft-shelf")
	first := r.drafts[0].ID
	tt.Click("新建")
	d.text = "第二条草稿"
	tt.Frame()
	tt.Click("草稿")
	if len(r.drafts) != 2 {
		t.Fatal("new draft replaced old one")
	}
	tt.Click("继续编辑 " + first)
	if d.text != "周末想去海边散步，拍一些照片。" || d.draftsOpen {
		t.Fatal("resume did not restore")
	}
	tt.Click("草稿")
	tt.SetSize(420, 560)
	tt.SetDark(true)
	savePreview(t, tt, "native-draft-shelf-compact-dark")
	tt.Click("删除草稿 " + first)
	if len(r.drafts) != 1 {
		t.Fatal("delete did not remove row")
	}
}

func TestPublishedDraftIsRemoved(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) { io.WriteString(w, `{"id":"published"}`) }))
	defer server.Close()
	r := draftReader(t, t.TempDir(), "one")
	r.session.Instance = server.URL
	r.showComposer()
	d := r.composer
	d.text = "publish this draft"
	callbacks := make(chan func(), 1)
	d.update = func(fn func()) { callbacks <- fn }
	r.publish(d)
	id := d.draftID
	if id == "" {
		t.Fatal("publication did not preserve a retry draft")
	}
	select {
	case fn := <-callbacks:
		fn()
	case <-time.After(3 * time.Second):
		t.Fatal("timeout")
	}
	if len(r.drafts) != 0 || d.draftID != "" || d.text != "" {
		t.Fatal("published draft remains in editor")
	}
	if _, err := os.Stat(filepath.Join(r.draftDirectory(), id+".json")); !os.IsNotExist(err) {
		t.Fatalf("published draft still on disk: %v", err)
	}
}
