package app

import (
	"slices"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestInstanceSuggestions(t *testing.T) {
	if got := instanceSuggestions("mast"); len(got) != maxInstanceSuggestions || got[0] != "mastodon.social" {
		t.Fatalf("mast: %v", got)
	}
	if got := instanceSuggestions("  HACHY "); !slices.Equal(got, []string{"hachyderm.io"}) {
		t.Fatalf("hachy: %v", got)
	}
	// A later label matches too, so "social" finds the many *.social domains.
	if got := instanceSuggestions("social"); len(got) != maxInstanceSuggestions {
		t.Fatalf("social: %v", got)
	}
	for _, q := range []string{"", "   ", "mastodon.social", "no-such-instance.example"} {
		if got := instanceSuggestions(q); len(got) != 0 {
			t.Fatalf("%q: %v", q, got)
		}
	}
}

func TestNativeLoginSuggestions(t *testing.T) {
	r := testReader(t)
	tt := ui.NewTester(r.view, 760, 820)
	if err := tt.Click("实例地址"); err != nil {
		t.Fatal(err)
	}
	tt.Type("hachy")
	tt.Frame()
	if !tt.HasText("hachyderm.io") {
		t.Fatal(tt.Texts())
	}
	savePreview(t, tt, "native-login-suggestions")
	tt.SetDark(true)
	savePreview(t, tt, "native-login-suggestions-dark")
	tt.SetDark(false)
	if err := tt.Click("hachyderm.io"); err != nil {
		t.Fatal(err)
	}
	if r.instance != "hachyderm.io" {
		t.Fatalf("chose %q", r.instance)
	}
	// Any domain still signs in: the directory only suggests.
	r.instance = "example.com"
	tt.Frame()
	if got := instanceSuggestions(r.instance); len(got) != 0 {
		t.Fatalf("example.com: %v", got)
	}
}
