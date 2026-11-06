package app

import "github.com/egoist/mygo/ui"

// Labels may change with the language; the selected value never does.
func localizedChoice[T comparable](c *ui.Context, selected *T, values []T, labels []string) ui.Element {
	label := labels[0]
	for i, value := range values {
		if *selected == value {
			label = labels[i]
			break
		}
	}
	element := ui.Select(c, &label, labels)
	if element.Changed() {
		for i, text := range labels {
			if text == label {
				*selected = values[i]
				break
			}
		}
	}
	return element
}

// Older drafts stored translated labels. Only this migration understands them.
func migratedAudience(value string) string {
	if stable, ok := map[string]string{"公开": "public", "低调公开": "unlisted", "仅关注者": "private", "私密提及": "direct"}[value]; ok {
		return stable
	}
	switch value {
	case "public", "unlisted", "private", "direct":
		return value
	}
	return "public"
}

func migratedDuration(d savedDraft) int {
	if d.PollSeconds > 0 {
		return d.PollSeconds
	}
	if seconds := map[string]int{"5 分钟": 300, "30 分钟": 1800, "1 小时": 3600, "6 小时": 21600, "1 天": 86400, "3 天": 259200, "7 天": 604800}[d.Duration]; seconds > 0 {
		return seconds
	}
	return 86400
}
