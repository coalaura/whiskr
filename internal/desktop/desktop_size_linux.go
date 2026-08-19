//go:build desktop && linux

package desktop

import "github.com/ebitengine/purego"

type gdkRectangle struct {
	X      int32
	Y      int32
	Width  int32
	Height int32
}

func primaryWorkArea() (int, int, bool) {
	lib, err := purego.Dlopen("libgdk-3.so.0", purego.RTLD_NOW|purego.RTLD_GLOBAL)
	if err != nil {
		lib, err = purego.Dlopen("libgtk-4.so.1", purego.RTLD_NOW|purego.RTLD_GLOBAL)
	}

	if err != nil {
		return 0, 0, false
	}

	var (
		displayGetDefault  func() uintptr
		displayGetPrimary  func(uintptr) uintptr
		displayGetMonitor  func(uintptr, int32) uintptr
		monitorGetWorkarea func(uintptr, *gdkRectangle)
	)

	purego.RegisterLibFunc(&displayGetDefault, lib, "gdk_display_get_default")

	display := displayGetDefault()
	if display == 0 {
		return 0, 0, false
	}

	var monitor uintptr

	addr, err := purego.Dlsym(lib, "gdk_display_get_primary_monitor")
	if err == nil && addr != 0 {
		purego.RegisterFunc(&displayGetPrimary, addr)

		monitor = displayGetPrimary(display)
	}

	if monitor == 0 {
		addr, err = purego.Dlsym(lib, "gdk_display_get_monitor")
		if err == nil && addr != 0 {
			purego.RegisterFunc(&displayGetMonitor, addr)

			monitor = displayGetMonitor(display, 0)
		}
	}

	if monitor == 0 {
		return 0, 0, false
	}

	purego.RegisterLibFunc(&monitorGetWorkarea, lib, "gdk_monitor_get_workarea")

	var r gdkRectangle

	monitorGetWorkarea(monitor, &r)

	if r.Width <= 0 || r.Height <= 0 {
		return 0, 0, false
	}

	return int(r.Width), int(r.Height), true
}
