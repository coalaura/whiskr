//go:build desktop && windows

package desktop

import (
	"os"
	"unsafe"

	"github.com/crgimenes/glaze"
	"golang.org/x/sys/windows"
)

const (
	wmSetIcon = 0x0080
	iconSmall = 0
	iconBig   = 1
)

var (
	shell32            = windows.NewLazySystemDLL("shell32.dll")
	procExtractIconExW = shell32.NewProc("ExtractIconExW")
	procSendMessageW   = user32.NewProc("SendMessageW")
	procDestroyIcon    = user32.NewProc("DestroyIcon")
)

func applyWindowIcons(w glaze.WebView) func() {
	var (
		large windows.Handle
		small windows.Handle
	)

	// Extracted icons belong to us and must outlive the window using them.
	release := func() {
		if large != 0 {
			_, _, _ = procDestroyIcon.Call(uintptr(large))
		}

		if small != 0 {
			_, _, _ = procDestroyIcon.Call(uintptr(small))
		}
	}

	executable, err := os.Executable()
	if err != nil {
		return release
	}

	path, err := windows.UTF16PtrFromString(executable)
	if err != nil {
		return release
	}

	// Use the first icon group, matching Explorer without depending on a resource ID.
	_, _, _ = procExtractIconExW.Call(uintptr(unsafe.Pointer(path)), 0, uintptr(unsafe.Pointer(&large)), uintptr(unsafe.Pointer(&small)), 1)

	hwnd := uintptr(w.Window())

	if small != 0 {
		_, _, _ = procSendMessageW.Call(hwnd, wmSetIcon, iconSmall, uintptr(small))
	}

	if large != 0 {
		_, _, _ = procSendMessageW.Call(hwnd, wmSetIcon, iconBig, uintptr(large))
	}

	return release
}
