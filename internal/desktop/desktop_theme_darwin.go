//go:build desktop && darwin

package desktop

/*
#cgo LDFLAGS: -framework Cocoa
#include "desktop_theme_darwin.h"
*/
import "C"

import "github.com/crgimenes/glaze"

func applyTitlebar(w glaze.WebView, t titlebar) {
	dark := C.int(0)
	if t.dark {
		dark = 1
	}

	C.whiskr_apply_titlebar(
		w.Window(),
		dark,
		C.double(t.bg.r)/255,
		C.double(t.bg.g)/255,
		C.double(t.bg.b)/255,
	)
}
