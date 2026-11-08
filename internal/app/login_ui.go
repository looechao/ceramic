package app

import (
	"ceramic/locales"
	"encoding/json"
	"strings"

	"github.com/egoist/mygo/ui"

	"ceramic/internal/mastodon"
	"ceramic/resources"
)

func (r *reader) login() {
	if r.busy || r.authorizing {
		return
	}
	instance, err := mastodon.NormalizeInstance(r.instance)
	if err != nil {
		r.message = localizedError(err)
		return
	}
	f := r.feedState
	r.authFeed = f
	gen := r.generation
	r.busy = true
	r.authorizing = true
	r.message = locales.ConnectingToServer()
	go func() {
		err := r.service.Login(instance)
		r.update(func() {
			if gen != r.generation || !r.authorizing {
				return
			}
			f.busy = false
			if err != nil {
				r.authorizing = false
				f.message = localizedError(err)
				return
			}
			r.authorizing = true
			f.message = locales.AuthorizeInYourBrowserThenReturnToCeramic()
		})
	}()
}

func (r *reader) logout() {
	if r.composer != nil && !r.saveDraft(r.composer) {
		return
	}
	if err := r.service.Logout(); err != nil {
		r.message = localizedError(err)
		return
	}
	r.stopStream()
	r.unread = 0
	r.interactions = nil
	r.relationships = nil
	r.generation++
	r.session = nil
	r.authorizing = false
	r.home = &feedState{kind: "home"}
	r.feedState = r.home
	r.searchFeed = &feedState{kind: "search", searchKind: "statuses"}
	r.notifications = &feedState{kind: "notifications"}
	r.bookmarks = &feedState{kind: "bookmarks"}
	r.favorites = &feedState{kind: "favorites"}
	if r.composer != nil && r.composer.cancelPublish != nil {
		r.composer.cancelPublish()
	}
	if r.composer != nil && r.composer.close != nil {
		r.composer.busy = false
		r.composer.open = false
	}
	if r.composer != nil {
		r.composer.stopMentions()
	}
	r.composer = nil
	r.postManager = nil
	r.deletedPosts = nil
	r.drafts = nil
	r.draftsOwner = ""
	r.ownProfile = nil
	r.workspaces = nil
	r.authFeed = nil
	r.searchAllowed = false
	r.selected = "home"
	r.query = ""
	r.searchTab = 0
	r.images = map[string]*imageEntry{}
}

func (r *reader) cancelAuthorization() {
	r.service.CancelLogin()
	r.authorizing = false
	for _, f := range []*feedState{r.authFeed, r.feedState, r.home, r.searchFeed, r.notifications, r.ownProfile} {
		if f != nil && f == r.authFeed {
			f.busy = false
			f.message = ""
		}
	}
}

var loginLogo = func() *ui.Bitmap {
	bitmap, err := ui.DecodeBitmap(resources.Icon)
	if err != nil {
		panic(err)
	}
	return bitmap
}()

// knownInstances is the bundled directory: suggestions only, any domain
// can still sign in.
var knownInstances = func() []string {
	var list []string
	if err := json.Unmarshal(resources.Instances, &list); err != nil {
		panic(err)
	}
	return list
}()

const maxInstanceSuggestions = 6

// flagshipInstance is the example under the field, so it leads its prefix.
const flagshipInstance = "mastodon.social"

// instanceSuggestions matches what was typed against the directory:
// domains starting with it first, then those with a later label starting
// with it (social → mastodon.social), then any containing it. ui.Autocomplete
// keeps this order and hides an exact match.
func instanceSuggestions(typed string) []string {
	q := strings.ToLower(strings.TrimSpace(typed))
	if q == "" {
		return nil
	}
	var starts, labels, contains []string
	for _, d := range knownInstances {
		switch {
		case d == q:
			return nil
		case strings.HasPrefix(d, q):
			if d == flagshipInstance {
				starts = append([]string{d}, starts...)
			} else {
				starts = append(starts, d)
			}
		case strings.Contains(d, "."+q) || strings.Contains(d, "-"+q):
			labels = append(labels, d)
		case strings.Contains(d, q):
			contains = append(contains, d)
		}
	}
	out := append(append(starts, labels...), contains...)
	return out[:min(len(out), maxInstanceSuggestions)]
}

func (r *reader) loginView(c *ui.Context) {
	t := c.Theme()
	_, height := c.Size()
	ui.Scroll(c).Grow(1).Children(func() {
		ui.Column(c).FillWidth().MinHeight(max(0, height-52)).Padding(24).AlignItems(ui.Center).Justify(ui.Center).Children(func() {
			ui.Column(c).FillWidth().MaxWidth(360).AlignItems(ui.Center).Children(func() {
				ui.Image(c, loginLogo).Size(72, 72).Fit(ui.Contain).Label("Ceramic")
				ui.Text(c, locales.LogInToCeramic()).TextAlign(ui.Center).FontSize(24).FontWeight(600).Margin(12, 0, 24, 0)
				ui.Column(c).FillWidth().AlignItems(ui.Center).Children(func() {
					ui.Text(c, locales.Instance()).FontSize(13).FontWeight(500).Margin(0, 0, 8, 0)
					hintInput(c, ui.Autocomplete(c.Key("instance"), &r.instance, instanceSuggestions(r.instance)).Label(locales.InstanceAddress()).Placeholder(locales.InstanceDomain()).TextAlign(ui.Center).FillWidth().Height(42).Disabled(r.busy || r.authorizing).OnSubmit(r.login))
					ui.Text(c, locales.ForExampleMastodonSocial()).TextAlign(ui.Center).FontSize(12).TextColor(t.TextMuted).Margin(8, 0, 0, 0)
					ui.Column(c).FillWidth().MinHeight(40).AlignItems(ui.Center).Justify(ui.Center).Margin(20, 0, 0, 0).Children(func() {
						if r.busy || r.authorizing {
							label := locales.Connecting()
							if !r.busy {
								label = locales.WaitingForBrowserAuthorization()
							}
							ui.Row(c).FillWidth().Gap(10).Justify(ui.Center).AlignItems(ui.Center).Children(func() {
								loadingIndicator(c).Size(18, 18).Label(label)
								ui.Text(c, label).FontSize(13).TextAlign(ui.Center).MinWidth(0)
							})
						} else {
							hintAction(c, ui.PrimaryButton(c, locales.Continue()).FontSize(13).FillWidth().Height(40).Radius(8), r.login)
						}
					})
					if r.message != "" {
						ui.Text(c, r.message).TextAlign(ui.Center).FontSize(12).LineHeight(1.5).TextColor(t.TextMuted).FillWidth().Margin(8, 0, 0, 0)
					}
					if r.authorizing {
						hintAction(c, ui.Button(c, locales.CancelConnection()).FontSize(13).Margin(12, 0, 0, 0), r.cancelAuthorization)
					} else if !r.busy {
						ui.Text(c, locales.AuthorizeReadWriteFollowAndPushAccessInYourBrowser()).TextAlign(ui.Center).FontSize(11).LineHeight(1.5).TextColor(t.TextMuted).FillWidth().Margin(16, 0, 0, 0)
					}
				})
			})
		})
	})
}
