//go:build desktop

package desktop

import (
	"fmt"
	"strconv"
	"strings"
)

type rgba struct {
	r uint8
	g uint8
	b uint8
	a uint8
}

type titlebar struct {
	bg   rgba
	fg   rgba
	dark bool
}

func parseTitlebar(bg, fg string) (titlebar, error) {
	cbg, err := parseColor(bg)
	if err != nil {
		return titlebar{}, err
	}

	cfg, err := parseColor(fg)
	if err != nil {
		return titlebar{}, err
	}

	y := 0.2126*float64(cbg.r) + 0.7152*float64(cbg.g) + 0.0722*float64(cbg.b)

	return titlebar{bg: cbg, fg: cfg, dark: y < 128}, nil
}

func parseColor(s string) (rgba, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return rgba{}, fmt.Errorf("empty color")
	}

	if strings.HasPrefix(s, "#") {
		hex := s[1:]
		if len(hex) == 3 {
			hex = string([]byte{hex[0], hex[0], hex[1], hex[1], hex[2], hex[2]})
		}

		if len(hex) != 6 && len(hex) != 8 {
			return rgba{}, fmt.Errorf("bad hex %q", s)
		}

		n, err := strconv.ParseUint(hex, 16, 32)
		if err != nil {
			return rgba{}, err
		}

		if len(hex) == 6 {
			return rgba{uint8(n >> 16), uint8(n >> 8), uint8(n), 0xff}, nil
		}

		return rgba{uint8(n >> 24), uint8(n >> 16), uint8(n >> 8), uint8(n)}, nil
	}

	if strings.HasPrefix(s, "rgb") {
		inner := s[strings.IndexByte(s, '(')+1 : strings.LastIndexByte(s, ')')]

		parts := strings.Split(inner, ",")
		if len(parts) < 3 {
			return rgba{}, fmt.Errorf("bad rgb %q", s)
		}

		ch := func(i int) uint8 {
			p := strings.TrimSpace(parts[i])
			if strings.Contains(p, ".") {
				f, _ := strconv.ParseFloat(p, 64)
				if f <= 1 {
					f *= 255
				}

				return uint8(f)
			}

			n, _ := strconv.Atoi(p)
			return uint8(n)
		}

		return rgba{ch(0), ch(1), ch(2), 0xff}, nil
	}

	return rgba{}, fmt.Errorf("bad color %q", s)
}
