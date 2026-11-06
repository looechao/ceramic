package app

import (
	"github.com/egoist/mygo/ui"
	"testing"
)

func TestNativeLoadingStates(t *testing.T) {
	r := testReader(t)
	source := "https://example.com/avatar.png"
	entry := &imageEntry{}
	r.images[source] = entry
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Padding(24).Gap(20).Children(func() {
			r.accountAvatar(c, "林间", source, 42)
			loadingStatus(c, "正在加载…")
		})
	}, 420, 560)
	if _, ok := tt.Find("正在加载 林间 的头像"); !ok {
		t.Fatal("pending avatar must show progress")
	}
	savePreview(t, tt, "native-loading")
	tt.SetDark(true)
	savePreview(t, tt, "native-loading-dark")
	entry.failed, entry.ready = true, true
	tt.SetSize(421, 560)
	if _, ok := tt.Find("正在加载 林间 的头像"); ok {
		t.Fatal("failed avatar must stop progress")
	}
	savePreview(t, tt, "native-loading-fallback")
}
