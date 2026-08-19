//go:build desktop && windows

package desktop

import (
	"unsafe"

	"github.com/crgimenes/glaze"
	"golang.org/x/sys/windows"
)

var (
	dwmapi                    = windows.NewLazySystemDLL("dwmapi.dll")
	procDwmSetWindowAttribute = dwmapi.NewProc("DwmSetWindowAttribute")
)

const (
	dwmwaUseImmersiveDarkMode   = 20
	dwmwaUseImmersiveDarkMode19 = 19 // Win10 1809
	dwmwaBorderColor            = 34
	dwmwaCaptionColor           = 35
	dwmwaTextColor              = 36
)

func applyTitlebar(w glaze.WebView, t titlebar) {
	hwnd := windows.HWND(uintptr(w.Window()))

	dark := int32(0)
	if t.dark {
		dark = 1
	}

	dwmSet(hwnd, dwmwaUseImmersiveDarkMode, unsafe.Pointer(&dark), 4)
	dwmSet(hwnd, dwmwaUseImmersiveDarkMode19, unsafe.Pointer(&dark), 4)

	caption := colorref(t.bg)
	text := colorref(t.fg)

	dwmSet(hwnd, dwmwaCaptionColor, unsafe.Pointer(&caption), 4)
	dwmSet(hwnd, dwmwaBorderColor, unsafe.Pointer(&caption), 4)
	dwmSet(hwnd, dwmwaTextColor, unsafe.Pointer(&text), 4)
}

func colorref(c rgba) uint32 {
	return uint32(c.r) | uint32(c.g)<<8 | uint32(c.b)<<16
}

func dwmSet(hwnd windows.HWND, attr uint32, val unsafe.Pointer, size uint32) {
	_, _, _ = procDwmSetWindowAttribute.Call(
		uintptr(hwnd), uintptr(attr), uintptr(val), uintptr(size),
	)
}
