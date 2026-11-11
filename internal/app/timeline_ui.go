package app

import (
	"ceramic/locales"
	"time"

	"github.com/egoist/mygo/ui"

	"ceramic/internal/mastodon"
)

func (r *reader) searchHeader(c *ui.Context) {
	ui.Column(c).Padding(12, 14).Gap(10).Children(func() {

		ui.Row(c).Gap(8).Children(func() {
			input := hintInput(c, ui.SearchField(c, &r.query).Label(locales.SearchQuery()).Placeholder(locales.KeywordsPostLinkOrAccount()).Grow(1).OnSubmit(r.submitSearch))
			if input.Focused() {
				r.typing = true
			}
			hintAction(c, ui.Button(c, locales.Search()), r.submitSearch)
		})
		if hintSegmented(c, &r.searchTab, locales.Posts(), locales.Accounts()) {
			r.submitSearch()
		}
	})
}

func (r *reader) timelineView(c *ui.Context, f *feedState, body ui.Element) {
	// Header and account rows belong to the virtual list, so long biographies
	// and many search results cannot push the post list out of the window.
	header := 0
	if f.kind == "profile" || f.kind == "following" || f.kind == "followers" {
		header = 1
	}
	people := len(f.accounts)
	count := len(f.posts)
	if f.kind == "notifications" {
		count = len(f.notifications)
	}
	if f.revealPost != "" {
		for i, p := range f.posts {
			if p.status.ID == f.revealPost {
				r.columnView(f).list.ScrollTo(i, ui.Start)
				break
			}
		}
		f.revealPost = ""
	}
	r.columnView(f).list.Key = func(i int) any {
		if i < header {
			return "profile-header"
		}
		i -= header
		if i < people {
			return "account:" + f.accounts[i].ID
		}
		i -= people
		if i < count {
			if f.kind == "notifications" {
				return "notification:" + f.notifications[i].ID
			}
			return "post:" + f.posts[i].status.ID
		}
		return "footer"
	}
	// Virtual rows build during layout, after the pane scope has returned.
	blockedHints := hints(c).contentOnlyBlocked
	var hintViewport ui.Element
	hintViewport = ui.List(c.Key(f), &r.columnView(f).list, header+people+count+1, func(i int) {
		h := hints(c)
		previousClip, previousBlocked := h.clip, h.contentOnlyBlocked
		h.clip, h.contentOnlyBlocked = intersectHintRect(hintViewport.Bounds(), body.Bounds()), blockedHints
		defer func() { h.clip, h.contentOnlyBlocked = previousClip, previousBlocked }()
		if i < header {
			if f.kind == "profile" {
				r.profileHeader(c, f, f.profile)
			} else {
				ui.Text(c, "@"+f.profile.Acct).FontSize(12).TextColor(c.Theme().TextMuted).Padding(14, 22)
			}
			return
		}
		i -= header
		if i < people {
			r.accountRow(c, f, f.accounts[i])
			return
		}
		i -= people
		if i < count {
			if f.kind == "notifications" {
				r.notificationRow(c, f, f.notifications[i])
				return
			}
			r.post(c, f, f.posts[i])
			return
		}
		footer := ui.Column(c).Padding(24, 22).Gap(12)
		if f.loaded && !f.busy && !f.end && f.message == "" {
			if footer.InView() {
				r.loadFeed(f, true)
			} else {
				c.After(150 * time.Millisecond)
			}
		}
		footer.Children(func() {
			switch {
			case f.busy:
				loadingStatus(c, locales.Loading())
			case f.kind == "search" && f.query == "":
				r.exploreBrowse(c, f)
			case count == 0 && people == 0 && f.message == "":
				label := locales.NoPostsYetFollowPeopleToSeeTheirPostsHere()
				if f.kind == "bookmarks" {
					label = locales.NoBookmarksYet()
				}
				if f.kind == "favorites" {
					label = locales.NoFavoritesYet()
				}
				if f.kind == "profile" || f.kind == "hashtag" {
					label = locales.NoPostsToDisplayYet()
				}
				if f.kind == "search" {
					label = locales.NoResultsPostSearchDependsOnYourServerSIndex()
				}
				if f.kind == "following" || f.kind == "followers" {
					label = locales.NoAccountsToDisplayTheListMayBeEmptyOrPrivate()
				}
				if f.kind == "notifications" {
					label = locales.NoNotificationsYetFollowsBoostsFavoritesAndMentionsAppearHere()
					if f.notificationType == "mention" {
						label = locales.NoMentionsYet()
					}
				}
				ui.Text(c, label).TextColor(c.Theme().TextMuted)
			}
			if f.loaded && !f.busy && f.message == "" {
				if f.end {
					ui.Text(c, locales.YouAreAllCaughtUp()).FontSize(12).TextColor(c.Theme().TextMuted)
				} else {
					ui.Text(c, locales.KeepReading()).FontSize(12).TextColor(c.Theme().TextMuted)
				}
			}
		})
	}).TrackScroll(&r.columnView(f).scroll).Grow(1)
}

