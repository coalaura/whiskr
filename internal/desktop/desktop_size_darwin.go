//go:build desktop && darwin

package desktop

/*
#cgo LDFLAGS: -framework Cocoa
#include "desktop_size_darwin.h"
*/
import "C"

func primaryWorkArea() (int, int, bool) {
	var (
		w C.int
		h C.int
	)

	C.whiskr_work_area(&w, &h)
	if w <= 0 || h <= 0 {
		return 0, 0, false
	}

	h -= 28

	return int(w), int(h), true
}
