//go:build desktop

package desktop

import (
	"runtime"

	"github.com/coalaura/plain"
	"github.com/crgimenes/glaze"
)

var log = plain.New(plain.WithDate(plain.RFC3339Local))

const IsDesktop = true

const (
	margin = 64

	minW = 1100
	minH = 720

	maxW = 1800
	maxH = 1080
)

func RunDesktop(url string, debug bool) {
	w, err := glaze.New(debug)
	log.MustFail(err)

	defer w.Destroy()

	applyTitlebar(w, titlebar{
		bg:   rgba{0x1e, 0x20, 0x30, 0xff},
		fg:   rgba{0xca, 0xd3, 0xf5, 0xff},
		dark: true,
	})

	log.MustFail(w.Bind("setTitlebarTheme", func(bg, fg string) {
		t, err := parseTitlebar(bg, fg)
		if err != nil {
			return
		}

		w.Dispatch(func() {
			applyTitlebar(w, t)
		})
	}))

	w.Init(`document.documentElement.classList.add("desktop","` + runtime.GOOS + `")`)

	w.SetTitle("whiskr")

	width, height := defaultWindowSize()

	w.SetSize(width, height, glaze.HintNone)
	w.SetSize(800, 560, glaze.HintMin)

	w.Navigate(url)
	w.Run()
}

func defaultWindowSize() (width, height int) {
	workW, workH, ok := primaryWorkArea()
	if !ok || workW < 640 || workH < 480 {
		return 1280, 840
	}

	return sizeForWorkArea(workW, workH)
}

func sizeForWorkArea(workW, workH int) (int, int) {
	usableW := max(workW-margin, 640)
	usableH := max(workH-margin, 480)

	w := workW * 72 / 100
	h := workH * 82 / 100

	w = min(max(w, minW), maxW)
	h = min(max(h, minH), maxH)

	return min(w, usableW), min(h, usableH)
}
