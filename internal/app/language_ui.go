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
