package app

import (
	"ceramic/locales"
	"slices"
	"strings"
	"time"

	"ceramic/internal/mastodon"
	"github.com/egoist/mygo/ui"
)

func pollPercentage(p *mastodon.Poll, option mastodon.PollOption) (float32, bool) {
	if option.VotesCount == nil {
		return 0, false
	}
	total := p.VotesCount
	if p.Multiple && p.VotersCount != nil {
		total = *p.VotersCount
	}
	if total <= 0 {
		return 0, true
	}
	return min(100, max(0, float32(*option.VotesCount)*100/float32(total))), true
}

// A missing count means unpublished results, not zero votes. Keep polls inside
// the same content-warning boundary as their question in posts and notifications.
func (r *reader) pollView(c *ui.Context, p *mastodon.Poll, statusURL string) {
	if p == nil || len(p.Options) == 0 {
		return
	}
	t := c.Theme()
	ui.Column(c).FillWidth().Gap(6).Label(locales.Poll()).Children(func() {
		for i, option := range p.Options {
			percent, published := pollPercentage(p, option)
			ui.Box(c.Key(i)).FillWidth().Radius(7).Clip().Background(t.Surface).Children(func() {
				if published && percent > 0 {
					ui.Box(c).Absolute().Left(0).Top(0).Bottom(0).WidthPercent(percent).Background(t.Text.Alpha(.08)).PassThrough()
				}
				ui.Row(c).FillWidth().Padding(8, 10).Gap(8).AlignItems(ui.Center).Children(func() {
					r.emojiText(c, option.Title, p.Emojis, 13).Grow(1).Shrink(1)
					if slices.Contains(p.OwnVotes, i) {
						ui.Text(c, locales.Selected()).FontSize(11).TextColor(t.TextMuted)
					}
					if published {
						ui.Text(c, locales.PollOptionResults(float64(percent), *option.VotesCount)).FontSize(11).TextColor(t.TextMuted).Shrink(0)
					}
				})
			})
		}
		mode := locales.SingleChoice()
		if p.Multiple {
			mode = locales.MultipleChoice()
		}
		total := locales.VoteCount(p.VotesCount)
		if p.VotersCount != nil {
			total = locales.VoterCount(*p.VotersCount)
		}
		parts := []string{mode, total}
		expires, err := time.Parse(time.RFC3339, p.ExpiresAt)
		ended := p.Expired || (err == nil && !expires.After(c.Now()))
		if ended {
			parts = append(parts, locales.Ended())
		} else if err == nil {
			parts = append(parts, locales.Ends()+expires.Local().Format("01-02 15:04"))
		}
		if p.Voted {
			parts = append(parts, locales.Voted())
		}
		ui.Text(c, strings.Join(parts, " · ")).FontSize(11).TextColor(t.TextMuted)
		if !ended && !p.Voted {
			if target := mastodon.SafeURL(statusURL); target != "" {
				externalTextLink(c, locales.VoteInBrowser(), target)
			}
		}
	})
}
