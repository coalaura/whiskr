//go:build desktop && linux

package open

import "os/exec"

func OpenFile(name string) error {
	return exec.Command("xdg-open", name).Start()
}
