//go:build !desktop

package open

func OpenFile(string) error {
	return nil
}
