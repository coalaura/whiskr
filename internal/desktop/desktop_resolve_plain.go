//go:build !desktop || !release

package desktop

func ResolveRelativePath(path string) string {
	return path
}
