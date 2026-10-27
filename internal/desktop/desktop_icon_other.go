//go:build desktop && !windows

package desktop

import "github.com/crgimenes/glaze"

func applyWindowIcons(glaze.WebView) func() {
	return func() {}
}