func notificationLabel(kind string) (string, *ui.SVG) {
	switch kind {
	case "mention":
		return locales.MentionedYou(), replyIcon
	case "reblog":
		return locales.BoostedYourPost(), boostIcon
	case "favourite":
		return locales.FavoritedYourPost(), heartIcon
	case "follow":
		return locales.FollowedYou(), navIcons[2]
	case "follow_request":
		return locales.RequestedToFollowYou(), navIcons[2]
	case "poll":
		return locales.PollEnded(), bellIcon
	case "update":
		return locales.EditedAPost(), bellIcon
	case "status":
		return locales.PublishedANewPost(), bellIcon
	default:
		return locales.Notifications(), bellIcon
	}
}

// notificationRow keeps the actor and event type visible; the post itself is
// only a preview so long boosts cannot push the next notification away.
func (r *reader) notificationRow(c *ui.Context, f *feedState, n mastodon.Notification) {
	t := c.Theme()
	warning := ""
	if n.Status != nil {
		hidden, filtered := statusFilter(*n.Status)
		if hidden {
			return
		}
		status := n.Status
		if status.Reblog != nil {
			status = status.Reblog
		}
		warning = status.Spoiler
		if filtered != "" {
			if warning != "" {
				warning += " · "
			}
			warning += filtered
		}
	}
	label, icon := notificationLabel(n.Type)
	var relation *mastodon.Status
	if n.Status != nil {
		s := *n.Status
		if s.Reblog != nil {
			s = *s.Reblog
		}
		relation = &s
	}
	r.listSurface(c, f, ui.Transparent, func(hover *listHoverState) {
		ui.Row(c).Gap(6).Children(func() {
			ui.Icon(c, icon).Size(14, 14).TextColor(t.Accent)
			ui.Text(c, label).FontSize(12).TextColor(t.TextMuted)
			ui.Spacer(c)
			if date, err := time.Parse(time.RFC3339, n.CreatedAt); err == nil {
				ui.Text(c, relativeTime(date)).FontSize(11).TextColor(t.TextMuted).Tooltip(date.Local().Format("2006-01-02 15:04"))
			}
		})
		ui.Row(c).Gap(10).AlignItems(ui.Start).Children(func() {
			r.avatar(c, f, n.Account, 34)
			ui.Column(c).Grow(1).Shrink(1).Gap(2).Children(func() {
				r.accountName(c, f, n.Account, 13).FontWeight(600).Shrink(1)
				ui.Text(c, "@"+n.Account.Acct).FontSize(11).TextColor(t.TextMuted).MaxLines(1)
			})
		})
		if relation != nil {
			visibilityBadge(c, relation.Visibility)
			content := func() {
				r.renderRichText(c, mastodon.ParseText(relation.Content, relation.Emojis...), 13, func() {
					r.openConversation(f, *relation)
				}, func(ev ui.InputEvent) {
					hover.input(ev, true)
					r.postTextInput(f, ev)
				}, r.postLink(f, *relation))
				r.pollView(c, relation.Poll, relation.URL)
			}
			if warning == "" {
				content()
			} else {
				if f.notificationExpanded == nil {
					f.notificationExpanded = map[string]*bool{}
				}
				expanded := f.notificationExpanded[n.ID]
				if expanded == nil {
					expanded = new(bool)
					f.notificationExpanded[n.ID] = expanded
				}
				concealedContent(c, warning, expanded, false, content)
			}
			hintAction(c, ui.Button(c, locales.ViewPost()).AlignSelf(ui.Start).Background(ui.Transparent).BorderWidth(0).Padding(4, 0).FontSize(12).TextColor(t.TextMuted), func() { r.openConversation(f, *relation) })
		}
	})
}

