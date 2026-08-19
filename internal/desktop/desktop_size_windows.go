//go:build desktop && windows

package desktop

import (
	"syscall"
	"unsafe"
)

var (
	user32                    = syscall.NewLazyDLL("user32.dll")
	procSystemParametersInfoW = user32.NewProc("SystemParametersInfoW")
)

type workarea struct {
	left   int32
	top    int32
	right  int32
	bottom int32
}

func primaryWorkArea() (int, int, bool) {
	var r workarea

	const spiGetWorkArea = 48

	ret, _, _ := procSystemParametersInfoW.Call(spiGetWorkArea, 0, uintptr(unsafe.Pointer(&r)), 0)
	if ret == 0 {
		return 0, 0, false
	}

	return int(r.right - r.left), int(r.bottom - r.top), true
}
