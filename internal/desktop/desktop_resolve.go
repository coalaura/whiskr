//go:build desktop && release

package desktop

import (
	"os"
	"path/filepath"
)

var exeDir = func() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}

	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return ""
	}

	return filepath.Dir(exe)
}()

func ResolveRelativePath(path string) string {
	if exeDir == "" || path == "" || filepath.IsAbs(path) {
		return path
	}

	return filepath.Join(exeDir, path)
}