func (r *reader) accountRow(c *ui.Context, f *feedState, p mastodon.Account) {
	t := c.Theme()
	r.listSurface(c, f, ui.Transparent, func(_ *listHoverState) {
		ui.Row(c).FillWidth().Gap(12).AlignItems(ui.Center).Children(func() {
			r.avatar(c, f, p, 40)
			ui.Column(c).Grow(1).Shrink(1).Gap(4).Children(func() {
				ui.Row(c).Children(func() {
					r.accountName(c, f, p, 14).Bold().Shrink(1)
				})
				ui.Text(c, "@"+p.Acct).FontSize(12).TextColor(t.TextMuted)
			})
		})
	}).OnClick(func() { r.openProfileFrom(f, p) })
}

func (r *reader) avatar(c *ui.Context, f *feedState, p mastodon.Account, size float32) {
	// Keep the hit area fixed while the portrait grows slightly inside it.
	hit := ui.Box(c).Size(size, size).Shrink(0).Cursor(ui.CursorPointer).
		Label(locales.View() + p.Name()).OnClick(func() { r.openProfileFrom(f, p) })
	target := float32(1)
	if hit.Hovered() && !hit.Pressed() && !c.Preferences().ReduceMotion {
		target = 1.05
	}
	scale := hit.Animate("avatar-hover", target, listHoverDuration)
	if c.Preferences().ReduceMotion {
		scale = 1
	}
	hit.Children(func() {
		r.accountAvatar(c, p.Name(), p.Avatar, size, p.AvatarStatic).Scale(scale, scale)
	})
}

func (r *reader) profileHeader(c *ui.Context, f *feedState, p mastodon.Account) {
	t := c.Theme()
	ui.Column(c).Padding(22).Gap(20).BorderWidth(0, 0, 1, 0).BorderColor(t.Border).Children(func() {
		identity := func() {
			ui.Column(c).Grow(1).Shrink(1).Gap(5).Children(func() {
				r.emojiText(c, p.Name(), p.Emojis, 23).FontWeight(600)
				ui.Text(c, "@"+p.Acct).FontSize(12).TextColor(t.TextMuted).Selectable()
			})
		}
		if r.session != nil && p.ID != "" && !r.isSelf(p.ID) {
			ui.Column(c).Gap(12).Children(func() {
				ui.Row(c).FillWidth().Gap(14).AlignItems(ui.Center).Children(func() {
					r.avatar(c, f, p, 56)
					ui.Spacer(c)
					r.profileRelationship(c, f, p)
				})
				identity()
			})
		} else {
			ui.Row(c).Gap(14).AlignItems(ui.Center).Children(func() {
				r.avatar(c, f, p, 56)
				identity()
			})
		}
		if runs := mastodon.ParseText(p.Note, p.Emojis...); len(runs) > 0 {
			r.renderRichText(c, runs, 14, nil, nil, r.postLink(f, mastodon.Status{})).LineHeight(1.65).FillWidth()
		}
		if len(p.Fields) > 0 {
			ui.Column(c).FillWidth().Gap(2).Children(func() {
				for i, field := range p.Fields {
					ui.Column(c.Key(i)).FillWidth().Padding(12).Gap(5).Radius(4).Background(t.Text.Alpha(.035)).Children(func() {
						ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
							ui.Text(c, field.Name).FontSize(11).FontWeight(600).TextColor(t.TextMuted).Shrink(1)
							if field.VerifiedAt != "" {
								ui.Icon(c, verifiedIcon).Size(13, 13).TextColor(t.TextMuted).Tooltip(locales.ThisLinkWasVerifiedByTheServer())
							}
						})
						r.renderRichText(c, mastodon.ParseText(field.Value, p.Emojis...), 13, nil, nil, r.postLink(f, mastodon.Status{})).FillWidth().LineHeight(1.45)
					})
				}
			})
		}
		ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
			ui.Text(c, locales.PostCount(p.Statuses)).FontSize(12).TextColor(t.TextMuted).Shrink(1)
			hintAction(c, ui.Button(c, locales.FollowingCount(p.Following)).FontSize(12).Background(ui.Transparent).BorderWidth(0).Padding(4, 0).Shrink(1), func() { r.openConnections(f, "following") })
			hintAction(c, ui.Button(c, locales.FollowerCount(p.Followers)).FontSize(12).Background(ui.Transparent).BorderWidth(0).Padding(4, 0).Shrink(1), func() { r.openConnections(f, "followers") })
		})
		if target := mastodon.SafeURL(p.URL); target != "" {
			externalTextLink(c, locales.ViewProfileInBrowser(), target)
		}
		ui.Text(c, locales.Posts()).FontSize(14).FontWeight(600).Margin(4, 0, 0, 0)
	})
}
